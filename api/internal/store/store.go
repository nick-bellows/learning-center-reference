// Package store reads and writes application data in PostgreSQL using the pgx driver.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nick-bellows/learning-center-reference/api/internal/safeguarding"
)

// ErrNotFound is returned when a requested record does not exist. Callers use errors.Is
// to detect it (so the HTTP layer can return 404 instead of 500).
var ErrNotFound = errors.New("not found")

// Store owns a connection pool. A pool manages many reusable connections for us, so we
// don't open/close one per request.
type Store struct {
	pool *pgxpool.Pool
}

// New opens the pool. ctx (a context.Context) carries cancellation/deadline — the standard
// first parameter of anything that does I/O in Go.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		// %w wraps the underlying error so callers can still inspect it with errors.Is/As.
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool. main() defers this so it runs on shutdown.
func (s *Store) Close() { s.pool.Close() }

// LoadSafeguardingInputs gathers a member's safeguarding facts into the shape the
// eligibility engine expects. A missing row becomes a nil pointer ("not on file"), which
// Evaluate treats as not-current.
func (s *Store) LoadSafeguardingInputs(ctx context.Context, memberID string) (safeguarding.Inputs, error) {
	in := safeguarding.Inputs{Now: time.Now().UTC()}

	// Does the member exist? ($1::uuid casts the string parameter to a uuid.)
	var one int
	err := s.pool.QueryRow(ctx, `select 1 from member where id = $1::uuid`, memberID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return in, ErrNotFound
	}
	if err != nil {
		return in, fmt.Errorf("looking up member: %w", err)
	}

	// Latest approved background-check expiry, and latest SafeSport expiry.
	if in.BackgroundCheckExpires, err = s.maxDate(ctx,
		`select max(expires_at) from background_check where member_id = $1::uuid and status = 'approved'`,
		memberID); err != nil {
		return in, fmt.Errorf("background check: %w", err)
	}
	if in.SafeSportExpires, err = s.maxDate(ctx,
		`select max(expires_at) from safesport_training where member_id = $1::uuid`,
		memberID); err != nil {
		return in, fmt.Errorf("safesport: %w", err)
	}

	// Active disciplinary holds (lifted_at is null). Any row here means suspended.
	rows, err := s.pool.Query(ctx,
		`select source from disciplinary_hold where member_id = $1::uuid and lifted_at is null`,
		memberID)
	if err != nil {
		return in, fmt.Errorf("holds: %w", err)
	}
	defer rows.Close() // always release the rows, even on an early return
	for rows.Next() {
		var source string
		if err := rows.Scan(&source); err != nil {
			return in, fmt.Errorf("scanning hold: %w", err)
		}
		in.ActiveHoldSources = append(in.ActiveHoldSources, source)
	}
	if err := rows.Err(); err != nil {
		return in, fmt.Errorf("iterating holds: %w", err)
	}

	// Role-credential (license/recert) enforcement arrives with the M3 license model.
	in.RoleCredentialRequired = false

	return in, nil
}

// maxDate runs a `select max(<date>)` query and returns a *time.Time (nil when the result
// is SQL NULL — i.e. the member has no such record). sql.NullTime models "a time, or null".
func (s *Store) maxDate(ctx context.Context, query, memberID string) (*time.Time, error) {
	var nt sql.NullTime
	if err := s.pool.QueryRow(ctx, query, memberID).Scan(&nt); err != nil {
		return nil, err
	}
	if !nt.Valid {
		return nil, nil
	}
	t := nt.Time
	return &t, nil
}
