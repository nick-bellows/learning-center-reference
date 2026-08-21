// Package httpapi builds the HTTP router for the API. It's named "httpapi" (not "http")
// so it doesn't collide with the standard library's net/http package.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

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

// Pinger is the health check's view of the database.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps holds everything the router needs handed in from outside.
type Deps struct {
	Eligibility EligibilityLoader
	DB          Pinger // nil when the server runs without a database
}

// uuidRe validates the {id} path parameter BEFORE it reaches the database, so
// a malformed id is a client error (400), not a database cast error (500).
var uuidRe = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// NewRouter wires middleware + routes and returns an http.Handler.
func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", deps.handleHealth)
	r.Get("/v1/members/{id}/eligibility", deps.handleEligibility)

	return r
}

// handleHealth reports readiness, not just liveness: when a database is
// configured, it must answer a ping or the endpoint returns 503.
func (deps Deps) handleHealth(w http.ResponseWriter, r *http.Request) {
	if deps.DB == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "not configured"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := deps.DB.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable,
			map[string]string{"status": "degraded", "database": "unreachable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "ok"})
}

// handleEligibility loads a member's safeguarding facts, computes eligibility, and returns it.
// Note the handler doesn't know the RULE — it just wires the store to the pure Evaluate
// function. That separation is what makes the rule easy to test on its own.
func (deps Deps) handleEligibility(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id") // the {id} from the route
	if !uuidRe.MatchString(id) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid member id"})
		return
	}

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
