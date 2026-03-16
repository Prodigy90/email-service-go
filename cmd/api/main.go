package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"

	"github.com/prodigy90/email-service-go/internal/api/routes"
	"github.com/prodigy90/email-service-go/internal/config"
	"github.com/prodigy90/email-service-go/internal/db"
	"github.com/prodigy90/email-service-go/internal/logger"
	"github.com/prodigy90/email-service-go/internal/repository/postgres"
	"github.com/prodigy90/email-service-go/internal/service"
)

func main() {
	// Load config first so we can use LogLevel
	cfg := config.Load()

	// Setup logger with configured level
	logger.Init(cfg.LogLevel)
	log := *logger.Get()

	// Validate production config
	if err := config.ValidateProduction(cfg); err != nil {
		log.Fatal().Err(err).Msg("Configuration validation failed")
	}

	// Warn about optional security config
	if cfg.UnsubscribeSecret == "" || cfg.UnsubscribeSecret == "change-me-unsubscribe-secret" {
		log.Warn().Msg("UNSUBSCRIBE_SECRET not configured - unsubscribe links will use default HMAC secret")
	}

	// Set Gin mode
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Connect to PostgreSQL
	sqlxDB, err := sqlx.Connect("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer sqlxDB.Close()
	log.Info().Msg("Connected to PostgreSQL")

	// Run database migrations
	if err := db.RunMigrations(sqlxDB.DB, cfg.MigrationsDir); err != nil {
		log.Fatal().Err(err).Msg("Failed to run database migrations")
	}
	log.Info().Msg("Database migrations completed")

	// Connect to Redis
	redisOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse Redis URL")
	}

	// Configure timeouts and pool settings for resilience
	redisOpt.DialTimeout = 10 * time.Second
	redisOpt.ReadTimeout = 30 * time.Second
	redisOpt.WriteTimeout = 30 * time.Second
	redisOpt.PoolSize = 10
	redisOpt.MinIdleConns = 2
	redisOpt.PoolTimeout = 60 * time.Second
	redisOpt.MaxRetries = 3

	redisClient := redis.NewClient(redisOpt)
	defer redisClient.Close()

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer pingCancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	log.Info().Msg("Connected to Redis")

	// Create Asynq client with timeouts
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:         redisOpt.Addr,
		DB:           redisOpt.DB,
		Password:     redisOpt.Password,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		PoolSize:     10,
	})
	defer asynqClient.Close()

	// Initialize services
	emailRepo := postgres.NewEmailRepository(sqlxDB)
	templateService, err := service.NewTemplateService(cfg.TemplateDir, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize template service")
	}

	// Choose email provider based on config
	var emailSender service.EmailSender
	switch cfg.EmailProvider {
	case "resend":
		if cfg.Resend.APIKey == "" {
			log.Fatal().Msg("RESEND_API_KEY is required when EMAIL_PROVIDER=resend")
		}
		emailSender = service.NewResendClient(cfg.Resend, log)
		log.Info().Msg("Using Resend as email provider")
	default:
		emailSender = service.NewSMTPClient(cfg.SMTP, log)
		log.Info().Msg("Using SMTP as email provider")
	}

	// Create suppression repository
	suppressionRepo := postgres.NewSuppressionRepository(sqlxDB)

	emailService := service.NewEmailService(emailRepo, emailSender, templateService, asynqClient, redisClient, suppressionRepo, log)

	// Create event repository and webhook service
	eventRepo := postgres.NewEventRepository(sqlxDB)
	webhookService := service.NewWebhookService(emailRepo, eventRepo, suppressionRepo, cfg.ResendWebhookSecret, log)
	if cfg.ResendWebhookSecret == "" {
		log.Warn().Msg("RESEND_WEBHOOK_SECRET not set, webhook signature verification disabled")
	}

	// Create unsubscribe service and wire to email service for auto-injection
	unsubscribeService := service.NewUnsubscribeService(cfg.UnsubscribeSecret, cfg.UnsubscribeBaseURL)
	emailService.SetUnsubscribeService(unsubscribeService)

	// Create router
	router := routes.New(routes.Deps{
		Logger:             log,
		EmailService:       emailService,
		WebhookService:     webhookService,
		UnsubscribeService: unsubscribeService,
		SuppressionRepo:    suppressionRepo,
		APIKey:             cfg.APIKey,
		SwaggerAllowedIPs:  cfg.SwaggerAllowedIPs,
		TrustedProxies:     cfg.TrustedProxies,
	})

	// Start server
	srv := &http.Server{
		Addr:         ":" + cfg.APIPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Info().Str("port", cfg.APIPort).Msg("Starting API server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	fmt.Println("Server exited")
}
