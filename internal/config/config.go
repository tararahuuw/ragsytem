package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration, loaded from environment variables
// (with an optional .env file for local development).
type Config struct {
	AppName    string
	AppEnv     string // development | staging | production
	ServerHost string
	ServerPort string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration

	// MinIO / object storage
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool

	// Upload
	UploadMaxFileSize   int64         // bytes; hard cap per file
	UploadPreviewExpiry time.Duration // presigned URL lifetime

	// AI / RAG service (PLN AI team). Empty base URL = use mock client.
	AIBaseURL string
	AIToken   string
	AITimeout time.Duration

	// Rate limiting (requests per minute per key; 0 = unlimited for that category)
	RateLimitEnabled      bool
	RateLimitAuthPerMin   int
	RateLimitChatPerMin   int
	RateLimitUploadPerMin int

	// Email / SMTP (empty host = mock: log emails instead of sending)
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	// App base URL (used to build password-reset links) + reset token TTL.
	AppBaseURL       string
	PasswordResetTTL time.Duration

	// Sentry error monitoring (empty DSN = disabled / no-op).
	SentryDSN              string
	SentryEnvironment      string  // defaults to AppEnv when empty
	SentryLevel            string  // minimum slog level forwarded: "error" (default) | "warn"
	SentryTracesSampleRate float64 // 0 = errors only (no performance tracing)

	// Redis / caching (empty RedisAddr or CacheEnabled=false = disabled / no-op).
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	CacheEnabled  bool
	CacheTTL      time.Duration
}

// Load reads configuration from the environment. It silently loads a .env file
// if present, then falls back to sensible defaults for local development.
func Load() *Config {
	_ = godotenv.Load() // ignore error: .env is optional

	return &Config{
		AppName:    getEnv("APP_NAME", "ragsystem"),
		AppEnv:     getEnv("APP_ENV", "development"),
		ServerHost: getEnv("SERVER_HOST", "0.0.0.0"),
		ServerPort: getEnv("SERVER_PORT", "8080"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "ragsystem"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),
		JWTAccessTTL:  getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL: getEnvDuration("JWT_REFRESH_TTL", 7*24*time.Hour),

		MinioEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinioBucket:    getEnv("MINIO_BUCKET", "ragsystem"),
		MinioUseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",

		UploadMaxFileSize:   getEnvInt64("UPLOAD_MAX_FILE_SIZE", 500*1024*1024), // 500 MB (ikut elArch)
		UploadPreviewExpiry: getEnvDuration("UPLOAD_PREVIEW_EXPIRY", 3*time.Hour),

		AIBaseURL: getEnv("AI_BASE_URL", ""), // kosong = mock (kontrak tim AI belum final)
		AIToken:   getEnv("AI_TOKEN", ""),
		AITimeout: getEnvDuration("AI_TIMEOUT", 30*time.Second),

		RateLimitEnabled:      getEnv("RATELIMIT_ENABLED", "true") == "true",
		RateLimitAuthPerMin:   int(getEnvInt64("RATELIMIT_AUTH_PER_MIN", 20)),   // anti brute-force (per IP)
		RateLimitChatPerMin:   int(getEnvInt64("RATELIMIT_CHAT_PER_MIN", 20)),   // AI mahal (per user)
		RateLimitUploadPerMin: int(getEnvInt64("RATELIMIT_UPLOAD_PER_MIN", 300)), // chunked = banyak req (per user)

		SMTPHost:     getEnv("SMTP_HOST", ""), // kosong = mock (log email)
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", "no-reply@ragsystem.local"),

		AppBaseURL:       getEnv("APP_BASE_URL", "http://localhost:8080"),
		PasswordResetTTL: getEnvDuration("PASSWORD_RESET_TTL", 30*time.Minute),

		SentryDSN:              getEnv("SENTRY_DSN", ""), // kosong = Sentry mati (no-op)
		SentryEnvironment:      getEnv("SENTRY_ENVIRONMENT", ""),
		SentryLevel:            getEnv("SENTRY_LEVEL", "error"),
		SentryTracesSampleRate: getEnvFloat("SENTRY_TRACES_SAMPLE_RATE", 0.0),

		RedisAddr:     getEnv("REDIS_ADDR", ""), // kosong = caching mati (no-op)
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       int(getEnvInt64("REDIS_DB", 0)),
		CacheEnabled:  getEnv("CACHE_ENABLED", "true") == "true",
		CacheTTL:      getEnvDuration("CACHE_TTL", 5*time.Minute),
	}
}

// CacheActive reports whether Redis caching should be wired (address set AND not
// explicitly disabled). When false the app uses a no-op cache (always a miss).
func (c *Config) CacheActive() bool {
	return c.CacheEnabled && c.RedisAddr != ""
}

// ServerAddr returns the host:port the HTTP server should bind to.
func (c *Config) ServerAddr() string {
	return c.ServerHost + ":" + c.ServerPort
}

// DSN returns the PostgreSQL connection string used by GORM.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Jakarta",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

// IsProduction reports whether the app runs in production mode.
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// getEnvInt64 reads an integer env var, falling back on empty/invalid input.
func getEnvInt64(key string, fallback int64) int64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

// getEnvFloat reads a float env var, falling back on empty/invalid input.
func getEnvFloat(key string, fallback float64) float64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

// getEnvDuration accepts either a Go duration string (e.g. "15m", "168h") or a
// plain integer interpreted as seconds. Falls back on empty/invalid input.
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	return fallback
}
