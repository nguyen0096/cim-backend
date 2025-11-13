package pkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

//go:generate stringer -type=ErrorCode -linecomment
type ErrorCode int

const (
	// Common Error Codes
	ErrorCodeInternal ErrorCode = 0 // internal
	// ErrorCodeInvalidRequestBody is used for invalid request body errors
	ErrorCodeInvalidRequestBody ErrorCode = 1 // invalid-request-body
	// ErrorCodeNotFound is used for resource not found errors
	ErrorCodeNotFound ErrorCode = 2 // not-found
	// ErrorCodeUnauthorized is used for unauthorized access errors
	ErrorCodeUnauthorized ErrorCode = 3 // unauthorized
	// ErrorCodeForbidden is used for forbidden access errors
	ErrorCodeForbidden ErrorCode = 4 // forbidden
	// ErrorCodeValidation is used for validation errors
	ErrorCodeValidation ErrorCode = 5 // validation
	// ErrorCodeDuplicate is used for duplicate resource errors
	ErrorCodeDuplicate ErrorCode = 6 // duplicate
	// ErrorCodeConflict is used for conflict errors
	ErrorCodeConflict ErrorCode = 9 // conflict

	// Purchase Order Error Codes

	ErrorCodePurchaseOrderNoItems ErrorCode = 7 // purchase-order-no-items
	// ErrorCodePurchaseOrderNoApprovedPaymentReceipt is used when completing a purchase order without approved payment receipt
	ErrorCodePurchaseOrderNoApprovedPaymentReceipt ErrorCode = 14 // purchase-order-no-approved-payment-receipt

	// Inventory Error Codes

	// ErrorCodeBadInventoryItemState is used when an inventory item has invalid state
	// e.g. quantity mismatch between inventory item and consumable transactions
	ErrorCodeBadInventoryItemState ErrorCode = 8 // bad-inventory-item-state
	// ErrorCodeReconcileValidationFailed is used when reconcile validation fails
	ErrorCodeReconcileValidationFailed ErrorCode = 10 // reconcile-validation-failed
	// ErrorCodeDisposeValidationFailed is used when dispose validation fails
	ErrorCodeDisposeValidationFailed ErrorCode = 11 // dispose-validation-failed
	// ErrorCodeTransferValidationFailed is used when transfer validation fails
	ErrorCodeTransferValidationFailed ErrorCode = 12 // transfer-validation-failed
	// ErrorCodeConsumeFIFOFailed is used when consume FIFO fails
	ErrorCodeConsumeFIFOFailed ErrorCode = 13 // consume-fifo-failed
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

// MarshalJSON implements json.Marshaler interface
func (e *AppError) MarshalJSON() ([]byte, error) {
	obj := map[string]interface{}{
		"code":    e.Code.String(),
		"message": e.Message,
	}
	if e.Cause != nil {
		obj["cause"] = e.Cause.Error()
	}
	return json.Marshal(obj)
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
	case ErrorCodeDuplicate, ErrorCodeConflict:
		return http.StatusConflict
	case ErrorCodePurchaseOrderNoItems, ErrorCodePurchaseOrderNoApprovedPaymentReceipt:
		return http.StatusBadRequest
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

func IsErrorCode(err error, code ErrorCode) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}
