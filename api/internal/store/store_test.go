package store

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/nick-bellows/learning-center-reference/api/internal/credentials"
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
	t.Cleanup(st.Close)

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

func TestLearningWorkflow_Integration(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("set DATABASE_URL to run (needs Postgres)")
	}

	ctx := context.Background()
	st, err := New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(st.Close)
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

	const memberID = "99990000-0000-0000-0000-000000000001"
	const courseID = "10000000-0000-0000-0000-000000000001"
	const firstLessonID = "30000000-0000-0000-0000-000000000001"
	const secondLessonID = "30000000-0000-0000-0000-000000000002"
	const finalLessonID = "30000000-0000-0000-0000-000000000003"

	// This member belongs only to the test and cascades its enrollment/events on cleanup.
	_, _ = st.Pool().Exec(ctx, `delete from member where id = $1::uuid`, memberID)
	if _, err := st.Pool().Exec(ctx, `
		insert into member (id, auth_subject, display_name, association_id)
		values ($1::uuid, 'test|workflow', 'Workflow Test (synthetic)',
		        '00000000-0000-0000-0000-0000000000aa')`, memberID); err != nil {
		t.Fatalf("insert member: %v", err)
	}
	t.Cleanup(func() {
		_, _ = st.Pool().Exec(context.Background(), `delete from progress_event where actor_member_id = $1::uuid`, memberID)
		_, _ = st.Pool().Exec(context.Background(), `delete from member where id = $1::uuid`, memberID)
	})
	if _, err := st.Pool().Exec(ctx, `insert into member_role (member_id, role) values ($1::uuid, 'learner')`, memberID); err != nil {
		t.Fatalf("insert role: %v", err)
	}

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

	if _, _, err := st.CompleteLesson(ctx, memberID, progress.EnrollmentID, secondLessonID); err != nil {
		t.Fatalf("second completion: %v", err)
	}
	progress, recorded, err = st.CompleteLesson(ctx, memberID, progress.EnrollmentID, finalLessonID)
	if err != nil || !recorded || progress.PercentComplete != 100 || progress.Status != "completed" {
		t.Fatalf("final completion = %#v, recorded=%v, err=%v", progress, recorded, err)
	}

	dashboard, err := st.LoadDashboard(ctx, memberID)
	if err != nil || len(dashboard.Enrollments) != 1 || len(dashboard.Enrollments[0].Lessons) != 3 {
		t.Fatalf("dashboard = %#v, err=%v", dashboard, err)
	}
	for _, lesson := range dashboard.Enrollments[0].Lessons {
		if !lesson.Completed || lesson.CompletedAt == nil {
			t.Errorf("lesson not projected complete: %#v", lesson)
		}
	}
}

// TestLoadMemberCredentials_Integration reads the seeded members by subject, the way the
// federation service does, and checks the contract facts the fixtures pin.
func TestLoadMemberCredentials_Integration(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("set DATABASE_URL to run (needs Postgres)")
	}

	ctx := context.Background()
	st, err := New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(st.Close)
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
