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
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

func TestDemoVerifier(t *testing.T) {
	verifier := DemoVerifier{"local-token": "demo|learner"}

	subject, err := verifier.Verify(context.Background(), "local-token")
	if err != nil || subject != "demo|learner" {
		t.Fatalf("Verify valid token = %q, %v", subject, err)
	}
	if _, err := verifier.Verify(context.Background(), "wrong"); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("Verify invalid token error = %v; want ErrInvalidCredential", err)
	}
}

func TestUnavailableVerifierFailsClosed(t *testing.T) {
	if _, err := (UnavailableVerifier{}).Verify(context.Background(), "anything"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v; want ErrUnavailable", err)
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

	sign := func(audience string, expires time.Time) string {
		t.Helper()
		claims, err := json.Marshal(map[string]any{
			"iss": issuer, "sub": "provider|member-123", "aud": audience,
			"iat": time.Now().Add(-time.Minute).Unix(), "exp": expires.Unix(),
		})
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

	subject, err := verifier.Verify(context.Background(), sign("learning-api", time.Now().Add(time.Hour)))
	if err != nil || subject != "provider|member-123" {
		t.Fatalf("valid token = %q, %v", subject, err)
	}
	if _, err := verifier.Verify(context.Background(), sign("wrong-audience", time.Now().Add(time.Hour))); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("wrong audience error = %v; want ErrInvalidCredential", err)
	}
	if _, err := verifier.Verify(context.Background(), sign("learning-api", time.Now().Add(-time.Hour))); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("expired token error = %v; want ErrInvalidCredential", err)
	}
}
