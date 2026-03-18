// Package apierr defines the canonical API error model for CronControl.
//
// All API errors use a consistent structure with code, message, hint, and optional details.
// See docs/product-specification.md for the canonical error contract.
package apierr

import (
	"fmt"
	"net/http"
)

// Error codes — machine-readable, stable across versions.
const (
	CodeValidationError    = "VALIDATION_ERROR"
	CodeNotFound           = "NOT_FOUND"
	CodeConflict           = "CONFLICT"
	CodeIdempotencyConflict = "IDEMPOTENCY_CONFLICT"
	CodeRateLimited        = "RATE_LIMITED"
	CodeWorkspaceSuspended = "WORKSPACE_SUSPENDED"
	CodeEmailNotVerified   = "EMAIL_NOT_VERIFIED"
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeForbidden          = "FORBIDDEN"
	CodeInternalError      = "INTERNAL_ERROR"
	CodeMethodNotAllowed   = "METHOD_NOT_ALLOWED"
	CodeUnsupportedMethod  = "UNSUPPORTED_EXECUTION_METHOD"
)

// APIError is the canonical error type returned by all API endpoints.
type APIError struct {
	HTTPStatus int               `json:"-"`
	Code       string            `json:"code"`
	Message    string            `json:"message"`
	Hint       string            `json:"hint,omitempty"`
	Details    []FieldError      `json:"details,omitempty"`
}

// FieldError represents a validation error on a specific field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ErrorResponse is the API response envelope for errors.
type ErrorResponse struct {
	Error APIError `json:"error"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Constructors for common errors.

func NotFound(resource, id string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusNotFound,
		Code:       CodeNotFound,
		Message:    fmt.Sprintf("%s %q not found", resource, id),
	}
}

func Conflict(message string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusConflict,
		Code:       CodeConflict,
		Message:    message,
	}
}

func IdempotencyConflict(existingJobID string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusConflict,
		Code:       CodeIdempotencyConflict,
		Message:    "Job with this idempotency key already exists",
		Hint:       fmt.Sprintf("existing_job_id: %s", existingJobID),
	}
}

func ValidationError(details []FieldError) *APIError {
	return &APIError{
		HTTPStatus: http.StatusBadRequest,
		Code:       CodeValidationError,
		Message:    "Validation failed",
		Details:    details,
	}
}

func RateLimited(retryAfterSeconds int) *APIError {
	return &APIError{
		HTTPStatus: http.StatusTooManyRequests,
		Code:       CodeRateLimited,
		Message:    "Rate limit exceeded",
		Hint:       fmt.Sprintf("Retry after %d seconds", retryAfterSeconds),
	}
}

func WorkspaceSuspended() *APIError {
	return &APIError{
		HTTPStatus: http.StatusForbidden,
		Code:       CodeWorkspaceSuspended,
		Message:    "Workspace is suspended",
		Hint:       "Contact support to reactivate",
	}
}

func EmailNotVerified() *APIError {
	return &APIError{
		HTTPStatus: http.StatusForbidden,
		Code:       CodeEmailNotVerified,
		Message:    "Email not verified",
		Hint:       "Check your inbox or resend verification at /api/v1/register/verify",
	}
}

func Unauthorized(message string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusUnauthorized,
		Code:       CodeUnauthorized,
		Message:    message,
	}
}

func Forbidden(message string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusForbidden,
		Code:       CodeForbidden,
		Message:    message,
	}
}

func Internal(message string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusInternalServerError,
		Code:       CodeInternalError,
		Message:    message,
	}
}
