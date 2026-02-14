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
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const (
	taskTypeSendEmail = "email:send"
	MaxRetries        = 3
)

// EmailService handles email operations.
type EmailService struct {
	repo            *postgres.EmailRepository
	sender          EmailSender
	templates       *TemplateService
	queue           *asynq.Client
	redis           *redis.Client
	suppressionRepo *postgres.SuppressionRepository
	unsubscribe     *UnsubscribeService
	logger          zerolog.Logger
}

// NewEmailService creates a new email service.
// The redis client and suppression repo are optional (pass nil to disable).
func NewEmailService(
	repo *postgres.EmailRepository,
	sender EmailSender,
	templates *TemplateService,
	queue *asynq.Client,
	redisClient *redis.Client,
	suppressionRepo *postgres.SuppressionRepository,
	logger zerolog.Logger,
) *EmailService {
	return &EmailService{
		repo:            repo,
		sender:          sender,
		templates:       templates,
		queue:           queue,
		redis:           redisClient,
		suppressionRepo: suppressionRepo,
		logger:          logger.With().Str("component", "email_service").Logger(),
	}
}

// SetUnsubscribeService sets the unsubscribe service for URL generation.
func (s *EmailService) SetUnsubscribeService(unsub *UnsubscribeService) {
	s.unsubscribe = unsub
}

// Send queues an email for delivery.
func (s *EmailService) Send(ctx context.Context, req *domain.SendEmailRequest) (*domain.SendEmailResponse, error) {
	// Pre-send suppression check
	if s.suppressionRepo != nil {
		suppressed, err := s.suppressionRepo.IsSuppressed(ctx, req.To)
		if err != nil {
			s.logger.Warn().Err(err).Str("to", req.To).Msg("Failed to check suppression list, proceeding with send")
		} else if suppressed {
			s.logger.Info().Str("to", req.To).Msg("Email suppressed, skipping send")
			return &domain.SendEmailResponse{
				ID:      uuid.Nil,
				Status:  "suppressed",
				Message: "Recipient is on the suppression list",
			}, nil
		}
	}

	// Inject UnsubscribeURL into template data if unsubscribe service is available
	if s.unsubscribe != nil && req.Template != "" {
		if req.TemplateData == nil {
			req.TemplateData = make(map[string]interface{})
		}
		if _, exists := req.TemplateData["UnsubscribeURL"]; !exists {
			req.TemplateData["UnsubscribeURL"] = s.unsubscribe.GenerateURL(req.To)
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

	// Add List-Unsubscribe headers if unsubscribe URL is available
	if s.unsubscribe != nil {
		unsubURL := s.unsubscribe.GenerateURL(req.To)
		email.Headers = map[string]string{
			"List-Unsubscribe":      "<" + unsubURL + ">",
			"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
		}
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

	// Atomically create or get existing email by idempotency ID
	// This prevents race conditions with concurrent duplicate requests
	savedEmail, existedBefore, err := s.repo.CreateOrGetByIdempotencyID(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to save email: %w", err)
	}

	// If email already existed (idempotent duplicate), return the existing record
	if existedBefore {
		s.logger.Debug().Str("idempotency_id", req.IdempotencyID).Msg("Duplicate request, returning existing")
		return &domain.SendEmailResponse{
			ID:      savedEmail.ID,
			Status:  savedEmail.Status,
			Message: "Email already queued (idempotent)",
		}, nil
	}

	// Use the saved email for further processing
	email = savedEmail

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
	result, err := s.sender.Send(email)
	if err != nil {
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

	// Store the provider email ID (e.g., Resend email ID) for webhook event mapping
	if result != nil && result.ProviderID != "" {
		if err := s.repo.UpdateResendEmailID(ctx, emailID, result.ProviderID); err != nil {
			s.logger.Warn().Err(err).Str("email_id", emailID.String()).Msg("Failed to store resend email ID")
		}
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

// GetCampaignStats returns aggregate stats for a campaign, with 30s Redis cache.
func (s *EmailService) GetCampaignStats(ctx context.Context, campaignTag string) (*domain.CampaignStats, error) {
	cacheKey := "campaign_stats:" + campaignTag

	// Try Redis cache first
	if s.redis != nil {
		cached, err := s.redis.Get(ctx, cacheKey).Bytes()
		if err == nil {
			var stats domain.CampaignStats
			if json.Unmarshal(cached, &stats) == nil {
				return &stats, nil
			}
		}
	}

	stats, err := s.repo.GetCampaignStats(ctx, campaignTag)
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign stats: %w", err)
	}

	// Cache for 30 seconds
	if s.redis != nil {
		if data, err := json.Marshal(stats); err == nil {
			_ = s.redis.Set(ctx, cacheKey, data, 30*time.Second).Err()
		}
	}

	return stats, nil
}

// GetCampaignNonOpeners returns email addresses from a campaign that have not opened the email.
func (s *EmailService) GetCampaignNonOpeners(ctx context.Context, campaignTag string) ([]string, error) {
	addresses, err := s.repo.GetCampaignNonOpeners(ctx, campaignTag)
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign non-openers: %w", err)
	}
	return addresses, nil
}

// GetCampaignBouncedEmails returns bounced/complained email addresses from a campaign.
func (s *EmailService) GetCampaignBouncedEmails(ctx context.Context, campaignTag string) ([]string, error) {
	return s.repo.GetCampaignBouncedEmails(ctx, campaignTag)
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
