package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/nick-bellows/learning-center-reference/api/internal/authn"
	"github.com/nick-bellows/learning-center-reference/api/internal/credentials"
	"github.com/nick-bellows/learning-center-reference/api/internal/safeguarding"
	"github.com/nick-bellows/learning-center-reference/api/internal/store"
)

// fakeServiceVerifier stands in for the shared identity provider: one scoped client, one
// client without the scope, and a person token that verifies but carries no scopes.
type fakeServiceVerifier struct{}

func (fakeServiceVerifier) VerifyClaims(_ context.Context, token string) (authn.Claims, error) {
	switch token {
	case "service-token":
		return authn.Claims{Subject: "client|federation", Scopes: []string{credentials.Scope}}, nil
	case "unscoped-service-token":
		return authn.Claims{Subject: "client|other", Scopes: []string{"other:read"}}, nil
	case "learner-token":
		return authn.Claims{Subject: "subject|learner"}, nil
	default:
		return authn.Claims{}, authn.ErrInvalidCredential
	}
}

type fakeCredentials map[string]credentials.Record

func (f fakeCredentials) LoadMemberCredentials(_ context.Context, subject string) (credentials.Record, error) {
	record, ok := f[subject]
	if !ok {
		return credentials.Record{}, store.ErrNotFound
	}
	return record, nil
}

func dateOf(year int, month time.Month, day int) *time.Time {
	t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return &t
}

func eligibleLearnerRecord() credentials.Record {
	return credentials.Record{
		MemberID: "11111111-1111-1111-1111-111111111111",
		Subject:  "demo|learner",
		Roles:    []string{"coach", "learner"},
		Inputs: safeguarding.Inputs{
			Now:                    time.Date(2026, 9, 3, 5, 0, 0, 0, time.UTC),
			SafeSportExpires:       dateOf(2027, 6, 1),
			BackgroundCheckExpires: dateOf(2028, 1, 1),
			RoleCredentialRequired: true,
			RoleCredentialExpires:  dateOf(2027, 2, 1),
		},
		RoleCredentials: []credentials.RoleCredential{{
			Role: "coach", CredentialType: "c_license",
			IssuedAt: *dateOf(2025, 2, 1), ExpiresAt: *dateOf(2027, 2, 1),
		}},
	}
}

func credentialsRouter(records fakeCredentials) http.Handler {
	return NewRouter(Deps{ServiceAuth: fakeServiceVerifier{}, Credentials: records})
}

func getCredentials(t *testing.T, handler http.Handler, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func assertErrorBody(t *testing.T, rec *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d; want %d; body=%s", rec.Code, status, rec.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != message {
		t.Fatalf("error = %q; want %q", body["error"], message)
	}
}

const learnerCredentialsPath = "/v1/members/demo%7Clearner/credentials"

func TestCredentialsRequireServiceToken(t *testing.T) {
	handler := credentialsRouter(fakeCredentials{"demo|learner": eligibleLearnerRecord()})
	assertErrorBody(t, getCredentials(t, handler, learnerCredentialsPath, ""), http.StatusUnauthorized, "unauthorized")
	assertErrorBody(t, getCredentials(t, handler, learnerCredentialsPath, "garbage"), http.StatusUnauthorized, "unauthorized")
}

func TestCredentialsRejectTokensWithoutScope(t *testing.T) {
	handler := credentialsRouter(fakeCredentials{"demo|learner": eligibleLearnerRecord()})
	assertErrorBody(t, getCredentials(t, handler, learnerCredentialsPath, "learner-token"), http.StatusForbidden, "forbidden")
	assertErrorBody(t, getCredentials(t, handler, learnerCredentialsPath, "unscoped-service-token"), http.StatusForbidden, "forbidden")
}

// Without a configured verifier the route fails closed, matching the person routes.
func TestCredentialsFailClosedWithoutServiceAuth(t *testing.T) {
	handler := NewRouter(Deps{Credentials: fakeCredentials{"demo|learner": eligibleLearnerRecord()}})
	rec := getCredentials(t, handler, learnerCredentialsPath, "service-token")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestCredentialsRejectMalformedSubject(t *testing.T) {
	handler := credentialsRouter(fakeCredentials{})
	assertErrorBody(t, getCredentials(t, handler, "/v1/members/%20/credentials", "service-token"),
		http.StatusBadRequest, "invalid subject")
	overlong := strings.Repeat("a", maxSubjectLength+1)
	assertErrorBody(t, getCredentials(t, handler, "/v1/members/"+overlong+"/credentials", "service-token"),
		http.StatusBadRequest, "invalid subject")
}

func TestCredentialsUnknownSubject(t *testing.T) {
	handler := credentialsRouter(fakeCredentials{})
	assertErrorBody(t, getCredentials(t, handler, "/v1/members/demo%7Cnobody/credentials", "service-token"),
		http.StatusNotFound, "member not found")
}

func TestCredentialsServeContractShape(t *testing.T) {
	handler := credentialsRouter(fakeCredentials{"demo|learner": eligibleLearnerRecord()})
	rec := getCredentials(t, handler, learnerCredentialsPath, "service-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []string{"as_of", "contract", "eligibility", "holds", "member", "role_credentials", "safeguarding"}
	got := make([]string, 0, len(body))
	for key := range body {
		got = append(got, key)
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("top-level keys = %v; want exactly %v", got, want)
	}
	if member := body["member"].(map[string]any); member["subject"] != "demo|learner" {
		t.Errorf("member.subject = %v; want the URL-decoded subject", member["subject"])
	}
	if body["contract"] != credentials.Contract || body["as_of"] != "2026-09-03T05:00:00Z" {
		t.Errorf("contract/as_of = %v / %v", body["contract"], body["as_of"])
	}
	if status := body["eligibility"].(map[string]any)["status"]; status != "eligible" {
		t.Errorf("eligibility.status = %v; want eligible", status)
	}
}

// The eligibility route keeps its UUID validation: a subject-shaped id is still a 400 there,
// and the credentials route never answers for it.
func TestEligibilityRouteStillValidatesUUID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/members/demo%7Clearner/eligibility", nil)
	rec := httptest.NewRecorder()
	credentialsRouter(fakeCredentials{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}
