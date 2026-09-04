package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateWindow struct {
	started time.Time
	count   int
}

type clientRateLimiter struct {
	mu         sync.Mutex
	visitors   map[string]rateWindow
	limit      int
	window     time.Duration
	trustProxy bool
	now        func() time.Time
	lastSweep  time.Time
}

func newClientRateLimiter(limit int, window time.Duration, trustProxy bool) *clientRateLimiter {
	return &clientRateLimiter{
		visitors:   make(map[string]rateWindow),
		limit:      limit,
		window:     window,
		trustProxy: trustProxy,
		now:        time.Now,
	}
}

func (l *clientRateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client := clientAddress(r)
		if l.trustProxy {
			client = proxiedClientAddress(r)
		}
		if !l.allow(client) {
			w.Header().Set("Retry-After", "60")
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "request rate exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *clientRateLimiter) allow(client string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if l.lastSweep.IsZero() || now.Sub(l.lastSweep) >= l.window {
		for key, value := range l.visitors {
			if now.Sub(value.started) >= l.window {
				delete(l.visitors, key)
			}
		}
		l.lastSweep = now
	}
	entry, found := l.visitors[client]
	if !found || now.Sub(entry.started) >= l.window {
		l.visitors[client] = rateWindow{started: now, count: 1}
		return true
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	l.visitors[client] = entry
	return true
}

func clientAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// proxiedClientAddress reads the rightmost X-Forwarded-For entry. Clients can prepend
// arbitrary values, but a single trusted proxy in front of the service appends the address
// it actually observed, so the last entry is the spoof-resistant one. Enable this only when
// exactly one trusted proxy sits ahead of the service (TRUST_PROXY=1).
func proxiedClientAddress(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	entries := strings.Split(forwarded, ",")
	for i := len(entries) - 1; i >= 0; i-- {
		if parsed := net.ParseIP(strings.TrimSpace(entries[i])); parsed != nil {
			return parsed.String()
		}
	}
	return clientAddress(r)
}
