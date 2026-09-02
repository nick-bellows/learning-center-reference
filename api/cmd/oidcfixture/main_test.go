package main

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAuthorizationCodePKCEIsSingleUse(t *testing.T) {
	p, err := newProvider(
		"http://oidc.localhost:5556", "learning-center-web", "learning-center-api",
		"http://localhost:3000/api/auth/callback",
	)
	if err != nil {
		t.Fatal(err)
	}
	verifier := strings.Repeat("v", 48)
	digest := sha256.Sum256([]byte(verifier))
	values := url.Values{
		"response_type": {"code"}, "client_id": {"learning-center-web"},
		"redirect_uri": {"http://localhost:3000/api/auth/callback"}, "scope": {"openid"},
		"audience": {"learning-center-api"},
		"state":    {"state-value"}, "nonce": {"nonce-value"}, "identity": {"demo|learner"},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(digest[:])},
		"code_challenge_method": {"S256"},
	}
	req := httptest.NewRequest(http.MethodPost, "/authorize", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	p.routes().ServeHTTP(response, req)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("authorize status = %d, body = %s", response.Code, response.Body)
	}
	redirect, _ := url.Parse(response.Header().Get("Location"))
	code := redirect.Query().Get("code")
	if code == "" || redirect.Query().Get("state") != "state-value" {
		t.Fatalf("unexpected redirect %s", redirect)
	}

	tokenValues := url.Values{
		"grant_type": {"authorization_code"}, "client_id": {"learning-center-web"},
		"redirect_uri": {"http://localhost:3000/api/auth/callback"},
		"code":         {code}, "code_verifier": {verifier},
	}
	requestToken := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(tokenValues.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res := httptest.NewRecorder()
		p.routes().ServeHTTP(res, req)
		return res
	}
	if first := requestToken(); first.Code != http.StatusOK || !strings.Contains(first.Body.String(), "access_token") {
		t.Fatalf("first exchange = %d %s", first.Code, first.Body)
	}
	if second := requestToken(); second.Code != http.StatusBadRequest {
		t.Fatalf("reused code status = %d", second.Code)
	}
}

func TestFixtureRejectsNonLocalRedirect(t *testing.T) {
	if _, err := newProvider("http://issuer.localhost", "client", "api", "https://attacker.example/callback"); err == nil {
		t.Fatal("expected non-local redirect to be rejected")
	}
}
