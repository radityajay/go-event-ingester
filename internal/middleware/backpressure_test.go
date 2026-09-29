package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestBackpressure_AllowsUnderLimit(t *testing.T) {
	bp := NewBackpressure(10)
	handler := bp.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestBackpressure_RejectsOverLimit(t *testing.T) {
	bp := NewBackpressure(1)

	// Block the first request so it holds the slot.
	blocker := make(chan struct{})
	handler := bp.Wrap(func(w http.ResponseWriter, r *http.Request) {
		<-blocker
		w.WriteHeader(http.StatusOK)
	})

	var wg sync.WaitGroup

	// First request: will block and hold the slot.
	wg.Add(1)
	go func() {
		defer wg.Done()
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)
	}()

	// Wait for first request to be in-flight.
	for bp.InFlight() < 1 {
	}

	// Second request should be rejected (slot is full).
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for over-limit request, got %d", rec.Code)
	}

	if rec.Header().Get("Retry-After") != "1" {
		t.Error("expected Retry-After header")
	}

	// Unblock first request and wait.
	close(blocker)
	wg.Wait()
}

func TestBackpressure_InFlightTracking(t *testing.T) {
	bp := NewBackpressure(100)

	if bp.InFlight() != 0 {
		t.Errorf("expected 0 in-flight initially, got %d", bp.InFlight())
	}

	done := make(chan struct{})
	handler := bp.Wrap(func(w http.ResponseWriter, r *http.Request) {
		<-done
	})

	go func() {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)
	}()

	// Wait for in-flight to increment.
	for bp.InFlight() < 1 {
	}

	if bp.InFlight() != 1 {
		t.Errorf("expected 1 in-flight, got %d", bp.InFlight())
	}

	close(done)

	// Wait for in-flight to decrement.
	for bp.InFlight() > 0 {
	}

	if bp.InFlight() != 0 {
		t.Errorf("expected 0 in-flight after done, got %d", bp.InFlight())
	}
}
