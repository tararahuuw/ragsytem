// Package sentry initializes the Sentry error-monitoring client. It follows the
// same mockable pattern as the AI/email adapters: an empty DSN disables Sentry
// entirely (no-op), so the app runs unchanged in dev without an account.
package sentry

import (
	"log/slog"
	"time"

	sentrygo "github.com/getsentry/sentry-go"

	"github.com/tararahuuw/ragsytem/internal/config"
)

// FlushFunc drains buffered events before the process exits. Always safe to call
// (no-op when Sentry is disabled).
type FlushFunc func()

// Init configures the global Sentry hub from config. When SENTRY_DSN is empty it
// logs a warning and returns a no-op flush — errors are still logged locally,
// just not shipped to Sentry. The returned flush MUST be called on shutdown
// (defer) so async events aren't lost.
func Init(cfg *config.Config) FlushFunc {
	if cfg.SentryDSN == "" {
		slog.Warn("sentry disabled: SENTRY_DSN kosong (error hanya di-log lokal, tak dikirim)")
		return func() {}
	}

	env := cfg.SentryEnvironment
	if env == "" {
		env = cfg.AppEnv
	}

	err := sentrygo.Init(sentrygo.ClientOptions{
		Dsn:         cfg.SentryDSN,
		Environment: env,
		Release:     cfg.AppName,
		// Attach a stack trace even to CaptureMessage/plain errors so events are
		// actionable regardless of where they originate.
		AttachStacktrace: true,
		EnableTracing:    cfg.SentryTracesSampleRate > 0,
		TracesSampleRate: cfg.SentryTracesSampleRate,
	})
	if err != nil {
		// Don't crash the app just because monitoring failed to start.
		slog.Error("sentry init failed (continuing without it)", "error", err)
		return func() {}
	}

	slog.Info("sentry enabled", "environment", env, "traces_sample_rate", cfg.SentryTracesSampleRate)
	return func() { sentrygo.Flush(2 * time.Second) }
}
