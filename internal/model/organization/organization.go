// Package organization holds the persistence entity for organizations (tenants).
// organization_code across the app references organizations.code — it must be a
// known, active organization, not an arbitrary string.
package organization

import (
	"time"

	"gorm.io/gorm"
)

// Organization is a tenant. Code is the identifier used everywhere
// (users.organization_code, retrieval filters, object paths).
type Organization struct {
	Code        string         `gorm:"primaryKey;type:varchar(64)" json:"code"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Active      bool           `gorm:"default:true;index" json:"active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Organization) TableName() string { return "organizations" }
