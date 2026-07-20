package organization

import (
	"context"
	"errors"
	"regexp"
	"strings"

	orgdto "github.com/tararahuuw/ragsytem/internal/dto/organization"
	"github.com/tararahuuw/ragsytem/internal/logger"
	orgmodel "github.com/tararahuuw/ragsytem/internal/model/organization"
	orgrepo "github.com/tararahuuw/ragsytem/internal/repository/organization"
)

// Domain errors surfaced to the controller for HTTP mapping.
var (
	ErrOrgNotFound = errors.New("organization not found")
	ErrOrgExists   = errors.New("organization code already exists")
	ErrOrgHasUsers = errors.New("organization still has active users")
	ErrInvalidCode = errors.New("invalid organization code")
)

// codeRe restricts org codes to a safe, predictable set (used as a key everywhere).
var codeRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{2,64}$`)

// Service holds organization business logic.
type Service interface {
	Create(ctx context.Context, req orgdto.CreateRequest) (orgdto.OrganizationResponse, error)
	Get(ctx context.Context, code string) (orgdto.OrganizationResponse, error)
	List(ctx context.Context) ([]orgdto.OrganizationResponse, error)
	Update(ctx context.Context, code string, req orgdto.UpdateRequest) (orgdto.OrganizationResponse, error)
	Delete(ctx context.Context, code string) error
}

type service struct {
	repo orgrepo.Repository
}

// NewService wires an organization Service.
func NewService(repo orgrepo.Repository) Service {
	return &service{repo: repo}
}

// NormalizeCode trims surrounding whitespace so "pln " and "pln" are the same
// (the exact typo class this module guards against). Case is preserved.
func NormalizeCode(code string) string { return strings.TrimSpace(code) }

func (s *service) Create(ctx context.Context, req orgdto.CreateRequest) (orgdto.OrganizationResponse, error) {
	log := logger.FromContext(ctx)
	code := NormalizeCode(req.Code)
	if !codeRe.MatchString(code) {
		log.Warn("organization: invalid code", "code", req.Code)
		return orgdto.OrganizationResponse{}, ErrInvalidCode
	}

	existing, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		log.Error("organization: lookup failed", "code", code, "error", err)
		return orgdto.OrganizationResponse{}, err
	}
	if existing != nil {
		log.Warn("organization: code already exists", "code", code)
		return orgdto.OrganizationResponse{}, ErrOrgExists
	}

	o := &orgmodel.Organization{
		Code:        code,
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		Active:      true,
	}
	if err := s.repo.Create(ctx, o); err != nil {
		log.Error("organization: create failed", "code", code, "error", err)
		return orgdto.OrganizationResponse{}, err
	}
	log.Info("organization: created", "code", code)
	return toResponse(o), nil
}

func (s *service) Get(ctx context.Context, code string) (orgdto.OrganizationResponse, error) {
	o, err := s.load(ctx, code)
	if err != nil {
		return orgdto.OrganizationResponse{}, err
	}
	return toResponse(o), nil
}

func (s *service) List(ctx context.Context) ([]orgdto.OrganizationResponse, error) {
	orgs, err := s.repo.List(ctx)
	if err != nil {
		logger.FromContext(ctx).Error("organization: list failed", "error", err)
		return nil, err
	}
	res := make([]orgdto.OrganizationResponse, 0, len(orgs))
	for i := range orgs {
		res = append(res, toResponse(&orgs[i]))
	}
	return res, nil
}

func (s *service) Update(ctx context.Context, code string, req orgdto.UpdateRequest) (orgdto.OrganizationResponse, error) {
	log := logger.FromContext(ctx)
	o, err := s.load(ctx, code)
	if err != nil {
		return orgdto.OrganizationResponse{}, err
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		o.Name = name
	}
	o.Description = req.Description
	if req.Active != nil {
		o.Active = *req.Active
	}
	if err := s.repo.Update(ctx, o); err != nil {
		log.Error("organization: update failed", "code", o.Code, "error", err)
		return orgdto.OrganizationResponse{}, err
	}
	log.Info("organization: updated", "code", o.Code, "active", o.Active)
	return toResponse(o), nil
}

func (s *service) Delete(ctx context.Context, code string) error {
	log := logger.FromContext(ctx)
	o, err := s.load(ctx, code)
	if err != nil {
		return err
	}
	n, err := s.repo.CountUsers(ctx, o.Code)
	if err != nil {
		log.Error("organization: count users failed", "code", o.Code, "error", err)
		return err
	}
	if n > 0 {
		log.Warn("organization: delete blocked, still has users", "code", o.Code, "users", n)
		return ErrOrgHasUsers
	}
	if err := s.repo.SoftDelete(ctx, o.Code); err != nil {
		log.Error("organization: delete failed", "code", o.Code, "error", err)
		return err
	}
	log.Info("organization: deleted", "code", o.Code)
	return nil
}

func (s *service) load(ctx context.Context, code string) (*orgmodel.Organization, error) {
	o, err := s.repo.GetByCode(ctx, NormalizeCode(code))
	if err != nil {
		logger.FromContext(ctx).Error("organization: lookup failed", "code", code, "error", err)
		return nil, err
	}
	if o == nil {
		return nil, ErrOrgNotFound
	}
	return o, nil
}

func toResponse(o *orgmodel.Organization) orgdto.OrganizationResponse {
	return orgdto.OrganizationResponse{
		Code:        o.Code,
		Name:        o.Name,
		Description: o.Description,
		Active:      o.Active,
		CreatedAt:   o.CreatedAt,
		UpdatedAt:   o.UpdatedAt,
	}
}
