package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tararahuuw/ragsytem/internal/config"
	appjwt "github.com/tararahuuw/ragsytem/internal/jwt"
	"github.com/tararahuuw/ragsytem/internal/middleware"
)

func newProtectedRouter(secret string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID())
	grp := r.Group("/x")
	grp.Use(middleware.JWTAuth(&config.Config{JWTSecret: secret}))
	grp.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"org": middleware.CurrentOrgCode(c),
			"uid": middleware.CurrentUserID(c),
		})
	})
	return r
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	r := newProtectedRouter("s")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x/ping", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuth_RefreshTokenRejected(t *testing.T) {
	secret := "s"
	// A refresh token must not be accepted as an access token.
	tok, _ := appjwt.Generate(secret, 1, "a@b.com", "pln", appjwt.TypeRefresh, time.Minute)
	r := newProtectedRouter(secret)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x/ping", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for refresh token, got %d", w.Code)
	}
}

func TestJWTAuth_ValidAccessToken(t *testing.T) {
	secret := "s"
	tok, _ := appjwt.Generate(secret, 7, "a@b.com", "pln", appjwt.TypeAccess, time.Minute)
	r := newProtectedRouter(secret)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x/ping", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	// organizationCode from the token must be available to the handler.
	if body := w.Body.String(); !strings.Contains(body, `"org":"pln"`) {
		t.Fatalf("expected org=pln in context, got %s", body)
	}
}
