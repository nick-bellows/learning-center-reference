// Command oidcfixture runs a local-only standards-compliant OIDC provider for
// browser/session tests. It offers two fixed fictional identities and must never
// be used as a public identity provider.
package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type authorization struct {
	subject       string
	redirectURI   string
	codeChallenge string
	nonce         string
	expiresAt     time.Time
}

type provider struct {
	issuer      string
	clientID    string
	audience    string
	redirectURI string
	key         *rsa.PrivateKey
	mu          sync.Mutex
	codes       map[string]authorization
}

var loginPage = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Local OIDC fixture</title><style>
body{margin:0;background:#f4f7fb;color:#10233f;font:16px/1.55 system-ui,sans-serif}main{max-width:650px;margin:5rem auto;padding:2rem;background:#fff;border:1px solid #dce4ee;border-radius:1rem}button{width:100%;margin:.5rem 0;padding:.8rem;border:0;border-radius:.5rem;background:#155eef;color:#fff;font-weight:700;cursor:pointer}button:focus-visible{outline:3px solid #f59e0b;outline-offset:3px}.note{color:#52627a;font-size:.9rem}</style></head>
<body><main><p class="note">Local test identity provider · fictional identities only</p><h1>Choose a demo identity</h1><p>This explicit sign-in step tests OIDC Authorization Code + PKCE. Roles still come from PostgreSQL.</p>
{{range .Identities}}<form method="post" action="/authorize">
{{range $.Hidden}}<input type="hidden" name="{{.Name}}" value="{{.Value}}">{{end}}
<input type="hidden" name="identity" value="{{.Subject}}"><button type="submit">Continue as {{.Label}}</button></form>{{end}}
<p class="note">Not Auth0 and not suitable for internet deployment.</p></main></body></html>`))

type hiddenField struct{ Name, Value string }
type identityChoice struct{ Subject, Label string }
type loginData struct {
	Hidden     []hiddenField
	Identities []identityChoice
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		response, err := (&http.Client{Timeout: 2 * time.Second}).Get("http://127.0.0.1:" + envOr("PORT", "5556") + "/health")
		if err != nil || response.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		_ = response.Body.Close()
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	p, err := newProvider(
		envOr("OIDC_FIXTURE_ISSUER", "http://oidc.localhost:5556"),
		envOr("OIDC_FIXTURE_CLIENT_ID", "learning-center-web"),
		envOr("OIDC_FIXTURE_AUDIENCE", "learning-center-api"),
		envOr("OIDC_FIXTURE_REDIRECT_URI", "http://localhost:3000/api/auth/callback"),
	)
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{
		Addr:              ":" + envOr("PORT", "5556"),
		Handler:           p.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	go func() {
		log.Printf("local OIDC fixture listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}

func newProvider(issuer, clientID, audience, redirectURI string) (*provider, error) {
	parsedIssuer, err := url.Parse(issuer)
	if err != nil || parsedIssuer.Scheme != "http" || parsedIssuer.Host == "" {
		return nil, fmt.Errorf("local fixture issuer must be an HTTP URL")
	}
	parsedRedirect, err := url.Parse(redirectURI)
	if err != nil || parsedRedirect.Scheme != "http" || parsedRedirect.Hostname() != "localhost" {
		return nil, fmt.Errorf("local fixture redirect must use http://localhost")
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generating signing key: %w", err)
	}
	return &provider{
		issuer: strings.TrimSuffix(issuer, "/"), clientID: clientID, audience: audience,
		redirectURI: redirectURI, key: key, codes: make(map[string]authorization),
	}, nil
}

func (p *provider) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "mode": "local-fixture"})
	})
	mux.HandleFunc("GET /.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("GET /jwks", p.jwks)
	mux.HandleFunc("GET /authorize", p.authorizeForm)
	mux.HandleFunc("POST /authorize", p.authorize)
	mux.HandleFunc("POST /token", p.token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}

func (p *provider) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                p.issuer,
		"authorization_endpoint":                p.issuer + "/authorize",
		"token_endpoint":                        p.issuer + "/token",
		"jwks_uri":                              p.issuer + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"code_challenge_methods_supported":      []string{"S256"},
		"scopes_supported":                      []string{"openid"},
	})
}

func (p *provider) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "use": "sig", "alg": "RS256", "kid": "local-fixture",
		"n": base64.RawURLEncoding.EncodeToString(p.key.PublicKey.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(p.key.PublicKey.E)).Bytes()),
	}}})
}

func (p *provider) validateAuthorization(values url.Values) error {
	if values.Get("response_type") != "code" || values.Get("client_id") != p.clientID ||
		values.Get("redirect_uri") != p.redirectURI || values.Get("code_challenge_method") != "S256" ||
		values.Get("code_challenge") == "" || values.Get("state") == "" || values.Get("nonce") == "" {
		return fmt.Errorf("invalid authorization request")
	}
	scopes := strings.Fields(values.Get("scope"))
	if !contains(scopes, "openid") || values.Get("audience") != p.audience {
		return fmt.Errorf("openid scope is required")
	}
	return nil
}

func (p *provider) authorizeForm(w http.ResponseWriter, r *http.Request) {
	if err := p.validateAuthorization(r.URL.Query()); err != nil {
		http.Error(w, "invalid authorization request", http.StatusBadRequest)
		return
	}
	fields := []hiddenField{}
	for _, name := range []string{"response_type", "client_id", "redirect_uri", "scope", "audience", "state", "nonce", "code_challenge", "code_challenge_method"} {
		fields = append(fields, hiddenField{Name: name, Value: r.URL.Query().Get(name)})
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := loginPage.Execute(w, loginData{
		Hidden: fields,
		Identities: []identityChoice{
			{Subject: "demo|learner", Label: "Alex Coach (learner)"},
			{Subject: "demo|admin", Label: "Casey Admin (administrator)"},
		},
	}); err != nil {
		http.Error(w, "rendering login", http.StatusInternalServerError)
	}
}

func (p *provider) authorize(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || p.validateAuthorization(r.PostForm) != nil {
		http.Error(w, "invalid authorization request", http.StatusBadRequest)
		return
	}
	subject := r.PostForm.Get("identity")
	if subject != "demo|learner" && subject != "demo|admin" {
		http.Error(w, "unknown fictional identity", http.StatusBadRequest)
		return
	}
	code, err := randomValue(32)
	if err != nil {
		http.Error(w, "authorization unavailable", http.StatusInternalServerError)
		return
	}
	p.mu.Lock()
	p.codes[code] = authorization{
		subject: subject, redirectURI: p.redirectURI,
		codeChallenge: r.PostForm.Get("code_challenge"), nonce: r.PostForm.Get("nonce"),
		expiresAt: time.Now().Add(2 * time.Minute),
	}
	p.mu.Unlock()
	redirect, _ := url.Parse(p.redirectURI)
	query := redirect.Query()
	query.Set("code", code)
	query.Set("state", r.PostForm.Get("state"))
	redirect.RawQuery = query.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusSeeOther)
}

func (p *provider) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "authorization_code" ||
		r.PostForm.Get("client_id") != p.clientID || r.PostForm.Get("redirect_uri") != p.redirectURI {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
		return
	}
	code := r.PostForm.Get("code")
	p.mu.Lock()
	auth, found := p.codes[code]
	delete(p.codes, code)
	p.mu.Unlock()
	digest := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	if !found || time.Now().After(auth.expiresAt) || auth.redirectURI != p.redirectURI ||
		challenge != auth.codeChallenge {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
		return
	}
	now := time.Now().UTC()
	accessToken, err := p.sign(map[string]any{
		"iss": p.issuer, "sub": auth.subject, "aud": p.audience,
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	})
	if err != nil {
		http.Error(w, "token unavailable", http.StatusInternalServerError)
		return
	}
	idToken, err := p.sign(map[string]any{
		"iss": p.issuer, "sub": auth.subject, "aud": p.clientID, "nonce": auth.nonce,
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	})
	if err != nil {
		http.Error(w, "token unavailable", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken, "id_token": idToken, "token_type": "Bearer", "expires_in": 3600,
	})
}

func (p *provider) sign(claims map[string]any) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": "local-fixture"})
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func randomValue(bytes int) (string, error) {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
