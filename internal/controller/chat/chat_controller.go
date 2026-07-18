package chat

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	chatdto "github.com/tararahuuw/ragsytem/internal/dto/chat"
	"github.com/tararahuuw/ragsytem/internal/logger"
	"github.com/tararahuuw/ragsytem/internal/middleware"
	"github.com/tararahuuw/ragsytem/internal/response"
	chatsvc "github.com/tararahuuw/ragsytem/internal/service/chat"
)

// Controller exposes conversation (chat) endpoints. All require a JWT.
type Controller struct {
	svc chatsvc.Service
}

// NewController wires a chat Controller.
func NewController(svc chatsvc.Service) *Controller {
	return &Controller{svc: svc}
}

// Ask godoc
//
//	@Summary		Ask a question (RAG chat)
//	@Description	Sends a question to the AI/RAG service within a conversation (client-generated session_id). Creates the session on first use. Scoped to the caller + their organization.
//	@Tags			chat
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		chatdto.AskRequest	true	"Question payload"
//	@Success		200		{object}	response.BaseResponse{data=chatdto.AskResponse}
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		401		{object}	response.ErrorResponse
//	@Failure		429		{object}	response.ErrorResponse
//	@Router			/chat/ask [post]
func (c *Controller) Ask(ctx *gin.Context) {
	log := logger.FromContext(ctx.Request.Context())

	var req chatdto.AskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Warn("chat: invalid payload", "error", err.Error())
		response.ValidationError(ctx, "payload tidak valid", err.Error())
		return
	}

	res, err := c.svc.Ask(ctx.Request.Context(), middleware.CurrentUserID(ctx), middleware.CurrentOrgCode(ctx), req)
	if err != nil {
		switch {
		case errors.Is(err, chatsvc.ErrInvalidSession):
			response.Error(ctx, http.StatusBadRequest, "session_id tidak valid", "INVALID_SESSION")
		case errors.Is(err, chatsvc.ErrSessionNotFound):
			response.Error(ctx, http.StatusNotFound, "session tidak ditemukan", "SESSION_NOT_FOUND")
		default:
			log.Error("chat: ask unexpected error", "error", err)
			response.Error(ctx, http.StatusInternalServerError, "gagal memproses pertanyaan", "INTERNAL_ERROR")
		}
		return
	}
	response.OK(ctx, "ok", res)
}

// ListSessions godoc
//
//	@Summary		List my conversations
//	@Description	Lists the caller's conversation sessions (newest activity first).
//	@Tags			chat
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.BaseResponse{data=[]chatdto.SessionResponse}
//	@Failure		401	{object}	response.ErrorResponse
//	@Router			/chat/sessions [get]
func (c *Controller) ListSessions(ctx *gin.Context) {
	res, err := c.svc.ListSessions(ctx.Request.Context(), middleware.CurrentUserID(ctx))
	if err != nil {
		response.Error(ctx, http.StatusInternalServerError, "gagal mengambil daftar percakapan", "INTERNAL_ERROR")
		return
	}
	response.OK(ctx, "ok", res)
}

// GetSession godoc
//
//	@Summary		Get one conversation (with messages)
//	@Description	Returns a conversation and all its messages. Only the owner can access it.
//	@Tags			chat
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Session ID (UUID)"
//	@Success		200	{object}	response.BaseResponse{data=chatdto.SessionDetailResponse}
//	@Failure		401	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/chat/sessions/{id} [get]
func (c *Controller) GetSession(ctx *gin.Context) {
	res, err := c.svc.GetSessionDetail(ctx.Request.Context(), ctx.Param("id"), middleware.CurrentUserID(ctx))
	if err != nil {
		if errors.Is(err, chatsvc.ErrSessionNotFound) {
			response.Error(ctx, http.StatusNotFound, "session tidak ditemukan", "SESSION_NOT_FOUND")
			return
		}
		response.Error(ctx, http.StatusInternalServerError, "gagal mengambil percakapan", "INTERNAL_ERROR")
		return
	}
	response.OK(ctx, "ok", res)
}

// DeleteSession godoc
//
//	@Summary		Delete a conversation
//	@Description	Deletes a conversation and its messages. Only the owner can delete it.
//	@Tags			chat
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Session ID (UUID)"
//	@Success		200	{object}	response.BaseResponse
//	@Failure		401	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/chat/sessions/{id} [delete]
func (c *Controller) DeleteSession(ctx *gin.Context) {
	err := c.svc.DeleteSession(ctx.Request.Context(), ctx.Param("id"), middleware.CurrentUserID(ctx))
	if err != nil {
		if errors.Is(err, chatsvc.ErrSessionNotFound) {
			response.Error(ctx, http.StatusNotFound, "session tidak ditemukan", "SESSION_NOT_FOUND")
			return
		}
		response.Error(ctx, http.StatusInternalServerError, "gagal menghapus percakapan", "INTERNAL_ERROR")
		return
	}
	response.OK(ctx, "percakapan dihapus", nil)
}
