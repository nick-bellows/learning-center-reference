// Package authn verifies bearer credentials. Authorization remains a separate
// concern: handlers resolve the verified subject to roles stored in PostgreSQL.
package authn

import (
	"context"
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
	token, err := v.verifier.Verify(ctx, rawToken)
	if err != nil || strings.TrimSpace(token.Subject) == "" {
		return "", ErrInvalidCredential
	}
	return token.Subject, nil
}

// DemoVerifier is a local-only adapter for the synthetic Docker demonstration.
// Tokens are identifiers, not secrets, and this verifier is never selected implicitly.
type DemoVerifier map[string]string

func (v DemoVerifier) Verify(_ context.Context, rawToken string) (string, error) {
	if subject := v[rawToken]; subject != "" {
		return subject, nil
	}
	return "", ErrInvalidCredential
}

// UnavailableVerifier fails closed when no authentication mode is configured.
type UnavailableVerifier struct{}

func (UnavailableVerifier) Verify(context.Context, string) (string, error) {
	return "", ErrUnavailable
}
