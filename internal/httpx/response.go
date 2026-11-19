package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type Envelope struct {
	Data  any        `json:"data,omitempty"`
	Error *ErrorBody `json:"error,omitempty"`
	Meta  any        `json:"meta,omitempty"`
}

type AppError struct {
	Status  int
	Code    string
	Message string
	Cause   error
}

func (e *AppError) Error() string { return e.Message }
func (e *AppError) Unwrap() error { return e.Cause }

func BadRequest(message string) error {
	return &AppError{Status: http.StatusBadRequest, Code: "bad_request", Message: message}
}
func Unauthorized(message string) error {
	return &AppError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: message}
}
func Forbidden(message string) error {
	return &AppError{Status: http.StatusForbidden, Code: "forbidden", Message: message}
}
func Conflict(message string) error {
	return &AppError{Status: http.StatusConflict, Code: "conflict", Message: message}
}
func NotFound(resource string) error {
	return &AppError{Status: http.StatusNotFound, Code: "not_found", Message: resource + " not found"}
}

func OK(c *gin.Context, data any)      { c.JSON(http.StatusOK, Envelope{Data: data}) }
func Created(c *gin.Context, data any) { c.JSON(http.StatusCreated, Envelope{Data: data}) }
func NoContent(c *gin.Context)         { c.Status(http.StatusNoContent) }

func Fail(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, Envelope{Error: &ErrorBody{Code: "not_found", Message: "resource not found"}})
		return
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		c.JSON(appErr.Status, Envelope{Error: &ErrorBody{Code: appErr.Code, Message: appErr.Message}})
		return
	}
	c.Error(err) //nolint:errcheck
	c.JSON(http.StatusInternalServerError, Envelope{Error: &ErrorBody{Code: "internal_error", Message: "an internal error occurred"}})
}

func Bind(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		c.JSON(http.StatusBadRequest, Envelope{Error: &ErrorBody{Code: "validation_error", Message: "request validation failed", Details: err.Error()}})
		return false
	}
	return true
}
