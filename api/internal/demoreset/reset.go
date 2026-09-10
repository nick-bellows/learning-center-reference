// Package demoreset resets only mutable state in the fictional demo association.
package demoreset

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const DemoAssociationID = "00000000-0000-0000-0000-0000000000aa"

// ConfirmValue is the exact RESET_CONFIRM value an operator must supply. The phrase
// names what is being reset so the command cannot be run by habit against the wrong
// database.
const ConfirmValue = "synthetic-demo"

// ErrNotConfirmed is returned when the confirmation phrase is missing or wrong.
var ErrNotConfirmed = errors.New("refusing reset: set RESET_CONFIRM=" + ConfirmValue)

// Confirm checks the operator's confirmation phrase. It is an exact, case-sensitive
// match: no trimming, no aliases.
func Confirm(value string) error {
	if value != ConfirmValue {
		return ErrNotConfirmed
	}
	return nil
}

// Reset deletes demo enrollments. Progress projections and events cascade from
// enrollment; identities, roles, courses, credentials, and audit fixtures remain.
func Reset(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning reset: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result, err := tx.Exec(ctx, `
		delete from enrollment e
		using member m
		where e.member_id = m.id and m.association_id = $1::uuid`, DemoAssociationID)
	if err != nil {
		return 0, fmt.Errorf("deleting demo enrollments: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing reset: %w", err)
	}
	return result.RowsAffected(), nil
}
