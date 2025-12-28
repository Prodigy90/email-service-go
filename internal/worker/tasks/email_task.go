package tasks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/prodigy90/email-service-go/internal/service"
	"github.com/rs/zerolog"
)

const TaskTypeSendEmail = "email:send"

// EmailTaskHandler handles email sending tasks.
type EmailTaskHandler struct {
	emailService *service.EmailService
	logger       zerolog.Logger
}

// NewEmailTaskHandler creates a new email task handler.
func NewEmailTaskHandler(emailService *service.EmailService, logger zerolog.Logger) *EmailTaskHandler {
	return &EmailTaskHandler{
		emailService: emailService,
		logger:       logger.With().Str("component", "email_task").Logger(),
	}
}

// ProcessTask processes an email sending task.
func (h *EmailTaskHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload struct {
		EmailID string `json:"email_id"`
	}

	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	emailID, err := uuid.Parse(payload.EmailID)
	if err != nil {
		return fmt.Errorf("invalid email ID: %w", err)
	}

	h.logger.Info().Str("email_id", payload.EmailID).Msg("Processing email task")

	if err := h.emailService.ProcessEmail(ctx, emailID); err != nil {
		h.logger.Error().Err(err).Str("email_id", payload.EmailID).Msg("Failed to process email")
		return err
	}

	h.logger.Info().Str("email_id", payload.EmailID).Msg("Email sent successfully")
	return nil
}
