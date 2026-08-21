package store

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/nick-bellows/learning-center-reference/api/internal/dbsetup"
	"github.com/nick-bellows/learning-center-reference/api/internal/safeguarding"
	"github.com/nick-bellows/learning-center-reference/api/migrations"
)

// This is an INTEGRATION test: it needs a real Postgres. It applies the
// embedded migrations and the seed itself, so all it needs is a database:
//
//	docker compose up -d db
//	DATABASE_URL="postgres://lcr:change-me-locally@localhost:5432/lcr?sslmode=disable" \
//	  go test ./internal/store -run Integration -v
//
// It skips without DATABASE_URL so plain `go test ./...` stays unit-only;
// CI provides a Postgres service container so the skip never hides a break.
func TestLoadSafeguardingInputs_Integration(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("set DATABASE_URL to run (needs Postgres)")
	}

	ctx := context.Background()
	st, err := New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer st.Close()

	if _, err := dbsetup.Migrate(ctx, st.Pool(), migrations.Files); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	seed, err := os.ReadFile("../../../db/seed/seed.sql")
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	if err := dbsetup.ApplySeed(ctx, st.Pool(), string(seed)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// IDs and expected statuses come from db/seed/seed.sql (synthetic members).
	cases := []struct {
		name       string
		id         string
		want       safeguarding.Status
		wantReason string // substring; "" = don't check
	}{
		{"eligible coach", "11111111-1111-1111-1111-111111111111", safeguarding.StatusEligible, ""},
		{"suspended referee (active hold)", "22222222-2222-2222-2222-222222222222", safeguarding.StatusSuspended, "hold"},
		{"lapsed referee (expired recert)", "33333333-3333-3333-3333-333333333333", safeguarding.StatusIneligible, "role credential"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in, err := st.LoadSafeguardingInputs(ctx, tc.id)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			d := safeguarding.Evaluate(in)
			if d.Status != tc.want {
				t.Errorf("status = %q (%s); want %q", d.Status, d.Reason, tc.want)
			}
			if tc.wantReason != "" && !strings.Contains(d.Reason, tc.wantReason) {
				t.Errorf("reason = %q; want it to mention %q", d.Reason, tc.wantReason)
			}
		})
	}

	// Unknown member -> ErrNotFound (the HTTP layer turns this into 404).
	if _, err := st.LoadSafeguardingInputs(ctx, "99999999-9999-9999-9999-999999999999"); err == nil {
		t.Error("unknown member: want ErrNotFound, got nil")
	}
}
