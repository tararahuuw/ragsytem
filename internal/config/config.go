package config

import (
	"fmt"
	"os"

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
	}
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
