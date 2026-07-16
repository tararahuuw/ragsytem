package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// AccessLog emits one structured log line per HTTP request, tagged with the
// request id for end-to-end tracing. Level is chosen by status:
// 5xx -> Error, 4xx -> Warn, otherwise Info.
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		status := c.Writer.Status()
		reqID, _ := c.Get("request_id")
		attrs := []any{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Int64("latency_ms", time.Since(start).Milliseconds()),
			slog.String("client_ip", c.ClientIP()),
			slog.Any("request_id", reqID),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("gin_errors", c.Errors.String()))
		}

		switch {
		case status >= 500:
			slog.Error("http_request", attrs...)
		case status >= 400:
			slog.Warn("http_request", attrs...)
		default:
			slog.Info("http_request", attrs...)
		}
	}
}
