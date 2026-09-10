package store

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/nick-bellows/learning-center-reference/api/internal/testdb"
)

// insufficientPrivilege is the SQLSTATE PostgreSQL raises when a role lacks a privilege.
const insufficientPrivilege = "42501"

// TestRuntimeRole_Integration proves migration 0007 gives the runtime role exactly the
// privileges the store needs and nothing more, through both adoption paths the server
// supports: SET ROLE on the owner's connection (DB_RUNTIME_ROLE) and a separate login
// role that is a member of lcr_runtime (RUNTIME_DATABASE_URL).
func TestRuntimeRole_Integration(t *testing.T) {
	db := testdb.New(t)
	db.Seed(t)
	ctx := context.Background()

	t.Run("set role on the owner connection", func(t *testing.T) {
		st, err := NewWithRole(ctx, db.URL, "lcr_runtime")
		if err != nil {
			t.Fatalf("NewWithRole: %v", err)
		}
		t.Cleanup(st.Close)
		assertRuntimePrivileges(t, st)
	})

	t.Run("separate login role that inherits lcr_runtime", func(t *testing.T) {
		login := "lcr_test_login_" + db.Name[len("lcr_test_"):]
		if _, err := db.Pool.Exec(ctx, fmt.Sprintf(
			`create role %q login password 'test-only' in role lcr_runtime`, login)); err != nil {
			t.Fatalf("create login role: %v", err)
		}
		t.Cleanup(func() {
			_, _ = db.Pool.Exec(context.Background(), fmt.Sprintf(`drop role if exists %q`, login))
		})

		asLogin, err := url.Parse(db.URL)
		if err != nil {
			t.Fatalf("parse url: %v", err)
		}
		asLogin.User = url.UserPassword(login, "test-only")
		st, err := New(ctx, asLogin.String())
		if err != nil {
			t.Fatalf("connect as login role: %v", err)
		}
		t.Cleanup(st.Close)
		assertRuntimePrivileges(t, st)
	})

	t.Run("unknown role fails at startup", func(t *testing.T) {
		if _, err := NewWithRole(ctx, db.URL, "lcr_does_not_exist"); err == nil {
			t.Fatal("NewWithRole with an unknown role succeeded; want a startup error")
		}
		if _, err := NewWithRole(ctx, db.URL, `lcr"; drop table member; --`); err == nil {
			t.Fatal("NewWithRole accepted a role name that is not an identifier")
		}
	})
}

func assertRuntimePrivileges(t *testing.T, st *Store) {
	t.Helper()
	ctx := context.Background()

	role, err := st.CurrentRole(ctx)
	if err != nil {
		t.Fatalf("current role: %v", err)
	}
	t.Logf("executing as %s", role)

	// Everything the request path does must work: role resolution, the learning
	// transaction (insert enrollment/progress/event, update projection and status),
	// dashboards, compliance, and the credentials contract.
	const learner = "11111111-1111-1111-1111-111111111111"
	const course = "10000000-0000-0000-0000-000000000001"
	const firstLesson = "30000000-0000-0000-0000-000000000001"
	if _, err := st.ResolveMemberBySubject(ctx, "demo|learner"); err != nil {
		t.Fatalf("resolve member: %v", err)
	}
	progress, _, err := st.Enroll(ctx, learner, course)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if _, _, err := st.CompleteLesson(ctx, learner, progress.EnrollmentID, firstLesson); err != nil {
		t.Fatalf("complete lesson: %v", err)
	}
	if _, err := st.LoadDashboard(ctx, learner); err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if _, err := st.ListCompliance(ctx); err != nil {
		t.Fatalf("compliance: %v", err)
	}
	if _, err := st.LoadMemberCredentials(ctx, "demo|referee-riley"); err != nil {
		t.Fatalf("credentials: %v", err)
	}
	if err := st.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// Nothing beyond DML is allowed, and the progress log is append-only.
	denied := map[string]string{
		"create table":            `create table runtime_role_probe (id int)`,
		"alter table":             `alter table member add column probe text`,
		"drop table":              `drop table disciplinary_hold`,
		"update progress_event":   `update progress_event set occurred_at = now()`,
		"delete progress_event":   `delete from progress_event`,
		"truncate progress_event": `truncate progress_event`,
		"read migration ledger":   `select count(*) from schema_migrations`,
		"write migration ledger":  `insert into schema_migrations (version) values ('9999_probe.up.sql')`,
	}
	for name, sql := range denied {
		_, err := st.Pool().Exec(ctx, sql)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != insufficientPrivilege {
			t.Errorf("%s: err = %v; want SQLSTATE %s (insufficient privilege)", name, err, insufficientPrivilege)
		}
	}

	// The event the learner just appended is still readable and intact.
	var events int
	if err := st.Pool().QueryRow(ctx, `select count(*) from progress_event`).Scan(&events); err != nil || events != 1 {
		t.Fatalf("progress_event count = %d, err = %v; want 1", events, err)
	}
}
