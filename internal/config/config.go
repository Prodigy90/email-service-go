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

func Load() *Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	} else {
		log.Println("Loaded .env file")
	}

	return &Config{
		Env:           getEnv("ENV", "development"),
		APIPort:       getEnv("API_PORT", "8082"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://email:email@localhost:55433/email_service?sslmode=disable"),
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:16380/0"),
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
