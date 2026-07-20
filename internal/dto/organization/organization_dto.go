// Package organization holds request/response DTOs for the organization module.
package organization

import "time"

// CreateRequest is the payload for POST /organizations.
type CreateRequest struct {
	Code        string `json:"code" binding:"required" example:"pln"`
	Name        string `json:"name" binding:"required" example:"PLN (Persero)"`
	Description string `json:"description" example:"Perusahaan Listrik Negara"`
}

// UpdateRequest is the payload for PUT /organizations/{code}. Active is a pointer
// so an omitted field is left unchanged.
type UpdateRequest struct {
	Name        string `json:"name" example:"PLN Persero"`
	Description string `json:"description"`
	Active      *bool  `json:"active" example:"true"`
}

// OrganizationResponse is the public representation of an organization.
type OrganizationResponse struct {
	Code        string    `json:"code" example:"pln"`
	Name        string    `json:"name" example:"PLN (Persero)"`
	Description string    `json:"description"`
	Active      bool      `json:"active" example:"true"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
