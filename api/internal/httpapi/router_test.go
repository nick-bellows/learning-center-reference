package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nick-bellows/learning-center-reference/api/internal/authn"
	"github.com/nick-bellows/learning-center-reference/api/internal/learning"
	"github.com/nick-bellows/learning-center-reference/api/internal/store"
)

// TestHealth checks the health route with no store configured.
func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	NewRouter(Deps{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`status field = %q; want "ok"`, body["status"])
	}
}

// failingPinger fakes a database that is down.
type failingPinger struct{}

func (failingPinger) Ping(context.Context) error { return errors.New("connection refused") }

// TestHealth_DatabaseDown: with a database configured but unreachable, readiness must
// report 503 rather than a hollow "ok".
func TestHealth_DatabaseDown(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	NewRouter(Deps{DB: failingPinger{}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

// TestEligibility_InvalidID: a malformed member id must be rejected as 400 before it
// reaches the database, where a bad path parameter would become a 500 cast error.
func TestEligibility_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/members/not-a-uuid/eligibility", nil)
	rec := httptest.NewRecorder()

	NewRouter(Deps{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

type fakeVerifier struct{}

func (fakeVerifier) Verify(_ context.Context, token string) (string, error) {
	switch token {
	case "learner-token":
		return "subject|learner", nil
	case "admin-token":
		return "subject|admin", nil
	default:
		return "", authn.ErrInvalidCredential
	}
}

type fakeApplication struct{}

func (fakeApplication) ResolveMemberBySubject(_ context.Context, subject string) (learning.Member, error) {
	switch subject {
	case "subject|learner":
		return learning.Member{ID: "11111111-1111-1111-1111-111111111111", Roles: []string{"learner"}}, nil
	case "subject|admin":
		return learning.Member{ID: "44444444-4444-4444-4444-444444444444", Roles: []string{"admin"}}, nil
	default:
		return learning.Member{}, store.ErrNotFound
	}
}

func (fakeApplication) ListPublishedCourses(context.Context) ([]learning.CourseSummary, error) {
	return []learning.CourseSummary{{ID: "10000000-0000-0000-0000-000000000001", Title: "Test course"}}, nil
}

func (fakeApplication) Enroll(_ context.Context, memberID, courseID string) (learning.EnrollmentProgress, bool, error) {
	return learning.EnrollmentProgress{EnrollmentID: "50000000-0000-0000-0000-000000000001", CourseID: courseID}, true, nil
}

func (fakeApplication) CompleteLesson(_ context.Context, memberID, enrollmentID, lessonID string) (learning.EnrollmentProgress, bool, error) {
	return learning.EnrollmentProgress{EnrollmentID: enrollmentID, CompletedLessons: 1}, true, nil
}

func (fakeApplication) LoadDashboard(_ context.Context, memberID string) (learning.Dashboard, error) {
	return learning.Dashboard{Member: learning.Member{ID: memberID}}, nil
}

func (fakeApplication) ListCompliance(context.Context) ([]learning.ComplianceMember, error) {
	return []learning.ComplianceMember{{ID: "11111111-1111-1111-1111-111111111111", Status: "eligible"}}, nil
}

func authenticatedRouter() http.Handler {
	app := fakeApplication{}
	return NewRouter(Deps{Auth: fakeVerifier{}, Identity: app, Learning: app})
}

func TestProtectedRouteRequiresBearerToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/courses", nil)
	rec := httptest.NewRecorder()

	authenticatedRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLearnerCanListCourses(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/courses", nil)
	req.Header.Set("Authorization", "Bearer learner-token")
	rec := httptest.NewRecorder()

	authenticatedRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Courses []learning.CourseSummary `json:"courses"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Courses) != 1 || body.Courses[0].Title != "Test course" {
		t.Fatalf("courses = %#v", body.Courses)
	}
}

func TestDatabaseRoleBlocksLearnerFromAdminRoute(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/compliance", nil)
	req.Header.Set("Authorization", "Bearer learner-token")
	rec := httptest.NewRecorder()

	authenticatedRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusForbidden)
	}
}

func TestAdminCanLoadCompliance(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/compliance", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec := httptest.NewRecorder()

	authenticatedRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestMalformedEnrollmentIDIsRejectedBeforeStore(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/courses/not-a-uuid/enrollments", nil)
	req.Header.Set("Authorization", "Bearer learner-token")
	rec := httptest.NewRecorder()

	authenticatedRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestBearerTokenParsing(t *testing.T) {
	tests := []struct {
		header string
		want   string
		ok     bool
	}{
		{"Bearer token", "token", true},
		{"bearer token", "token", true},
		{"Basic token", "", false},
		{"Bearer", "", false},
		{"Bearer too many", "", false},
	}
	for _, tc := range tests {
		got, ok := bearerToken(tc.header)
		if got != tc.want || ok != tc.ok {
			t.Errorf("bearerToken(%q) = %q, %v; want %q, %v", tc.header, got, ok, tc.want, tc.ok)
		}
	}
}
