package database

import (
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/tararahuuw/ragsytem/internal/config"
	usermodel "github.com/tararahuuw/ragsytem/internal/model/user"
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
	if err := db.AutoMigrate(&usermodel.User{}); err != nil {
		return err
	}
	// Backfill role for rows created before the column existed.
	if err := db.Exec(`UPDATE users SET role = 'user' WHERE role IS NULL OR role = ''`).Error; err != nil {
		return err
	}
	// Partial unique index: email must be unique among ACTIVE (non-deleted)
	// users, so a soft-deleted email can be reused on re-registration.
	return db.Exec(
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_active ON users (email) WHERE deleted_at IS NULL`,
	).Error
}
