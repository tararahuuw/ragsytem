package database

import (
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/tararahuuw/ragsytem/internal/config"
	chatmodel "github.com/tararahuuw/ragsytem/internal/model/chat"
	orgmodel "github.com/tararahuuw/ragsytem/internal/model/organization"
	uploadmodel "github.com/tararahuuw/ragsytem/internal/model/upload"
	usermodel "github.com/tararahuuw/ragsytem/internal/model/user"
	"github.com/tararahuuw/ragsytem/internal/rbac"
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
	if err := db.AutoMigrate(
		&usermodel.User{},
		&uploadmodel.UploadLog{},
		&uploadmodel.UploadQuotaConfig{},
		&uploadmodel.UploadQuotaUsage{},
		&chatmodel.Session{},
		&chatmodel.Message{},
		&orgmodel.Organization{},
		&usermodel.PasswordResetToken{},
	); err != nil {
		return err
	}
	// Backfill token_version for rows created before the column existed.
	if err := db.Exec(`UPDATE users SET token_version = 1 WHERE token_version IS NULL OR token_version = 0`).Error; err != nil {
		return err
	}
	// Seed organizations from org codes already present on users, so existing
	// data stays valid once org validation is enforced.
	if err := db.Exec(`
		INSERT INTO organizations (code, name, active, created_at, updated_at)
		SELECT DISTINCT organization_code, organization_code, true, now(), now()
		FROM users
		WHERE organization_code IS NOT NULL AND organization_code <> ''
		ON CONFLICT (code) DO NOTHING`).Error; err != nil {
		return err
	}
	// Backfill role for rows created before the column existed.
	if err := db.Exec(`UPDATE users SET role = 'user' WHERE role IS NULL OR role = ''`).Error; err != nil {
		return err
	}
	// Partial unique index: email must be unique among ACTIVE (non-deleted)
	// users, so a soft-deleted email can be reused on re-registration.
	if err := db.Exec(
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_active ON users (email) WHERE deleted_at IS NULL`,
	).Error; err != nil {
		return err
	}
	return seedUploadQuota(db)
}

// seedUploadQuota inserts default per-role upload limits if absent (idempotent).
func seedUploadQuota(db *gorm.DB) error {
	defaults := []uploadmodel.UploadQuotaConfig{
		{Role: rbac.RoleUser, MonthlyLimit: 100, LifetimeLimit: 1000, Enabled: true},
		{Role: rbac.RoleAdmin, MonthlyLimit: 1000, LifetimeLimit: 100000, Enabled: true},
	}
	for _, d := range defaults {
		if err := db.Where(uploadmodel.UploadQuotaConfig{Role: d.Role}).
			FirstOrCreate(&d).Error; err != nil {
			return err
		}
	}
	return nil
}
