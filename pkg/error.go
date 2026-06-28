package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"runtime/debug"
	"strings"
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
	// ErrorCodeActivePendingReconcileConflict is returned when an inventory already
	// has a pending reconcile in flight (one-active-pending-reconcile guard, #38 P3).
	ErrorCodeActivePendingReconcileConflict ErrorCode = 17 // active-pending-reconcile-conflict
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
)

// AppError represents an application error with code, cause, and display message
type AppError struct {
	Code    ErrorCode
	Cause   error
	Message string
	// MessageKey, when set, is an ErrorMessages catalog key the error handler
	// resolves to a request-localized message at response time (formatted with
	// MessageArgs). Lets lower layers (e.g. repositories) return a domain error
	// without knowing the caller's language. Message is the fallback if unset.
	MessageKey  string
	MessageArgs []interface{}
	// Stack holds a stack trace captured at error creation time. It is used for
	// server-side logging only and MUST NOT be exposed in MarshalJSON / client
	// responses.
	Stack string
}

// LocalizedMessage returns the request-localized message for ctx: the
// MessageKey resolved against the catalog (formatted with MessageArgs) when set,
// otherwise the static Message.
func (e *AppError) LocalizedMessage(ctx context.Context) string {
	if e.MessageKey == "" {
		return e.Message
	}
	tmpl := getErrorMessage(ctx, e.MessageKey)
	if tmpl == "" {
		return e.Message
	}
	if len(e.MessageArgs) > 0 {
		return fmt.Sprintf(tmpl, e.MessageArgs...)
	}
	return tmpl
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// MarshalJSON implements json.Marshaler interface.
//
// When MessageKey is set it is emitted as a stable, language-independent "key"
// field so clients can route an error to the offending field/control without
// parsing the localized "message" (issue #42). The field is omitted entirely for
// errors that carry no key, so this is additive and harmless for existing errors.
func (e *AppError) MarshalJSON() ([]byte, error) {
	obj := map[string]interface{}{
		"code":    e.Code.String(),
		"message": e.Message,
	}
	if e.MessageKey != "" {
		obj["key"] = e.MessageKey
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
	case ErrorCodeDuplicate, ErrorCodeConflict, ErrorCodeActivePendingReconcileConflict:
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
		Stack:   captureStack(3),
	}
}

// captureStack records the current call stack as a formatted string, skipping
// the given number of leading frames (this helper + the constructor frames) so
// the trace starts at the caller that created the error.
func captureStack(skip int) string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(skip, pcs[:])
	if n == 0 {
		return ""
	}

	frames := runtime.CallersFrames(pcs[:n])
	var b strings.Builder
	for {
		frame, more := frames.Next()
		fmt.Fprintf(&b, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
	return b.String()
}

// capturedStack returns the stack captured at construction time. It is defined
// on *AppError and is promoted to *BatchError via embedding, so both satisfy
// the stackCapturer interface used by StackTrace.
func (e *AppError) capturedStack() string { return e.Stack }

// stackCapturer is implemented by errors that carry a stack captured at
// creation time (*AppError, and *BatchError via promotion).
type stackCapturer interface{ capturedStack() string }

// StackTrace returns a stack trace associated with err for server-side logging.
// If err (or anything it wraps) carries a captured creation stack (*AppError or
// *BatchError), that stack is returned. Otherwise it falls back to the current
// goroutine stack via debug.Stack so raw errors still get a usable trace.
//
// Note: errors.As(&appErr) does NOT reach the AppError embedded in *BatchError
// (Go does not traverse embedded fields without Unwrap/As), so we walk the
// Unwrap chain and check the stackCapturer interface, which *BatchError
// satisfies through method promotion.
func StackTrace(err error) string {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if sc, ok := e.(stackCapturer); ok {
			if s := sc.capturedStack(); s != "" {
				return s
			}
		}
	}
	return string(debug.Stack())
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
			Stack:   captureStack(3),
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

// MarshalJSON implements json.Marshaler interface. Like AppError it emits the
// stable "key" field when MessageKey is set (omitted otherwise); see #42.
func (e *BatchError) MarshalJSON() ([]byte, error) {
	obj := map[string]interface{}{
		"code":    e.Code.String(),
		"message": e.Message,
	}
	if e.MessageKey != "" {
		obj["key"] = e.MessageKey
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
