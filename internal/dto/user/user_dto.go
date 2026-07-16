// Package user holds the request/response DTOs for the user module.
package user

// UserResponse is the public representation of a user (never exposes password).
type UserResponse struct {
	ID               uint   `json:"id" example:"1"`
	Name             string `json:"name" example:"John Doe"`
	Email            string `json:"email" example:"john@example.com"`
	OrganizationCode string `json:"organization_code" example:"pln"`
}

// UpdateUserRequest is the payload for PUT /users/{id}. All fields optional;
// only provided fields are updated. Email & organizationCode are immutable.
type UpdateUserRequest struct {
	Name     string `json:"name" example:"John Updated"`
	Password string `json:"password" example:"newSecret123"`
}
