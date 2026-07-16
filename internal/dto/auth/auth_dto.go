// Package auth holds the request/response DTOs for the auth module.
package auth

// RegisterRequest is the payload for POST /auth/register.
type RegisterRequest struct {
	Name     string `json:"name" binding:"required" example:"John Doe"`
	Email    string `json:"email" binding:"required,email" example:"john@example.com"`
	Password string `json:"password" binding:"required,min=6" example:"secret123"`
}

// LoginRequest is the payload for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email" example:"john@example.com"`
	Password string `json:"password" binding:"required" example:"secret123"`
}

// UserResponse is the public representation of a user (never exposes password).
type UserResponse struct {
	ID    uint   `json:"id" example:"1"`
	Name  string `json:"name" example:"John Doe"`
	Email string `json:"email" example:"john@example.com"`
}

// LoginResponse is returned on a successful login.
type LoginResponse struct {
	Token string       `json:"token" example:"dummy-token-1-1700000000"`
	User  UserResponse `json:"user"`
}
