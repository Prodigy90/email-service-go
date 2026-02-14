package handlers

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prodigy90/email-service-go/internal/repository/postgres"
	"github.com/prodigy90/email-service-go/internal/service"
	"github.com/rs/zerolog"
)

// UnsubscribeHandler handles unsubscribe requests.
type UnsubscribeHandler struct {
	unsubService   *service.UnsubscribeService
	suppressionRepo *postgres.SuppressionRepository
	logger         zerolog.Logger
}

// NewUnsubscribeHandler creates a new unsubscribe handler.
func NewUnsubscribeHandler(
	unsubService *service.UnsubscribeService,
	suppressionRepo *postgres.SuppressionRepository,
	logger zerolog.Logger,
) *UnsubscribeHandler {
	return &UnsubscribeHandler{
		unsubService:   unsubService,
		suppressionRepo: suppressionRepo,
		logger:         logger.With().Str("component", "unsubscribe").Logger(),
	}
}

// GetUnsubscribe renders the unsubscribe confirmation page.
func (h *UnsubscribeHandler) GetUnsubscribe(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.Query("email")))
	token := c.Query("token")

	if email == "" || token == "" {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusBadRequest, unsubscribeErrorPage("Invalid unsubscribe link."))
		return
	}

	if !h.unsubService.Verify(email, token) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusForbidden, unsubscribeErrorPage("Invalid or expired unsubscribe link."))
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, unsubscribeConfirmPage(email, token))
}

// PostUnsubscribe processes the unsubscribe request.
func (h *UnsubscribeHandler) PostUnsubscribe(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	token := c.PostForm("token")

	// Also support JSON body
	if email == "" || token == "" {
		var req struct {
			Email string `json:"email"`
			Token string `json:"token"`
		}
		if err := c.ShouldBindJSON(&req); err == nil {
			email = strings.ToLower(strings.TrimSpace(req.Email))
			token = req.Token
		}
	}

	if email == "" || token == "" {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusBadRequest, unsubscribeErrorPage("Missing email or token."))
		return
	}

	if !h.unsubService.Verify(email, token) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusForbidden, unsubscribeErrorPage("Invalid or expired unsubscribe link."))
		return
	}

	// Add to suppression list
	if err := h.suppressionRepo.Add(c.Request.Context(), email, "unsubscribe", "user", nil); err != nil {
		h.logger.Error().Err(err).Str("email", email).Msg("Failed to add to suppression list")
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusInternalServerError, unsubscribeErrorPage("Something went wrong. Please try again."))
		return
	}

	h.logger.Info().Str("email", email).Msg("User unsubscribed")

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, unsubscribeSuccessPage(email))
}

func unsubscribeConfirmPage(email, token string) string {
	safeEmail := template.HTMLEscapeString(email)
	safeToken := template.HTMLEscapeString(token)
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Unsubscribe - WASBOT</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; background: #f9fafb; }
.card { background: white; border-radius: 12px; padding: 48px; max-width: 420px; text-align: center; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
h1 { font-size: 24px; color: #111827; margin-bottom: 12px; }
p { color: #6b7280; line-height: 1.6; }
.email { font-weight: 600; color: #111827; }
button { background: #ef4444; color: white; border: none; padding: 12px 32px; border-radius: 8px; font-size: 16px; cursor: pointer; margin-top: 24px; }
button:hover { background: #dc2626; }
</style>
</head>
<body>
<div class="card">
<h1>Unsubscribe</h1>
<p>Are you sure you want to unsubscribe <span class="email">` + safeEmail + `</span> from WASBOT emails?</p>
<form method="POST" action="/unsubscribe">
<input type="hidden" name="email" value="` + safeEmail + `">
<input type="hidden" name="token" value="` + safeToken + `">
<button type="submit">Yes, Unsubscribe</button>
</form>
</div>
</body>
</html>`
}

func unsubscribeSuccessPage(email string) string {
	safeEmail := template.HTMLEscapeString(email)
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Unsubscribed - WASBOT</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; background: #f9fafb; }
.card { background: white; border-radius: 12px; padding: 48px; max-width: 420px; text-align: center; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
h1 { font-size: 24px; color: #10b981; margin-bottom: 12px; }
p { color: #6b7280; line-height: 1.6; }
.email { font-weight: 600; color: #111827; }
</style>
</head>
<body>
<div class="card">
<h1>Unsubscribed</h1>
<p><span class="email">` + safeEmail + `</span> has been removed from our mailing list. You will no longer receive emails from WASBOT.</p>
</div>
</body>
</html>`
}

func unsubscribeErrorPage(message string) string {
	safeMessage := template.HTMLEscapeString(message)
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Unsubscribe Error - WASBOT</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; background: #f9fafb; }
.card { background: white; border-radius: 12px; padding: 48px; max-width: 420px; text-align: center; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
h1 { font-size: 24px; color: #ef4444; margin-bottom: 12px; }
p { color: #6b7280; line-height: 1.6; }
</style>
</head>
<body>
<div class="card">
<h1>Error</h1>
<p>` + safeMessage + `</p>
</div>
</body>
</html>`
}
