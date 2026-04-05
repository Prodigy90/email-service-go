package config

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

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

	// Logging
	LogLevel string // debug, info, warn, error (default: info)

	// Email Provider: "smtp" or "resend"
	EmailProvider string

	// SMTP
	SMTP SMTPConfig

	// Resend
	Resend              ResendConfig
	ResendWebhookSecret string

	// API Authentication
	APIKey string

	// Unsubscribe
	UnsubscribeSecret  string
	UnsubscribeBaseURL string

	// Templates
	TemplateDir string

	// Database migrations
	MigrationsDir string

	// Swagger UI
	// SwaggerAllowedIPs is a comma-separated list of IPs or CIDR ranges
	// If empty, Swagger UI is accessible to everyone (dev mode)
	SwaggerAllowedIPs string

	// TrustedProxies is a list of trusted proxy IPs or CIDR ranges.
	// Set to the ingress/load balancer IP range in production (e.g., "10.0.0.0/8").
	// If empty, no proxies are trusted (safest default).
	TrustedProxies []string

	// CORSOrigins is the explicit allowlist of origins permitted for CORS.
	// If empty, CORS headers are not set (same-origin only). Wildcard is rejected in production.
	CORSOrigins []string
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

// ValidateProduction checks that required environment variables are set for production.
// Returns an error listing all missing required variables.
func ValidateProduction(cfg *Config) error {
	if cfg.Env != "production" {
		return nil // Skip validation in development
	}

	var missing []string

	if cfg.DatabaseURL == "" || cfg.DatabaseURL == "postgres://email:email@localhost:55433/email_service?sslmode=disable" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.RedisURL == "" || cfg.RedisURL == "redis://localhost:16380/0" {
		missing = append(missing, "REDIS_URL")
	}
	if cfg.APIKey == "" || cfg.APIKey == "dev-api-key" {
		missing = append(missing, "API_KEY")
	}

	// Reject wildcard CORS in production.
	for _, o := range cfg.CORSOrigins {
		if o == "*" {
			return fmt.Errorf("CORS_ORIGINS must not contain wildcard (*) in production")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables for production: %s", strings.Join(missing, ", "))
	}
	return nil
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
		LogLevel:      strings.ToLower(getEnv("LOG_LEVEL", "info")),
		EmailProvider: getEnv("EMAIL_PROVIDER", "smtp"),
		SMTP: SMTPConfig{
			Host:        getEnv("SMTP_HOST", "localhost"),
			Port:        getEnvInt("SMTP_PORT", 1025),
			Username:    getEnv("SMTP_USERNAME", ""),
			Password:    getEnv("SMTP_PASSWORD", ""),
			FromAddress: getEnvWithFallback("FROM_EMAIL", "SMTP_FROM_ADDRESS", "noreply@localhost"),
			FromName:    getEnvWithFallback("FROM_NAME", "SMTP_FROM_NAME", "Email Service"),
		},
		Resend: ResendConfig{
			APIKey:      getEnv("RESEND_API_KEY", ""),
			FromAddress: getEnvWithFallback("FROM_EMAIL", "RESEND_FROM_ADDRESS", ""),
			FromName:    getEnvWithFallback("FROM_NAME", "RESEND_FROM_NAME", ""),
		},
		ResendWebhookSecret: getEnv("RESEND_WEBHOOK_SECRET", ""),
		APIKey:              getEnv("API_KEY", "dev-api-key"),
		UnsubscribeSecret:  getEnv("UNSUBSCRIBE_SECRET", "change-me-unsubscribe-secret"),
		UnsubscribeBaseURL: getEnv("UNSUBSCRIBE_BASE_URL", "https://emails.wasbot.app"),
		TemplateDir:       getEnv("TEMPLATE_DIR", "./pkg/templates"),
		MigrationsDir:     getEnv("MIGRATIONS_DIR", "./migrations"),
		SwaggerAllowedIPs: getEnv("SWAGGER_ALLOWED_IPS", ""),
		TrustedProxies:    parseTrustedProxies(getEnv("TRUSTED_PROXIES", "")),
		CORSOrigins:       parseCSVList(getEnv("CORS_ORIGINS", "")),
	}
}

// parseCSVList parses a comma-separated list of values, trimming whitespace
// and dropping empty entries. Returns nil if the input is empty.
func parseCSVList(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, v := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// parseTrustedProxies parses a comma-separated list of trusted proxy IPs/CIDRs.
// Returns nil if the input is empty. Invalid entries are logged and skipped.
func parseTrustedProxies(s string) []string {
	if s == "" {
		return nil
	}
	var proxies []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Validate IP or CIDR format
		if _, _, err := net.ParseCIDR(p); err != nil {
			if net.ParseIP(p) == nil {
				log.Printf("warning: invalid trusted proxy %q (not a valid IP or CIDR), skipping", p)
				continue
			}
		}
		proxies = append(proxies, p)
	}
	return proxies
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvWithFallback tries primary key first, then fallback key, then default value
func getEnvWithFallback(primary, fallback, defaultVal string) string {
	if v := os.Getenv(primary); v != "" {
		return v
	}
	if v := os.Getenv(fallback); v != "" {
		return v
	}
	return defaultVal
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
