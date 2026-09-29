package model

import (
	"testing"
)

func TestEventInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   EventInput
		wantErr bool
	}{
		{
			name:    "valid event",
			input:   EventInput{UserID: "usr_1", EventType: "page_view", Page: "/home", Device: "mobile", Country: "ID"},
			wantErr: false,
		},
		{
			name:    "missing user_id",
			input:   EventInput{EventType: "page_view"},
			wantErr: true,
		},
		{
			name:    "missing event_type",
			input:   EventInput{UserID: "usr_1"},
			wantErr: true,
		},
		{
			name:    "invalid event_type",
			input:   EventInput{UserID: "usr_1", EventType: "invalid_type"},
			wantErr: true,
		},
		{
			name:    "all valid event types - click",
			input:   EventInput{UserID: "usr_1", EventType: "click"},
			wantErr: false,
		},
		{
			name:    "all valid event types - purchase",
			input:   EventInput{UserID: "usr_1", EventType: "purchase"},
			wantErr: false,
		},
		{
			name:    "optional fields empty is ok",
			input:   EventInput{UserID: "usr_1", EventType: "login"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEventInput_ToEvent(t *testing.T) {
	input := EventInput{
		UserID:    "usr_123",
		EventType: "page_view",
		Page:      "/pricing",
		Device:    "desktop",
		Country:   "US",
	}

	event := input.ToEvent()

	if event.ID == "" {
		t.Error("expected generated ID, got empty")
	}
	if event.UserID != input.UserID {
		t.Errorf("expected UserID %s, got %s", input.UserID, event.UserID)
	}
	if event.EventType != input.EventType {
		t.Errorf("expected EventType %s, got %s", input.EventType, event.EventType)
	}
	if event.Timestamp.IsZero() {
		t.Error("expected non-zero Timestamp")
	}
	if event.IngestedAt.IsZero() {
		t.Error("expected non-zero IngestedAt")
	}
}
