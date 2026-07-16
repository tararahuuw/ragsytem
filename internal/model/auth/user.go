// Package auth holds the persistence entities for the auth module.
package auth

import "time"

// User is the account entity.
//
// NOTE: the current auth module is a DUMMY that stores users in-memory (see
// internal/repository/auth). When switching to persistence, add
// &auth.User{} to database.Migrate and back the repository with GORM.
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Email     string    `gorm:"uniqueIndex" json:"email"`
	Password  string    `json:"-"` // never serialized
	CreatedAt time.Time `json:"created_at"`
}
