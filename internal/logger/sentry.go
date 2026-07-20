package logger

import (
	"context"
	"fmt"
	"log/slog"

	sentrygo "github.com/getsentry/sentry-go"
)

// sentryHandler wraps another slog.Handler and, for records at/above minLevel,
// forwards them to Sentry — turning the codebase's existing error-logging
// discipline (§4b: every error branch logs ERROR) into Sentry events for free.
// Local logging (text/JSON) is untouched: the inner handler still runs.
type sentryHandler struct {
	inner    slog.Handler
	minLevel slog.Level
	attrs    []slog.Attr // preset attrs (e.g. request_id) added via With(...)
}

// EnableSentry re-wraps the default slog handler so error logs are also sent to
// Sentry. Call AFTER sentry.Init and only when Sentry is enabled. minLevel picks
// the threshold ("error" by default, "warn" to include expected client errors).
func EnableSentry(minLevel slog.Level) {
	inner := slog.Default().Handler()
	slog.SetDefault(slog.New(&sentryHandler{inner: inner, minLevel: minLevel}))
}

// ParseLevel maps a config string to an slog.Level (defaults to Error).
func ParseLevel(s string) slog.Level {
	switch s {
	case "warn", "warning":
		return slog.LevelWarn
	case "info":
		return slog.LevelInfo
	case "debug":
		return slog.LevelDebug
	default:
		return slog.LevelError
	}
}

func (h *sentryHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *sentryHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= h.minLevel {
		h.forward(r)
	}
	return h.inner.Handle(ctx, r)
}

func (h *sentryHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	merged = append(merged, h.attrs...)
	merged = append(merged, attrs...)
	return &sentryHandler{inner: h.inner.WithAttrs(attrs), minLevel: h.minLevel, attrs: merged}
}

func (h *sentryHandler) WithGroup(name string) slog.Handler {
	return &sentryHandler{inner: h.inner.WithGroup(name), minLevel: h.minLevel, attrs: h.attrs}
}

// forward ships one record to Sentry. It prefers an exception (better grouping):
// a panic (attr "panic") or an error value (attr "error") becomes CaptureException;
// otherwise CaptureMessage. The HTTP method/route (from the request context) is
// set as the Sentry transaction + tags so events show WHICH endpoint failed.
func (h *sentryHandler) forward(r slog.Record) {
	// The AccessLog "http_request" line is a per-request summary that duplicates
	// the real error (already captured with endpoint context) and has no error
	// message — skip it to keep Sentry issues clean.
	if r.Message == "http_request" {
		return
	}

	extras := make(map[string]any)
	var errVal error
	var panicVal any
	var httpMethod, httpRoute string

	collect := func(a slog.Attr) {
		switch a.Key {
		case "error":
			if e, ok := a.Value.Any().(error); ok {
				errVal = e
			}
			extras["error"] = a.Value.String()
		case "panic":
			panicVal = a.Value.Any()
			extras["panic"] = a.Value.String()
		case "http_method":
			httpMethod = a.Value.String()
		case "http_route":
			httpRoute = a.Value.String()
		default:
			extras[a.Key] = a.Value.Any()
		}
	}
	for _, a := range h.attrs {
		collect(a)
	}
	r.Attrs(func(a slog.Attr) bool { collect(a); return true })

	extras["log_message"] = r.Message

	// Clone the hub so concurrent log calls don't race on the shared global scope.
	hub := sentrygo.CurrentHub().Clone()
	hub.WithScope(func(scope *sentrygo.Scope) {
		scope.SetLevel(sentryLevel(r.Level))
		// v0.48 replaced Extra(s) with Context(s); group log attrs under "log".
		scope.SetContext("log", sentrygo.Context(extras))
		if rid, ok := extras["request_id"]; ok {
			scope.SetTag("request_id", fmt.Sprint(rid))
		}
		// Attribute the event to an endpoint: transaction "GET /documents/:id"
		// shows in the issue title/culprit; tags make it filterable; fingerprint
		// keeps the same error on different endpoints as separate issues.
		if httpRoute != "" {
			txn := httpRoute
			if httpMethod != "" {
				txn = httpMethod + " " + httpRoute
				scope.SetTag("http.method", httpMethod)
			}
			scope.SetTag("http.route", httpRoute)
			// v0.48 has no Scope.SetTransaction; set the event's Transaction field
			// directly via a processor so the endpoint shows as the issue culprit.
			scope.AddEventProcessor(func(event *sentrygo.Event, _ *sentrygo.EventHint) *sentrygo.Event {
				event.Transaction = txn
				return event
			})
			scope.SetFingerprint([]string{"{{ default }}", httpMethod, httpRoute})
		}

		switch {
		case panicVal != nil:
			hub.CaptureException(fmt.Errorf("panic: %v", panicVal))
		case errVal != nil:
			hub.CaptureException(errVal)
		default:
			hub.CaptureMessage(r.Message)
		}
	})
}

func sentryLevel(l slog.Level) sentrygo.Level {
	switch {
	case l >= slog.LevelError:
		return sentrygo.LevelError
	case l >= slog.LevelWarn:
		return sentrygo.LevelWarning
	case l >= slog.LevelInfo:
		return sentrygo.LevelInfo
	default:
		return sentrygo.LevelDebug
	}
}
