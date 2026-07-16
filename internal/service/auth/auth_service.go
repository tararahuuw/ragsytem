package auth

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	authdto "github.com/tararahuuw/ragsytem/internal/dto/auth"
	userdto "github.com/tararahuuw/ragsytem/internal/dto/user"
	appjwt "github.com/tararahuuw/ragsytem/internal/jwt"
	"github.com/tararahuuw/ragsytem/internal/logger"
	usermodel "github.com/tararahuuw/ragsytem/internal/model/user"
	userrepo "github.com/tararahuuw/ragsytem/internal/repository/user"
)

// Domain errors surfaced to the controller for HTTP mapping.
var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidRefresh     = errors.New("invalid or expired refresh token")
)

// Config carries the JWT settings the auth service needs (decoupled from the
// global config package).
type Config struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// Service holds authentication business logic.
type Service interface {
	Register(ctx context.Context, req authdto.RegisterRequest) (userdto.UserResponse, error)
	Login(ctx context.Context, req authdto.LoginRequest) (authdto.TokenResponse, error)
	Refresh(ctx context.Context, req authdto.RefreshRequest) (authdto.TokenResponse, error)
}

type service struct {
	repo userrepo.Repository
	cfg  Config
}

// NewService wires an auth Service over the given user repository and JWT config.
func NewService(repo userrepo.Repository, cfg Config) Service {
	return &service{repo: repo, cfg: cfg}
}

func (s *service) Register(ctx context.Context, req authdto.RegisterRequest) (userdto.UserResponse, error) {
	log := logger.FromContext(ctx)
	log.Info("register: attempt", "email", req.Email, "organization_code", req.OrganizationCode)

	exists, err := s.repo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		log.Error("register: failed to check email", "email", req.Email, "error", err)
		return userdto.UserResponse{}, err
	}
	if exists {
		log.Warn("register: rejected, email already registered", "email", req.Email)
		return userdto.UserResponse{}, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("register: failed to hash password", "email", req.Email, "error", err)
		return userdto.UserResponse{}, err
	}

	u := &usermodel.User{
		Name:             req.Name,
		Email:            req.Email,
		Password:         string(hash),
		OrganizationCode: req.OrganizationCode,
	}
	if err := s.repo.Create(ctx, u); err != nil {
		log.Error("register: failed to create user", "email", req.Email, "error", err)
		return userdto.UserResponse{}, err
	}

	log.Info("register: success", "user_id", u.ID, "organization_code", u.OrganizationCode)
	return toUserResponse(u), nil
}

func (s *service) Login(ctx context.Context, req authdto.LoginRequest) (authdto.TokenResponse, error) {
	log := logger.FromContext(ctx)
	log.Info("login: attempt", "email", req.Email)

	u, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		log.Error("login: failed to look up user", "email", req.Email, "error", err)
		return authdto.TokenResponse{}, err
	}
	// Generic error whether the email is unknown or the password is wrong
	// (avoids user enumeration).
	if u == nil || bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)) != nil {
		log.Warn("login: rejected, invalid credentials", "email", req.Email)
		return authdto.TokenResponse{}, ErrInvalidCredentials
	}

	tokens, err := s.issueTokens(u)
	if err != nil {
		log.Error("login: failed to issue tokens", "user_id", u.ID, "error", err)
		return authdto.TokenResponse{}, err
	}

	log.Info("login: success", "user_id", u.ID, "organization_code", u.OrganizationCode)
	return tokens, nil
}

func (s *service) Refresh(ctx context.Context, req authdto.RefreshRequest) (authdto.TokenResponse, error) {
	log := logger.FromContext(ctx)

	claims, err := appjwt.Parse(s.cfg.Secret, req.RefreshToken)
	if err != nil || claims.TokenType != appjwt.TypeRefresh {
		log.Warn("refresh: rejected, invalid refresh token")
		return authdto.TokenResponse{}, ErrInvalidRefresh
	}

	// Ensure the user still exists (not soft-deleted) before re-issuing.
	u, err := s.repo.FindByID(ctx, claims.UserID)
	if err != nil {
		log.Error("refresh: failed to look up user", "user_id", claims.UserID, "error", err)
		return authdto.TokenResponse{}, err
	}
	if u == nil {
		log.Warn("refresh: rejected, user no longer exists", "user_id", claims.UserID)
		return authdto.TokenResponse{}, ErrInvalidRefresh
	}

	tokens, err := s.issueTokens(u)
	if err != nil {
		log.Error("refresh: failed to issue tokens", "user_id", u.ID, "error", err)
		return authdto.TokenResponse{}, err
	}

	log.Info("refresh: success", "user_id", u.ID, "organization_code", u.OrganizationCode)
	return tokens, nil
}

func (s *service) issueTokens(u *usermodel.User) (authdto.TokenResponse, error) {
	access, err := appjwt.Generate(s.cfg.Secret, u.ID, u.Email, u.OrganizationCode, appjwt.TypeAccess, s.cfg.AccessTTL)
	if err != nil {
		return authdto.TokenResponse{}, err
	}
	refresh, err := appjwt.Generate(s.cfg.Secret, u.ID, u.Email, u.OrganizationCode, appjwt.TypeRefresh, s.cfg.RefreshTTL)
	if err != nil {
		return authdto.TokenResponse{}, err
	}
	return authdto.TokenResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.cfg.AccessTTL.Seconds()),
	}, nil
}

func toUserResponse(u *usermodel.User) userdto.UserResponse {
	return userdto.UserResponse{
		ID:               u.ID,
		Name:             u.Name,
		Email:            u.Email,
		OrganizationCode: u.OrganizationCode,
	}
}
