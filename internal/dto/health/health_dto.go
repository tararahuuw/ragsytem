// Package health holds the response DTOs for the health module.
package health

// HealthResponse describes the service health payload.
type HealthResponse struct {
	Status   string `json:"status" example:"ok"`
	Database string `json:"database" example:"up"`
}
