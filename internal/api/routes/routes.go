package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prodigy90/email-service-go/internal/api/handlers"
	"github.com/prodigy90/email-service-go/internal/api/middleware"
	"github.com/prodigy90/email-service-go/internal/service"
	"github.com/rs/zerolog"
)

// Deps holds the router dependencies.
type Deps struct {
	Logger            zerolog.Logger
	EmailService      *service.EmailService
	APIKey            string
	SwaggerAllowedIPs string
}

// New creates the Gin router with all routes.
func New(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), middleware.Logger(d.Logger), middleware.CORS())

	// Health endpoints (public)
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// Swagger UI routes (with optional IP whitelist)
	swaggerGroup := r.Group("/swagger")
	swaggerGroup.Use(middleware.IPWhitelist(d.SwaggerAllowedIPs))
	{
		swaggerGroup.StaticFile("/openapi.yaml", "docs/openapi.yaml")
		swaggerGroup.GET("", swaggerUI)
		swaggerGroup.GET("/index.html", swaggerUI)
	}

	// API routes (protected by API key)
	api := r.Group("/api/v1")
	api.Use(middleware.APIKeyAuth(d.APIKey))

	// Email endpoints
	emailHandler := handlers.NewEmailHandler(d.EmailService)
	emailHandler.RegisterRoutes(api)

	return r
}

// swaggerUI returns a minimal HTML page that loads Swagger UI from CDN with SRI hashes
func swaggerUI(c *gin.Context) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>Email Service API - Swagger UI</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.18.2/swagger-ui.css" integrity="sha384-rcbEi6xgdPk0iWkAQzT2F3FeBJXdG+ydrawGlfHAFIZG7wU6aKbQaRewysYpmrlW" crossorigin="anonymous" />
<style>html,body,#swagger-ui{height:100%;margin:0}</style>
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5.18.2/swagger-ui-bundle.js" integrity="sha384-NXtFPpN61oWCuN4D42K6Zd5Rt2+uxeIT36R7kpXBuY9tLnZorzrJ4ykpqwJfgjpZ" crossorigin="anonymous"></script>
<script src="https://unpkg.com/swagger-ui-dist@5.18.2/swagger-ui-standalone-preset.js" integrity="sha384-qr68CD0cvHa88PmVu7e1a58Ego4qvKtcvcLdS2a8Mo5zILI01gyIV9jVwJk7X2NU" crossorigin="anonymous"></script>
<script>
window.ui = SwaggerUIBundle({
  url: '/swagger/openapi.yaml',
  dom_id: '#swagger-ui',
  deepLinking: true,
  presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
  layout: 'BaseLayout'
});
</script>
</body>
</html>`
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}
