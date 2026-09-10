package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// blockingPinger holds /health open until released, so a test can fill the concurrency
// slots deterministically.
type blockingPinger struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingPinger() *blockingPinger {
	return &blockingPinger{entered: make(chan struct{}), release: make(chan struct{})}
}

func (p *blockingPinger) Ping(ctx context.Context) error {
	p.once.Do(func() { close(p.entered) })
	select {
	case <-p.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TestConcurrencyLimitRefusesWithJSON: with one slot taken, the next request is refused
// immediately with a JSON 429 and Retry-After, and the slot is returned once the first
// request finishes.
func TestConcurrencyLimitRefusesWithJSON(t *testing.T) {
	pinger := newBlockingPinger()
	handler := NewRouter(Deps{DB: pinger, MaxConcurrentRequests: 1})

	first := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/health", nil))
	}()
	<-pinger.entered

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/health", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d; want 429", second.Code)
	}
	if got := second.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("second request Content-Type = %q; want application/json", got)
	}
	if got := second.Header().Get("Retry-After"); got == "" {
		t.Fatal("second request has no Retry-After header")
	}
	var body map[string]string
	if err := json.NewDecoder(second.Body).Decode(&body); err != nil || body["error"] == "" {
		t.Fatalf("second request body = %q, err = %v; want {\"error\": ...}", second.Body.String(), err)
	}

	close(pinger.release)
	<-done
	if first.Code != http.StatusOK {
		t.Fatalf("first request status = %d; want 200 once released", first.Code)
	}

	third := httptest.NewRecorder()
	handler.ServeHTTP(third, httptest.NewRequest(http.MethodGet, "/health", nil))
	if third.Code != http.StatusOK {
		t.Fatalf("third request status = %d; want 200 after the slot was released", third.Code)
	}
}
