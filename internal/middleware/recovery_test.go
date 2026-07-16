package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/tararahuuw/ragsytem/internal/middleware"
)

func TestRecovery_CatchesPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(middleware.RequestID(), middleware.Recovery())
	r.GET("/boom", func(c *gin.Context) {
		panic("something exploded")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	r.ServeHTTP(w, req)

	// Server must not crash; client gets a standardized 500 envelope.
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	var body struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v (%s)", err, w.Body.String())
	}
	if body.Success || body.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected error envelope: %s", w.Body.String())
	}
	// The safety net must not leak the raw stack trace to the client.
	if bodyStr := w.Body.String(); strings.Contains(bodyStr, "goroutine") || strings.Contains(bodyStr, "something exploded") {
		t.Fatalf("response leaked internal details: %s", bodyStr)
	}
}
