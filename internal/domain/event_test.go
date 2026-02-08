package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEmailEvent_Fields(t *testing.T) {
	id := uuid.New()
	emailID := uuid.New()
	now := time.Now()

	event := EmailEvent{
		ID:            id,
		EmailID:       emailID,
		EventType:     "email.delivered",
		ResendEventID: "evt_123",
		Payload:       map[string]interface{}{"key": "value"},
		CreatedAt:     now,
	}

	if event.ID != id {
		t.Errorf("expected ID %v, got %v", id, event.ID)
	}
	if event.EventType != "email.delivered" {
		t.Errorf("expected event type 'email.delivered', got %q", event.EventType)
	}
	if event.ResendEventID != "evt_123" {
		t.Errorf("expected resend event ID 'evt_123', got %q", event.ResendEventID)
	}
}

func TestCampaignStats_Fields(t *testing.T) {
	stats := CampaignStats{
		CampaignTag: "migration-v1",
		TotalSent:   1000,
		Delivered:   950,
		Opened:      400,
		Clicked:     100,
		Bounced:     30,
		Complained:  2,
	}

	if stats.CampaignTag != "migration-v1" {
		t.Errorf("expected campaign tag 'migration-v1', got %q", stats.CampaignTag)
	}
	if stats.TotalSent != 1000 {
		t.Errorf("expected 1000 total sent, got %d", stats.TotalSent)
	}
	// Open rate should be 40%
	openRate := float64(stats.Opened) / float64(stats.TotalSent) * 100
	if openRate != 40.0 {
		t.Errorf("expected 40%% open rate, got %.1f%%", openRate)
	}
}

func TestEmailStatus_Constants(t *testing.T) {
	// Verify all expected status values exist
	statuses := []EmailStatus{
		StatusPending,
		StatusQueued,
		StatusSending,
		StatusSent,
		StatusFailed,
		StatusBounced,
		StatusComplaint,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("found empty status constant")
		}
	}
}
