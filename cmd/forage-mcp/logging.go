package main

import (
	"log/slog"
	"os"
)

// InitLogging configures the global slog logger to use a text handler writing to stderr.
func InitLogging(level slog.Level) {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}
