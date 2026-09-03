// Package authn verifies bearer credentials. Authorization remains a separate
// concern: person routes resolve the verified subject to roles stored in
// PostgreSQL, and service routes authorise on the scopes the token was granted.
package authn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

var (
	// ErrInvalidCredential intentionally hides verification details from clients.
	ErrInvalidCredential = errors.New("invalid credential")
	// ErrUnavailable marks protected routes unavailable when auth is not configured.
	ErrUnavailable = errors.New("authentication unavailable")
)

// Verifier turns an opaque bearer credential into a stable identity-provider subject.
type Verifier interface {
	Verify(ctx context.Context, rawToken string) (string, error)
}

// Claims are the verified token facts a service route authorises on. Scopes are
// granted by the identity provider to a client, never to a person, so person routes
// ignore them and keep resolving roles from the database.
type Claims struct {
	Subject string
	Scopes  []string
}

// HasScope reports whether the token was granted scope.
func (c Claims) HasScope(scope string) bool {
	for _, granted := range c.Scopes {
		if granted == scope {
			return true
		}
	}
	return false
}

// ClaimsVerifier is the service-route view of a verifier: the subject plus granted
// scopes. Every verifier in this package implements both interfaces, so one
// configured mode serves person and service routes without a second trust setup.
type ClaimsVerifier interface {
	VerifyClaims(ctx context.Context, rawToken string) (Claims, error)
}

// OIDCVerifier validates signature, issuer, audience, and expiry using provider
// discovery. It works with Auth0 and other standards-compliant OIDC providers.
type OIDCVerifier struct {
	verifier *oidc.IDTokenVerifier
}

// NewOIDCVerifier discovers issuer metadata at startup. Failure is fatal rather than
// silently falling back to weaker authentication.
func NewOIDCVerifier(ctx context.Context, issuer, audience string) (*OIDCVerifier, error) {
	if strings.TrimSpace(issuer) == "" || strings.TrimSpace(audience) == "" {
		return nil, errors.New("OIDC issuer and audience are required")
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discovering OIDC provider: %w", err)
	}
	return &OIDCVerifier{verifier: provider.Verifier(&oidc.Config{ClientID: audience})}, nil
}

// Verify returns only the stable subject. Application roles are deliberately ignored.
func (v *OIDCVerifier) Verify(ctx context.Context, rawToken string) (string, error) {
	claims, err := v.VerifyClaims(ctx, rawToken)
	if err != nil {
		return "", err
	}
	return claims.Subject, nil
}

// VerifyClaims validates the token exactly like Verify and also reads the granted
// scopes. Providers disagree on where those live: JWT access tokens carry a
// space-separated "scope" string (RFC 9068 section 2.2.3), other providers an "scp"
// array. Both are accepted and merged; an unreadable scope claim fails closed.
func (v *OIDCVerifier) VerifyClaims(ctx context.Context, rawToken string) (Claims, error) {
	token, err := v.verifier.Verify(ctx, rawToken)
	if err != nil || strings.TrimSpace(token.Subject) == "" {
		return Claims{}, ErrInvalidCredential
	}
	var granted struct {
		Scope scopeList `json:"scope"`
		Scp   scopeList `json:"scp"`
	}
	if err := token.Claims(&granted); err != nil {
		return Claims{}, ErrInvalidCredential
	}
	return Claims{Subject: token.Subject, Scopes: mergeScopes(granted.Scope, granted.Scp)}, nil
}

// scopeList decodes a scope claim written either as one space-separated string or
// as an array of strings.
type scopeList []string

func (s *scopeList) UnmarshalJSON(data []byte) error {
	var joined string
	if err := json.Unmarshal(data, &joined); err == nil {
		*s = strings.Fields(joined)
		return nil
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return errors.New("scope claim must be a string or an array of strings")
	}
	*s = list
	return nil
}

func mergeScopes(lists ...scopeList) []string {
	seen := make(map[string]bool)
	merged := make([]string, 0)
	for _, list := range lists {
		for _, scope := range list {
			scope = strings.TrimSpace(scope)
			if scope == "" || seen[scope] {
				continue
			}
			seen[scope] = true
			merged = append(merged, scope)
		}
	}
	return merged
}

// DemoVerifier is a local-only adapter for the synthetic Docker demonstration.
// Tokens are identifiers, not secrets, and this verifier is never selected implicitly.
// Person entries carry a subject only; the service entry also carries its scopes, so
// a person token presented to a service route is valid but unscoped (403), exactly
// as it would be with a real provider.
type DemoVerifier map[string]Claims

func (v DemoVerifier) Verify(ctx context.Context, rawToken string) (string, error) {
	claims, err := v.VerifyClaims(ctx, rawToken)
	if err != nil {
		return "", err
	}
	return claims.Subject, nil
}

func (v DemoVerifier) VerifyClaims(_ context.Context, rawToken string) (Claims, error) {
	if claims, ok := v[rawToken]; ok && claims.Subject != "" {
		return claims, nil
	}
	return Claims{}, ErrInvalidCredential
}

// UnavailableVerifier fails closed when no authentication mode is configured.
type UnavailableVerifier struct{}

func (UnavailableVerifier) Verify(context.Context, string) (string, error) {
	return "", ErrUnavailable
}

func (UnavailableVerifier) VerifyClaims(context.Context, string) (Claims, error) {
	return Claims{}, ErrUnavailable
}
