package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Event represents a user activity event received by the ingestion API.
type Event struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	EventType string    `json:"event_type"`
	Page      string    `json:"page"`
	Device    string    `json:"device"`
	Country   string    `json:"country"`
	Timestamp time.Time `json:"timestamp"`
	IngestedAt time.Time `json:"ingested_at"`
}

// EventInput represents the raw payload from the client.
type EventInput struct {
	UserID    string `json:"user_id"`
	EventType string `json:"event_type"`
	Page      string `json:"page"`
	Device    string `json:"device"`
	Country   string `json:"country"`
}

// Validate checks required fields and returns an error if invalid.
func (e *EventInput) Validate() error {
	if e.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if e.EventType == "" {
		return fmt.Errorf("event_type is required")
	}
	if !isValidEventType(e.EventType) {
		return fmt.Errorf("invalid event_type: %s", e.EventType)
	}
	return nil
}

// ToEvent enriches the input into a full Event with generated ID and timestamps.
func (e *EventInput) ToEvent() Event {
	now := time.Now().UTC()
	return Event{
		ID:         uuid.New().String(),
		UserID:     e.UserID,
		EventType:  e.EventType,
		Page:       e.Page,
		Device:     e.Device,
		Country:    e.Country,
		Timestamp:  now,
		IngestedAt: now,
	}
}

var validEventTypes = map[string]bool{
	"page_view":   true,
	"click":       true,
	"scroll":      true,
	"purchase":    true,
	"signup":      true,
	"login":       true,
	"logout":      true,
	"search":      true,
	"add_to_cart": true,
	"checkout":    true,
}

func isValidEventType(t string) bool {
	return validEventTypes[t]
}
