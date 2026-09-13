package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nick-bellows/learning-center-reference/api/internal/learning"
)

// The API's contract is that EVERY error body is JSON {"error": ...}. chi's defaults for an
// unmatched path, a wrong method, and a recovered panic are text/plain or empty, so these
// three paths are overridden and pinned here.

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q; want application/json (body %q)", ct, rec.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if body["error"] == "" {
		t.Fatalf("body has no error field: %v", body)
	}
	return body["error"]
}

func TestUnmatchedRouteIsJSON404(t *testing.T) {
	rec := httptest.NewRecorder()
	NewRouter(Deps{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want 404", rec.Code)
	}
	decodeError(t, rec)
}

func TestWrongMethodIsJSON405(t *testing.T) {
	rec := httptest.NewRecorder()
	NewRouter(Deps{}).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/health", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d; want 405", rec.Code)
	}
	decodeError(t, rec)
}

// TestHandlerPanicIsJSON500AndLogged: a panic must become the documented 500 body and a
// structured error log entry carrying the request id, not an empty response and a raw
// stack on stderr.
func TestHandlerPanicIsJSON500AndLogged(t *testing.T) {
	var logs bytes.Buffer
	s := newStubStore()
	s.courses = func() ([]learning.CourseSummary, error) { panic("simulated handler bug") }
	deps := fullDeps(s)
	deps.Logger = slog.New(slog.NewJSONHandler(&logs, nil))

	req := httptest.NewRequest(http.MethodGet, "/v1/courses", nil)
	req.Header.Set("Authorization", "Bearer learner-token")
	rec := httptest.NewRecorder()
	NewRouter(deps).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want 500 (body %q)", rec.Code, rec.Body.String())
	}
	if msg := decodeError(t, rec); msg != "internal error" {
		t.Errorf("error = %q; want the generic message, never the panic value", msg)
	}
	requestID := rec.Header().Get("X-Request-Id")
	if requestID == "" {
		t.Fatal("response carries no X-Request-Id")
	}
	out := logs.String()
	if !strings.Contains(out, `"msg":"handler panic"`) || !strings.Contains(out, "simulated handler bug") {
		t.Errorf("panic was not logged through slog: %s", out)
	}
	if !strings.Contains(out, requestID) {
		t.Errorf("log entry does not carry the request id %q the client received: %s", requestID, out)
	}
}

// TestRequestIDIsServerGenerated: the id the client gets back is the one in the log, and a
// client cannot choose it (an inbound X-Request-Id would let anyone plant text in the logs).
func TestRequestIDIsServerGenerated(t *testing.T) {
	var logs bytes.Buffer
	deps := Deps{Logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Request-Id", "attacker-chosen-value")
	rec := httptest.NewRecorder()
	NewRouter(deps).ServeHTTP(rec, req)

	got := rec.Header().Get("X-Request-Id")
	if got == "" || got == "attacker-chosen-value" {
		t.Fatalf("X-Request-Id = %q; want a server-generated id", got)
	}
	if !strings.Contains(logs.String(), `"request_id":"`+got+`"`) {
		t.Errorf("request log does not carry the returned id %q: %s", got, logs.String())
	}
	if strings.Contains(logs.String(), "attacker-chosen-value") {
		t.Errorf("inbound request id reached the log: %s", logs.String())
	}
}

// TestRejectedTokenReasonIsLogged: the client sees only "invalid bearer token"; the
// operator's log carries the verifier's reason so an identity-provider outage does not
// look like a wave of bad tokens.
func TestRejectedTokenReasonIsLogged(t *testing.T) {
	var logs bytes.Buffer
	deps := fullDeps(newStubStore())
	deps.Logger = slog.New(slog.NewJSONHandler(&logs, nil))

	req := httptest.NewRequest(http.MethodGet, "/v1/courses", nil)
	req.Header.Set("Authorization", "Bearer forged-token")
	rec := httptest.NewRecorder()
	NewRouter(deps).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d; want 401", rec.Code)
	}
	if !strings.Contains(logs.String(), `"msg":"bearer token rejected"`) {
		t.Errorf("rejection reason not logged: %s", logs.String())
	}
	if strings.Contains(logs.String(), "forged-token") {
		t.Errorf("the token itself reached the log: %s", logs.String())
	}
}
