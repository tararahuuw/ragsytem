// Package logger configures the global structured logger (slog) and provides
// request-scoped loggers for tracing across layers.
package logger

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

// Init configures the global slog logger:
//   - non-production: human-readable text (easy to read while developing)
//   - production: JSON (machine-parseable for log aggregation / tracing)
func Init(env string) {
	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	slog.SetDefault(slog.New(handler))
}

// WithRequestID stores a request id in the context so downstream layers can log
// it for correlation.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext returns the request id, or "" if absent.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// FromContext returns a logger pre-tagged with the request id, so every log line
// in a request can be traced end-to-end (controller -> service -> repository).
func FromContext(ctx context.Context) *slog.Logger {
	if id := RequestIDFromContext(ctx); id != "" {
		return slog.Default().With("request_id", id)
	}
	return slog.Default()
}
