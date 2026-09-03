package authn

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

func TestDemoVerifier(t *testing.T) {
	verifier := DemoVerifier{
		"local-token":   {Subject: "demo|learner"},
		"service-token": {Subject: "demo|federation-api", Scopes: []string{"credentials:read"}},
	}

	subject, err := verifier.Verify(context.Background(), "local-token")
	if err != nil || subject != "demo|learner" {
		t.Fatalf("Verify valid token = %q, %v", subject, err)
	}
	if _, err := verifier.Verify(context.Background(), "wrong"); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("Verify invalid token error = %v; want ErrInvalidCredential", err)
	}

	claims, err := verifier.VerifyClaims(context.Background(), "service-token")
	if err != nil || claims.Subject != "demo|federation-api" || !claims.HasScope("credentials:read") {
		t.Fatalf("VerifyClaims service token = %#v, %v", claims, err)
	}
	claims, err = verifier.VerifyClaims(context.Background(), "local-token")
	if err != nil || claims.HasScope("credentials:read") {
		t.Fatalf("person token must verify without the service scope: %#v, %v", claims, err)
	}
	if _, err := verifier.VerifyClaims(context.Background(), "wrong"); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("VerifyClaims invalid token error = %v; want ErrInvalidCredential", err)
	}
}

func TestUnavailableVerifierFailsClosed(t *testing.T) {
	if _, err := (UnavailableVerifier{}).Verify(context.Background(), "anything"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v; want ErrUnavailable", err)
	}
	if _, err := (UnavailableVerifier{}).VerifyClaims(context.Background(), "anything"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("claims error = %v; want ErrUnavailable", err)
	}
}

func TestScopeListAcceptsStringAndArray(t *testing.T) {
	cases := []struct {
		raw  string
		want []string
	}{
		{`"credentials:read openid"`, []string{"credentials:read", "openid"}},
		{`["credentials:read", "openid"]`, []string{"credentials:read", "openid"}},
		{`""`, nil},
		{`null`, nil},
	}
	for _, tc := range cases {
		var got scopeList
		if err := json.Unmarshal([]byte(tc.raw), &got); err != nil {
			t.Fatalf("unmarshal %s: %v", tc.raw, err)
		}
		if len(got) != len(tc.want) || (len(got) > 0 && !reflect.DeepEqual([]string(got), tc.want)) {
			t.Errorf("unmarshal %s = %v; want %v", tc.raw, got, tc.want)
		}
	}
	var rejected scopeList
	if err := json.Unmarshal([]byte(`42`), &rejected); err == nil {
		t.Error("a numeric scope claim must be rejected")
	}
}

func TestOIDCVerifierValidatesSignedTokenClaims(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	publicKey := jose.JSONWebKey{
		Key: privateKey.Public(), KeyID: "test-key", Algorithm: string(jose.ES256), Use: "sig",
	}

	var issuer string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                                issuer,
				"jwks_uri":                              issuer + "/keys",
				"id_token_signing_alg_values_supported": []string{"ES256"},
			})
		case "/keys":
			_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{publicKey}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	issuer = server.URL

	verifier, err := NewOIDCVerifier(context.Background(), issuer, "learning-api")
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}

	sign := func(audience string, expires time.Time, extra map[string]any) string {
		t.Helper()
		payload := map[string]any{
			"iss": issuer, "sub": "provider|member-123", "aud": audience,
			"iat": time.Now().Add(-time.Minute).Unix(), "exp": expires.Unix(),
		}
		for key, value := range extra {
			payload[key] = value
		}
		claims, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal claims: %v", err)
		}
		signer, err := jose.NewSigner(
			jose.SigningKey{Algorithm: jose.ES256, Key: &jose.JSONWebKey{
				Key: privateKey, KeyID: "test-key", Algorithm: string(jose.ES256),
			}}, nil)
		if err != nil {
			t.Fatalf("new signer: %v", err)
		}
		signed, err := signer.Sign(claims)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		compact, err := signed.CompactSerialize()
		if err != nil {
			t.Fatalf("serialize: %v", err)
		}
		return compact
	}
	valid := time.Now().Add(time.Hour)

	subject, err := verifier.Verify(context.Background(), sign("learning-api", valid, nil))
	if err != nil || subject != "provider|member-123" {
		t.Fatalf("valid token = %q, %v", subject, err)
	}
	if _, err := verifier.Verify(context.Background(), sign("wrong-audience", valid, nil)); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("wrong audience error = %v; want ErrInvalidCredential", err)
	}
	if _, err := verifier.Verify(context.Background(), sign("learning-api", time.Now().Add(-time.Hour), nil)); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("expired token error = %v; want ErrInvalidCredential", err)
	}

	claims, err := verifier.VerifyClaims(context.Background(), sign("learning-api", valid, nil))
	if err != nil || claims.Subject != "provider|member-123" || len(claims.Scopes) != 0 {
		t.Fatalf("token without scopes = %#v, %v", claims, err)
	}
	claims, err = verifier.VerifyClaims(context.Background(),
		sign("learning-api", valid, map[string]any{"sub": "client@clients", "scope": "credentials:read other:write"}))
	if err != nil || claims.Subject != "client@clients" || !claims.HasScope("credentials:read") || !claims.HasScope("other:write") {
		t.Fatalf("space-separated scope claim = %#v, %v", claims, err)
	}
	claims, err = verifier.VerifyClaims(context.Background(),
		sign("learning-api", valid, map[string]any{"scp": []string{"credentials:read"}}))
	if err != nil || !claims.HasScope("credentials:read") {
		t.Fatalf("scp array claim = %#v, %v", claims, err)
	}
	if _, err := verifier.VerifyClaims(context.Background(),
		sign("wrong-audience", valid, map[string]any{"scope": "credentials:read"})); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("scoped token with wrong audience error = %v; want ErrInvalidCredential", err)
	}
}
