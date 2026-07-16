package router_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/tararahuuw/ragsytem/internal/config"
	"github.com/tararahuuw/ragsytem/internal/router"
)

// doJSON is a small helper for firing a JSON request at the router.
func doJSON(r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

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

func TestAuth_RegisterLoginFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gdb, _ := newMockGorm(t) // auth uses in-memory store; db only needed for router.New wiring
	r := router.New(&config.Config{AppEnv: "test"}, gdb)

	const (
		reg   = `{"name":"QA","email":"qa@example.com","password":"secret123"}`
		login = `{"email":"qa@example.com","password":"secret123"}`
	)

	// register -> 201
	if w := doJSON(r, http.MethodPost, "/api/v1/auth/register", reg); w.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d (%s)", w.Code, w.Body.String())
	}

	// duplicate register -> 409
	if w := doJSON(r, http.MethodPost, "/api/v1/auth/register", reg); w.Code != http.StatusConflict {
		t.Fatalf("duplicate register: expected 409, got %d", w.Code)
	}

	// login ok -> 200 + token
	w := doJSON(r, http.MethodPost, "/api/v1/auth/login", login)
	if w.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Data.Token == "" {
		t.Fatalf("login: expected token in body, got %s (err=%v)", w.Body.String(), err)
	}

	// wrong password -> 401
	if w := doJSON(r, http.MethodPost, "/api/v1/auth/login",
		`{"email":"qa@example.com","password":"wrong"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: expected 401, got %d", w.Code)
	}

	// invalid payload (bad email, short password) -> 400
	if w := doJSON(r, http.MethodPost, "/api/v1/auth/register",
		`{"name":"x","email":"not-an-email","password":"1"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid register: expected 400, got %d", w.Code)
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
