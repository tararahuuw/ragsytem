package document

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/tararahuuw/ragsytem/internal/logger"
	"github.com/tararahuuw/ragsytem/internal/middleware"
	"github.com/tararahuuw/ragsytem/internal/response"
	documentsvc "github.com/tararahuuw/ragsytem/internal/service/document"
)

// Controller exposes document (uploaded file) read endpoints. All require a JWT.
type Controller struct {
	svc documentsvc.Service
}

// NewController wires a document Controller.
func NewController(svc documentsvc.Service) *Controller {
	return &Controller{svc: svc}
}

// List godoc
//
//	@Summary		List documents by organization
//	@Description	Lists uploaded documents. Regular users see only their own organization; admins see all.
//	@Tags			document
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.BaseResponse{data=[]document.DocumentResponse}
//	@Failure		401	{object}	response.ErrorResponse
//	@Router			/documents [get]
func (c *Controller) List(ctx *gin.Context) {
	res, err := c.svc.List(ctx.Request.Context(), middleware.CurrentOrgCode(ctx), middleware.CurrentRole(ctx))
	if err != nil {
		logger.FromContext(ctx.Request.Context()).Error("document: list unexpected error", "error", err)
		response.Error(ctx, http.StatusInternalServerError, "gagal mengambil daftar dokumen", "INTERNAL_ERROR")
		return
	}
	response.OK(ctx, "ok", res)
}

// GetByID godoc
//
//	@Summary		Get one document
//	@Description	Returns a document within the caller's organization (admin: any org), with a presigned download URL.
//	@Tags			document
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Document ID"
//	@Success		200	{object}	response.BaseResponse{data=document.DocumentResponse}
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		401	{object}	response.ErrorResponse
//	@Failure		403	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/documents/{id} [get]
func (c *Controller) GetByID(ctx *gin.Context) {
	raw := ctx.Param("id")
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		response.Error(ctx, http.StatusBadRequest, "id dokumen tidak valid", "VALIDATION_ERROR")
		return
	}

	res, err := c.svc.GetByID(ctx.Request.Context(), uint(id), middleware.CurrentOrgCode(ctx), middleware.CurrentRole(ctx))
	if err != nil {
		switch {
		case errors.Is(err, documentsvc.ErrDocumentNotFound):
			response.Error(ctx, http.StatusNotFound, err.Error(), "DOCUMENT_NOT_FOUND")
		case errors.Is(err, documentsvc.ErrForbiddenOrg):
			response.Error(ctx, http.StatusForbidden, err.Error(), "FORBIDDEN_ORGANIZATION")
		default:
			logger.FromContext(ctx.Request.Context()).Error("document: get unexpected error", "error", err)
			response.Error(ctx, http.StatusInternalServerError, "gagal mengambil dokumen", "INTERNAL_ERROR")
		}
		return
	}
	response.OK(ctx, "ok", res)
}
