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
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/prodigy90/email-service-go/internal/api/routes"
	"github.com/prodigy90/email-service-go/internal/config"
	"github.com/prodigy90/email-service-go/internal/repository/postgres"
	"github.com/prodigy90/email-service-go/internal/service"
)

func main() {
	// Setup logger
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	if os.Getenv("ENV") == "development" {
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}

	// Load config
	cfg := config.Load()

	// Set Gin mode
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Connect to PostgreSQL
	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()
	logger.Info().Msg("Connected to PostgreSQL")

	// Connect to Redis
	redisOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to parse Redis URL")
	}
	redisClient := redis.NewClient(redisOpt)
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	logger.Info().Msg("Connected to Redis")

	// Create Asynq client
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr: redisOpt.Addr,
		DB:   redisOpt.DB,
	})
	defer asynqClient.Close()

	// Initialize services
	emailRepo := postgres.NewEmailRepository(db)
	templateService, err := service.NewTemplateService(cfg.TemplateDir, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize template service")
	}

	// Choose email provider based on config
	var emailSender service.EmailSender
	switch cfg.EmailProvider {
	case "resend":
		if cfg.Resend.APIKey == "" {
			logger.Fatal().Msg("RESEND_API_KEY is required when EMAIL_PROVIDER=resend")
		}
		emailSender = service.NewResendClient(cfg.Resend, logger)
		logger.Info().Msg("Using Resend as email provider")
	default:
		emailSender = service.NewSMTPClient(cfg.SMTP, logger)
		logger.Info().Msg("Using SMTP as email provider")
	}

	emailService := service.NewEmailService(emailRepo, emailSender, templateService, asynqClient, logger)

	// Create router
	router := routes.New(routes.Deps{
		Logger:            logger,
		EmailService:      emailService,
		APIKey:            cfg.APIKey,
		SwaggerAllowedIPs: cfg.SwaggerAllowedIPs,
	})

	// Start server
	srv := &http.Server{
		Addr:         ":" + cfg.APIPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info().Str("port", cfg.APIPort).Msg("Starting API server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	fmt.Println("Server exited")
}
