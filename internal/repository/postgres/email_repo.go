package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/prodigy90/email-service-go/internal/domain"
)

// EmailRepository handles email persistence.
type EmailRepository struct {
	db *sqlx.DB
}

// NewEmailRepository creates a new email repository.
func NewEmailRepository(db *sqlx.DB) *EmailRepository {
	return &EmailRepository{db: db}
}

// Create inserts a new email record.
func (r *EmailRepository) Create(ctx context.Context, email *domain.Email) error {
	metadata, _ := json.Marshal(email.Metadata)
	templateData, _ := json.Marshal(email.TemplateData)

	query := `
		INSERT INTO emails (
			id, to_address, from_address, subject, body, html_body,
			template, template_data, status, max_retries, idempotency_id,
			source_service, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)`

	_, err := r.db.ExecContext(ctx, query,
		email.ID,
		email.To,
		email.From,
		email.Subject,
		email.Body,
		email.HTMLBody,
		email.Template,
		templateData,
		email.Status,
		email.MaxRetries,
		nullString(email.IdempotencyID),
		nullString(email.SourceService),
		metadata,
		email.CreatedAt,
		email.UpdatedAt,
	)
	return err
}

// CreateOrGetByIdempotencyID atomically creates an email or returns existing one if idempotency_id matches.
// This prevents race conditions by using INSERT ON CONFLICT DO NOTHING.
// Returns (email, true) if existing was found, (email, false) if new was created.
func (r *EmailRepository) CreateOrGetByIdempotencyID(ctx context.Context, email *domain.Email) (*domain.Email, bool, error) {
	if email.IdempotencyID == "" {
		// No idempotency ID - just create normally
		if err := r.Create(ctx, email); err != nil {
			return nil, false, err
		}
		return email, false, nil
	}

	metadata, _ := json.Marshal(email.Metadata)
	templateData, _ := json.Marshal(email.TemplateData)

	// Use INSERT ON CONFLICT DO NOTHING to atomically try insertion
	// If conflict on idempotency_id, nothing is inserted
	query := `
		INSERT INTO emails (
			id, to_address, from_address, subject, body, html_body,
			template, template_data, status, max_retries, idempotency_id,
			source_service, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
		ON CONFLICT (idempotency_id) DO NOTHING`

	result, err := r.db.ExecContext(ctx, query,
		email.ID,
		email.To,
		email.From,
		email.Subject,
		email.Body,
		email.HTMLBody,
		email.Template,
		templateData,
		email.Status,
		email.MaxRetries,
		nullString(email.IdempotencyID),
		nullString(email.SourceService),
		metadata,
		email.CreatedAt,
		email.UpdatedAt,
	)
	if err != nil {
		return nil, false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, false, err
	}

	if rowsAffected == 0 {
		// Conflict occurred - fetch the existing record
		existing, err := r.GetByIdempotencyID(ctx, email.IdempotencyID)
		if err != nil {
			return nil, false, err
		}
		if existing == nil {
			// Edge case: conflict row was deleted between INSERT and SELECT
			return nil, false, fmt.Errorf("idempotency conflict but record not found")
		}
		return existing, true, nil
	}

	// New record was inserted
	return email, false, nil
}

// GetByID retrieves an email by ID.
func (r *EmailRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Email, error) {
	var email emailRow
	query := `SELECT * FROM emails WHERE id = $1`
	if err := r.db.GetContext(ctx, &email, query, id); err != nil {
		return nil, err
	}
	return email.toDomain(), nil
}

// GetByIdempotencyID retrieves an email by idempotency ID.
func (r *EmailRepository) GetByIdempotencyID(ctx context.Context, idempotencyID string) (*domain.Email, error) {
	var email emailRow
	query := `SELECT * FROM emails WHERE idempotency_id = $1`
	if err := r.db.GetContext(ctx, &email, query, idempotencyID); err != nil {
		return nil, err
	}
	return email.toDomain(), nil
}

// UpdateStatus updates the email status.
func (r *EmailRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.EmailStatus, errorMsg string) error {
	query := `UPDATE emails SET status = $1, error_message = $2, updated_at = $3 WHERE id = $4`
	_, err := r.db.ExecContext(ctx, query, status, errorMsg, time.Now(), id)
	return err
}

// IncrementRetry increments the retry count.
func (r *EmailRepository) IncrementRetry(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE emails SET retry_count = retry_count + 1, updated_at = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

// MarkSent marks an email as sent.
func (r *EmailRepository) MarkSent(ctx context.Context, id uuid.UUID, sentAt time.Time) error {
	query := `UPDATE emails SET status = $1, sent_at = $2, updated_at = $3 WHERE id = $4`
	_, err := r.db.ExecContext(ctx, query, domain.StatusSent, sentAt, time.Now(), id)
	return err
}

// ListByStatus retrieves emails by status with pagination.
func (r *EmailRepository) ListByStatus(ctx context.Context, status domain.EmailStatus, limit, offset int) ([]*domain.Email, error) {
	var rows []emailRow
	query := `SELECT * FROM emails WHERE status = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	if err := r.db.SelectContext(ctx, &rows, query, status, limit, offset); err != nil {
		return nil, err
	}

	emails := make([]*domain.Email, len(rows))
	for i, row := range rows {
		emails[i] = row.toDomain()
	}
	return emails, nil
}

// emailRow is the database representation.
type emailRow struct {
	ID            uuid.UUID       `db:"id"`
	To            string          `db:"to_address"`
	From          sql.NullString  `db:"from_address"`
	Subject       string          `db:"subject"`
	Body          string          `db:"body"`
	HTMLBody      sql.NullString  `db:"html_body"`
	Template      sql.NullString  `db:"template"`
	TemplateData  []byte          `db:"template_data"`
	Status        domain.EmailStatus `db:"status"`
	ErrorMessage  sql.NullString  `db:"error_message"`
	RetryCount    int             `db:"retry_count"`
	MaxRetries    int             `db:"max_retries"`
	IdempotencyID sql.NullString  `db:"idempotency_id"`
	SourceService sql.NullString  `db:"source_service"`
	Metadata      []byte          `db:"metadata"`
	SentAt        sql.NullTime    `db:"sent_at"`
	CreatedAt     time.Time       `db:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at"`
}

func (r *emailRow) toDomain() *domain.Email {
	email := &domain.Email{
		ID:         r.ID,
		To:         r.To,
		Subject:    r.Subject,
		Body:       r.Body,
		Status:     r.Status,
		RetryCount: r.RetryCount,
		MaxRetries: r.MaxRetries,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}

	if r.From.Valid {
		email.From = r.From.String
	}
	if r.HTMLBody.Valid {
		email.HTMLBody = r.HTMLBody.String
	}
	if r.Template.Valid {
		email.Template = r.Template.String
	}
	if r.ErrorMessage.Valid {
		email.ErrorMessage = r.ErrorMessage.String
	}
	if r.IdempotencyID.Valid {
		email.IdempotencyID = r.IdempotencyID.String
	}
	if r.SourceService.Valid {
		email.SourceService = r.SourceService.String
	}
	if r.SentAt.Valid {
		email.SentAt = &r.SentAt.Time
	}

	if len(r.TemplateData) > 0 {
		_ = json.Unmarshal(r.TemplateData, &email.TemplateData)
	}
	if len(r.Metadata) > 0 {
		_ = json.Unmarshal(r.Metadata, &email.Metadata)
	}

	return email
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
