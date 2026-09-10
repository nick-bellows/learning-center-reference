package demoreset

import (
	"context"
	"testing"

	"github.com/nick-bellows/learning-center-reference/api/internal/store"
	"github.com/nick-bellows/learning-center-reference/api/internal/testdb"
)

func TestConfirm(t *testing.T) {
	cases := map[string]bool{
		ConfirmValue:      true,
		"":                false,
		"yes":             false,
		"Synthetic-Demo":  false,
		" synthetic-demo": false,
	}
	for value, want := range cases {
		if got := Confirm(value) == nil; got != want {
			t.Errorf("Confirm(%q) accepted = %v; want %v", value, got, want)
		}
	}
}

// TestReset_Integration proves the reset is scoped: it removes the fictional demo
// association's enrollments (progress events and projections cascade) and nothing else —
// not its identities, roles, or credential facts, and not another association's
// enrollments.
func TestReset_Integration(t *testing.T) {
	db := testdb.New(t)
	db.Seed(t)
	ctx := context.Background()
	st, err := store.New(ctx, db.URL)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(st.Close)

	const demoLearner = "11111111-1111-1111-1111-111111111111"
	const course = "10000000-0000-0000-0000-000000000001"
	const firstLesson = "30000000-0000-0000-0000-000000000001"
	progress, _, err := st.Enroll(ctx, demoLearner, course)
	if err != nil {
		t.Fatalf("enroll demo learner: %v", err)
	}
	if _, _, err := st.CompleteLesson(ctx, demoLearner, progress.EnrollmentID, firstLesson); err != nil {
		t.Fatalf("complete lesson: %v", err)
	}

	// A member of a different association with their own enrollment must be untouched.
	const otherAssociation = "00000000-0000-0000-0000-0000000000bb"
	const otherLearner = "99990000-0000-0000-0000-000000000002"
	fixtures := []struct {
		sql  string
		args []any
	}{
		{`insert into member_association (id, name, slug) values ($1::uuid, 'Other Association', 'other')`,
			[]any{otherAssociation}},
		{`insert into member (id, auth_subject, display_name, association_id)
		  values ($1::uuid, 'test|other', 'Other Learner (synthetic)', $2::uuid)`,
			[]any{otherLearner, otherAssociation}},
		{`insert into member_role (member_id, role) values ($1::uuid, 'learner')`,
			[]any{otherLearner}},
	}
	for _, f := range fixtures {
		if _, err := db.Pool.Exec(ctx, f.sql, f.args...); err != nil {
			t.Fatalf("insert other association fixture: %v", err)
		}
	}
	if _, _, err := st.Enroll(ctx, otherLearner, course); err != nil {
		t.Fatalf("enroll other learner: %v", err)
	}

	before := snapshot(t, db)
	if before["enrollment"] != 2 || before["progress_event"] != 1 || before["enrollment_progress"] != 2 {
		t.Fatalf("fixture counts = %v", before)
	}

	deleted, err := Reset(ctx, db.Pool)
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("reset deleted %d enrollments; want 1 (only the demo association's)", deleted)
	}

	after := snapshot(t, db)
	want := map[string]int{
		"enrollment":          1, // the other association's enrollment survives
		"enrollment_progress": 1,
		"progress_event":      0, // cascaded with the demo enrollment
	}
	for table, n := range want {
		if after[table] != n {
			t.Errorf("%s rows after reset = %d; want %d", table, after[table], n)
		}
	}
	for _, table := range []string{"member", "member_role", "role_credential", "background_check", "safesport_training", "disciplinary_hold", "course", "lesson"} {
		if after[table] != before[table] {
			t.Errorf("%s rows changed by reset: %d -> %d", table, before[table], after[table])
		}
	}

	// A second reset finds nothing to do and does not fail.
	if deleted, err := Reset(ctx, db.Pool); err != nil || deleted != 0 {
		t.Fatalf("repeat reset = %d, %v; want 0, nil", deleted, err)
	}
}

func snapshot(t *testing.T, db *testdb.DB) map[string]int {
	t.Helper()
	counts := make(map[string]int)
	for _, table := range []string{
		"enrollment", "enrollment_progress", "progress_event",
		"member", "member_role", "role_credential", "background_check", "safesport_training",
		"disciplinary_hold", "course", "lesson",
	} {
		var n int
		if err := db.Pool.QueryRow(context.Background(), `select count(*) from `+table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		counts[table] = n
	}
	return counts
}
