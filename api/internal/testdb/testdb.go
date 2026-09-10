// Package testdb gives integration tests a private, migrated PostgreSQL database.
//
// Test packages run in parallel under `go test ./...`, so two packages that migrate,
// seed, and mutate the one database named by DATABASE_URL could observe each other's
// rows. Each call to New creates a throwaway database on the same server instead,
// applies the embedded migrations to it, and drops it when the test finishes. Tests
// skip cleanly when DATABASE_URL is unset so plain `go test ./...` stays unit-only.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nick-bellows/learning-center-reference/api/internal/dbsetup"
	"github.com/nick-bellows/learning-center-reference/api/migrations"
)

// DB is a private database created for one test.
type DB struct {
	// URL connects to the private database with the same credentials as DATABASE_URL.
	URL string
	// Name is the database's name, for tests that need to reference it in SQL.
	Name string
	// Pool is an open pool on the private database. It is closed on cleanup.
	Pool *pgxpool.Pool
}

// New creates a fresh database, applies every embedded migration to it, and registers
// cleanup that closes the pool and drops the database. It skips the test when
// DATABASE_URL is unset.
func New(t *testing.T) *DB {
	t.Helper()
	return create(t, true)
}

// NewEmpty creates a fresh database WITHOUT applying migrations, for tests that
// exercise the migration runner itself.
func NewEmpty(t *testing.T) *DB {
	t.Helper()
	return create(t, false)
}

func create(t *testing.T, migrate bool) *DB {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("set DATABASE_URL to run (needs Postgres)")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(admin.Close)

	name := "lcr_test_" + randomSuffix()
	if _, err := admin.Exec(ctx, `create database `+quoteIdent(name)); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		// Force-drop terminates any connection a test forgot to close.
		if _, err := admin.Exec(context.Background(), `drop database if exists `+quoteIdent(name)+` with (force)`); err != nil {
			t.Errorf("drop database %s: %v", name, err)
		}
	})

	private, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	private.Path = "/" + name

	pool, err := pgxpool.New(ctx, private.String())
	if err != nil {
		t.Fatalf("connect to %s: %v", name, err)
	}
	t.Cleanup(pool.Close)

	if migrate {
		if _, err := dbsetup.Migrate(ctx, pool, migrations.Files); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	return &DB{URL: private.String(), Name: name, Pool: pool}
}

// Seed applies the repository's synthetic seed (db/seed/seed.sql) to the database.
func (db *DB) Seed(t *testing.T) {
	t.Helper()
	sql, err := os.ReadFile(SeedPath())
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	if err := dbsetup.ApplySeed(context.Background(), db.Pool, string(sql)); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// SeedPath locates db/seed/seed.sql relative to this source file, so the helper works
// from any package directory.
func SeedPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "seed", "seed.sql")
}

func randomSuffix() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("random suffix: %v", err))
	}
	return hex.EncodeToString(b[:])
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
