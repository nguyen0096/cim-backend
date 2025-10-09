package pkg

import (
	"fmt"
	"net/http"
)

//go:generate stringer -type=ErrorCode -linecomment
type ErrorCode int

const (
	ErrorCodeInternal           ErrorCode = 0 // internal
	ErrorCodeInvalidRequestBody ErrorCode = 1 // invalid-request-body
	ErrorCodeNotFound           ErrorCode = 2 // not-found
	ErrorCodeUnauthorized       ErrorCode = 3 // unauthorized
	ErrorCodeForbidden          ErrorCode = 4 // forbidden
	ErrorCodeValidation         ErrorCode = 5 // validation
	ErrorCodeDuplicate          ErrorCode = 6 // duplicate

	// Purchase Order Error Codes
	ErrorCodePurchaseOrderNoItems ErrorCode = 7 // purchase-order-no-items
)

// AppError represents an application error with code, cause, and display message
type AppError struct {
	Code    ErrorCode
	Cause   error
	Message string
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// HTTPStatus returns the appropriate HTTP status code for the error
func (e *AppError) HTTPStatus() int {
	switch e.Code {
	case ErrorCodeInvalidRequestBody, ErrorCodeValidation:
		return http.StatusBadRequest
	case ErrorCodeNotFound:
		return http.StatusNotFound
	case ErrorCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrorCodeForbidden:
		return http.StatusForbidden
	case ErrorCodeDuplicate:
		return http.StatusConflict
	case ErrorCodeInternal:
		fallthrough
	default:
		return http.StatusInternalServerError
	}
}

// NewAppError creates a new AppError
func NewAppError(code ErrorCode, message string, cause error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}
