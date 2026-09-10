package projection

import (
	"context"
	"testing"

	"github.com/nick-bellows/learning-center-reference/api/internal/store"
	"github.com/nick-bellows/learning-center-reference/api/internal/testdb"
)

// Seeded member/course/lesson ids from db/seed/seed.sql; the learner is Alex Coach.
const (
	memberID       = "11111111-1111-1111-1111-111111111111"
	courseID       = "10000000-0000-0000-0000-000000000001"
	firstLessonID  = "30000000-0000-0000-0000-000000000001"
	secondLessonID = "30000000-0000-0000-0000-000000000002"
)

// TestReconcile_Integration corrupts the projection three different ways — wrong counts,
// a stale status, and a missing row — and checks that a dry run reports each without
// touching data, that apply rebuilds them, and that a clean projection reconciles to no
// drift before and after the repair.
func TestReconcile_Integration(t *testing.T) {
	db := testdb.New(t)
	db.Seed(t)
	ctx := context.Background()
	st, err := store.New(ctx, db.URL)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(st.Close)

	progress, _, err := st.Enroll(ctx, memberID, courseID)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	for _, lesson := range []string{firstLessonID, secondLessonID} {
		if _, _, err := st.CompleteLesson(ctx, memberID, progress.EnrollmentID, lesson); err != nil {
			t.Fatalf("complete %s: %v", lesson, err)
		}
	}

	// A consistent projection reports nothing to do.
	report, err := Reconcile(ctx, db.Pool, false)
	if err != nil {
		t.Fatalf("clean reconcile: %v", err)
	}
	if report.Checked != 1 || len(report.Drifted) != 0 {
		t.Fatalf("clean report = %+v; want 1 checked, 0 drifted", report)
	}

	// Corrupt the counts and the status directly, bypassing the store.
	if _, err := db.Pool.Exec(ctx, `
		update enrollment_progress set completed_lessons = 0, percent_complete = 0
		where enrollment_id = $1::uuid`, progress.EnrollmentID); err != nil {
		t.Fatalf("corrupt projection: %v", err)
	}
	if _, err := db.Pool.Exec(ctx, `update enrollment set status = 'completed' where id = $1::uuid`,
		progress.EnrollmentID); err != nil {
		t.Fatalf("corrupt status: %v", err)
	}

	// Dry run: drift is reported with the exact expected values, and nothing changes.
	report, err = Reconcile(ctx, db.Pool, false)
	if err != nil {
		t.Fatalf("dry-run reconcile: %v", err)
	}
	if len(report.Drifted) != 1 || report.Applied {
		t.Fatalf("dry-run report = %+v; want exactly one drift, not applied", report)
	}
	drift := report.Drifted[0]
	want := Counts{CompletedLessons: 2, TotalLessons: 3, PercentComplete: 66, Status: "active"}
	if drift.Expected != want || drift.Projected.CompletedLessons != 0 || drift.Projected.Status != "completed" {
		t.Fatalf("drift = %+v; want expected %+v", drift, want)
	}
	var completed int
	if err := db.Pool.QueryRow(ctx, `select completed_lessons from enrollment_progress where enrollment_id = $1::uuid`,
		progress.EnrollmentID).Scan(&completed); err != nil || completed != 0 {
		t.Fatalf("dry run changed data: completed=%d err=%v", completed, err)
	}

	// Remove the projection row entirely: the enrollment must still be rebuilt.
	if _, err := db.Pool.Exec(ctx, `delete from enrollment_progress where enrollment_id = $1::uuid`,
		progress.EnrollmentID); err != nil {
		t.Fatalf("delete projection: %v", err)
	}

	report, err = Reconcile(ctx, db.Pool, true)
	if err != nil {
		t.Fatalf("apply reconcile: %v", err)
	}
	if len(report.Drifted) != 1 || !report.Drifted[0].Missing || !report.Applied {
		t.Fatalf("apply report = %+v; want one missing projection, applied", report)
	}

	// The dashboard now reads the rebuilt projection, and a second run is clean.
	rebuilt, err := st.LoadDashboard(ctx, memberID)
	if err != nil || len(rebuilt.Enrollments) != 1 {
		t.Fatalf("dashboard = %+v, err=%v", rebuilt, err)
	}
	if got := rebuilt.Enrollments[0]; got.CompletedLessons != 2 || got.PercentComplete != 66 || got.Status != "active" {
		t.Fatalf("rebuilt progress = %+v; want 2/3 lessons, 66%%, active", got)
	}
	report, err = Reconcile(ctx, db.Pool, false)
	if err != nil || len(report.Drifted) != 0 {
		t.Fatalf("post-repair reconcile = %+v, err=%v; want no drift", report, err)
	}
}
