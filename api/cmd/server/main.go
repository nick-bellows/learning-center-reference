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
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nick-bellows/learning-center-reference/api/internal/dbsetup"
	"github.com/nick-bellows/learning-center-reference/api/internal/httpapi"
	"github.com/nick-bellows/learning-center-reference/api/internal/store"
	"github.com/nick-bellows/learning-center-reference/api/migrations"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var deps httpapi.Deps
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
		deps.DB = st
	} else {
		log.Println("DATABASE_URL not set; /v1/members/{id}/eligibility will be unavailable")
	}

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

// envOr returns the environment variable named key, or def if it is unset/empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
