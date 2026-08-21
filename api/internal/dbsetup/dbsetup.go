// Package dbsetup applies embedded migrations (and, for local demos, seed
// data) against Postgres. It is a deliberately small hand-rolled runner:
// a schema_migrations table records which files have been applied, and each
// unapplied NNNN_*.up.sql runs inside its own transaction, in filename order.
package dbsetup

import (
	"context"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate applies every embedded up-migration that has not been applied yet
// and returns the filenames it ran. Safe to call on every startup.
func Migrate(ctx context.Context, pool *pgxpool.Pool, files fs.FS) ([]string, error) {
	if _, err := pool.Exec(ctx, `
		create table if not exists schema_migrations (
			version    text primary key,
			applied_at timestamptz not null default now()
		)`); err != nil {
		return nil, fmt.Errorf("creating schema_migrations: %w", err)
	}

	names, err := fs.Glob(files, "*.up.sql")
	if err != nil {
		return nil, fmt.Errorf("listing migrations: %w", err)
	}
	sort.Strings(names) // NNNN_ prefixes make lexical order the run order

	var applied []string
	for _, name := range names {
		var exists bool
		if err := pool.QueryRow(ctx,
			`select exists (select 1 from schema_migrations where version = $1)`,
			name).Scan(&exists); err != nil {
			return applied, fmt.Errorf("checking %s: %w", name, err)
		}
		if exists {
			continue
		}

		sql, err := fs.ReadFile(files, name)
		if err != nil {
			return applied, fmt.Errorf("reading %s: %w", name, err)
		}

		// One transaction per migration: either the whole file lands and is
		// recorded, or neither happens.
		tx, err := pool.Begin(ctx)
		if err != nil {
			return applied, fmt.Errorf("beginning tx for %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("applying %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			`insert into schema_migrations (version) values ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("recording %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return applied, fmt.Errorf("committing %s: %w", name, err)
		}
		applied = append(applied, name)
	}
	return applied, nil
}

// ApplySeed executes a seed script. The script itself must be idempotent
// (fixed ids + ON CONFLICT DO NOTHING) — this function adds no magic.
func ApplySeed(ctx context.Context, pool *pgxpool.Pool, sql string) error {
	if _, err := pool.Exec(ctx, sql); err != nil {
		return fmt.Errorf("applying seed: %w", err)
	}
	return nil
}
