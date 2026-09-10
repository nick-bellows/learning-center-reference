// Command reconcileprogress compares the enrollment_progress projection with the
// append-only progress_event log and, with --apply, rebuilds any drifted rows.
//
// Exit codes: 0 = no drift (or drift repaired); 3 = drift found and not applied;
// 1 = error. The dry run is the default so an operator sees what would change first.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/nick-bellows/learning-center-reference/api/internal/projection"
	"github.com/nick-bellows/learning-center-reference/api/internal/store"
)

const exitDrift = 3

func main() {
	apply := flag.Bool("apply", false, "rewrite drifted projections (default: report only)")
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	st, err := store.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer st.Close()

	report, err := projection.Reconcile(ctx, st.Pool(), *apply)
	if err != nil {
		log.Fatalf("reconcile: %v", err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		log.Fatalf("writing report: %v", err)
	}
	if len(report.Drifted) > 0 && !report.Applied {
		os.Exit(exitDrift)
	}
}
