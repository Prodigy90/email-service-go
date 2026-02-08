package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prodigy90/email-service-go/internal/service"
)

// CampaignHandler handles campaign API requests.
type CampaignHandler struct {
	emailService *service.EmailService
}

// NewCampaignHandler creates a new campaign handler.
func NewCampaignHandler(emailService *service.EmailService) *CampaignHandler {
	return &CampaignHandler{emailService: emailService}
}

// RegisterRoutes registers the campaign routes.
func (h *CampaignHandler) RegisterRoutes(r *gin.RouterGroup) {
	campaigns := r.Group("/campaigns")
	{
		campaigns.GET("/:tag/stats", h.GetStats)
		campaigns.GET("/:tag/non-openers", h.GetNonOpeners)
	}
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
