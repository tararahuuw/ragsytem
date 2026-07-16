package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/tararahuuw/ragsytem/internal/config"
	appjwt "github.com/tararahuuw/ragsytem/internal/jwt"
	"github.com/tararahuuw/ragsytem/internal/logger"
	"github.com/tararahuuw/ragsytem/internal/response"
)

// Context keys for the authenticated identity extracted from the JWT.
const (
	ctxUserID  = "auth_user_id"
	ctxEmail   = "auth_email"
	ctxOrgCode = "auth_org_code"
)

// JWTAuth validates the Bearer access token and stores the caller's identity
// (user id, email, organizationCode) in the context for downstream handlers.
func JWTAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			response.Error(c, http.StatusUnauthorized, "missing or malformed Authorization header", "UNAUTHORIZED")
			c.Abort()
			return
		}

		tokenStr := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		claims, err := appjwt.Parse(cfg.JWTSecret, tokenStr)
		if err != nil || claims.TokenType != appjwt.TypeAccess {
			logger.FromContext(c.Request.Context()).Warn("auth: invalid access token")
			response.Error(c, http.StatusUnauthorized, "invalid or expired token", "UNAUTHORIZED")
			c.Abort()
			return
		}

		c.Set(ctxUserID, claims.UserID)
		c.Set(ctxEmail, claims.Email)
		c.Set(ctxOrgCode, claims.OrganizationCode)
		c.Next()
	}
}

// CurrentUserID returns the authenticated user's id (0 if unauthenticated).
func CurrentUserID(c *gin.Context) uint {
	if v, ok := c.Get(ctxUserID); ok {
		if id, ok := v.(uint); ok {
			return id
		}
	}
	return 0
}

// CurrentOrgCode returns the authenticated user's organizationCode.
func CurrentOrgCode(c *gin.Context) string {
	if v, ok := c.Get(ctxOrgCode); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
