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

	// One structured JSON log stream for startup, requests, and shutdown alike.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	fatal := func(msg string, err error) {
		logger.Error(msg, "error", err)
		os.Exit(1)
	}
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
		fatal("configuration", err)
	}
	// RATE_LIMIT_PER_MINUTE=0 disables the per-client limit; MAX_CONCURRENT_REQUESTS=0
	// selects the router's default.
	rateLimit, err := envIntOr("RATE_LIMIT_PER_MINUTE", 120)
	if err != nil {
		fatal("configuration", err)
	}
	maxConcurrent, err := envIntOr("MAX_CONCURRENT_REQUESTS", 0)
	if err != nil {
		fatal("configuration", err)
	}
	deps := httpapi.Deps{
		Logger:                logger,
		RateLimitPerMinute:    rateLimit,
		TrustProxy:            os.Getenv("TRUST_PROXY") == "1",
		MaxConcurrentRequests: maxConcurrent,
	}
	if databaseURL != "" {
		// Setup pool: the owner applies migrations and the seed, then is not used for requests.
		setup, err := store.New(ctx, databaseURL)
		if err != nil {
			fatal("database", err)
		}

		if os.Getenv("MIGRATE_ON_START") == "1" {
			applied, err := dbsetup.Migrate(ctx, setup.Pool(), migrations.Files)
			if err != nil {
				fatal("migrate", err)
			}
			logger.Info("migrations applied", "count", len(applied))
		}
		if seedPath := os.Getenv("SEED_FILE"); seedPath != "" {
			sql, err := os.ReadFile(seedPath)
			if err != nil {
				fatal("seed", err)
			}
			if err := dbsetup.ApplySeed(ctx, setup.Pool(), string(sql)); err != nil {
				fatal("seed", err)
			}
			logger.Info("seed applied", "path", seedPath)
		}

		// Runtime pool: request handling runs with the restricted role's privileges when
		// either knob is set; otherwise it is the owner pool (local default without them).
		// Once the runtime pool exists the owner pool has no further job, so it is closed
		// rather than left idle with schema-changing privileges for the process lifetime.
		st := setup
		if runtimeURL != "" || runtimeRole != "" {
			st, err = store.NewWithRole(ctx, envOr("RUNTIME_DATABASE_URL", databaseURL), runtimeRole)
			if err != nil {
				fatal("runtime database", err)
			}
			setup.Close()
		}
		defer st.Close()
		role, err := st.CurrentRole(ctx)
		if err != nil {
			fatal("runtime database", err)
		}
		logger.Info("request handling database role", "role", role)

		deps.Eligibility = st
		deps.Identity = st
		deps.Learning = st
		deps.Credentials = st
		deps.DB = st
	} else {
		logger.Warn("DATABASE_URL not set; database-backed routes will be unavailable")
	}

	verifier, err := configureAuth(ctx, logger, authMode)
	if err != nil {
		fatal("authentication", err)
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
		logger.Info("Learning Center API listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		fatal("serve", err)
	case <-ctx.Done():
		// Graceful shutdown: stop accepting, let in-flight requests finish.
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			logger.Error("shutdown", "error", err)
		}
	}
}

// verifier is what every auth mode provides: subject verification for person routes and
// claims verification for service routes, from one trust configuration.
type verifier interface {
	authn.Verifier
	authn.ClaimsVerifier
}

func configureAuth(ctx context.Context, logger *slog.Logger, authMode string) (verifier, error) {
	switch authMode {
	case "disabled":
		logger.Warn("AUTH_MODE=disabled; protected routes fail closed with 503")
		return authn.UnavailableVerifier{}, nil
	case "demo":
		logger.Warn("AUTH_MODE=demo; using synthetic local identities only")
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

// envIntOr reads a non-negative integer setting; zero is meaningful to the callers (it
// disables the rate limit or selects the default concurrency bound).
func envIntOr(key string, def int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return def, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer, got %q", key, value)
	}
	return parsed, nil
}
