// Package upload holds persistence entities for the file-upload module:
// an audit/dedup log and per-role upload quota.
package upload

import "time"

// UploadLog records every completed upload — used both for audit and for
// SHA-256 deduplication.
type UploadLog struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	SessionID        string    `gorm:"index" json:"session_id"`
	FileName         string    `json:"file_name"`
	Sha256           string    `gorm:"index" json:"sha256"`
	FileSize         int64     `json:"file_size"`
	TotalChunks      int       `json:"total_chunks"`
	ObjectPath       string    `json:"object_path"`
	Status           string    `gorm:"index" json:"status"` // completed | failed
	UserID           uint      `gorm:"index" json:"user_id"`
	OrganizationCode string    `gorm:"index" json:"organization_code"`
	CreatedAt        time.Time `json:"created_at"`
}

// Upload status constants.
const (
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// UploadQuotaConfig is the per-role upload limit (monthly + lifetime).
type UploadQuotaConfig struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	Role          string `gorm:"uniqueIndex" json:"role"`
	MonthlyLimit  int    `json:"monthly_limit"`
	LifetimeLimit int    `json:"lifetime_limit"`
	Enabled       bool   `gorm:"default:true" json:"enabled"`
}

// UploadQuotaUsage tracks a user's upload counts (one row per user per month;
// lifetime carried forward).
type UploadQuotaUsage struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"uniqueIndex:idx_usage_user_month" json:"user_id"`
	YearMonth     string    `gorm:"uniqueIndex:idx_usage_user_month" json:"year_month"` // "2006-01"
	MonthlyCount  int       `json:"monthly_count"`
	LifetimeCount int       `json:"lifetime_count"`
	UpdatedAt     time.Time `json:"updated_at"`
}
