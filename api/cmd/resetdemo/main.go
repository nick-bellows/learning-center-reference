// Command resetdemo clears mutable enrollment/progress state for the fictional demo.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/nick-bellows/learning-center-reference/api/internal/demoreset"
	"github.com/nick-bellows/learning-center-reference/api/internal/store"
)

func main() {
	if os.Getenv("RESET_CONFIRM") != "synthetic-demo" {
		log.Fatal("refusing reset: set RESET_CONFIRM=synthetic-demo")
	}
	if os.Getenv("DATABASE_URL") == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	st, err := store.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer st.Close()
	count, err := demoreset.Reset(ctx, st.Pool())
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("synthetic demo reset complete: enrollments_deleted=%d", count)
}
