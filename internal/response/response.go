// Package response provides the standard HTTP response envelopes (base success
// and error) plus helpers to write them consistently from any controller.
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BaseResponse is the standard envelope for successful responses.
type BaseResponse struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message,omitempty" example:"ok"`
	Data    any    `json:"data,omitempty"`
}

// ErrorResponse is the standard envelope for error responses.
type ErrorResponse struct {
	Success bool         `json:"success" example:"false"`
	Message string       `json:"message" example:"something went wrong"`
	Error   *ErrorDetail `json:"error,omitempty"`
}

// ErrorDetail carries machine-readable error information.
type ErrorDetail struct {
	Code    string `json:"code,omitempty" example:"VALIDATION_ERROR"`
	Details any    `json:"details,omitempty"`
}

// Success writes a BaseResponse with the given status code.
func Success(c *gin.Context, status int, message string, data any) {
	c.JSON(status, BaseResponse{Success: true, Message: message, Data: data})
}

// OK is a shortcut for a 200 response.
func OK(c *gin.Context, message string, data any) {
	Success(c, http.StatusOK, message, data)
}

// Created is a shortcut for a 201 response.
func Created(c *gin.Context, message string, data any) {
	Success(c, http.StatusCreated, message, data)
}

// Error writes an ErrorResponse with a machine-readable code.
func Error(c *gin.Context, status int, message, code string) {
	c.JSON(status, ErrorResponse{
		Success: false,
		Message: message,
		Error:   &ErrorDetail{Code: code},
	})
}

// ValidationError writes a 400 with per-field validation details.
func ValidationError(c *gin.Context, message string, details any) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Success: false,
		Message: message,
		Error:   &ErrorDetail{Code: "VALIDATION_ERROR", Details: details},
	})
}
