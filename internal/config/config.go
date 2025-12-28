package config

import (
	"os"
	"strconv"
)

type Config struct {
	Env     string
	APIPort string

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// SMTP
	SMTP SMTPConfig

	// API Authentication
	APIKey string

	// Templates
	TemplateDir string
}

type SMTPConfig struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	FromName    string
}

func Load() *Config {
	return &Config{
		Env:         getEnv("ENV", "development"),
		APIPort:     getEnv("API_PORT", "8082"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://email:email@localhost:55433/email_service?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:16380/0"),
		SMTP: SMTPConfig{
			Host:        getEnv("SMTP_HOST", "localhost"),
			Port:        getEnvInt("SMTP_PORT", 1025),
			Username:    getEnv("SMTP_USERNAME", ""),
			Password:    getEnv("SMTP_PASSWORD", ""),
			FromAddress: getEnv("SMTP_FROM_ADDRESS", "noreply@localhost"),
			FromName:    getEnv("SMTP_FROM_NAME", "Email Service"),
		},
		APIKey:      getEnv("API_KEY", "dev-api-key"),
		TemplateDir: getEnv("TEMPLATE_DIR", "./pkg/templates"),
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
