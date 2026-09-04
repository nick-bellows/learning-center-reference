package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientRateLimiterResetsAndSeparatesClients(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	limiter := newClientRateLimiter(2, time.Minute, false)
	limiter.now = func() time.Time { return now }

	if !limiter.allow("192.0.2.1") || !limiter.allow("192.0.2.1") {
		t.Fatal("first two requests should pass")
	}
	if limiter.allow("192.0.2.1") {
		t.Fatal("third request should be rate limited")
	}
	if !limiter.allow("192.0.2.2") {
		t.Fatal("a separate client should have its own window")
	}
	now = now.Add(time.Minute)
	if !limiter.allow("192.0.2.1") {
		t.Fatal("request should pass after the window resets")
	}
}

func TestRateLimitMiddlewareUsesForwardedAddressOnlyWhenTrusted(t *testing.T) {
	limiter := newClientRateLimiter(1, time.Minute, true)
	handler := limiter.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := func(forwarded string) int {
		req := httptest.NewRequest(http.MethodGet, "/v1/courses", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		req.Header.Set("X-Forwarded-For", forwarded)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res.Code
	}

	if status := request("192.0.2.1"); status != http.StatusNoContent {
		t.Fatalf("first client status = %d", status)
	}
	if status := request("192.0.2.2"); status != http.StatusNoContent {
		t.Fatalf("second forwarded client status = %d", status)
	}
	if status := request("192.0.2.1"); status != http.StatusTooManyRequests {
		t.Fatalf("repeat status = %d", status)
	}
}

// A client cannot escape its window by prepending spoofed X-Forwarded-For entries: the
// rightmost address, appended by the trusted proxy, is the one the limiter keys on.
func TestRateLimitMiddlewareUsesRightmostForwardedEntry(t *testing.T) {
	limiter := newClientRateLimiter(1, time.Minute, true)
	handler := limiter.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := func(forwarded string) int {
		req := httptest.NewRequest(http.MethodGet, "/v1/courses", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		req.Header.Set("X-Forwarded-For", forwarded)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res.Code
	}

	if status := request("192.0.2.50"); status != http.StatusNoContent {
		t.Fatalf("first request status = %d", status)
	}
	// Same real client (rightmost 192.0.2.50), only the spoofed leftmost value changed.
	if status := request("10.0.0.9, 192.0.2.50"); status != http.StatusTooManyRequests {
		t.Fatalf("spoofed-prefix retry status = %d; want 429", status)
	}
}
