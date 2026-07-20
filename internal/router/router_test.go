package router_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/tararahuuw/ragsytem/internal/config"
	appjwt "github.com/tararahuuw/ragsytem/internal/jwt"
	"github.com/tararahuuw/ragsytem/internal/router"
)

// newMockGorm returns a GORM DB backed by go-sqlmock so we can exercise the
// full HTTP stack without a real PostgreSQL instance.
func newMockGorm(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	mock.ExpectPing() // gorm.Open pings the DB during initialization
	gdb, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return gdb, mock
}

func TestHealthz_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gdb, mock := newMockGorm(t)
	mock.ExpectPing() // DB reachable

	r := router.New(&config.Config{AppEnv: "test"}, gdb, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["status"] != "ok" || body["database"] != "up" {
		t.Fatalf("unexpected health body: %v", body)
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID header from middleware")
	}
}

func TestHealthz_DBDown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gdb, mock := newMockGorm(t)
	mock.ExpectPing().WillReturnError(sqlmock.ErrCancelled) // DB unreachable

	r := router.New(&config.Config{AppEnv: "test"}, gdb, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// Protected /users routes must reject requests without a valid Bearer token.
// (Full auth happy-path is covered by the live smoke test / testing playbook,
// since it needs a real PostgreSQL.)
func TestUsers_RequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gdb, _ := newMockGorm(t)
	r := router.New(&config.Config{AppEnv: "test", JWTSecret: "test-secret"}, gdb, nil)

	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/v1/users/me"},
		{http.MethodGet, "/api/v1/users/1"},
		{http.MethodPut, "/api/v1/users/1"},
		{http.MethodDelete, "/api/v1/users/1"},
		{http.MethodGet, "/api/v1/documents"},
		{http.MethodGet, "/api/v1/documents/1"},
		{http.MethodPost, "/api/v1/chat/ask"},
		{http.MethodGet, "/api/v1/chat/sessions"},
		{http.MethodGet, "/api/v1/chat/sessions/abc"},
		{http.MethodDelete, "/api/v1/chat/sessions/abc"},
		{http.MethodGet, "/api/v1/organizations"},
		{http.MethodGet, "/api/v1/organizations/pln"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: expected 401 without token, got %d", tc.method, tc.path, w.Code)
		}
	}
}

// Register and soft-delete are admin-only: unauthenticated -> 401, non-admin
// (user role) -> 403. (These abort in middleware before any DB access.)
func TestRBAC_AdminOnlyRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret"
	gdb, _ := newMockGorm(t)
	r := router.New(&config.Config{AppEnv: "test", JWTSecret: secret}, gdb, nil)

	userTok, _ := appjwt.Generate(secret, 1, "u@x.com", "pln", "user", 1, appjwt.TypeAccess, time.Minute)

	cases := []struct {
		name, method, path, auth string
		want                     int
	}{
		{"register no token", http.MethodPost, "/api/v1/auth/register", "", http.StatusUnauthorized},
		{"register user role", http.MethodPost, "/api/v1/auth/register", "Bearer " + userTok, http.StatusForbidden},
		{"bulk register no token", http.MethodPost, "/api/v1/auth/register/bulk", "", http.StatusUnauthorized},
		{"bulk register user role", http.MethodPost, "/api/v1/auth/register/bulk", "Bearer " + userTok, http.StatusForbidden},
		{"delete no token", http.MethodDelete, "/api/v1/users/1", "", http.StatusUnauthorized},
		{"delete user role", http.MethodDelete, "/api/v1/users/1", "Bearer " + userTok, http.StatusForbidden},
		{"role change no token", http.MethodPatch, "/api/v1/users/1/role", "", http.StatusUnauthorized},
		{"role change user role", http.MethodPatch, "/api/v1/users/1/role", "Bearer " + userTok, http.StatusForbidden},
		{"org create no token", http.MethodPost, "/api/v1/organizations", "", http.StatusUnauthorized},
		{"org create user role", http.MethodPost, "/api/v1/organizations", "Bearer " + userTok, http.StatusForbidden},
		{"org delete user role", http.MethodDelete, "/api/v1/organizations/pln", "Bearer " + userTok, http.StatusForbidden},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.auth != "" {
			req.Header.Set("Authorization", tc.auth)
		}
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("%s: expected %d, got %d", tc.name, tc.want, w.Code)
		}
	}
}

func TestSwaggerRouteRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gdb, _ := newMockGorm(t)

	r := router.New(&config.Config{AppEnv: "test"}, gdb, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	r.ServeHTTP(w, req)

	// swagger handler serves the UI (200) or redirects to it (3xx); either proves
	// the route is wired.
	if w.Code != http.StatusOK && w.Code < 300 {
		t.Fatalf("swagger route not served, got %d", w.Code)
	}
}
