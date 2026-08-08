// Command server starts the Learning Center API.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/nick-bellows/learning-center-reference/api/internal/httpapi"
	"github.com/nick-bellows/learning-center-reference/api/internal/store"
)

func main() {
	ctx := context.Background()

	var deps httpapi.Deps
	if url := os.Getenv("DATABASE_URL"); url != "" {
		st, err := store.New(ctx, url)
		if err != nil {
			log.Fatalf("database: %v", err)
		}
		defer st.Close()
		deps.Eligibility = st
	} else {
		log.Println("DATABASE_URL not set; /v1/members/{id}/eligibility will be unavailable")
	}

	addr := ":" + envOr("PORT", "8080")
	log.Printf("Learning Center API listening on %s", addr)
	if err := http.ListenAndServe(addr, httpapi.NewRouter(deps)); err != nil {
		log.Fatal(err)
	}
}

// envOr returns the environment variable named key, or def if it is unset/empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
