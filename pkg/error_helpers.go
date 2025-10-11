package pkg

import "fmt"

// Common error helper functions for creating AppErrors

// ErrInvalidRequestBody creates an error for invalid request body
func ErrInvalidRequestBody(cause error) *AppError {
	return NewAppError(
		ErrorCodeInvalidRequestBody,
		fmt.Sprintf("Invalid request body: %s", cause),
		cause,
	)
}

// ErrInternal creates an error for internal server errors
func ErrInternal(message string, cause error) *AppError {
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrNotFound creates an error for resource not found
func ErrNotFound(message string, cause error) *AppError {
	return NewAppError(ErrorCodeNotFound, message, cause)
}

// ErrUnauthorized creates an error for unauthorized access
func ErrUnauthorized(message string, cause error) *AppError {
	return NewAppError(ErrorCodeUnauthorized, message, cause)
}

// ErrForbidden creates an error for forbidden access
func ErrForbidden(message string, cause error) *AppError {
	return NewAppError(ErrorCodeForbidden, message, cause)
}

// ErrValidation creates an error for validation failures
func ErrValidation(message string, cause error) *AppError {
	return NewAppError(
		ErrorCodeValidation,
		fmt.Sprintf("Validation failed: %s", cause),
		cause,
	)
}

// ErrDuplicate creates an error for duplicate resources
func ErrDuplicate(message string, cause error) *AppError {
	return NewAppError(ErrorCodeDuplicate, message, cause)
}

// ErrPurchaseOrderNoItems creates an error for purchase order no items
func ErrPurchaseOrderNoItems() *AppError {
	return NewAppError(ErrorCodePurchaseOrderNoItems, "Purchase order has no items", nil)
}

// ErrBadInventoryItemStatus creates an error for bad inventory item status
func ErrBadInventoryItemState(message string, cause error) *AppError {
	return NewAppError(ErrorCodeBadInventoryItemState, message, cause)
}
