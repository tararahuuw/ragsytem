package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	authdto "github.com/tararahuuw/ragsytem/internal/dto/auth"
	"github.com/tararahuuw/ragsytem/internal/logger"
	authmodel "github.com/tararahuuw/ragsytem/internal/model/auth"
	authrepo "github.com/tararahuuw/ragsytem/internal/repository/auth"
)

// Domain errors surfaced to the controller for HTTP mapping.
var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

// Service holds auth business logic.
type Service interface {
	Register(ctx context.Context, req authdto.RegisterRequest) (authdto.UserResponse, error)
	Login(ctx context.Context, req authdto.LoginRequest) (authdto.LoginResponse, error)
}

type service struct {
	repo authrepo.Repository
}

// NewService wires an auth Service over the given repository.
func NewService(repo authrepo.Repository) Service {
	return &service{repo: repo}
}

func (s *service) Register(ctx context.Context, req authdto.RegisterRequest) (authdto.UserResponse, error) {
	log := logger.FromContext(ctx)
	log.Info("register: attempt", "email", req.Email)

	if s.repo.ExistsByEmail(req.Email) {
		log.Warn("register: rejected, email already registered", "email", req.Email)
		return authdto.UserResponse{}, ErrEmailTaken
	}

	// DUMMY: password stored as-is. A real implementation MUST hash it
	// (e.g. bcrypt) before persisting.
	user, err := s.repo.Create(&authmodel.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		log.Error("register: failed to create user", "email", req.Email, "error", err)
		return authdto.UserResponse{}, err
	}

	log.Info("register: success", "user_id", user.ID, "email", user.Email)
	return toUserResponse(user), nil
}

func (s *service) Login(ctx context.Context, req authdto.LoginRequest) (authdto.LoginResponse, error) {
	log := logger.FromContext(ctx)
	log.Info("login: attempt", "email", req.Email)

	user, err := s.repo.FindByEmail(req.Email)
	if err != nil {
		log.Error("login: failed to look up user", "email", req.Email, "error", err)
		return authdto.LoginResponse{}, err
	}
	// Same generic error whether the email is unknown or the password is wrong
	// (avoids user enumeration).
	if user == nil || user.Password != req.Password {
		log.Warn("login: rejected, invalid credentials", "email", req.Email)
		return authdto.LoginResponse{}, ErrInvalidCredentials
	}

	// DUMMY token. Replace with a signed JWT in a real implementation.
	token := fmt.Sprintf("dummy-token-%d-%d", user.ID, time.Now().Unix())

	log.Info("login: success", "user_id", user.ID, "email", user.Email)
	return authdto.LoginResponse{
		Token: token,
		User:  toUserResponse(user),
	}, nil
}

func toUserResponse(u *authmodel.User) authdto.UserResponse {
	return authdto.UserResponse{ID: u.ID, Name: u.Name, Email: u.Email}
}
