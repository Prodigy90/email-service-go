package domain

import (
	"time"

	"github.com/google/uuid"
)

// EmailEvent represents a webhook event from Resend (delivered, opened, clicked, etc.).
type EmailEvent struct {
	ID            uuid.UUID              `json:"id" db:"id"`
	EmailID       uuid.UUID              `json:"email_id" db:"email_id"`
	EventType     string                 `json:"event_type" db:"event_type"`
	ResendEventID string                 `json:"resend_event_id,omitempty" db:"resend_event_id"`
	Payload       map[string]interface{} `json:"payload,omitempty" db:"-"`
	CreatedAt     time.Time              `json:"created_at" db:"created_at"`
}

// CampaignStats holds aggregate stats for a campaign tag.
type CampaignStats struct {
	CampaignTag string `json:"campaign_tag"`
	TotalSent   int    `json:"total_sent"`
	Delivered   int    `json:"delivered"`
	Opened      int    `json:"opened"`
	Clicked     int    `json:"clicked"`
	Bounced     int    `json:"bounced"`
	Complained  int    `json:"complained"`
}
