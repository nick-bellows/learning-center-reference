package dbsetup_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/nick-bellows/learning-center-reference/api/internal/dbsetup"
	"github.com/nick-bellows/learning-center-reference/api/internal/testdb"
	"github.com/nick-bellows/learning-center-reference/api/migrations"
)

// These are INTEGRATION tests against a private throwaway database (see internal/testdb);
// they skip without DATABASE_URL. They pin the runner's operational claims: every embedded
// migration applies once and in order, a re-run is a no-op, a failing migration leaves no
// partial DDL and no record, and concurrent runners serialize on the advisory lock.

func TestMigrate_AppliesEmbeddedMigrationsOnceInOrder(t *testing.T) {
	db := testdb.NewEmpty(t)
	ctx := context.Background()

	applied, err := dbsetup.Migrate(ctx, db.Pool, migrations.Files)
	if err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if len(applied) == 0 {
		t.Fatal("first migrate applied nothing")
	}
	for i := 1; i < len(applied); i++ {
		if applied[i-1] >= applied[i] {
			t.Fatalf("migrations applied out of order: %v", applied)
		}
	}

	again, err := dbsetup.Migrate(ctx, db.Pool, migrations.Files)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("second migrate re-applied %v; want nothing", again)
	}

	var recorded int
	if err := db.Pool.QueryRow(ctx, `select count(*) from schema_migrations`).Scan(&recorded); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if recorded != len(applied) {
		t.Fatalf("schema_migrations has %d rows; want %d", recorded, len(applied))
	}
}

func TestMigrate_RollsBackFailedMigrationAndRecordsNothing(t *testing.T) {
	db := testdb.NewEmpty(t)
	ctx := context.Background()

	broken := fstest.MapFS{
		"0001_alpha.up.sql": {Data: []byte(`create table alpha (id int primary key);`)},
		// The first statement succeeds, the second violates a NOT NULL constraint. If the
		// runner were not transactional, table beta would survive with no migration record.
		"0002_beta.up.sql": {Data: []byte(`
			create table beta (id int primary key, label text not null);
			insert into beta (id, label) values (1, null);`)},
		"0003_gamma.up.sql": {Data: []byte(`create table gamma (id int primary key);`)},
	}

	applied, err := dbsetup.Migrate(ctx, db.Pool, broken)
	if err == nil {
		t.Fatal("migrate succeeded; want the 0002 failure")
	}
	if !strings.Contains(err.Error(), "0002_beta.up.sql") {
		t.Fatalf("error %q does not name the failing migration", err)
	}
	if len(applied) != 1 || applied[0] != "0001_alpha.up.sql" {
		t.Fatalf("applied = %v; want only 0001", applied)
	}

	for table, wantExists := range map[string]bool{"alpha": true, "beta": false, "gamma": false} {
		if got := tableExists(t, db, table); got != wantExists {
			t.Errorf("table %s exists = %v; want %v", table, got, wantExists)
		}
	}
	if got := recordedVersions(t, db); len(got) != 1 || got[0] != "0001_alpha.up.sql" {
		t.Fatalf("schema_migrations = %v; want only 0001 recorded", got)
	}

	// Once the migration is fixed, the runner resumes from the failure point: 0001 is not
	// re-run, 0002 and 0003 apply.
	fixed := fstest.MapFS{
		"0001_alpha.up.sql": broken["0001_alpha.up.sql"],
		"0002_beta.up.sql":  {Data: []byte(`create table beta (id int primary key, label text not null);`)},
		"0003_gamma.up.sql": broken["0003_gamma.up.sql"],
	}
	applied, err = dbsetup.Migrate(ctx, db.Pool, fixed)
	if err != nil {
		t.Fatalf("migrate after fix: %v", err)
	}
	if len(applied) != 2 || applied[0] != "0002_beta.up.sql" || applied[1] != "0003_gamma.up.sql" {
		t.Fatalf("applied after fix = %v; want 0002 then 0003", applied)
	}
	if got := recordedVersions(t, db); len(got) != 3 {
		t.Fatalf("schema_migrations = %v; want three versions", got)
	}
}

func TestMigrate_ConcurrentRunnersApplyEachMigrationOnce(t *testing.T) {
	db := testdb.NewEmpty(t)
	ctx := context.Background()

	const runners = 5
	var wg sync.WaitGroup
	results := make([][]string, runners)
	errs := make([]error, runners)
	for i := 0; i < runners; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = dbsetup.Migrate(ctx, db.Pool, migrations.Files)
		}(i)
	}
	wg.Wait()

	total := 0
	for i := 0; i < runners; i++ {
		if errs[i] != nil {
			t.Fatalf("runner %d: %v", i, errs[i])
		}
		total += len(results[i])
	}
	recorded := recordedVersions(t, db)
	if total != len(recorded) {
		t.Fatalf("runners applied %d migrations in total; schema_migrations has %d", total, len(recorded))
	}
}

func TestApplySeed_IsIdempotent(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()

	db.Seed(t)
	db.Seed(t)

	var members, credentials int
	if err := db.Pool.QueryRow(ctx, `select count(*) from member`).Scan(&members); err != nil {
		t.Fatalf("count members: %v", err)
	}
	if err := db.Pool.QueryRow(ctx, `select count(*) from role_credential`).Scan(&credentials); err != nil {
		t.Fatalf("count credentials: %v", err)
	}
	if members != 4 || credentials != 3 {
		t.Fatalf("after seeding twice: members=%d credentials=%d; want 4 and 3 (no duplicates)", members, credentials)
	}
}

func TestMigrate_ReturnsContextError(t *testing.T) {
	db := testdb.NewEmpty(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := dbsetup.Migrate(ctx, db.Pool, migrations.Files); !errors.Is(err, context.Canceled) {
		t.Fatalf("migrate with cancelled context: err = %v; want context.Canceled", err)
	}
}

func tableExists(t *testing.T, db *testdb.DB, name string) bool {
	t.Helper()
	var exists bool
	if err := db.Pool.QueryRow(context.Background(),
		`select exists (select 1 from information_schema.tables where table_schema = 'public' and table_name = $1)`,
		name).Scan(&exists); err != nil {
		t.Fatalf("checking table %s: %v", name, err)
	}
	return exists
}

func recordedVersions(t *testing.T, db *testdb.DB) []string {
	t.Helper()
	rows, err := db.Pool.Query(context.Background(), `select version from schema_migrations order by version`)
	if err != nil {
		t.Fatalf("listing schema_migrations: %v", err)
	}
	defer rows.Close()
	var versions []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scanning version: %v", err)
		}
		versions = append(versions, v)
	}
	return versions
}
