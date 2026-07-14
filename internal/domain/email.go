package domain

import (
	"time"

	"github.com/google/uuid"
)

// EmailStatus represents the delivery status of an email.
type EmailStatus string

const (
	StatusPending   EmailStatus = "pending"
	StatusQueued    EmailStatus = "queued"
	StatusSending   EmailStatus = "sending"
	StatusSent      EmailStatus = "sent"
	StatusFailed    EmailStatus = "failed"
	StatusBounced   EmailStatus = "bounced"
	StatusComplaint EmailStatus = "complaint"
)

// Email represents an email to be sent.
type Email struct {
	ID            uuid.UUID              `json:"id" db:"id"`
	To            string                 `json:"to" db:"to_address"`
	From          string                 `json:"from" db:"from_address"`
	Subject       string                 `json:"subject" db:"subject"`
	Body          string                 `json:"body" db:"body"`
	HTMLBody      string                 `json:"html_body,omitempty" db:"html_body"`
	Template      string                 `json:"template,omitempty" db:"template"`
	TemplateData  map[string]interface{} `json:"template_data,omitempty" db:"-"`
	Status        EmailStatus            `json:"status" db:"status"`
	ErrorMessage  string                 `json:"error_message,omitempty" db:"error_message"`
	RetryCount    int                    `json:"retry_count" db:"retry_count"`
	MaxRetries    int                    `json:"max_retries" db:"max_retries"`
	IdempotencyID string                 `json:"idempotency_id,omitempty" db:"idempotency_id"`
	SourceService string                 `json:"source_service,omitempty" db:"source_service"`
	Metadata      map[string]interface{} `json:"metadata,omitempty" db:"-"`
	Headers       map[string]string      `json:"headers,omitempty" db:"-"` // populated from DB via emailRow
	ResendEmailID string                 `json:"resend_email_id,omitempty" db:"resend_email_id"`
	SentAt        *time.Time             `json:"sent_at,omitempty" db:"sent_at"`
	CreatedAt     time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at" db:"updated_at"`
}

// SendEmailRequest represents a request to send an email.
type SendEmailRequest struct {
	To            string                 `json:"to" binding:"required,email"`
	From          string                 `json:"from,omitempty"` // optional sender override ("Name <addr@domain>"); falls back to configured default
	Subject       string                 `json:"subject,omitempty"`
	Body          string                 `json:"body,omitempty"`
	HTMLBody      string                 `json:"html_body,omitempty"`
	Template      string                 `json:"template,omitempty"`
	TemplateData  map[string]interface{} `json:"template_data,omitempty"`
	IdempotencyID string                 `json:"idempotency_id,omitempty"`
	SourceService string                 `json:"source_service,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	ProductID     string                 `json:"product_id,omitempty"`
	Headers       map[string]string      `json:"headers,omitempty"`
	Branding      *BrandingConfig        `json:"branding,omitempty"`
}

// SendBulkRequest represents a request to send emails to multiple recipients.
type SendBulkRequest struct {
	Recipients    []string               `json:"recipients" binding:"required,min=1,dive,email"`
	Subject       string                 `json:"subject,omitempty"`
	Body          string                 `json:"body,omitempty"`
	HTMLBody      string                 `json:"html_body,omitempty"`
	Template      string                 `json:"template,omitempty"`
	TemplateData  map[string]interface{} `json:"template_data,omitempty"`
	SourceService string                 `json:"source_service,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	ProductID     string                 `json:"product_id,omitempty"`
	Branding      *BrandingConfig        `json:"branding,omitempty"`
}

// RecipientWithData represents a recipient with per-recipient template data overrides.
type RecipientWithData struct {
	Email        string                 `json:"email" binding:"required,email"`
	TemplateData map[string]interface{} `json:"template_data,omitempty"`
}

// SendBulkPersonalizedRequest represents a request to send personalized emails to multiple recipients.
// Each recipient can have their own template data that overrides the base template data.
type SendBulkPersonalizedRequest struct {
	Recipients    []RecipientWithData    `json:"recipients" binding:"required,min=1"`
	Subject       string                 `json:"subject,omitempty"`
	Body          string                 `json:"body,omitempty"`
	HTMLBody      string                 `json:"html_body,omitempty"`
	Template      string                 `json:"template,omitempty"`
	TemplateData  map[string]interface{} `json:"template_data,omitempty"`
	SourceService string                 `json:"source_service,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	ProductID     string                 `json:"product_id,omitempty"`
	Branding      *BrandingConfig        `json:"branding,omitempty"`
}

// SendEmailResponse represents the response after queuing an email.
type SendEmailResponse struct {
	ID      uuid.UUID   `json:"id"`
	Status  EmailStatus `json:"status"`
	Message string      `json:"message"`
}

// SendBulkResponse represents the response after queuing bulk emails.
type SendBulkResponse struct {
	TotalQueued int         `json:"total_queued"`
	EmailIDs    []uuid.UUID `json:"email_ids"`
	Message     string      `json:"message"`
}

// EmailStatusResponse represents the status check response.
type EmailStatusResponse struct {
	ID           uuid.UUID   `json:"id"`
	To           string      `json:"to"`
	Subject      string      `json:"subject"`
	Status       EmailStatus `json:"status"`
	ErrorMessage string      `json:"error_message,omitempty"`
	RetryCount   int         `json:"retry_count"`
	SentAt       *time.Time  `json:"sent_at,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
}
