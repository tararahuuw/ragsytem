package database

import (
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/tararahuuw/ragsytem/internal/config"
)

// Connect opens a pooled GORM connection to PostgreSQL.
func Connect(cfg *config.Config) (*gorm.DB, error) {
	logLevel := logger.Info
	if cfg.IsProduction() {
		logLevel = logger.Warn
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	slog.Info("database connected", "host", cfg.DBHost, "db", cfg.DBName)
	return db, nil
}

// Migrate runs GORM auto-migration for all registered models.
// Register new models here as the domain grows, e.g.:
//
//	return db.AutoMigrate(&model.Document{}, &model.Chunk{})
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
	// no models yet
	)
}
