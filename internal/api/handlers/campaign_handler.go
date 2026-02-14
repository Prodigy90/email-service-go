package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prodigy90/email-service-go/internal/repository/postgres"
	"github.com/prodigy90/email-service-go/internal/service"
	"github.com/rs/zerolog"
)

// CampaignHandler handles campaign API requests.
type CampaignHandler struct {
	emailService    *service.EmailService
	suppressionRepo *postgres.SuppressionRepository
	logger          zerolog.Logger
}

// NewCampaignHandler creates a new campaign handler.
func NewCampaignHandler(emailService *service.EmailService, suppressionRepo *postgres.SuppressionRepository, logger zerolog.Logger) *CampaignHandler {
	return &CampaignHandler{
		emailService:    emailService,
		suppressionRepo: suppressionRepo,
		logger:          logger.With().Str("component", "campaign_handler").Logger(),
	}
}

// RegisterRoutes registers the campaign routes.
func (h *CampaignHandler) RegisterRoutes(r *gin.RouterGroup) {
	campaigns := r.Group("/campaigns")
	{
		campaigns.GET("/:tag/stats", h.GetStats)
		campaigns.GET("/:tag/non-openers", h.GetNonOpeners)
		campaigns.GET("/:tag/bounced", h.GetBounced)
	}
	r.POST("/suppressions/check", h.CheckSuppressions)
}

// GetStats returns aggregate stats for a campaign.
func (h *CampaignHandler) GetStats(c *gin.Context) {
	tag := c.Param("tag")
	if tag == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "campaign tag is required"})
		return
	}

	stats, err := h.emailService.GetCampaignStats(c.Request.Context(), tag)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetNonOpeners returns email addresses from a campaign that haven't opened.
func (h *CampaignHandler) GetNonOpeners(c *gin.Context) {
	tag := c.Param("tag")
	if tag == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "campaign tag is required"})
		return
	}

	addresses, err := h.emailService.GetCampaignNonOpeners(c.Request.Context(), tag)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"campaign_tag": tag,
		"count":        len(addresses),
		"addresses":    addresses,
	})
}

// GetBounced returns email addresses from a campaign that bounced or complained.
func (h *CampaignHandler) GetBounced(c *gin.Context) {
	tag := c.Param("tag")
	if tag == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "campaign tag is required"})
		return
	}

	addresses, err := h.emailService.GetCampaignBouncedEmails(c.Request.Context(), tag)
	if err != nil {
		h.logger.Error().Err(err).Str("campaign_tag", tag).Msg("Failed to get bounced emails")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"campaign_tag": tag,
		"count":        len(addresses),
		"addresses":    addresses,
	})
}

// CheckSuppressions checks which emails from a list are suppressed.
func (h *CampaignHandler) CheckSuppressions(c *gin.Context) {
	var req struct {
		Emails []string `json:"emails" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if len(req.Emails) > 10000 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "too many emails, max 10000"})
		return
	}

	if h.suppressionRepo == nil {
		c.JSON(http.StatusOK, gin.H{
			"suppressed": []string{},
			"count":      0,
		})
		return
	}

	suppressed, err := h.suppressionRepo.CheckSuppressed(c.Request.Context(), req.Emails)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to check suppressions")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"suppressed": suppressed,
		"count":      len(suppressed),
	})
}
