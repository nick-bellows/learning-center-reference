package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nick-bellows/learning-center-reference/api/internal/credentials"
	"github.com/nick-bellows/learning-center-reference/api/internal/safeguarding"
	"github.com/nick-bellows/learning-center-reference/api/internal/testdb"
)

// These are INTEGRATION tests: they need a real Postgres. Each one gets a private,
// migrated, seeded throwaway database from internal/testdb, so nothing here touches the
// database named by DATABASE_URL beyond creating and dropping that copy:
//
//	docker compose up -d db
//	DATABASE_URL="postgres://lcr:change-me-locally@localhost:5432/lcr?sslmode=disable" \
//	  go test ./internal/store -run Integration -v
//
// They skip without DATABASE_URL so plain `go test ./...` stays unit-only;
// CI provides a Postgres service container so the skip never hides a break.

// seededStore opens the store on a fresh database carrying the synthetic seed.
func seededStore(t *testing.T) (*Store, *testdb.DB) {
	t.Helper()
	db := testdb.New(t)
	db.Seed(t)
	st, err := New(context.Background(), db.URL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(st.Close)
	return st, db
}

const testAssociationID = "00000000-0000-0000-0000-0000000000aa"

// insertMember adds a synthetic member with the given roles to the private database.
func insertMember(t *testing.T, db *testdb.DB, id, subject string, roles ...string) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `
		insert into member (id, auth_subject, display_name, association_id)
		values ($1::uuid, $2, 'Integration Test (synthetic)', $3::uuid)`, id, subject, testAssociationID); err != nil {
		t.Fatalf("insert member: %v", err)
	}
	for _, role := range roles {
		if _, err := db.Pool.Exec(ctx, `insert into member_role (member_id, role) values ($1::uuid, $2)`, id, role); err != nil {
			t.Fatalf("insert role %s: %v", role, err)
		}
	}
}

// currentSafeguarding gives a member an in-effect background check and SafeSport record.
func currentSafeguarding(t *testing.T, db *testdb.DB, memberID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `
		insert into background_check (member_id, source, approved_at, expires_at, status)
		values ($1::uuid, 'sample-ysa', current_date - interval '3 months', (current_date + interval '2 years')::date, 'approved')`, memberID); err != nil {
		t.Fatalf("insert background check: %v", err)
	}
	if _, err := db.Pool.Exec(ctx, `
		insert into safesport_training (member_id, training_type, completed_at, expires_at)
		values ($1::uuid, 'core', current_date - interval '2 months', (current_date + interval '10 months')::date)`, memberID); err != nil {
		t.Fatalf("insert safesport: %v", err)
	}
}

func evaluate(t *testing.T, st *Store, memberID string) safeguarding.Decision {
	t.Helper()
	in, err := st.LoadSafeguardingInputs(context.Background(), memberID)
	if err != nil {
		t.Fatalf("load safeguarding inputs: %v", err)
	}
	return safeguarding.Evaluate(in)
}

func TestLoadSafeguardingInputs_Integration(t *testing.T) {
	st, _ := seededStore(t)
	ctx := context.Background()

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
			d := evaluate(t, st, tc.id)
			if d.Status != tc.want {
				t.Errorf("status = %q (%s); want %q", d.Status, d.Reason, tc.want)
			}
			if tc.wantReason != "" && !strings.Contains(d.Reason, tc.wantReason) {
				t.Errorf("reason = %q; want it to mention %q", d.Reason, tc.wantReason)
			}
		})
	}

	// Unknown member -> ErrNotFound (the HTTP layer turns this into 404).
	if _, err := st.LoadSafeguardingInputs(ctx, "99999999-9999-9999-9999-999999999999"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown member: want ErrNotFound, got %v", err)
	}
}

// TestFutureDatedRecordsDoNotCount_Integration: a record whose approval, completion, or
// issue date is still in the future is not yet in effect, so it must not grant eligibility.
func TestFutureDatedRecordsDoNotCount_Integration(t *testing.T) {
	st, db := seededStore(t)
	ctx := context.Background()
	const memberID = "99990000-0000-0000-0000-000000000010"
	insertMember(t, db, memberID, "test|future", "coach")

	// Everything on file, everything dated tomorrow or later.
	for _, stmt := range []string{
		`insert into background_check (member_id, source, approved_at, expires_at, status)
		 values ($1::uuid, 'sample-ysa', current_date + interval '1 day', (current_date + interval '2 years')::date, 'approved')`,
		`insert into safesport_training (member_id, training_type, completed_at, expires_at)
		 values ($1::uuid, 'core', current_date + interval '1 day', (current_date + interval '1 year')::date)`,
		`insert into role_credential (member_id, role, credential_type, issued_at, expires_at)
		 values ($1::uuid, 'coach', 'c_license', current_date + interval '1 day', (current_date + interval '1 year')::date)`,
	} {
		if _, err := db.Pool.Exec(ctx, stmt, memberID); err != nil {
			t.Fatalf("insert future-dated record: %v", err)
		}
	}

	in, err := st.LoadSafeguardingInputs(ctx, memberID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if in.BackgroundCheckExpires != nil || in.SafeSportExpires != nil || in.RoleCredentialExpires != nil {
		t.Errorf("future-dated records were loaded as in effect: %+v", in)
	}
	if d := safeguarding.Evaluate(in); d.Status != safeguarding.StatusIneligible {
		t.Errorf("status = %q (%s); want %q", d.Status, d.Reason, safeguarding.StatusIneligible)
	}

	// The credentials contract must agree: the future-dated credential is reported valid=false.
	record, err := st.LoadMemberCredentials(ctx, "test|future")
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	response := credentials.Build(record)
	if len(response.RoleCredentials) != 1 || response.RoleCredentials[0].Valid {
		t.Errorf("role credentials = %#v; want one with valid=false", response.RoleCredentials)
	}
}

// TestWeakestLinkAcrossRoles_Integration: a member who is both coach and referee needs a
// current credential for EACH role; the earliest of the latest per-role expiries governs,
// and one role with nothing on file means "missing".
func TestWeakestLinkAcrossRoles_Integration(t *testing.T) {
	st, db := seededStore(t)
	ctx := context.Background()
	const memberID = "99990000-0000-0000-0000-000000000020"
	insertMember(t, db, memberID, "test|dual-role", "coach", "referee")
	currentSafeguarding(t, db, memberID)

	// Coach license current; no referee credential at all -> missing.
	if _, err := db.Pool.Exec(ctx, `
		insert into role_credential (member_id, role, credential_type, issued_at, expires_at)
		values ($1::uuid, 'coach', 'c_license', current_date - interval '1 year', (current_date + interval '2 years')::date)`, memberID); err != nil {
		t.Fatalf("insert coach credential: %v", err)
	}
	if d := evaluate(t, st, memberID); d.Status != safeguarding.StatusIneligible || !strings.Contains(d.Reason, "role credential") {
		t.Fatalf("with no referee credential: %q (%s); want ineligible_lapsed for the missing role", d.Status, d.Reason)
	}

	// An expired referee recert, then a current one: the LATEST per role counts, and the
	// referee's expiry (sooner than the coach's) becomes the governing date.
	if _, err := db.Pool.Exec(ctx, `
		insert into role_credential (member_id, role, credential_type, issued_at, expires_at) values
		($1::uuid, 'referee', 'referee_recert', current_date - interval '2 years', (current_date - interval '1 year')::date),
		($1::uuid, 'referee', 'referee_recert', current_date - interval '6 months', (current_date + interval '6 months')::date)`, memberID); err != nil {
		t.Fatalf("insert referee credentials: %v", err)
	}
	in, err := st.LoadSafeguardingInputs(ctx, memberID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if d := safeguarding.Evaluate(in); d.Status != safeguarding.StatusEligible {
		t.Fatalf("with both roles current: %q (%s); want eligible", d.Status, d.Reason)
	}
	var refereeExpiry string
	if err := db.Pool.QueryRow(ctx, `select (current_date + interval '6 months')::date::text`).Scan(&refereeExpiry); err != nil {
		t.Fatalf("expected expiry: %v", err)
	}
	if in.RoleCredentialExpires == nil || in.RoleCredentialExpires.Format("2006-01-02") != refereeExpiry {
		t.Errorf("weakest-link expiry = %v; want the referee's %s", in.RoleCredentialExpires, refereeExpiry)
	}
}

func TestLearningWorkflow_Integration(t *testing.T) {
	st, db := seededStore(t)
	ctx := context.Background()

	const memberID = "99990000-0000-0000-0000-000000000001"
	const courseID = "10000000-0000-0000-0000-000000000001"
	const firstLessonID = "30000000-0000-0000-0000-000000000001"
	const secondLessonID = "30000000-0000-0000-0000-000000000002"
	const finalLessonID = "30000000-0000-0000-0000-000000000003"
	insertMember(t, db, memberID, "test|workflow", "learner")

	member, err := st.ResolveMemberBySubject(ctx, "test|workflow")
	if err != nil || !member.HasRole("learner") {
		t.Fatalf("resolved member = %#v, %v", member, err)
	}

	progress, created, err := st.Enroll(ctx, memberID, courseID)
	if err != nil || !created {
		t.Fatalf("first enroll = %#v, created=%v, err=%v", progress, created, err)
	}
	if progress.TotalLessons != 3 || progress.PercentComplete != 0 {
		t.Fatalf("initial progress = %#v", progress)
	}

	retry, created, err := st.Enroll(ctx, memberID, courseID)
	if err != nil || created || retry.EnrollmentID != progress.EnrollmentID {
		t.Fatalf("retry enroll = %#v, created=%v, err=%v", retry, created, err)
	}

	if _, _, err := st.CompleteLesson(ctx, memberID, progress.EnrollmentID, secondLessonID); !errors.Is(err, ErrOutOfOrder) {
		t.Fatalf("second lesson first error = %v; want ErrOutOfOrder", err)
	}

	progress, recorded, err := st.CompleteLesson(ctx, memberID, progress.EnrollmentID, firstLessonID)
	if err != nil || !recorded || progress.CompletedLessons != 1 || progress.PercentComplete != 33 {
		t.Fatalf("first completion = %#v, recorded=%v, err=%v", progress, recorded, err)
	}
	progress, recorded, err = st.CompleteLesson(ctx, memberID, progress.EnrollmentID, firstLessonID)
	if err != nil || recorded || progress.CompletedLessons != 1 {
		t.Fatalf("completion retry = %#v, recorded=%v, err=%v", progress, recorded, err)
	}

	// A missing projection row (the drift cmd/reconcileprogress repairs) must not take the
	// whole dashboard down: the enrollment still exists, so it renders with zero progress.
	if _, err := db.Pool.Exec(ctx, `delete from enrollment_progress where enrollment_id = $1::uuid`, progress.EnrollmentID); err != nil {
		t.Fatalf("delete projection: %v", err)
	}
	dashboard, err := st.LoadDashboard(ctx, memberID)
	if err != nil || len(dashboard.Enrollments) != 1 {
		t.Fatalf("dashboard without projection row = %#v, err=%v", dashboard, err)
	}
	if e := dashboard.Enrollments[0]; e.CompletedLessons != 0 || e.TotalLessons != 3 || e.PercentComplete != 0 {
		t.Fatalf("dashboard without projection row = %#v; want zero progress over 3 lessons", e)
	}
	// The next completion recreates the projection from the event log.
	if _, err := db.Pool.Exec(ctx, `
		insert into enrollment_progress (enrollment_id, completed_lessons, total_lessons, percent_complete)
		values ($1::uuid, 0, 3, 0)`, progress.EnrollmentID); err != nil {
		t.Fatalf("restore projection row: %v", err)
	}

	if _, _, err := st.CompleteLesson(ctx, memberID, progress.EnrollmentID, secondLessonID); err != nil {
		t.Fatalf("second completion: %v", err)
	}
	progress, recorded, err = st.CompleteLesson(ctx, memberID, progress.EnrollmentID, finalLessonID)
	if err != nil || !recorded || progress.PercentComplete != 100 || progress.Status != "completed" {
		t.Fatalf("final completion = %#v, recorded=%v, err=%v", progress, recorded, err)
	}

	dashboard, err = st.LoadDashboard(ctx, memberID)
	if err != nil || len(dashboard.Enrollments) != 1 || len(dashboard.Enrollments[0].Lessons) != 3 {
		t.Fatalf("dashboard = %#v, err=%v", dashboard, err)
	}
	for _, lesson := range dashboard.Enrollments[0].Lessons {
		if !lesson.Completed || lesson.CompletedAt == nil {
			t.Errorf("lesson not projected complete: %#v", lesson)
		}
	}
}

// TestWithdrawnEnrollmentRejectsCompletion_Integration: withdrawal is an explicit decision.
// A completion against a withdrawn enrollment is refused, writes no event, and leaves the
// status alone, matching the reconcile command, which also preserves "withdrawn".
func TestWithdrawnEnrollmentRejectsCompletion_Integration(t *testing.T) {
	st, db := seededStore(t)
	ctx := context.Background()
	const memberID = "99990000-0000-0000-0000-000000000030"
	const courseID = "10000000-0000-0000-0000-000000000001"
	const firstLessonID = "30000000-0000-0000-0000-000000000001"
	insertMember(t, db, memberID, "test|withdrawn", "learner")

	progress, _, err := st.Enroll(ctx, memberID, courseID)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if _, err := db.Pool.Exec(ctx, `update enrollment set status = 'withdrawn' where id = $1::uuid`, progress.EnrollmentID); err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	if _, _, err := st.CompleteLesson(ctx, memberID, progress.EnrollmentID, firstLessonID); !errors.Is(err, ErrEnrollmentWithdrawn) {
		t.Fatalf("completion on withdrawn enrollment error = %v; want ErrEnrollmentWithdrawn", err)
	}
	var status string
	var events int
	if err := db.Pool.QueryRow(ctx, `
		select e.status, (select count(*) from progress_event where enrollment_id = e.id)
		from enrollment e where e.id = $1::uuid`, progress.EnrollmentID).Scan(&status, &events); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if status != "withdrawn" || events != 0 {
		t.Errorf("after refused completion: status=%q events=%d; want withdrawn and 0", status, events)
	}
}

// TestLoadMemberCredentials_Integration reads the seeded members by subject, the way the
// federation service does, and checks the contract facts the fixtures pin.
func TestLoadMemberCredentials_Integration(t *testing.T) {
	st, _ := seededStore(t)
	ctx := context.Background()

	cases := []struct {
		name, subject, id   string
		want                safeguarding.Status
		holds               int
		roleCredentialValid bool
	}{
		{"eligible coach", "demo|learner", "11111111-1111-1111-1111-111111111111", safeguarding.StatusEligible, 0, true},
		{"suspended referee (active hold)", "demo|referee-sam", "22222222-2222-2222-2222-222222222222", safeguarding.StatusSuspended, 1, true},
		{"lapsed referee (expired recert)", "demo|referee-riley", "33333333-3333-3333-3333-333333333333", safeguarding.StatusIneligible, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record, err := st.LoadMemberCredentials(ctx, tc.subject)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if record.MemberID != tc.id || record.Subject != tc.subject {
				t.Errorf("record identity = %q / %q; want %q / %q", record.MemberID, record.Subject, tc.id, tc.subject)
			}
			response := credentials.Build(record)
			if response.Eligibility.Status != tc.want {
				t.Errorf("status = %q (%s); want %q", response.Eligibility.Status, response.Eligibility.Reason, tc.want)
			}
			if len(response.Holds) != tc.holds {
				t.Errorf("holds = %#v; want %d", response.Holds, tc.holds)
			}
			if len(response.RoleCredentials) != 1 || response.RoleCredentials[0].Valid != tc.roleCredentialValid {
				t.Errorf("role credentials = %#v; want one with valid=%v", response.RoleCredentials, tc.roleCredentialValid)
			}
			if !response.Safeguarding.SafeSportTraining.Valid || !response.Safeguarding.BackgroundCheck.Valid {
				t.Errorf("safeguarding = %#v; want both valid", response.Safeguarding)
			}
		})
	}

	if _, err := st.LoadMemberCredentials(ctx, "demo|nobody"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown subject error = %v; want ErrNotFound", err)
	}
}
