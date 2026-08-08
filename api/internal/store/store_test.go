package store

import (
	"context"
	"os"
	"testing"

	"github.com/nick-bellows/learning-center-reference/api/internal/safeguarding"
)

// This is an INTEGRATION test: it needs a real Postgres with the migrations + seed applied.
// It skips itself unless DATABASE_URL is set, so the normal `go test ./...` (and CI without a
// database) still passes. To run it:
//
//	docker compose up -d db
//	# apply migrations 0001..0003 and db/seed/seed.sql
//	DATABASE_URL="postgres://lcr:change-me-locally@localhost:5432/lcr?sslmode=disable" \
//	  go test ./internal/store -run Integration -v
func TestLoadSafeguardingInputs_Integration(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("set DATABASE_URL to run (needs Postgres + migrations + seed)")
	}

	ctx := context.Background()
	st, err := New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer st.Close()

	// IDs come from db/seed/seed.sql (synthetic members).
	const eligibleID = "11111111-1111-1111-1111-111111111111"
	const suspendedID = "22222222-2222-2222-2222-222222222222"

	in, err := st.LoadSafeguardingInputs(ctx, eligibleID)
	if err != nil {
		t.Fatalf("load eligible member: %v", err)
	}
	if d := safeguarding.Evaluate(in); d.Status != safeguarding.StatusEligible {
		t.Errorf("eligible member: got %q (%s); want %q", d.Status, d.Reason, safeguarding.StatusEligible)
	}

	in, err = st.LoadSafeguardingInputs(ctx, suspendedID)
	if err != nil {
		t.Fatalf("load suspended member: %v", err)
	}
	if d := safeguarding.Evaluate(in); d.Status != safeguarding.StatusSuspended {
		t.Errorf("suspended member: got %q (%s); want %q", d.Status, d.Reason, safeguarding.StatusSuspended)
	}
}
