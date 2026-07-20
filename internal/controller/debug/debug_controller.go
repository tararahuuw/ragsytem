// Package debug exposes endpoints that deliberately trigger error conditions to
// verify the Sentry pipeline end-to-end. These routes are mounted ONLY when the
// app is not in production (see router/debug); they must never ship live.
package debug

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tararahuuw/ragsytem/internal/logger"
	"github.com/tararahuuw/ragsytem/internal/response"
)

// Controller holds the debug/verification handlers.
type Controller struct{}

// NewController builds a debug Controller.
func NewController() *Controller { return &Controller{} }

// ForceError godoc
//
//	@Summary		[debug] Force a 500 error (Sentry test)
//	@Description	Non-production only. Logs an ERROR (captured by Sentry as an exception) and returns 500. Use to verify errors reach Sentry.
//	@Tags			debug
//	@Produce		json
//	@Success		500	{object}	response.ErrorResponse
//	@Router			/debug/error [get]
func (c *Controller) ForceError(ctx *gin.Context) {
	err := errors.New("forced debug error (Sentry pipeline test)")
	logger.FromContext(ctx.Request.Context()).Error("debug: forced error", "error", err)
	response.Error(ctx, http.StatusInternalServerError, "forced error (debug)", "DEBUG_FORCED_ERROR")
}

// ForcePanic godoc
//
//	@Summary		[debug] Force a panic (Sentry test)
//	@Description	Non-production only. Panics so middleware.Recovery catches it, logs it, and Sentry captures it. Returns a standardized 500.
//	@Tags			debug
//	@Produce		json
//	@Success		500	{object}	response.ErrorResponse
//	@Router			/debug/panic [get]
func (c *Controller) ForcePanic(ctx *gin.Context) {
	panic("forced debug panic (Sentry pipeline test)")
}

// ForceMessage godoc
//
//	@Summary		[debug] Emit a WARN message (Sentry test)
//	@Description	Non-production only. Logs a WARN message — reaches Sentry only if SENTRY_LEVEL=warn. Returns 200. Use to verify the level threshold.
//	@Tags			debug
//	@Produce		json
//	@Success		200	{object}	response.BaseResponse
//	@Router			/debug/message [get]
func (c *Controller) ForceMessage(ctx *gin.Context) {
	logger.FromContext(ctx.Request.Context()).Warn("debug: forced warn message", "source", "debug-endpoint")
	response.OK(ctx, "warn message emitted (cek Sentry bila SENTRY_LEVEL=warn)", nil)
}
