package service

import "github.com/prodigy90/email-service-go/internal/domain"

// SendResult holds the result of sending an email.
type SendResult struct {
	ProviderID string // Resend email ID, empty for SMTP
}

// EmailSender is the interface for sending emails.
// Both SMTPClient and ResendClient implement this interface.
type EmailSender interface {
	Send(email *domain.Email) (*SendResult, error)
}
