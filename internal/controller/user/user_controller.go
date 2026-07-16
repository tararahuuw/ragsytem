package user

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	userdto "github.com/tararahuuw/ragsytem/internal/dto/user"
	"github.com/tararahuuw/ragsytem/internal/logger"
	"github.com/tararahuuw/ragsytem/internal/middleware"
	"github.com/tararahuuw/ragsytem/internal/response"
	usersvc "github.com/tararahuuw/ragsytem/internal/service/user"
)

// Controller exposes user-management endpoints (all require a valid JWT).
type Controller struct {
	svc usersvc.Service
}

// NewController wires a user Controller over the given service.
func NewController(svc usersvc.Service) *Controller {
	return &Controller{svc: svc}
}

// Me godoc
//
//	@Summary		Get current user
//	@Description	Returns the authenticated user (identified by the JWT).
//	@Tags			user
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.BaseResponse{data=userdto.UserResponse}
//	@Failure		401	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/users/me [get]
func (c *Controller) Me(ctx *gin.Context) {
	id := middleware.CurrentUserID(ctx)
	org := middleware.CurrentOrgCode(ctx)
	role := middleware.CurrentRole(ctx)

	res, err := c.svc.GetByID(ctx.Request.Context(), id, org, role)
	if err != nil {
		c.mapError(ctx, err, "get current user")
		return
	}
	response.OK(ctx, "ok", res)
}

// GetByID godoc
//
//	@Summary		Get user by id
//	@Description	Returns a user within the caller's organization (tenant-scoped).
//	@Tags			user
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"User ID"
//	@Success		200	{object}	response.BaseResponse{data=userdto.UserResponse}
//	@Failure		401	{object}	response.ErrorResponse
//	@Failure		403	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/users/{id} [get]
func (c *Controller) GetByID(ctx *gin.Context) {
	id, ok := c.parseID(ctx)
	if !ok {
		return
	}

	res, err := c.svc.GetByID(ctx.Request.Context(), id, middleware.CurrentOrgCode(ctx), middleware.CurrentRole(ctx))
	if err != nil {
		c.mapError(ctx, err, "get user")
		return
	}
	response.OK(ctx, "ok", res)
}

// Update godoc
//
//	@Summary		Update user
//	@Description	Updates name and/or password of a user within the caller's organization.
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int							true	"User ID"
//	@Param			payload	body		userdto.UpdateUserRequest	true	"Fields to update"
//	@Success		200		{object}	response.BaseResponse{data=userdto.UserResponse}
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		401		{object}	response.ErrorResponse
//	@Failure		403		{object}	response.ErrorResponse
//	@Failure		404		{object}	response.ErrorResponse
//	@Router			/users/{id} [put]
func (c *Controller) Update(ctx *gin.Context) {
	log := logger.FromContext(ctx.Request.Context())

	id, ok := c.parseID(ctx)
	if !ok {
		return
	}

	var req userdto.UpdateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Warn("update user: invalid payload", "error", err.Error())
		response.ValidationError(ctx, "invalid request payload", err.Error())
		return
	}

	res, err := c.svc.Update(ctx.Request.Context(), id, middleware.CurrentOrgCode(ctx), middleware.CurrentRole(ctx), req)
	if err != nil {
		c.mapError(ctx, err, "update user")
		return
	}
	response.OK(ctx, "user updated", res)
}

// Delete godoc
//
//	@Summary		Soft delete user (admin only)
//	@Description	Admin-only. Soft-deletes a user (sets deleted_at). Admin is global (any organization).
//	@Tags			user
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"User ID"
//	@Success		200	{object}	response.BaseResponse
//	@Failure		401	{object}	response.ErrorResponse
//	@Failure		403	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/users/{id} [delete]
func (c *Controller) Delete(ctx *gin.Context) {
	id, ok := c.parseID(ctx)
	if !ok {
		return
	}

	if err := c.svc.SoftDelete(ctx.Request.Context(), id, middleware.CurrentOrgCode(ctx), middleware.CurrentRole(ctx)); err != nil {
		c.mapError(ctx, err, "delete user")
		return
	}
	response.OK(ctx, "user deleted", nil)
}

// parseID reads and validates the :id path param, writing a 400 on failure.
func (c *Controller) parseID(ctx *gin.Context) (uint, bool) {
	raw := ctx.Param("id")
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		response.Error(ctx, http.StatusBadRequest, "invalid user id", "VALIDATION_ERROR")
		return 0, false
	}
	return uint(id), true
}

// mapError translates service errors to HTTP responses.
func (c *Controller) mapError(ctx *gin.Context, err error, action string) {
	switch {
	case errors.Is(err, usersvc.ErrUserNotFound):
		response.Error(ctx, http.StatusNotFound, err.Error(), "USER_NOT_FOUND")
	case errors.Is(err, usersvc.ErrForbiddenOrg):
		response.Error(ctx, http.StatusForbidden, err.Error(), "FORBIDDEN_ORGANIZATION")
	default:
		logger.FromContext(ctx.Request.Context()).Error(action+": unexpected error", "error", err)
		response.Error(ctx, http.StatusInternalServerError, "failed to "+action, "INTERNAL_ERROR")
	}
}
