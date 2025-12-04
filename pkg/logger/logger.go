// Package logger provides structured logging using slog.
package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type contextKey int

const loggerKey contextKey = 0

// Logger is an alias for slog.Logger.
type Logger = slog.Logger

// New creates a new logger with the specified level.
func New(level string) *Logger {
	var slogLevel slog.Level

	switch strings.ToLower(level) {
	case "trace", "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slogLevel,
	})

	return slog.New(handler)
}

// WithContext adds a logger to the context.
func WithContext(ctx context.Context, log *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, log)
}

// FromContext retrieves a logger from the context.
func FromContext(ctx context.Context) *Logger {
	if log, ok := ctx.Value(loggerKey).(*Logger); ok {
		return log
	}

	return New("info")
}
