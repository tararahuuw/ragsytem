package organization

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	orgdto "github.com/tararahuuw/ragsytem/internal/dto/organization"
	"github.com/tararahuuw/ragsytem/internal/logger"
	"github.com/tararahuuw/ragsytem/internal/response"
	orgsvc "github.com/tararahuuw/ragsytem/internal/service/organization"
)

// Controller exposes organization endpoints.
type Controller struct {
	svc orgsvc.Service
}

// NewController wires an organization Controller.
func NewController(svc orgsvc.Service) *Controller {
	return &Controller{svc: svc}
}

// Create godoc
//
//	@Summary		Create organization (admin only)
//	@Description	Admin-only. Registers a valid organization; org codes used elsewhere must exist here.
//	@Tags			organization
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		orgdto.CreateRequest	true	"Organization"
//	@Success		201		{object}	response.BaseResponse{data=orgdto.OrganizationResponse}
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		401		{object}	response.ErrorResponse
//	@Failure		403		{object}	response.ErrorResponse
//	@Failure		409		{object}	response.ErrorResponse
//	@Router			/organizations [post]
func (c *Controller) Create(ctx *gin.Context) {
	log := logger.FromContext(ctx.Request.Context())
	var req orgdto.CreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Warn("organization: invalid payload", "error", err.Error())
		response.ValidationError(ctx, "payload tidak valid", err.Error())
		return
	}
	res, err := c.svc.Create(ctx.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, orgsvc.ErrInvalidCode):
			response.Error(ctx, http.StatusBadRequest, "kode organization tidak valid (2-64 char alfanumerik/-/_)", "INVALID_CODE")
		case errors.Is(err, orgsvc.ErrOrgExists):
			response.Error(ctx, http.StatusConflict, "kode organization sudah ada", "ORG_EXISTS")
		default:
			log.Error("organization: create unexpected error", "error", err)
			response.Error(ctx, http.StatusInternalServerError, "gagal membuat organization", "INTERNAL_ERROR")
		}
		return
	}
	response.Created(ctx, "organization dibuat", res)
}

// List godoc
//
//	@Summary		List organizations
//	@Description	Lists all organizations (any authenticated user).
//	@Tags			organization
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.BaseResponse{data=[]orgdto.OrganizationResponse}
//	@Failure		401	{object}	response.ErrorResponse
//	@Router			/organizations [get]
func (c *Controller) List(ctx *gin.Context) {
	res, err := c.svc.List(ctx.Request.Context())
	if err != nil {
		response.Error(ctx, http.StatusInternalServerError, "gagal mengambil daftar organization", "INTERNAL_ERROR")
		return
	}
	response.OK(ctx, "ok", res)
}

// Get godoc
//
//	@Summary		Get organization by code
//	@Tags			organization
//	@Produce		json
//	@Security		BearerAuth
//	@Param			code	path		string	true	"Organization code"
//	@Success		200		{object}	response.BaseResponse{data=orgdto.OrganizationResponse}
//	@Failure		401		{object}	response.ErrorResponse
//	@Failure		404		{object}	response.ErrorResponse
//	@Router			/organizations/{code} [get]
func (c *Controller) Get(ctx *gin.Context) {
	res, err := c.svc.Get(ctx.Request.Context(), ctx.Param("code"))
	if err != nil {
		c.mapError(ctx, err, "get organization")
		return
	}
	response.OK(ctx, "ok", res)
}

// Update godoc
//
//	@Summary		Update organization (admin only)
//	@Description	Admin-only. Update name/description/active. Deactivating blocks new registrations but keeps existing users.
//	@Tags			organization
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			code	path		string					true	"Organization code"
//	@Param			payload	body		orgdto.UpdateRequest	true	"Fields to update"
//	@Success		200		{object}	response.BaseResponse{data=orgdto.OrganizationResponse}
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		401		{object}	response.ErrorResponse
//	@Failure		403		{object}	response.ErrorResponse
//	@Failure		404		{object}	response.ErrorResponse
//	@Router			/organizations/{code} [put]
func (c *Controller) Update(ctx *gin.Context) {
	log := logger.FromContext(ctx.Request.Context())
	var req orgdto.UpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Warn("organization: invalid payload", "error", err.Error())
		response.ValidationError(ctx, "payload tidak valid", err.Error())
		return
	}
	res, err := c.svc.Update(ctx.Request.Context(), ctx.Param("code"), req)
	if err != nil {
		c.mapError(ctx, err, "update organization")
		return
	}
	response.OK(ctx, "organization diperbarui", res)
}

// Delete godoc
//
//	@Summary		Delete organization (admin only)
//	@Description	Admin-only. Soft-deletes an organization. Blocked (409) while it still has active users.
//	@Tags			organization
//	@Produce		json
//	@Security		BearerAuth
//	@Param			code	path		string	true	"Organization code"
//	@Success		200		{object}	response.BaseResponse
//	@Failure		401		{object}	response.ErrorResponse
//	@Failure		403		{object}	response.ErrorResponse
//	@Failure		404		{object}	response.ErrorResponse
//	@Failure		409		{object}	response.ErrorResponse
//	@Router			/organizations/{code} [delete]
func (c *Controller) Delete(ctx *gin.Context) {
	err := c.svc.Delete(ctx.Request.Context(), ctx.Param("code"))
	if err != nil {
		if errors.Is(err, orgsvc.ErrOrgHasUsers) {
			response.Error(ctx, http.StatusConflict, "organization masih memiliki user aktif", "ORG_HAS_USERS")
			return
		}
		c.mapError(ctx, err, "delete organization")
		return
	}
	response.OK(ctx, "organization dihapus", nil)
}

func (c *Controller) mapError(ctx *gin.Context, err error, action string) {
	if errors.Is(err, orgsvc.ErrOrgNotFound) {
		response.Error(ctx, http.StatusNotFound, "organization tidak ditemukan", "ORG_NOT_FOUND")
		return
	}
	logger.FromContext(ctx.Request.Context()).Error(action+": unexpected error", "error", err)
	response.Error(ctx, http.StatusInternalServerError, "gagal memproses organization", "INTERNAL_ERROR")
}
