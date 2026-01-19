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
	"github.com/prodigy90/email-service-go/internal/db"
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
	sqlxDB, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer sqlxDB.Close()
	logger.Info().Msg("Connected to PostgreSQL")

	// Run database migrations
	if err := db.RunMigrations(sqlxDB.DB, cfg.MigrationsDir); err != nil {
		logger.Fatal().Err(err).Msg("Failed to run database migrations")
	}
	logger.Info().Msg("Database migrations completed")

	// Connect to Redis
	redisOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to parse Redis URL")
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
		logger.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	logger.Info().Msg("Connected to Redis")

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
