package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/prodigy90/email-service-go/internal/config"
	"github.com/prodigy90/email-service-go/internal/logger"
	"github.com/prodigy90/email-service-go/internal/repository/postgres"
	"github.com/prodigy90/email-service-go/internal/service"
	"github.com/prodigy90/email-service-go/internal/worker/tasks"
)

// buildRedisOpt creates an asynq RedisClientOpt from parsed redis options and config
func buildRedisOpt(redisOpt *redis.Options, cfg *config.Config) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:         redisOpt.Addr,
		DB:           redisOpt.DB,
		Password:     redisOpt.Password,
		DialTimeout:  time.Duration(cfg.Redis.DialTimeoutSecs) * time.Second,
		ReadTimeout:  time.Duration(cfg.Redis.ReadTimeoutSecs) * time.Second,
		WriteTimeout: time.Duration(cfg.Redis.WriteTimeoutSecs) * time.Second,
		PoolSize:     cfg.Redis.PoolSize,
	}
}

func main() {
	// Load config first so we can use LogLevel
	cfg := config.Load()

	// Setup logger with configured level
	logger.Init(cfg.LogLevel)
	log := *logger.Get()

	// Connect to PostgreSQL
	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()
	log.Info().Msg("Connected to PostgreSQL")

	// Parse Redis URL
	redisOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse Redis URL")
	}

	// Build asynq Redis options with timeouts
	asynqRedisOpt := buildRedisOpt(redisOpt, cfg)

	// Create Asynq client (for re-enqueueing if needed)
	asynqClient := asynq.NewClient(asynqRedisOpt)
	defer asynqClient.Close()

	// Initialize services
	emailRepo := postgres.NewEmailRepository(db)
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

	emailService := service.NewEmailService(emailRepo, emailSender, templateService, asynqClient, log)

	// Create task handler
	emailTaskHandler := tasks.NewEmailTaskHandler(emailService, log)

	// Create Asynq server
	srv := asynq.NewServer(
		asynqRedisOpt,
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"email":   6,
				"default": 3,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log.Error().
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
	log.Info().Msg("Starting email worker")
	go func() {
		if err := srv.Run(mux); err != nil {
			log.Fatal().Err(err).Msg("Worker failed")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down worker...")
	srv.Shutdown()
	log.Info().Msg("Worker exited")
}
