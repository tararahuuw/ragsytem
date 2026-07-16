package user

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"

	userdto "github.com/tararahuuw/ragsytem/internal/dto/user"
	"github.com/tararahuuw/ragsytem/internal/logger"
	usermodel "github.com/tararahuuw/ragsytem/internal/model/user"
	userrepo "github.com/tararahuuw/ragsytem/internal/repository/user"
)

// Domain errors surfaced to the controller for HTTP mapping.
var (
	ErrUserNotFound = errors.New("user not found")
	ErrForbiddenOrg = errors.New("forbidden: user belongs to a different organization")
)

// Service holds user-management business logic. Every operation is scoped to
// the actor's organizationCode (tenant isolation): a caller may only read/modify
// users within their own organization.
type Service interface {
	GetByID(ctx context.Context, id uint, actorOrg string) (userdto.UserResponse, error)
	Update(ctx context.Context, id uint, actorOrg string, req userdto.UpdateUserRequest) (userdto.UserResponse, error)
	SoftDelete(ctx context.Context, id uint, actorOrg string) error
}

type service struct {
	repo userrepo.Repository
}

// NewService wires a user Service over the given repository.
func NewService(repo userrepo.Repository) Service {
	return &service{repo: repo}
}

// fetchScoped loads a user and enforces tenant isolation against actorOrg.
func (s *service) fetchScoped(ctx context.Context, id uint, actorOrg string) (*usermodel.User, error) {
	log := logger.FromContext(ctx)
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		log.Error("user: failed to look up", "user_id", id, "error", err)
		return nil, err
	}
	if u == nil {
		log.Warn("user: not found", "user_id", id)
		return nil, ErrUserNotFound
	}
	if u.OrganizationCode != actorOrg {
		log.Warn("user: cross-organization access blocked",
			"user_id", id, "target_org", u.OrganizationCode, "actor_org", actorOrg)
		return nil, ErrForbiddenOrg
	}
	return u, nil
}

func (s *service) GetByID(ctx context.Context, id uint, actorOrg string) (userdto.UserResponse, error) {
	u, err := s.fetchScoped(ctx, id, actorOrg)
	if err != nil {
		return userdto.UserResponse{}, err
	}
	return toUserResponse(u), nil
}

func (s *service) Update(ctx context.Context, id uint, actorOrg string, req userdto.UpdateUserRequest) (userdto.UserResponse, error) {
	log := logger.FromContext(ctx)

	u, err := s.fetchScoped(ctx, id, actorOrg)
	if err != nil {
		return userdto.UserResponse{}, err
	}

	if req.Name != "" {
		u.Name = req.Name
	}
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Error("user: failed to hash new password", "user_id", id, "error", err)
			return userdto.UserResponse{}, err
		}
		u.Password = string(hash)
	}

	if err := s.repo.Update(ctx, u); err != nil {
		log.Error("user: failed to update", "user_id", id, "error", err)
		return userdto.UserResponse{}, err
	}

	log.Info("user: updated", "user_id", id, "organization_code", u.OrganizationCode)
	return toUserResponse(u), nil
}

func (s *service) SoftDelete(ctx context.Context, id uint, actorOrg string) error {
	log := logger.FromContext(ctx)

	u, err := s.fetchScoped(ctx, id, actorOrg)
	if err != nil {
		return err
	}
	if err := s.repo.SoftDelete(ctx, u.ID); err != nil {
		log.Error("user: failed to soft delete", "user_id", id, "error", err)
		return err
	}

	log.Info("user: soft deleted", "user_id", id, "organization_code", u.OrganizationCode)
	return nil
}

func toUserResponse(u *usermodel.User) userdto.UserResponse {
	return userdto.UserResponse{
		ID:               u.ID,
		Name:             u.Name,
		Email:            u.Email,
		OrganizationCode: u.OrganizationCode,
	}
}
