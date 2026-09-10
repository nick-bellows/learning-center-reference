// Command server starts the Learning Center API.
//
// Startup order: connect to Postgres (when DATABASE_URL is set), apply
// embedded migrations (MIGRATE_ON_START=1), apply a seed file (SEED_FILE=
// path, local demo only), then serve. The server carries explicit timeouts
// and shuts down gracefully on SIGINT/SIGTERM.
//
// Two connection scopes: DATABASE_URL is the owner used for migrations and
// seeding; request handling uses a least-privilege pool instead when
// DB_RUNTIME_ROLE (SET ROLE per connection) or RUNTIME_DATABASE_URL (a
// separate login) is set. Public deployments must set one of them.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/nick-bellows/learning-center-reference/api/internal/authn"
	"github.com/nick-bellows/learning-center-reference/api/internal/credentials"
	"github.com/nick-bellows/learning-center-reference/api/internal/dbsetup"
	"github.com/nick-bellows/learning-center-reference/api/internal/httpapi"
	"github.com/nick-bellows/learning-center-reference/api/internal/store"
	"github.com/nick-bellows/learning-center-reference/api/migrations"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	authMode := strings.ToLower(envOr("AUTH_MODE", "disabled"))
	databaseURL := os.Getenv("DATABASE_URL")
	runtimeURL := os.Getenv("RUNTIME_DATABASE_URL")
	runtimeRole := os.Getenv("DB_RUNTIME_ROLE")
	if err := validateDeploymentConfig(deploymentConfig{
		env:         envOr("DEPLOYMENT_ENV", "local"),
		authMode:    authMode,
		databaseURL: databaseURL,
		issuer:      os.Getenv("OIDC_ISSUER_URL"),
		runtimeURL:  runtimeURL,
		runtimeRole: runtimeRole,
	}); err != nil {
		log.Fatalf("configuration: %v", err)
	}
	deps := httpapi.Deps{
		Logger:             logger,
		RateLimitPerMinute: envIntOr("RATE_LIMIT_PER_MINUTE", 120),
		TrustProxy:         os.Getenv("TRUST_PROXY") == "1",
	}
	if databaseURL != "" {
		// Setup pool: the owner applies migrations and the seed, then is not used for requests.
		setup, err := store.New(ctx, databaseURL)
		if err != nil {
			log.Fatalf("database: %v", err)
		}
		defer setup.Close()

		if os.Getenv("MIGRATE_ON_START") == "1" {
			applied, err := dbsetup.Migrate(ctx, setup.Pool(), migrations.Files)
			if err != nil {
				log.Fatalf("migrate: %v", err)
			}
			log.Printf("migrations: %d applied", len(applied))
		}
		if seedPath := os.Getenv("SEED_FILE"); seedPath != "" {
			sql, err := os.ReadFile(seedPath)
			if err != nil {
				log.Fatalf("seed: %v", err)
			}
			if err := dbsetup.ApplySeed(ctx, setup.Pool(), string(sql)); err != nil {
				log.Fatalf("seed: %v", err)
			}
			log.Printf("seed applied from %s", seedPath)
		}

		// Runtime pool: request handling runs with the restricted role's privileges when
		// either knob is set; otherwise it is the owner pool (local default without them).
		st := setup
		if runtimeURL != "" || runtimeRole != "" {
			st, err = store.NewWithRole(ctx, envOr("RUNTIME_DATABASE_URL", databaseURL), runtimeRole)
			if err != nil {
				log.Fatalf("runtime database: %v", err)
			}
			defer st.Close()
		}
		role, err := st.CurrentRole(ctx)
		if err != nil {
			log.Fatalf("runtime database: %v", err)
		}
		log.Printf("request handling runs as database role %q", role)

		deps.Eligibility = st
		deps.Identity = st
		deps.Learning = st
		deps.Credentials = st
		deps.DB = st
	} else {
		log.Println("DATABASE_URL not set; database-backed routes will be unavailable")
	}

	verifier, err := configureAuth(ctx, authMode)
	if err != nil {
		log.Fatalf("authentication: %v", err)
	}
	deps.Auth = verifier
	deps.ServiceAuth = verifier

	srv := &http.Server{
		Addr:              ":" + envOr("PORT", "8080"),
		Handler:           httpapi.NewRouter(deps),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Learning Center API listening on %s", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		log.Fatal(err)
	case <-ctx.Done():
		// Graceful shutdown: stop accepting, let in-flight requests finish.
		log.Println("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			log.Printf("shutdown: %v", err)
		}
	}
}

// verifier is what every auth mode provides: subject verification for person routes and
// claims verification for service routes, from one trust configuration.
type verifier interface {
	authn.Verifier
	authn.ClaimsVerifier
}

func configureAuth(ctx context.Context, authMode string) (verifier, error) {
	switch authMode {
	case "disabled":
		log.Println("AUTH_MODE=disabled; protected routes fail closed with 503")
		return authn.UnavailableVerifier{}, nil
	case "demo":
		log.Println("AUTH_MODE=demo; using synthetic local identities only")
		return authn.DemoVerifier{
			envOr("DEMO_LEARNER_TOKEN", "local-learner-token"): {Subject: "demo|learner"},
			envOr("DEMO_ADMIN_TOKEN", "local-admin-token"):     {Subject: "demo|admin"},
			// The service identity is a client, not a member: it carries a scope and no roles.
			envOr("DEMO_SERVICE_TOKEN", "local-service-token"): {
				Subject: "demo|federation-api", Scopes: []string{credentials.Scope},
			},
		}, nil
	case "oidc":
		return authn.NewOIDCVerifier(ctx, os.Getenv("OIDC_ISSUER_URL"), os.Getenv("OIDC_AUDIENCE"))
	default:
		return nil, fmt.Errorf("unsupported AUTH_MODE %q", os.Getenv("AUTH_MODE"))
	}
}

type deploymentConfig struct {
	env, authMode, databaseURL, issuer string
	runtimeURL, runtimeRole            string
}

func validateDeploymentConfig(c deploymentConfig) error {
	if c.env != "local" && c.env != "public" {
		return fmt.Errorf("DEPLOYMENT_ENV must be local or public, got %q", c.env)
	}
	if c.env != "public" {
		return nil
	}
	if c.authMode != "oidc" {
		return errors.New("public deployment requires AUTH_MODE=oidc")
	}
	if strings.TrimSpace(c.databaseURL) == "" {
		return errors.New("public deployment requires DATABASE_URL")
	}
	parsed, err := url.Parse(c.issuer)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return errors.New("public deployment requires an HTTPS OIDC_ISSUER_URL")
	}
	if strings.TrimSpace(c.runtimeURL) == "" && strings.TrimSpace(c.runtimeRole) == "" {
		return errors.New("public deployment requires a least-privilege runtime database scope: set DB_RUNTIME_ROLE or RUNTIME_DATABASE_URL")
	}
	return nil
}

// envOr returns the environment variable named key, or def if it is unset/empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOr(key string, def int) int {
	value := os.Getenv(key)
	if value == "" {
		return def
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		log.Fatalf("%s must be a positive integer", key)
	}
	return parsed
}
