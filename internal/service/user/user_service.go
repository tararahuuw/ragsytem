package user

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"

	userdto "github.com/tararahuuw/ragsytem/internal/dto/user"
	"github.com/tararahuuw/ragsytem/internal/logger"
	usermodel "github.com/tararahuuw/ragsytem/internal/model/user"
	"github.com/tararahuuw/ragsytem/internal/rbac"
	userrepo "github.com/tararahuuw/ragsytem/internal/repository/user"
)

// Domain errors surfaced to the controller for HTTP mapping.
var (
	ErrUserNotFound = errors.New("user not found")
	ErrForbiddenOrg = errors.New("forbidden: user belongs to a different organization")
)

// Service holds user-management business logic. Non-admin callers are scoped to
// their own organizationCode (tenant isolation); admins (super-admin) bypass the
// org check and may act across organizations.
type Service interface {
	GetByID(ctx context.Context, id uint, actorOrg, actorRole string) (userdto.UserResponse, error)
	Update(ctx context.Context, id uint, actorOrg, actorRole string, req userdto.UpdateUserRequest) (userdto.UserResponse, error)
	SoftDelete(ctx context.Context, id uint, actorOrg, actorRole string) error
}

type service struct {
	repo userrepo.Repository
}

// NewService wires a user Service over the given repository.
func NewService(repo userrepo.Repository) Service {
	return &service{repo: repo}
}

// fetchScoped loads a user and enforces tenant isolation unless the actor is an
// admin (super-admin bypasses the org check).
func (s *service) fetchScoped(ctx context.Context, id uint, actorOrg, actorRole string) (*usermodel.User, error) {
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
	if actorRole != rbac.RoleAdmin && u.OrganizationCode != actorOrg {
		log.Warn("user: cross-organization access blocked",
			"user_id", id, "target_org", u.OrganizationCode, "actor_org", actorOrg)
		return nil, ErrForbiddenOrg
	}
	return u, nil
}

func (s *service) GetByID(ctx context.Context, id uint, actorOrg, actorRole string) (userdto.UserResponse, error) {
	u, err := s.fetchScoped(ctx, id, actorOrg, actorRole)
	if err != nil {
		return userdto.UserResponse{}, err
	}
	return toUserResponse(u), nil
}

func (s *service) Update(ctx context.Context, id uint, actorOrg, actorRole string, req userdto.UpdateUserRequest) (userdto.UserResponse, error) {
	log := logger.FromContext(ctx)

	u, err := s.fetchScoped(ctx, id, actorOrg, actorRole)
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

func (s *service) SoftDelete(ctx context.Context, id uint, actorOrg, actorRole string) error {
	log := logger.FromContext(ctx)

	u, err := s.fetchScoped(ctx, id, actorOrg, actorRole)
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
		Role:             u.Role,
	}
}
