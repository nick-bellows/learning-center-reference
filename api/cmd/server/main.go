// Command server starts the Learning Center API.
//
// Startup order: connect to Postgres (when DATABASE_URL is set), apply
// embedded migrations (MIGRATE_ON_START=1), apply a seed file (SEED_FILE=
// path, local demo only), then serve. The server carries explicit timeouts
// and shuts down gracefully on SIGINT/SIGTERM.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nick-bellows/learning-center-reference/api/internal/authn"
	"github.com/nick-bellows/learning-center-reference/api/internal/dbsetup"
	"github.com/nick-bellows/learning-center-reference/api/internal/httpapi"
	"github.com/nick-bellows/learning-center-reference/api/internal/store"
	"github.com/nick-bellows/learning-center-reference/api/migrations"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	deps := httpapi.Deps{Logger: logger}
	if url := os.Getenv("DATABASE_URL"); url != "" {
		st, err := store.New(ctx, url)
		if err != nil {
			log.Fatalf("database: %v", err)
		}
		defer st.Close()

		if os.Getenv("MIGRATE_ON_START") == "1" {
			applied, err := dbsetup.Migrate(ctx, st.Pool(), migrations.Files)
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
			if err := dbsetup.ApplySeed(ctx, st.Pool(), string(sql)); err != nil {
				log.Fatalf("seed: %v", err)
			}
			log.Printf("seed applied from %s", seedPath)
		}

		deps.Eligibility = st
		deps.Identity = st
		deps.Learning = st
		deps.DB = st
	} else {
		log.Println("DATABASE_URL not set; database-backed routes will be unavailable")
	}

	verifier, err := configureAuth(ctx)
	if err != nil {
		log.Fatalf("authentication: %v", err)
	}
	deps.Auth = verifier

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
		log.Println("shutting down…")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			log.Printf("shutdown: %v", err)
		}
	}
}

func configureAuth(ctx context.Context) (authn.Verifier, error) {
	switch strings.ToLower(envOr("AUTH_MODE", "disabled")) {
	case "disabled":
		log.Println("AUTH_MODE=disabled; protected routes fail closed with 503")
		return authn.UnavailableVerifier{}, nil
	case "demo":
		log.Println("AUTH_MODE=demo; using synthetic local identities only")
		return authn.DemoVerifier{
			envOr("DEMO_LEARNER_TOKEN", "local-learner-token"): "demo|learner",
			envOr("DEMO_ADMIN_TOKEN", "local-admin-token"):     "demo|admin",
		}, nil
	case "oidc":
		return authn.NewOIDCVerifier(ctx, os.Getenv("OIDC_ISSUER_URL"), os.Getenv("OIDC_AUDIENCE"))
	default:
		return nil, fmt.Errorf("unsupported AUTH_MODE %q", os.Getenv("AUTH_MODE"))
	}
}

// envOr returns the environment variable named key, or def if it is unset/empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
