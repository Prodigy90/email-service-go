package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/prodigy90/email-service-go/internal/domain"
	"github.com/prodigy90/email-service-go/internal/repository/postgres"
	"github.com/rs/zerolog"
)

const (
	taskTypeSendEmail = "email:send"
	MaxRetries        = 3
)

// EmailService handles email operations.
type EmailService struct {
	repo      *postgres.EmailRepository
	sender    EmailSender
	templates *TemplateService
	queue     *asynq.Client
	logger    zerolog.Logger
}

// NewEmailService creates a new email service.
func NewEmailService(
	repo *postgres.EmailRepository,
	sender EmailSender,
	templates *TemplateService,
	queue *asynq.Client,
	logger zerolog.Logger,
) *EmailService {
	return &EmailService{
		repo:      repo,
		sender:    sender,
		templates: templates,
		queue:     queue,
		logger:    logger.With().Str("component", "email_service").Logger(),
	}
}

// Send queues an email for delivery.
func (s *EmailService) Send(ctx context.Context, req *domain.SendEmailRequest) (*domain.SendEmailResponse, error) {
	// Check idempotency if ID provided
	if req.IdempotencyID != "" {
		existing, err := s.repo.GetByIdempotencyID(ctx, req.IdempotencyID)
		if err == nil && existing != nil {
			s.logger.Debug().Str("idempotency_id", req.IdempotencyID).Msg("Duplicate request, returning existing")
			return &domain.SendEmailResponse{
				ID:      existing.ID,
				Status:  existing.Status,
				Message: "Email already queued (idempotent)",
			}, nil
		}
	}

	// Build email
	email := &domain.Email{
		ID:            uuid.New(),
		To:            req.To,
		Subject:       req.Subject,
		Body:          req.Body,
		HTMLBody:      req.HTMLBody,
		Template:      req.Template,
		TemplateData:  req.TemplateData,
		Status:        domain.StatusPending,
		MaxRetries:    MaxRetries,
		IdempotencyID: req.IdempotencyID,
		SourceService: req.SourceService,
		Metadata:      req.Metadata,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// If template specified, render it
	if req.Template != "" {
		subject, body, htmlBody, err := s.templates.RenderWithBranding(req.Template, req.TemplateData, req.Branding)
		if err != nil {
			return nil, fmt.Errorf("template render failed: %w", err)
		}
		email.Subject = subject
		email.Body = body
		email.HTMLBody = htmlBody
	}

	// Validate
	if email.Subject == "" {
		return nil, fmt.Errorf("subject is required")
	}
	if email.Body == "" && email.HTMLBody == "" {
		return nil, fmt.Errorf("body or html_body is required")
	}

	// Save to database
	if err := s.repo.Create(ctx, email); err != nil {
		return nil, fmt.Errorf("failed to save email: %w", err)
	}

	// Queue for sending
	if err := s.enqueue(email); err != nil {
		s.logger.Error().Err(err).Str("email_id", email.ID.String()).Msg("Failed to enqueue email")
		// Update status to failed
		_ = s.repo.UpdateStatus(ctx, email.ID, domain.StatusFailed, "Failed to enqueue")
		return nil, fmt.Errorf("failed to queue email: %w", err)
	}

	// Update status to queued
	_ = s.repo.UpdateStatus(ctx, email.ID, domain.StatusQueued, "")

	s.logger.Info().
		Str("email_id", email.ID.String()).
		Str("to", email.To).
		Str("template", email.Template).
		Msg("Email queued for delivery")

	return &domain.SendEmailResponse{
		ID:      email.ID,
		Status:  domain.StatusQueued,
		Message: "Email queued for delivery",
	}, nil
}

// SendBulk queues multiple emails for delivery.
func (s *EmailService) SendBulk(ctx context.Context, req *domain.SendBulkRequest) (*domain.SendBulkResponse, error) {
	emailIDs := make([]uuid.UUID, 0, len(req.Recipients))

	for _, recipient := range req.Recipients {
		singleReq := &domain.SendEmailRequest{
			To:            recipient,
			Subject:       req.Subject,
			Body:          req.Body,
			HTMLBody:      req.HTMLBody,
			Template:      req.Template,
			TemplateData:  req.TemplateData,
			SourceService: req.SourceService,
			Metadata:      req.Metadata,
			ProductID:     req.ProductID,
			Branding:      req.Branding,
		}

		resp, err := s.Send(ctx, singleReq)
		if err != nil {
			s.logger.Error().Err(err).Str("to", recipient).Msg("Failed to queue bulk email")
			continue
		}
		emailIDs = append(emailIDs, resp.ID)
	}

	return &domain.SendBulkResponse{
		TotalQueued: len(emailIDs),
		EmailIDs:    emailIDs,
		Message:     fmt.Sprintf("Queued %d of %d emails", len(emailIDs), len(req.Recipients)),
	}, nil
}

// GetStatus retrieves the status of an email.
func (s *EmailService) GetStatus(ctx context.Context, id uuid.UUID) (*domain.EmailStatusResponse, error) {
	email, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &domain.EmailStatusResponse{
		ID:           email.ID,
		To:           email.To,
		Subject:      email.Subject,
		Status:       email.Status,
		ErrorMessage: email.ErrorMessage,
		RetryCount:   email.RetryCount,
		SentAt:       email.SentAt,
		CreatedAt:    email.CreatedAt,
	}, nil
}

// ProcessEmail sends an email (called by worker).
func (s *EmailService) ProcessEmail(ctx context.Context, emailID uuid.UUID) error {
	email, err := s.repo.GetByID(ctx, emailID)
	if err != nil {
		return fmt.Errorf("email not found: %w", err)
	}

	// Update status to sending
	_ = s.repo.UpdateStatus(ctx, emailID, domain.StatusSending, "")

	// Send via configured provider
	if err := s.sender.Send(email); err != nil {
		// Increment retry count
		email.RetryCount++
		_ = s.repo.IncrementRetry(ctx, emailID)

		if email.RetryCount >= email.MaxRetries {
			_ = s.repo.UpdateStatus(ctx, emailID, domain.StatusFailed, err.Error())
			return fmt.Errorf("max retries exceeded: %w", err)
		}

		_ = s.repo.UpdateStatus(ctx, emailID, domain.StatusQueued, err.Error())
		return err // Asynq will retry
	}

	// Mark as sent
	now := time.Now()
	email.SentAt = &now
	_ = s.repo.MarkSent(ctx, emailID, now)

	return nil
}

// ListTemplates returns available templates.
func (s *EmailService) ListTemplates() []domain.TemplateInfo {
	return s.templates.List()
}

// enqueue adds an email to the processing queue.
func (s *EmailService) enqueue(email *domain.Email) error {
	payload, err := json.Marshal(map[string]string{"email_id": email.ID.String()})
	if err != nil {
		return err
	}

	task := asynq.NewTask(taskTypeSendEmail, payload,
		asynq.MaxRetry(MaxRetries),
		asynq.Timeout(30*time.Second),
		asynq.Queue("email"),
	)

	_, err = s.queue.Enqueue(task)
	return err
}
