package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHealth exercises the router without opening a real network port: httptest gives us
// a fake request and a recorder that captures the response. Deps{} is empty because the
// /health route doesn't need the store.
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

// TestHealth_DatabaseDown: /health is a READINESS check — with a database
// configured but unreachable, it must say so with a 503, not a hollow "ok".
func TestHealth_DatabaseDown(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	NewRouter(Deps{DB: failingPinger{}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

// TestEligibility_InvalidID: a malformed member id is the CLIENT's error.
// It must be rejected as 400 before touching the database (which would have
// turned a bad path parameter into a 500 cast error).
func TestEligibility_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/members/not-a-uuid/eligibility", nil)
	rec := httptest.NewRecorder()

	NewRouter(Deps{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}
