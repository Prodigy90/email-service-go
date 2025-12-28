package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/prodigy90/email-service-go/internal/config"
	"github.com/prodigy90/email-service-go/internal/repository/postgres"
	"github.com/prodigy90/email-service-go/internal/service"
	"github.com/prodigy90/email-service-go/internal/worker/tasks"
)

func main() {
	// Setup logger
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	if os.Getenv("ENV") == "development" {
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}

	// Load config
	cfg := config.Load()

	// Connect to PostgreSQL
	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()
	logger.Info().Msg("Connected to PostgreSQL")

	// Parse Redis URL
	redisOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to parse Redis URL")
	}

	// Create Asynq client (for re-enqueueing if needed)
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr: redisOpt.Addr,
		DB:   redisOpt.DB,
	})
	defer asynqClient.Close()

	// Initialize services
	emailRepo := postgres.NewEmailRepository(db)
	smtpClient := service.NewSMTPClient(cfg.SMTP, logger)
	templateService, err := service.NewTemplateService(cfg.TemplateDir, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize template service")
	}
	emailService := service.NewEmailService(emailRepo, smtpClient, templateService, asynqClient, logger)

	// Create task handler
	emailTaskHandler := tasks.NewEmailTaskHandler(emailService, logger)

	// Create Asynq server
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr: redisOpt.Addr,
			DB:   redisOpt.DB,
		},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"email":   6,
				"default": 3,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				logger.Error().
					Err(err).
					Str("task_type", task.Type()).
					Bytes("payload", task.Payload()).
					Msg("Task failed")
			}),
		},
	)

	// Register handlers
	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TaskTypeSendEmail, emailTaskHandler.ProcessTask)

	// Start server
	logger.Info().Msg("Starting email worker")
	go func() {
		if err := srv.Run(mux); err != nil {
			logger.Fatal().Err(err).Msg("Worker failed")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down worker...")
	srv.Shutdown()
	logger.Info().Msg("Worker exited")
}
