package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	authdto "github.com/tararahuuw/ragsytem/internal/dto/auth"
	"github.com/tararahuuw/ragsytem/internal/logger"
	"github.com/tararahuuw/ragsytem/internal/middleware"
	"github.com/tararahuuw/ragsytem/internal/response"
	authsvc "github.com/tararahuuw/ragsytem/internal/service/auth"
)

// ChangePassword godoc
//
//	@Summary		Change password
//	@Description	Authenticated. Verifies the current password, sets a new one, and revokes existing sessions.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		authdto.ChangePasswordRequest	true	"Passwords"
//	@Success		200		{object}	response.BaseResponse
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		401		{object}	response.ErrorResponse
//	@Router			/auth/change-password [post]
func (c *Controller) ChangePassword(ctx *gin.Context) {
	log := logger.FromContext(ctx.Request.Context())
	var req authdto.ChangePasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ValidationError(ctx, "payload tidak valid", err.Error())
		return
	}
	err := c.svc.ChangePassword(ctx.Request.Context(), middleware.CurrentUserID(ctx), req)
	if err != nil {
		if errors.Is(err, authsvc.ErrInvalidOldPassword) {
			response.Error(ctx, http.StatusBadRequest, "password lama salah", "INVALID_OLD_PASSWORD")
			return
		}
		log.Error("change password: unexpected error", "error", err)
		response.Error(ctx, http.StatusInternalServerError, "gagal mengubah password", "INTERNAL_ERROR")
		return
	}
	response.OK(ctx, "password diubah, silakan login ulang", nil)
}

// Logout godoc
//
//	@Summary		Logout
//	@Description	Authenticated. Revokes the caller's refresh tokens (access tokens expire within their TTL).
//	@Tags			auth
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.BaseResponse
//	@Failure		401	{object}	response.ErrorResponse
//	@Router			/auth/logout [post]
func (c *Controller) Logout(ctx *gin.Context) {
	if err := c.svc.Logout(ctx.Request.Context(), middleware.CurrentUserID(ctx)); err != nil {
		response.Error(ctx, http.StatusInternalServerError, "gagal logout", "INTERNAL_ERROR")
		return
	}
	response.OK(ctx, "logout berhasil", nil)
}

// ForgotPassword godoc
//
//	@Summary		Forgot password
//	@Description	Public. Sends a password-reset link if the email exists. Always returns 200 (anti user-enumeration).
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		authdto.ForgotPasswordRequest	true	"Email"
//	@Success		200		{object}	response.BaseResponse
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		429		{object}	response.ErrorResponse
//	@Router			/auth/forgot-password [post]
func (c *Controller) ForgotPassword(ctx *gin.Context) {
	var req authdto.ForgotPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ValidationError(ctx, "email tidak valid", err.Error())
		return
	}
	// Always OK regardless of outcome (never reveals whether the email exists).
	_ = c.svc.ForgotPassword(ctx.Request.Context(), req)
	response.OK(ctx, "Jika email terdaftar, tautan reset telah dikirim.", nil)
}

// ResetPassword godoc
//
//	@Summary		Reset password
//	@Description	Public. Sets a new password using a valid reset token, then revokes existing sessions.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		authdto.ResetPasswordRequest	true	"Token + new password"
//	@Success		200		{object}	response.BaseResponse
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		429		{object}	response.ErrorResponse
//	@Router			/auth/reset-password [post]
func (c *Controller) ResetPassword(ctx *gin.Context) {
	log := logger.FromContext(ctx.Request.Context())
	var req authdto.ResetPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ValidationError(ctx, "payload tidak valid", err.Error())
		return
	}
	err := c.svc.ResetPassword(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, authsvc.ErrInvalidResetToken) {
			response.Error(ctx, http.StatusBadRequest, "token reset tidak valid atau kedaluwarsa", "INVALID_RESET_TOKEN")
			return
		}
		log.Error("reset password: unexpected error", "error", err)
		response.Error(ctx, http.StatusInternalServerError, "gagal reset password", "INTERNAL_ERROR")
		return
	}
	response.OK(ctx, "password berhasil direset, silakan login", nil)
}
