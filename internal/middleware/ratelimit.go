package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tararahuuw/ragsytem/internal/logger"
	"github.com/tararahuuw/ragsytem/internal/ratelimit"
	"github.com/tararahuuw/ragsytem/internal/response"
)

// RateLimit throttles requests per (category, caller). The caller key is the
// authenticated user id when present (chain after JWTAuth), otherwise the client
// IP (good for public/brute-force-prone endpoints like login). Exceeding the
// limit returns 429.
func RateLimit(l *ratelimit.Limiter, category string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "ip:" + c.ClientIP()
		if uid := CurrentUserID(c); uid != 0 {
			key = fmt.Sprintf("user:%d", uid)
		}

		if !l.Allow(category, key) {
			logger.FromContext(c.Request.Context()).Warn("rate limit exceeded",
				"category", category, "key", key, "path", c.Request.URL.Path)
			response.Error(c, http.StatusTooManyRequests,
				"Terlalu banyak permintaan. Silakan coba lagi nanti.", "RATE_LIMITED")
			c.Abort()
			return
		}
		c.Next()
	}
}
