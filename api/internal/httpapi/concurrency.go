package httpapi

import "net/http"

// defaultMaxConcurrentRequests bounds in-flight requests so a burst cannot exhaust the
// database pool or memory. It replaces chi's middleware.Throttle, which answered with a
// text/plain body; every error this API emits shares the JSON {"error": ...} contract.
const defaultMaxConcurrentRequests = 64

// concurrencyLimit admits at most limit requests at once. A request arriving while every
// slot is taken is refused immediately with 429 and Retry-After rather than queued, so a
// saturated service degrades predictably instead of building a backlog.
func concurrencyLimit(limit int) func(http.Handler) http.Handler {
	slots := make(chan struct{}, limit)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
				next.ServeHTTP(w, r)
			default:
				w.Header().Set("Retry-After", "1")
				writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many concurrent requests"})
			}
		})
	}
}
