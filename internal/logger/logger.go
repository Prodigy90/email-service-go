package logger

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Logger is the application logger
var Logger zerolog.Logger

// Init initializes the application logger with the specified log level.
// Valid levels: debug, info, warn, error (default: info)
func Init(logLevel string) {
	zerolog.TimeFieldFormat = time.RFC3339

	// Parse log level
	level := zerolog.InfoLevel
	switch strings.ToLower(logLevel) {
	case "debug":
		level = zerolog.DebugLevel
	case "info":
		level = zerolog.InfoLevel
	case "warn", "warning":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	}

	// Create console writer with colors
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		NoColor:    false,
	}

	Logger = zerolog.New(output).With().Timestamp().Logger().Level(level)
}

// Get returns the application logger. Initializes with default level if not set.
func Get() *zerolog.Logger {
	if Logger.GetLevel() == zerolog.Disabled {
		Init("info")
	}
	return &Logger
}
