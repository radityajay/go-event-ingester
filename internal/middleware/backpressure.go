package middleware

import (
	"net/http"
	"sync/atomic"
)

// Backpressure rejects new requests when the system is overloaded.
// It tracks in-flight requests and compares against a max concurrency limit.
type Backpressure struct {
	inFlight atomic.Int64
	maxInFlight int64
}

// NewBackpressure creates a backpressure middleware with the given concurrency limit.
func NewBackpressure(maxInFlight int64) *Backpressure {
	return &Backpressure{
		maxInFlight: maxInFlight,
	}
}

// Wrap returns an HTTP middleware that enforces backpressure.
// When the limit is reached, it responds with 503 Service Unavailable.
func (bp *Backpressure) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		current := bp.inFlight.Add(1)
		defer bp.inFlight.Add(-1)

		if current > bp.maxInFlight {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"server overloaded, try again later"}`))
			return
		}

		next(w, r)
	}
}

// InFlight returns the current number of in-flight requests.
func (bp *Backpressure) InFlight() int64 {
	return bp.inFlight.Load()
}
