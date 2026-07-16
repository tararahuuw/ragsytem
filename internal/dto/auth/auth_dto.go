// Package auth holds the request/response DTOs for the auth module.
package auth

// RegisterRequest is the payload for POST /auth/register.
type RegisterRequest struct {
	Name             string `json:"name" binding:"required" example:"John Doe"`
	Email            string `json:"email" binding:"required,email" example:"john@example.com"`
	Password         string `json:"password" binding:"required,min=6" example:"secret123"`
	OrganizationCode string `json:"organization_code" binding:"required" example:"pln"`
}

// LoginRequest is the payload for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email" example:"john@example.com"`
	Password string `json:"password" binding:"required" example:"secret123"`
}

// RefreshRequest is the payload for POST /auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// TokenResponse carries the issued JWT pair.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type" example:"Bearer"`
	ExpiresIn    int64  `json:"expires_in" example:"900"` // access token TTL in seconds
}
