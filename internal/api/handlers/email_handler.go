package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prodigy90/email-service-go/internal/domain"
	"github.com/prodigy90/email-service-go/internal/service"
)

// EmailHandler handles email API requests.
type EmailHandler struct {
	emailService *service.EmailService
}

// NewEmailHandler creates a new email handler.
func NewEmailHandler(emailService *service.EmailService) *EmailHandler {
	return &EmailHandler{emailService: emailService}
}

// RegisterRoutes registers the email routes.
func (h *EmailHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/send", h.Send)
	r.POST("/send/bulk", h.SendBulk)
	r.POST("/send/bulk-personalized", h.SendBulkPersonalized)
	r.GET("/status/:id", h.GetStatus)
	r.GET("/templates", h.ListTemplates)
}

// Send handles single email sending.
// @Summary Send an email
// @Tags emails
// @Accept json
// @Produce json
// @Param request body domain.SendEmailRequest true "Email request"
// @Success 202 {object} domain.SendEmailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} CapRefusal "marketing send held by the daily cap (MARKETING_CAP_MODE=enforce)"
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse "daily cap could not be checked; marketing send held"
// @Router /send [post]
func (h *EmailHandler) Send(c *gin.Context) {
	var req domain.SendEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	resp, err := h.emailService.Send(c.Request.Context(), &req)
	if err != nil {
		writeSendError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, resp)
}

// CapRefusal is the 409 body for a send held by the daily marketing cap.
// Error starts with the code, so a client that only surfaces the error
// string (pkg/client) can still tell it apart.
type CapRefusal struct {
	Error          string    `json:"error"`
	Code           string    `json:"code"`
	MarketingToday int       `json:"marketing_today"`
	Cap            int       `json:"cap"`
	RetryAfter     time.Time `json:"retry_after"`
}

// writeSendError maps Send errors: 409 cap refusal, 503 cap unreadable
// (the send holds), 400 bad request, 500 the rest.
func writeSendError(c *gin.Context, err error) {
	var capped *service.ErrMarketingCapped
	switch {
	case errors.As(err, &capped):
		c.JSON(http.StatusConflict, CapRefusal{Error: capped.Error(), Code: "marketing_daily_cap",
			MarketingToday: capped.MarketingToday, Cap: capped.Cap, RetryAfter: capped.RetryAfter})
	case errors.Is(err, service.ErrMarketingCapUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "code": "marketing_cap_unavailable"})
	case errors.Is(err, service.ErrInvalidRequest):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}
}

// SendBulk handles bulk email sending.
// @Summary Send emails to multiple recipients
// @Tags emails
// @Accept json
// @Produce json
// @Param request body domain.SendBulkRequest true "Bulk email request"
// @Success 202 {object} domain.SendBulkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /send/bulk [post]
func (h *EmailHandler) SendBulk(c *gin.Context) {
	var req domain.SendBulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	resp, err := h.emailService.SendBulk(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, resp)
}

// SendBulkPersonalized handles personalized bulk email sending.
// @Summary Send personalized emails to multiple recipients
// @Tags emails
// @Accept json
// @Produce json
// @Param request body domain.SendBulkPersonalizedRequest true "Personalized bulk email request"
// @Success 202 {object} domain.SendBulkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /send/bulk-personalized [post]
func (h *EmailHandler) SendBulkPersonalized(c *gin.Context) {
	var req domain.SendBulkPersonalizedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	resp, err := h.emailService.SendBulkPersonalized(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, resp)
}

// GetStatus retrieves the status of an email.
// @Summary Get email status
// @Tags emails
// @Produce json
// @Param id path string true "Email ID"
// @Success 200 {object} domain.EmailStatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /status/{id} [get]
func (h *EmailHandler) GetStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid email id"})
		return
	}

	resp, err := h.emailService.GetStatus(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "email not found"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListTemplates returns available email templates.
// @Summary List email templates
// @Tags templates
// @Produce json
// @Success 200 {object} domain.TemplateListResponse
// @Router /templates [get]
func (h *EmailHandler) ListTemplates(c *gin.Context) {
	templates := h.emailService.ListTemplates()
	c.JSON(http.StatusOK, domain.TemplateListResponse{Templates: templates})
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error string `json:"error"`
}
