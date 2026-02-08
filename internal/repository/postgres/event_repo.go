package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/prodigy90/email-service-go/internal/domain"
)

// EventRepository handles email event persistence.
type EventRepository struct {
	db *sqlx.DB
}

// NewEventRepository creates a new event repository.
func NewEventRepository(db *sqlx.DB) *EventRepository {
	return &EventRepository{db: db}
}

// Create inserts a new email event, silently skipping duplicates via resend_event_id.
func (r *EventRepository) Create(ctx context.Context, event *domain.EmailEvent) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		// Fall back to empty object but still insert the event record
		payload = []byte("{}")
	}


	query := `
		INSERT INTO email_events (id, email_id, event_type, resend_event_id, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (resend_event_id) DO NOTHING`

	_, err = r.db.ExecContext(ctx, query,
		event.ID,
		event.EmailID,
		event.EventType,
		nullString(event.ResendEventID),
		payload,
		event.CreatedAt,
	)
	return err
}

// eventRow is the database representation of an email event.
type eventRow struct {
	ID            uuid.UUID      `db:"id"`
	EmailID       uuid.UUID      `db:"email_id"`
	EventType     string         `db:"event_type"`
	ResendEventID sql.NullString `db:"resend_event_id"`
	Payload       []byte         `db:"payload"`
	CreatedAt     time.Time      `db:"created_at"`
}

func (r *eventRow) toDomain() *domain.EmailEvent {
	event := &domain.EmailEvent{
		ID:        r.ID,
		EmailID:   r.EmailID,
		EventType: r.EventType,
		CreatedAt: r.CreatedAt,
	}
	if r.ResendEventID.Valid {
		event.ResendEventID = r.ResendEventID.String
	}
	if len(r.Payload) > 0 {
		_ = json.Unmarshal(r.Payload, &event.Payload)
	}
	return event
}
