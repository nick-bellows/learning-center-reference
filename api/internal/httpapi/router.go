// Package httpapi builds the HTTP router for the API. It's named "httpapi" (not "http")
// so it doesn't collide with the standard library's net/http package.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/nick-bellows/learning-center-reference/api/internal/safeguarding"
	"github.com/nick-bellows/learning-center-reference/api/internal/store"
)

// EligibilityLoader is the slice of the store the eligibility handler actually needs.
// Depending on a small interface (not the concrete *store.Store) keeps the HTTP layer
// testable with a fake and decoupled from the database — this is dependency injection.
type EligibilityLoader interface {
	LoadSafeguardingInputs(ctx context.Context, memberID string) (safeguarding.Inputs, error)
}

// Deps holds everything the router needs handed in from outside.
type Deps struct {
	Eligibility EligibilityLoader
}

// NewRouter wires middleware + routes and returns an http.Handler.
func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", handleHealth)
	r.Get("/v1/members/{id}/eligibility", deps.handleEligibility)

	return r
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleEligibility loads a member's safeguarding facts, computes eligibility, and returns it.
// Note the handler doesn't know the RULE — it just wires the store to the pure Evaluate
// function. That separation is what makes the rule easy to test on its own.
func (deps Deps) handleEligibility(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id") // the {id} from the route

	in, err := deps.Eligibility.LoadSafeguardingInputs(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "member not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	d := safeguarding.Evaluate(in)
	writeJSON(w, http.StatusOK, map[string]any{
		"member_id": id,
		"status":    d.Status,
		"reason":    d.Reason,
	})
}

// writeJSON is a small helper we reuse for every JSON response. `any` is Go's alias for
// "any type" (like C#'s object), fine here because we hand it straight to the JSON encoder.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body) // the leading _ = deliberately ignores the error
}
