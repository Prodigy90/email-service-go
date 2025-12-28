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
	Logger       zerolog.Logger
	EmailService *service.EmailService
	APIKey       string
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

	// API routes (protected by API key)
	api := r.Group("/api/v1")
	api.Use(middleware.APIKeyAuth(d.APIKey))

	// Email endpoints
	emailHandler := handlers.NewEmailHandler(d.EmailService)
	emailHandler.RegisterRoutes(api)

	return r
}
