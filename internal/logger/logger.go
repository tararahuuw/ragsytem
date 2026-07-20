// Package logger configures the global structured logger (slog) and provides
// request-scoped loggers for tracing across layers.
package logger

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey string

const (
	requestIDKey ctxKey = "request_id"
	httpMethodKey ctxKey = "http_method"
	httpRouteKey  ctxKey = "http_route"
)

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

// WithHTTP stores the request's method and route template (e.g. "GET",
// "/documents/:id") so downstream logs — and Sentry events derived from them —
// can show WHICH endpoint failed.
func WithHTTP(ctx context.Context, method, route string) context.Context {
	ctx = context.WithValue(ctx, httpMethodKey, method)
	return context.WithValue(ctx, httpRouteKey, route)
}

// HTTPFromContext returns the request method and route (both "" if absent).
func HTTPFromContext(ctx context.Context) (method, route string) {
	method, _ = ctx.Value(httpMethodKey).(string)
	route, _ = ctx.Value(httpRouteKey).(string)
	return method, route
}

// FromContext returns a logger pre-tagged with the request id (and HTTP
// method/route when present), so every log line in a request can be traced
// end-to-end (controller -> service -> repository) and attributed to an endpoint.
func FromContext(ctx context.Context) *slog.Logger {
	var args []any
	if id := RequestIDFromContext(ctx); id != "" {
		args = append(args, "request_id", id)
	}
	if method, route := HTTPFromContext(ctx); route != "" {
		args = append(args, "http_method", method, "http_route", route)
	}
	if len(args) > 0 {
		return slog.Default().With(args...)
	}
	return slog.Default()
}
