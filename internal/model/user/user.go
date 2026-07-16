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
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}
