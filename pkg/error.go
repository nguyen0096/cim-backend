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

	// File Upload Error Codes

	// ErrorCodeUnsupportedFileFormat is used when an unsupported file format is uploaded.
	ErrorCodeUnsupportedFileFormat ErrorCode = 15 // unsupported-file-format
	// ErrorCodeEmptyDataFile is used when a data row is not found.
	ErrorCodeEmptyDataFile ErrorCode = 16 // data-row-not-found

	// Unit Error Codes
	ErrorCodeUnitConversionAlreadyExists ErrorCode = 17 // unit-conversion-already-exists
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

// HTTPStatus returns the appropriate HTTP status code for the error.
// This method is used by the error handler to return correct HTTP status code.
// Using this in app layer is not recommended.
func (e *AppError) HTTPStatus() int {
	switch e.Code {
	case ErrorCodeInvalidRequestBody,
		ErrorCodeValidation,
		ErrorCodeEmptyDataFile,
		ErrorCodePurchaseOrderNoItems,
		ErrorCodePurchaseOrderNoApprovedPaymentReceipt:
		return http.StatusBadRequest
	case ErrorCodeNotFound:
		return http.StatusNotFound
	case ErrorCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrorCodeForbidden:
		return http.StatusForbidden
	case ErrorCodeDuplicate, ErrorCodeConflict:
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

func IsErrorCode(err error, code ErrorCode) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

func NewBatchError(
	code ErrorCode,
	message string,
	cause error,
) *BatchError {
	return &BatchError{
		AppError: AppError{
			Code:    code,
			Message: message,
			Cause:   cause,
		},
		Locations: []BatchErrorLocation{},
	}
}

func (e *BatchError) AddLocation(location string, message string) {
	e.Locations = append(e.Locations, BatchErrorLocation{Location: location, Message: message})
}

// BatchError represents a batch error with code, message, cause, and locations.
// This error is usually used by data import to return multiple errors in a single error.
type BatchError struct {
	AppError
	Locations []BatchErrorLocation `json:"locations,omitempty"`
}

type BatchErrorLocation struct {
	Location string `json:"location"`
	Message  string `json:"message"`
}

// Error implements the error interface
func (e *BatchError) Error() string {
	result := e.Message

	if e.Cause != nil {
		result += fmt.Sprintf(": %v", e.Cause)
	}

	if len(e.Locations) > 0 {
		result += "\nLocations:\n"
		for _, location := range e.Locations {
			result += fmt.Sprintf("\n- %s: %s", location.Location, location.Message)
		}
	}
	return result
}

// MarshalJSON implements json.Marshaler interface
func (e *BatchError) MarshalJSON() ([]byte, error) {
	obj := map[string]interface{}{
		"code":    e.Code.String(),
		"message": e.Message,
	}
	if e.Cause != nil {
		obj["cause"] = e.Cause.Error()
	}
	if len(e.Locations) > 0 {
		obj["locations"] = e.Locations
	}
	return json.Marshal(obj)
}

func (e *BatchError) HasErrors() bool {
	return len(e.Locations) > 0
}
