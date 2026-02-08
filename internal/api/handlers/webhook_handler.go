package handlers

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prodigy90/email-service-go/internal/service"
)

const maxWebhookBodySize = 256 * 1024

// WebhookHandler handles webhook API requests.
type WebhookHandler struct {
	webhookService *service.WebhookService
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(webhookService *service.WebhookService) *WebhookHandler {
	return &WebhookHandler{webhookService: webhookService}
}

// HandleResendWebhook processes incoming Resend webhook events.
// Resend uses Svix for webhook delivery with signature verification.
func (h *WebhookHandler) HandleResendWebhook(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxWebhookBodySize))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "failed to read body"})
		return
	}

	svixID := c.GetHeader("svix-id")
	svixTimestamp := c.GetHeader("svix-timestamp")
	svixSignature := c.GetHeader("svix-signature")

	if err := h.webhookService.ProcessResendWebhook(
		c.Request.Context(),
		body,
		svixID,
		svixTimestamp,
		svixSignature,
	); err != nil {
		// Signature verification failures return 401 to signal bad auth
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	// Return 200 for all successfully processed events (even if email not found)
	// to prevent Resend from retrying
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
