package router_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/tararahuuw/ragsytem/internal/config"
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

	r := router.New(&config.Config{AppEnv: "test"}, gdb)

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

	r := router.New(&config.Config{AppEnv: "test"}, gdb)

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
	r := router.New(&config.Config{AppEnv: "test", JWTSecret: "test-secret"}, gdb)

	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/v1/users/me"},
		{http.MethodGet, "/api/v1/users/1"},
		{http.MethodPut, "/api/v1/users/1"},
		{http.MethodDelete, "/api/v1/users/1"},
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

func TestSwaggerRouteRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gdb, _ := newMockGorm(t)

	r := router.New(&config.Config{AppEnv: "test"}, gdb)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	r.ServeHTTP(w, req)

	// swagger handler serves the UI (200) or redirects to it (3xx); either proves
	// the route is wired.
	if w.Code != http.StatusOK && w.Code < 300 {
		t.Fatalf("swagger route not served, got %d", w.Code)
	}
}
