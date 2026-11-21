package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prodigy90/email-service-go/internal/service"
	"github.com/rs/zerolog"
)

// RecipientStateHandler serves per-address send and suppression state, for
// callers that need a shared daily cap and one track at a time (the
// wasbot-backend person journey reads it).
type RecipientStateHandler struct {
	svc    *service.RecipientStateService
	logger zerolog.Logger
}

// NewRecipientStateHandler creates the handler.
func NewRecipientStateHandler(svc *service.RecipientStateService, logger zerolog.Logger) *RecipientStateHandler {
	return &RecipientStateHandler{svc: svc, logger: logger.With().Str("component", "recipient_state_handler").Logger()}
}

// RegisterRoutes registers the route on the API-key group.
func (h *RecipientStateHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/recipients/state", h.Post)
}

// Post handles POST /api/v1/recipients/state {"emails": [...]} (at most 500).
func (h *RecipientStateHandler) Post(c *gin.Context) {
	var req struct {
		Emails []string `json:"emails"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid body: want {\"emails\": [...]}"})
		return
	}
	if len(req.Emails) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "emails is required"})
		return
	}
	if len(req.Emails) > service.MaxRecipientStateBatch {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "too many emails, max 500"})
		return
	}
	resp, err := h.svc.Get(c.Request.Context(), req.Emails)
	if err != nil {
		h.logger.Error().Err(err).Int("count", len(req.Emails)).Msg("Failed to read recipient state")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}
	c.JSON(http.StatusOK, resp)
}
