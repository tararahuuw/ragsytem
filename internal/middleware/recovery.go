package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/tararahuuw/ragsytem/internal/logger"
	"github.com/tararahuuw/ragsytem/internal/response"
)

// Recovery is the outermost safety net (Go's equivalent of a top-level
// try/catch): it recovers from ANY panic in the request lifecycle, logs it with
// the request id and a full stack trace, and returns a standardized 500 error
// envelope — so an unexpected failure never crashes the server nor leaks a raw
// stack trace to the client.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.FromContext(c.Request.Context()).Error("panic recovered",
					"panic", rec,
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"stack", string(debug.Stack()),
				)

				// Only write if the handler hasn't already started a response.
				if !c.Writer.Written() {
					response.Error(c, http.StatusInternalServerError,
						"internal server error", "INTERNAL_ERROR")
				}
				c.Abort()
			}
		}()

		c.Next()
	}
}
