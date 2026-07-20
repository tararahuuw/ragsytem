// Package user holds the persistence entity for users.
package user

import (
	"time"

	"gorm.io/gorm"
)

// User is the account entity. Soft-deleted via gorm.DeletedAt (queries exclude
// deleted rows automatically). Email uniqueness among ACTIVE users is enforced
// by a partial unique index created in database.Migrate.
type User struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	Name             string         `json:"name"`
	Email            string         `gorm:"index" json:"email"`
	Password         string         `json:"-"` // bcrypt hash, never serialized
	OrganizationCode string         `gorm:"index" json:"organization_code"`
	Role             string         `gorm:"index;default:user" json:"role"`
	// TokenVersion is embedded in issued JWTs; bumping it (logout / change /
	// reset password) invalidates existing refresh tokens.
	TokenVersion int            `gorm:"default:1" json:"-"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// PasswordResetToken stores a hashed, single-use password-reset token.
type PasswordResetToken struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"index" json:"user_id"`
	TokenHash string     `gorm:"index" json:"-"` // sha-256 of the emailed token
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at"`
	CreatedAt time.Time  `json:"created_at"`
}
