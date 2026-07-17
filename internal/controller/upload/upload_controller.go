package upload

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	uploaddto "github.com/tararahuuw/ragsytem/internal/dto/upload"
	"github.com/tararahuuw/ragsytem/internal/logger"
	"github.com/tararahuuw/ragsytem/internal/middleware"
	"github.com/tararahuuw/ragsytem/internal/response"
	uploadsvc "github.com/tararahuuw/ragsytem/internal/service/upload"
)

// Controller exposes chunked upload endpoints.
type Controller struct {
	svc uploadsvc.Service
}

// NewController wires an upload Controller.
func NewController(svc uploadsvc.Service) *Controller {
	return &Controller{svc: svc}
}

// Chunk godoc
//
//	@Summary		Upload one file chunk (resumable large upload)
//	@Description	Streams a single chunk to object storage; the server merges chunks once all are received. PDF only, max size per config.
//	@Tags			upload
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		BearerAuth
//	@Param			file		formData	file	true	"Chunk binary"
//	@Param			sessionId	formData	string	true	"Upload session id (stable across chunks)"
//	@Param			fileName	formData	string	true	"Original file name (.pdf)"
//	@Param			chunkIndex	formData	int		true	"0-based chunk index"
//	@Param			totalChunks	formData	int		true	"Total number of chunks"
//	@Param			fileSize	formData	int		true	"Total file size in bytes"
//	@Param			sha256		formData	string	false	"SHA-256 of the whole file (for dedup)"
//	@Param			chunkSize	formData	int		false	"Chunk size in bytes (for count validation)"
//	@Param			forceUpload	formData	bool	false	"Bypass duplicate check"
//	@Success		200	{object}	response.BaseResponse{data=uploaddto.ChunkResult}
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		401	{object}	response.ErrorResponse
//	@Failure		409	{object}	response.ErrorResponse
//	@Failure		429	{object}	response.ErrorResponse
//	@Router			/uploads/chunk [post]
func (c *Controller) Chunk(ctx *gin.Context) {
	log := logger.FromContext(ctx.Request.Context())

	header, err := ctx.FormFile("file")
	if err != nil {
		response.ValidationError(ctx, "chunk file wajib disertakan", err.Error())
		return
	}

	chunkIndex, e1 := strconv.Atoi(ctx.PostForm("chunkIndex"))
	totalChunks, e2 := strconv.Atoi(ctx.PostForm("totalChunks"))
	fileSize, _ := strconv.ParseInt(ctx.PostForm("fileSize"), 10, 64)
	chunkSize, _ := strconv.ParseInt(ctx.DefaultPostForm("chunkSize", "0"), 10, 64)
	sessionID := ctx.PostForm("sessionId")
	fileName := ctx.PostForm("fileName")

	if e1 != nil || e2 != nil || sessionID == "" || fileName == "" {
		response.ValidationError(ctx, "parameter chunk tidak lengkap atau tidak valid", nil)
		return
	}

	req := uploaddto.ChunkRequest{
		SessionID:   sessionID,
		ChunkIndex:  chunkIndex,
		TotalChunks: totalChunks,
		FileName:    fileName,
		Sha256:      ctx.PostForm("sha256"),
		FileSize:    fileSize,
		ChunkSize:   chunkSize,
		ForceUpload: ctx.DefaultPostForm("forceUpload", "false") == "true",
	}

	f, err := header.Open()
	if err != nil {
		log.Error("upload: cannot open chunk", "error", err)
		response.Error(ctx, http.StatusInternalServerError, "gagal membaca chunk", "INTERNAL_ERROR")
		return
	}
	defer f.Close()

	actor := uploadsvc.Actor{
		UserID:  middleware.CurrentUserID(ctx),
		OrgCode: middleware.CurrentOrgCode(ctx),
		Role:    middleware.CurrentRole(ctx),
	}

	res, err := c.svc.UploadChunk(ctx.Request.Context(), req, f, header.Size, actor)
	if err != nil {
		var ue *uploadsvc.Error
		if errors.As(err, &ue) {
			response.Error(ctx, ue.Status, ue.Message, ue.Code)
			return
		}
		log.Error("upload: unexpected error", "session", sessionID, "error", err)
		response.Error(ctx, http.StatusInternalServerError, "gagal memproses upload", "INTERNAL_ERROR")
		return
	}

	if res.UploadComplete {
		response.OK(ctx, "upload selesai", res)
		return
	}
	response.OK(ctx, "chunk diterima", res)
}
