package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Env     string
	APIPort string

	// Database
	DatabaseURL string

	// Redis
	RedisURL string
	Redis    RedisConfig

	// Email Provider: "smtp" or "resend"
	EmailProvider string

	// SMTP
	SMTP SMTPConfig

	// Resend
	Resend ResendConfig

	// API Authentication
	APIKey string

	// Templates
	TemplateDir string

	// Database migrations
	MigrationsDir string

	// Swagger UI
	// SwaggerAllowedIPs is a comma-separated list of IPs or CIDR ranges
	// If empty, Swagger UI is accessible to everyone (dev mode)
	SwaggerAllowedIPs string
}

type SMTPConfig struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	FromName    string
}

type ResendConfig struct {
	APIKey      string
	FromAddress string
	FromName    string
}

type RedisConfig struct {
	DialTimeoutSecs  int
	ReadTimeoutSecs  int
	WriteTimeoutSecs int
	PoolSize         int
}

func Load() *Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	} else {
		log.Println("Loaded .env file")
	}

	return &Config{
		Env:         getEnv("ENV", "development"),
		APIPort:     getEnv("API_PORT", "8083"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://email:email@localhost:55433/email_service?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:16380/0"),
		Redis: RedisConfig{
			DialTimeoutSecs:  getEnvIntPositive("REDIS_DIAL_TIMEOUT", 30),
			ReadTimeoutSecs:  getEnvIntPositive("REDIS_READ_TIMEOUT", 30),
			WriteTimeoutSecs: getEnvIntPositive("REDIS_WRITE_TIMEOUT", 30),
			PoolSize:         getEnvIntPositive("REDIS_POOL_SIZE", 10),
		},
		EmailProvider: getEnv("EMAIL_PROVIDER", "smtp"),
		SMTP: SMTPConfig{
			Host:        getEnv("SMTP_HOST", "localhost"),
			Port:        getEnvInt("SMTP_PORT", 1025),
			Username:    getEnv("SMTP_USERNAME", ""),
			Password:    getEnv("SMTP_PASSWORD", ""),
			FromAddress: getEnv("SMTP_FROM_ADDRESS", "noreply@localhost"),
			FromName:    getEnv("SMTP_FROM_NAME", "Email Service"),
		},
		Resend: ResendConfig{
			APIKey:      getEnv("RESEND_API_KEY", ""),
			FromAddress: getEnv("RESEND_FROM_ADDRESS", ""),
			FromName:    getEnv("RESEND_FROM_NAME", ""),
		},
		APIKey:            getEnv("API_KEY", "dev-api-key"),
		TemplateDir:       getEnv("TEMPLATE_DIR", "./pkg/templates"),
		MigrationsDir:     getEnv("MIGRATIONS_DIR", "./migrations"),
		SwaggerAllowedIPs: getEnv("SWAGGER_ALLOWED_IPS", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

// getEnvIntPositive returns a positive integer from env or fallback if <= 0.
// Logs a warning if the value is invalid or non-positive.
func getEnvIntPositive(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		i, err := strconv.Atoi(v)
		if err != nil {
			log.Printf("Warning: %s=%q is not a valid integer, using default %d", key, v, fallback)
			return fallback
		}
		if i <= 0 {
			log.Printf("Warning: %s=%d is not positive, using default %d", key, i, fallback)
			return fallback
		}
		return i
	}
	return fallback
}
