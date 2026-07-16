package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tararahuuw/ragsytem/internal/logger"
)

// RequestIDKey is the context/header key used for correlating a request.
const RequestIDKey = "X-Request-ID"

// RequestID attaches a unique request id to every request, reusing an
// incoming X-Request-ID header when present. It also stores the id in the
// request context so downstream layers (service/repository) can log it.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDKey)
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("request_id", id)
		c.Header(RequestIDKey, id)
		c.Request = c.Request.WithContext(logger.WithRequestID(c.Request.Context(), id))
		c.Next()
	}
}
