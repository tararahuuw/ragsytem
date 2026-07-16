package health

import (
	"context"

	healthdto "github.com/tararahuuw/ragsytem/internal/dto/health"
	healthrepo "github.com/tararahuuw/ragsytem/internal/repository/health"
)

// Service holds health-check business logic.
type Service interface {
	Check(ctx context.Context) healthdto.HealthResponse
}

type service struct {
	repo healthrepo.Repository
}

// NewService wires a health Service over the given repository.
func NewService(repo healthrepo.Repository) Service {
	return &service{repo: repo}
}

func (s *service) Check(ctx context.Context) healthdto.HealthResponse {
	res := healthdto.HealthResponse{Status: "ok", Database: "up"}
	if err := s.repo.Ping(ctx); err != nil {
		res.Status = "degraded"
		res.Database = "down"
	}
	return res
}
