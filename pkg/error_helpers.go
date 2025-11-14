package pkg

import (
	"fmt"

	"github.com/shopspring/decimal"
)

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
		fmt.Sprintf("Validation failed: %s", message),
		cause,
	)
}

// ErrDuplicate creates an error for duplicate resources
func ErrDuplicate(message string, cause error) *AppError {
	return NewAppError(ErrorCodeDuplicate, message, cause)
}

// ErrPurchaseOrderNoItems creates an error for purchase order no items
func ErrPurchaseOrderNoItems() *AppError {
	return NewAppError(ErrorCodePurchaseOrderNoItems, "Đơn hàng không có sản phẩm", nil)
}

// ErrBadInventoryItemStatus creates an error for bad inventory item status
func ErrBadInventoryItemState(message string, cause error) *AppError {
	return NewAppError(ErrorCodeBadInventoryItemState, message, cause)
}

// ErrOptimisticLockConflict creates an error for optimistic locking conflicts
func ErrOptimisticLockConflict(resourceType string, resourceID uint, expectedValue, actualValue decimal.Decimal) *AppError {
	message := fmt.Sprintf("Số lượng của %s %d đã thay đổi (trước đó: %s, hiện tại: %s)",
		resourceType, resourceID, expectedValue, actualValue)
	return NewAppError(ErrorCodeConflict, message, nil)
}

// ErrInventoryItemNotFound creates an error for inventory item not found.
func ErrInventoryItemNotFound(itemID uint) *AppError {
	return NewAppError(ErrorCodeNotFound, fmt.Sprintf("Sản phẩm %d không tìm thấy", itemID), nil)
}

func ErrReconcileValidationFailed(message string) *AppError {
	return NewAppError(ErrorCodeReconcileValidationFailed, message, nil)
}

func ErrDisposeValidationFailed(message string) *AppError {
	return NewAppError(ErrorCodeDisposeValidationFailed, message, nil)
}

func ErrTransferValidationFailed(message string) *AppError {
	return NewAppError(ErrorCodeTransferValidationFailed, message, nil)
}

func ErrConsumeFIFOFailed(message string) *AppError {
	return NewAppError(ErrorCodeConsumeFIFOFailed, message, nil)
}

// ErrNoApprovedPaymentReceiptForm creates an error for missing approved payment receipt form
func ErrNoApprovedPaymentReceiptForm() *AppError {
	return NewAppError(ErrorCodePurchaseOrderNoApprovedPaymentReceipt, "Không thể hoàn thành đơn hàng: không tìm thấy phiếu thu chi đã được duyệt", nil)
}
