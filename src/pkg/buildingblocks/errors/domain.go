package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Domain error sentinels — lowercased, no punctuation, per Go conventions.
var (
	ErrNotFound          = errors.New("resource not found")
	ErrConflict          = errors.New("resource already exists")
	ErrValidation        = errors.New("validation failed")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrInvalidQuantity   = errors.New("invalid quantity")
	ErrInvalidState      = errors.New("invalid state transition")
	ErrConcurrencyConflict = errors.New("concurrency conflict, please retry")
)

// DomainError carries structured context alongside a sentinel error.
type DomainError struct {
	Err     error
	Message string
	Details map[string]interface{}
}

func (e *DomainError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Err.Error()
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// NewDomainError creates a domain error wrapping a sentinel.
func NewDomainError(sentinel error, message string) *DomainError {
	return &DomainError{
		Err:     sentinel,
		Message: message,
	}
}

// WithDetails adds structured context to a domain error.
func (e *DomainError) WithDetails(details map[string]interface{}) *DomainError {
	e.Details = details
	return e
}

// NotFoundError creates a not-found domain error.
func NotFoundError(entity string, id interface{}) *DomainError {
	return &DomainError{
		Err:     ErrNotFound,
		Message: fmt.Sprintf("%s not found", entity),
		Details: map[string]interface{}{"entity": entity, "id": id},
	}
}

// ValidationError creates a validation domain error.
func ValidationError(message string) *DomainError {
	return &DomainError{
		Err:     ErrValidation,
		Message: message,
	}
}

// ConflictError creates a conflict domain error.
func ConflictError(entity string, field string, value interface{}) *DomainError {
	return &DomainError{
		Err:     ErrConflict,
		Message: fmt.Sprintf("%s with %s already exists", entity, field),
		Details: map[string]interface{}{"entity": entity, "field": field, "value": value},
	}
}

// ErrorResponse is the standardized JSON error response body.
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// MapToHTTPStatus maps domain errors to HTTP status codes.
func MapToHTTPStatus(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrValidation):
		return http.StatusBadRequest
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrInsufficientStock):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrInvalidQuantity):
		return http.StatusBadRequest
	case errors.Is(err, ErrInvalidState):
		return http.StatusConflict
	case errors.Is(err, ErrConcurrencyConflict):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// ToErrorResponse converts a domain error to a JSON-serializable response.
func ToErrorResponse(err error) ErrorResponse {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return ErrorResponse{
			Error:   errorCode(domainErr.Err),
			Message: domainErr.Message,
			Details: domainErr.Details,
		}
	}
	// Never expose internal error details to clients.
	return ErrorResponse{
		Error:   "InternalError",
		Message: "an unexpected error occurred",
	}
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, ErrNotFound):
		return "NotFound"
	case errors.Is(err, ErrConflict):
		return "Conflict"
	case errors.Is(err, ErrValidation):
		return "ValidationError"
	case errors.Is(err, ErrUnauthorized):
		return "Unauthorized"
	case errors.Is(err, ErrForbidden):
		return "Forbidden"
	case errors.Is(err, ErrInsufficientStock):
		return "InsufficientStock"
	case errors.Is(err, ErrInvalidState):
		return "InvalidState"
	case errors.Is(err, ErrConcurrencyConflict):
		return "ConcurrencyConflict"
	default:
		return "InternalError"
	}
}
