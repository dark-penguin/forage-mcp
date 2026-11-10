package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/pflag"
)

// Config holds the runtime configuration for the forage-mcp server.
type Config struct {
	ForageURL      string
	APIKey         string
	TimeoutSeconds int
	LogLevel       slog.Level
}

// LoadConfig reads environment variables, parses CLI flags (with env-var defaults),
// resolves the log level, and validates that a Forage URL is present.
func LoadConfig() (*Config, error) {
	fs := pflag.NewFlagSet("forage-mcp", pflag.ContinueOnError)

	envURL := os.Getenv("FORAGE_URL")
	envKey := os.Getenv("FORAGE_API_KEY")

	var (
		flagURL   string
		flagKey   string
		timeout   int
		verbose   bool
		quiet     bool
	)

	fs.StringVarP(&flagURL, "forage-url", "u", envURL, "Base URL of the Forage instance (env: FORAGE_URL)")
	fs.StringVarP(&flagKey, "forage-api-key", "k", envKey, "Bearer token for authenticated Forage instances (env: FORAGE_API_KEY)")
	fs.IntVarP(&timeout, "timeout", "t", 120, "Default timeout for API calls in seconds")
	fs.BoolVarP(&verbose, "verbose", "v", false, "Set log level to DEBUG")
	fs.BoolVarP(&quiet, "quiet", "q", false, "Set log level to WARN")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, fmt.Errorf("flag parsing failed: %w", err)
	}

	// Resolve log level: flag > LOG_LEVEL env > default info
	var level slog.Level
	switch {
	case verbose:
		level = slog.LevelDebug
	case quiet:
		level = slog.LevelWarn
	default:
		if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
			if err := level.UnmarshalText([]byte(envLevel)); err != nil {
				return nil, fmt.Errorf("invalid LOG_LEVEL %q: %w", envLevel, err)
			}
		} else {
			level = slog.LevelInfo
		}
	}

	cfg := &Config{
		ForageURL:      flagURL,
		APIKey:         flagKey,
		TimeoutSeconds: timeout,
		LogLevel:       level,
	}

	if cfg.ForageURL == "" {
		return nil, fmt.Errorf("FORAGE_URL environment variable or --forage-url flag is required")
	}

	if cfg.TimeoutSeconds < 1 {
		return nil, fmt.Errorf("timeout must be at least 1 second (got %d)", cfg.TimeoutSeconds)
	}

	return cfg, nil
}
