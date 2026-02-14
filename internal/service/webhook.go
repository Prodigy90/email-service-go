package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prodigy90/email-service-go/internal/domain"
	"github.com/prodigy90/email-service-go/internal/repository/postgres"
	"github.com/rs/zerolog"
)

// WebhookService processes Resend webhook events.
type WebhookService struct {
	emailRepo       *postgres.EmailRepository
	eventRepo       *postgres.EventRepository
	suppressionRepo *postgres.SuppressionRepository
	signingSecret   string
	logger          zerolog.Logger
}

// NewWebhookService creates a new webhook service.
func NewWebhookService(
	emailRepo *postgres.EmailRepository,
	eventRepo *postgres.EventRepository,
	suppressionRepo *postgres.SuppressionRepository,
	signingSecret string,
	logger zerolog.Logger,
) *WebhookService {
	return &WebhookService{
		emailRepo:       emailRepo,
		eventRepo:       eventRepo,
		suppressionRepo: suppressionRepo,
		signingSecret:   signingSecret,
		logger:          logger.With().Str("component", "webhook").Logger(),
	}
}

// resendWebhookPayload represents a Resend webhook event.
type resendWebhookPayload struct {
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Data      struct {
		EmailID string `json:"email_id"`
	} `json:"data"`
}

// ProcessResendWebhook verifies and processes a Resend webhook event.
func (s *WebhookService) ProcessResendWebhook(ctx context.Context, body []byte, svixID, svixTimestamp, svixSignature string) error {
	// Verify Svix signature
	if s.signingSecret != "" {
		if err := s.verifySvixSignature(body, svixID, svixTimestamp, svixSignature); err != nil {
			return fmt.Errorf("signature verification failed: %w", err)
		}
	}

	// Parse the webhook payload
	var payload resendWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		s.logger.Error().Err(err).Msg("Failed to parse webhook payload")
		return nil // Return nil so we don't get retried for bad payloads
	}

	s.logger.Info().
		Str("event_type", payload.Type).
		Str("resend_email_id", payload.Data.EmailID).
		Msg("Processing Resend webhook event")

	// Look up our email by the Resend email ID
	email, err := s.emailRepo.GetByResendEmailID(ctx, payload.Data.EmailID)
	if err != nil {
		s.logger.Warn().
			Str("resend_email_id", payload.Data.EmailID).
			Str("event_type", payload.Type).
			Msg("Email not found for Resend webhook event")
		return nil // Don't retry - the email may not be in our system
	}

	// Store the event
	event := &domain.EmailEvent{
		ID:            uuid.New(),
		EmailID:       email.ID,
		EventType:     payload.Type,
		ResendEventID: svixID,
		CreatedAt:     payload.CreatedAt,
	}
	// Store full payload as event metadata
	var payloadMap map[string]interface{}
	_ = json.Unmarshal(body, &payloadMap)
	event.Payload = payloadMap

	if err := s.eventRepo.Create(ctx, event); err != nil {
		s.logger.Error().Err(err).
			Str("email_id", email.ID.String()).
			Str("event_type", payload.Type).
			Msg("Failed to store webhook event")
		return nil // Don't retry to avoid duplicate processing
	}

	// Update email status for terminal events and auto-suppress
	switch payload.Type {
	case "email.bounced":
		if err := s.emailRepo.UpdateStatus(ctx, email.ID, domain.StatusBounced, ""); err != nil {
			s.logger.Error().Err(err).Str("email_id", email.ID.String()).Msg("Failed to update email status to bounced")
		}
		// Auto-suppress bounced email
		if s.suppressionRepo != nil {
			if err := s.suppressionRepo.Add(ctx, email.To, "bounce", "webhook", nil); err != nil {
				s.logger.Error().Err(err).Str("email", email.To).Msg("Failed to auto-suppress bounced email")
			} else {
				s.logger.Info().Str("email", email.To).Msg("Auto-suppressed bounced email")
			}
		}
	case "email.complained":
		if err := s.emailRepo.UpdateStatus(ctx, email.ID, domain.StatusComplaint, ""); err != nil {
			s.logger.Error().Err(err).Str("email_id", email.ID.String()).Msg("Failed to update email status to complained")
		}
		// Auto-suppress complained email
		if s.suppressionRepo != nil {
			if err := s.suppressionRepo.Add(ctx, email.To, "complaint", "webhook", nil); err != nil {
				s.logger.Error().Err(err).Str("email", email.To).Msg("Failed to auto-suppress complained email")
			} else {
				s.logger.Info().Str("email", email.To).Msg("Auto-suppressed complained email")
			}
		}
	}

	s.logger.Info().
		Str("email_id", email.ID.String()).
		Str("event_type", payload.Type).
		Msg("Webhook event processed")

	return nil
}

// verifySvixSignature verifies the Svix webhook signature.
// Resend uses Svix for webhook delivery. The signing secret starts with "whsec_".
func (s *WebhookService) verifySvixSignature(body []byte, msgID, msgTimestamp, msgSignature string) error {
	// Validate required headers
	if msgID == "" || msgTimestamp == "" || msgSignature == "" {
		return fmt.Errorf("missing required svix headers")
	}

	// Check timestamp is not too old (reject if >5 min) to prevent replay attacks
	ts, err := strconv.ParseInt(msgTimestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	diff := time.Since(time.Unix(ts, 0))
	if math.Abs(diff.Seconds()) > 300 {
		return fmt.Errorf("timestamp too old or too far in the future")
	}

	// Decode the secret (strip "whsec_" prefix and base64 decode)
	secret := s.signingSecret
	if strings.HasPrefix(secret, "whsec_") {
		secret = secret[6:]
	}
	secretBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return fmt.Errorf("invalid signing secret: %w", err)
	}

	// Compute expected signature: HMAC-SHA256(secret, "{msg_id}.{timestamp}.{body}")
	toSign := fmt.Sprintf("%s.%s.%s", msgID, msgTimestamp, string(body))
	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(toSign))
	expectedSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// The signature header may contain multiple signatures separated by spaces
	// Each signature is prefixed with "v1,"
	signatures := strings.Split(msgSignature, " ")
	for _, sig := range signatures {
		parts := strings.SplitN(sig, ",", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] == "v1" && hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
			return nil
		}
	}

	return fmt.Errorf("no matching signature found")
}
