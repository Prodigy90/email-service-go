package service

import "github.com/prodigy90/email-service-go/internal/domain"

// EmailSender is the interface for sending emails.
// Both SMTPClient and ResendClient implement this interface.
type EmailSender interface {
	Send(email *domain.Email) error
}
