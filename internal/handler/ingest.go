package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/radityajayantara/go-event-ingester/internal/metrics"
	"github.com/radityajayantara/go-event-ingester/internal/model"
	redispkg "github.com/radityajayantara/go-event-ingester/internal/redis"
)

// IngestHandler handles event ingestion requests.
type IngestHandler struct {
	producer *redispkg.Producer
	metrics  *metrics.Collector
}

// NewIngestHandler creates a new ingestion handler.
func NewIngestHandler(producer *redispkg.Producer, m *metrics.Collector) *IngestHandler {
	return &IngestHandler{
		producer: producer,
		metrics:  m,
	}
}

// HandleSingle handles POST /api/v1/events — single event ingestion.
func (h *IngestHandler) HandleSingle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var input model.EventInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if err := input.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	event := input.ToEvent()

	if err := h.producer.Publish(r.Context(), event); err != nil {
		slog.Error("failed to publish event", "error", err)
		writeError(w, http.StatusServiceUnavailable, "failed to queue event")
		return
	}

	h.metrics.RecordIngested(1)
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "accepted",
		"id":     event.ID,
	})
}

// HandleBatch handles POST /api/v1/events/batch — batch event ingestion.
func (h *IngestHandler) HandleBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var inputs []model.EventInput
	if err := json.NewDecoder(r.Body).Decode(&inputs); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if len(inputs) == 0 {
		writeError(w, http.StatusBadRequest, "empty batch")
		return
	}

	if len(inputs) > 1000 {
		writeError(w, http.StatusRequestEntityTooLarge, "batch size exceeds maximum of 1000")
		return
	}

	// Validate all before publishing any.
	events := make([]model.Event, 0, len(inputs))
	for i, input := range inputs {
		if err := input.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, map[string]interface{}{
				"error": err.Error(),
				"index": i,
			})
			return
		}
		events = append(events, input.ToEvent())
	}

	if err := h.producer.PublishBatch(r.Context(), events); err != nil {
		slog.Error("failed to publish batch", "error", err, "count", len(events))
		writeError(w, http.StatusServiceUnavailable, "failed to queue events")
		return
	}

	h.metrics.RecordIngested(len(events))

	ids := make([]string, len(events))
	for i, e := range events {
		ids[i] = e.ID
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"status": "accepted",
		"count":  len(events),
		"ids":    ids,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message interface{}) {
	writeJSON(w, status, map[string]interface{}{
		"error": message,
	})
}
