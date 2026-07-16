package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	authdto "github.com/tararahuuw/ragsytem/internal/dto/auth"
	"github.com/tararahuuw/ragsytem/internal/logger"
	"github.com/tararahuuw/ragsytem/internal/response"
	authsvc "github.com/tararahuuw/ragsytem/internal/service/auth"
)

// Controller exposes authentication endpoints.
type Controller struct {
	svc authsvc.Service
}

// NewController wires an auth Controller over the given service.
func NewController(svc authsvc.Service) *Controller {
	return &Controller{svc: svc}
}

// Register godoc
//
//	@Summary		Register a new user (admin only)
//	@Description	Admin-only. Creates a user (role always "user") in the given organizationCode, bcrypt-hashed.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		authdto.RegisterRequest	true	"Registration payload"
//	@Success		201		{object}	response.BaseResponse{data=user.UserResponse}
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		401		{object}	response.ErrorResponse
//	@Failure		403		{object}	response.ErrorResponse
//	@Failure		409		{object}	response.ErrorResponse
//	@Router			/auth/register [post]
func (c *Controller) Register(ctx *gin.Context) {
	log := logger.FromContext(ctx.Request.Context())

	var req authdto.RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Warn("register: invalid payload", "error", err.Error())
		response.ValidationError(ctx, "invalid request payload", err.Error())
		return
	}

	user, err := c.svc.Register(ctx.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, authsvc.ErrEmailTaken):
			response.Error(ctx, http.StatusConflict, err.Error(), "EMAIL_TAKEN")
		default:
			log.Error("register: unexpected error", "error", err)
			response.Error(ctx, http.StatusInternalServerError, "failed to register user", "INTERNAL_ERROR")
		}
		return
	}

	response.Created(ctx, "user registered", user)
}

// Login godoc
//
//	@Summary		Login
//	@Description	Validates credentials and returns access + refresh JWT (with organizationCode claim).
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		authdto.LoginRequest	true	"Login payload"
//	@Success		200		{object}	response.BaseResponse{data=authdto.TokenResponse}
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		401		{object}	response.ErrorResponse
//	@Router			/auth/login [post]
func (c *Controller) Login(ctx *gin.Context) {
	log := logger.FromContext(ctx.Request.Context())

	var req authdto.LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Warn("login: invalid payload", "error", err.Error())
		response.ValidationError(ctx, "invalid request payload", err.Error())
		return
	}

	tokens, err := c.svc.Login(ctx.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, authsvc.ErrInvalidCredentials):
			response.Error(ctx, http.StatusUnauthorized, err.Error(), "INVALID_CREDENTIALS")
		default:
			log.Error("login: unexpected error", "error", err)
			response.Error(ctx, http.StatusInternalServerError, "failed to login", "INTERNAL_ERROR")
		}
		return
	}

	response.OK(ctx, "login success", tokens)
}

// Refresh godoc
//
//	@Summary		Refresh access token
//	@Description	Exchanges a valid refresh token for a new access + refresh pair.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		authdto.RefreshRequest	true	"Refresh payload"
//	@Success		200		{object}	response.BaseResponse{data=authdto.TokenResponse}
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		401		{object}	response.ErrorResponse
//	@Router			/auth/refresh [post]
func (c *Controller) Refresh(ctx *gin.Context) {
	log := logger.FromContext(ctx.Request.Context())

	var req authdto.RefreshRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Warn("refresh: invalid payload", "error", err.Error())
		response.ValidationError(ctx, "invalid request payload", err.Error())
		return
	}

	tokens, err := c.svc.Refresh(ctx.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, authsvc.ErrInvalidRefresh):
			response.Error(ctx, http.StatusUnauthorized, err.Error(), "INVALID_REFRESH_TOKEN")
		default:
			log.Error("refresh: unexpected error", "error", err)
			response.Error(ctx, http.StatusInternalServerError, "failed to refresh token", "INTERNAL_ERROR")
		}
		return
	}

	response.OK(ctx, "token refreshed", tokens)
}
