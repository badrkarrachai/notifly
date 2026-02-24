package common

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIResponse is the standardized JSON response envelope.
type APIResponse struct {
	Success bool     `json:"success"`
	Data    any      `json:"data,omitempty"`
	Error   *APIError `json:"error,omitempty"`
}

// APIError contains error details in the response.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Success sends a successful JSON response with data.
func Success(c *gin.Context, statusCode int, data any) {
	c.JSON(statusCode, APIResponse{
		Success: true,
		Data:    data,
	})
}

// Error sends an error JSON response.
func Error(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    statusCode,
			Message: message,
		},
	})
}

// HandleError inspects a domain error and sends the appropriate HTTP response.
// Uses errors.As to traverse the full error chain, supporting wrapped errors.
func HandleError(c *gin.Context, err error) {
	var notFound *NotFoundError
	var validation *ValidationError
	var unauthorized *UnauthorizedError
	var provider *ProviderError

	switch {
	case errors.As(err, &notFound):
		Error(c, http.StatusNotFound, notFound.Error())
	case errors.As(err, &validation):
		Error(c, http.StatusBadRequest, validation.Error())
	case errors.As(err, &unauthorized):
		Error(c, http.StatusUnauthorized, unauthorized.Error())
	case errors.As(err, &provider):
		Error(c, http.StatusBadGateway, "notification delivery failed")
	default:
		Error(c, http.StatusInternalServerError, "internal server error")
	}
}
