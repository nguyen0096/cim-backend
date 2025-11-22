package pkg

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
)

// ErrorMessage contains error messages in multiple languages
type ErrorMessage struct {
	EN string
	VI string
}

// Error message keys - predefined constants for error message map keys
const (
	// Purchase Order Error Keys
	ErrKeyPurchaseOrderNoItems                      = "purchase_order_no_items"
	ErrKeyPurchaseOrderNoApprovedPaymentReceipt     = "purchase_order_no_approved_payment_receipt"
	ErrKeyFailedToUpdatePurchaseOrderStatus         = "failed_to_update_purchase_order_status"
	ErrKeyFailedToDeletePurchaseOrder               = "failed_to_delete_purchase_order"
	ErrKeyFailedToUpdatePurchaseOrderDeliveryStatus = "failed_to_update_purchase_order_delivery_status"
	ErrKeyFailedToGetApprovedPaymentReceiptForms    = "failed_to_get_approved_payment_receipt_forms"
	ErrKeyPurchaseOrderStatusChangeDenied           = "purchase_order_status_change_denied"
	ErrKeyPurchaseOrderDeliveryStatusChangeDenied   = "purchase_order_delivery_status_change_denied"

	// Inventory Error Keys
	ErrKeyInventoryItemNotFound  = "inventory_item_not_found"
	ErrKeyOptimisticLockConflict = "optimistic_lock_conflict"
)

// ErrorMessages maps error keys to multilingual messages
var ErrorMessages = map[string]ErrorMessage{
	// Purchase Order Errors
	ErrKeyPurchaseOrderNoItems: {
		EN: "Purchase order has no items",
		VI: "Đơn hàng không có sản phẩm",
	},
	ErrKeyPurchaseOrderNoApprovedPaymentReceipt: {
		EN: "Cannot complete purchase order: no approved payment receipt form found",
		VI: "Không thể hoàn thành đơn hàng: không tìm thấy phiếu thu chi đã được duyệt",
	},
	ErrKeyFailedToUpdatePurchaseOrderStatus: {
		EN: "Failed to update purchase order status",
		VI: "Không thể cập nhật trạng thái đơn hàng",
	},
	ErrKeyFailedToDeletePurchaseOrder: {
		EN: "Failed to delete purchase order",
		VI: "Không thể xóa đơn hàng",
	},
	ErrKeyFailedToUpdatePurchaseOrderDeliveryStatus: {
		EN: "Failed to update purchase order delivery status",
		VI: "Không thể cập nhật trạng thái giao hàng",
	},
	ErrKeyFailedToGetApprovedPaymentReceiptForms: {
		EN: "Failed to get approved payment receipt forms",
		VI: "Không thể lấy danh sách phiếu thu chi đã được duyệt",
	},
	ErrKeyPurchaseOrderStatusChangeDenied: {
		EN: "Access denied: %s role cannot change purchase order status to %s",
		VI: "Truy cập bị từ chối: vai trò %s không thể thay đổi trạng thái đơn hàng thành %s",
	},
	ErrKeyPurchaseOrderDeliveryStatusChangeDenied: {
		EN: "Access denied: %s role cannot confirm or update purchase order delivery status",
		VI: "Truy cập bị từ chối: vai trò %s không thể xác nhận hoặc cập nhật trạng thái giao hàng",
	},
	// Inventory Errors
	ErrKeyInventoryItemNotFound: {
		EN: "Product %d not found",
		VI: "Sản phẩm %d không tìm thấy",
	},
	ErrKeyOptimisticLockConflict: {
		EN: "Quantity of %s %d has changed (previous: %s, current: %s)",
		VI: "Số lượng của %s %d đã thay đổi (trước đó: %s, hiện tại: %s)",
	},
}

// getErrorMessage returns the message for the given error key based on context language
func getErrorMessage(ctx context.Context, key string) string {
	if msg, exists := ErrorMessages[key]; exists {
		lang := GetLanguageFromContext(ctx)
		if lang == LangVI {
			return msg.VI
		}
		return msg.EN
	}
	return ""
}

// GetErrorMessageByLang returns the error message for the given key and language
func GetErrorMessageByLang(key, lang string) string {
	if msg, exists := ErrorMessages[key]; exists {
		if lang == LangVI {
			return msg.VI
		}
		return msg.EN
	}
	return ""
}

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
	err := NewAppError(
		ErrorCodeValidation,
		"Dữ liệu không hợp lệ",
		cause,
	)
	if message != "" {
		err.Message += fmt.Sprintf(": %s", message)
	}
	return err
}

func ErrValidationBatchError() *BatchError {
	err := NewBatchError(ErrorCodeValidation, "Dữ liệu không hợp lệ", nil)
	return err
}

// ErrDuplicate creates an error for duplicate resources
func ErrDuplicate(message string, cause error) *AppError {
	return NewAppError(ErrorCodeDuplicate, message, cause)
}

// ErrPurchaseOrderNoItems creates an error for purchase order no items
func ErrPurchaseOrderNoItems(ctx context.Context) *AppError {
	return NewAppError(ErrorCodePurchaseOrderNoItems, getErrorMessage(ctx, ErrKeyPurchaseOrderNoItems), nil)
}

// ErrBadInventoryItemStatus creates an error for bad inventory item status
func ErrBadInventoryItemState(message string, cause error) *AppError {
	return NewAppError(ErrorCodeBadInventoryItemState, message, cause)
}

// ErrOptimisticLockConflict creates an error for optimistic locking conflicts
func ErrOptimisticLockConflict(ctx context.Context, resourceType string, resourceID uint, expectedValue, actualValue decimal.Decimal) *AppError {
	template := getErrorMessage(ctx, ErrKeyOptimisticLockConflict)
	message := fmt.Sprintf(template, resourceType, resourceID, expectedValue, actualValue)
	return NewAppError(ErrorCodeConflict, message, nil)
}

// ErrInventoryItemNotFound creates an error for inventory item not found.
func ErrInventoryItemNotFound(ctx context.Context, itemID uint) *AppError {
	template := getErrorMessage(ctx, ErrKeyInventoryItemNotFound)
	message := fmt.Sprintf(template, itemID)
	return NewAppError(ErrorCodeNotFound, message, nil)
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
func ErrNoApprovedPaymentReceiptForm(ctx context.Context) *AppError {
	return NewAppError(ErrorCodePurchaseOrderNoApprovedPaymentReceipt, getErrorMessage(ctx, ErrKeyPurchaseOrderNoApprovedPaymentReceipt), nil)
}

// Purchase Order Handler Error Helpers

// ErrFailedToUpdatePurchaseOrderStatus creates an error for failed purchase order status update
func ErrFailedToUpdatePurchaseOrderStatus(ctx context.Context, cause error) *AppError {
	return NewAppError(ErrorCodeInternal, getErrorMessage(ctx, ErrKeyFailedToUpdatePurchaseOrderStatus), cause)
}

// ErrFailedToDeletePurchaseOrder creates an error for failed purchase order deletion
func ErrFailedToDeletePurchaseOrder(ctx context.Context, cause error) *AppError {
	return NewAppError(ErrorCodeInternal, getErrorMessage(ctx, ErrKeyFailedToDeletePurchaseOrder), cause)
}

// ErrFailedToUpdatePurchaseOrderDeliveryStatus creates an error for failed purchase order delivery status update
func ErrFailedToUpdatePurchaseOrderDeliveryStatus(ctx context.Context, cause error) *AppError {
	return NewAppError(ErrorCodeInternal, getErrorMessage(ctx, ErrKeyFailedToUpdatePurchaseOrderDeliveryStatus), cause)
}

// ErrFailedToGetApprovedPaymentReceiptForms creates an error for failed to get approved payment receipt forms
func ErrFailedToGetApprovedPaymentReceiptForms(ctx context.Context, cause error) *AppError {
	return NewAppError(ErrorCodeInternal, getErrorMessage(ctx, ErrKeyFailedToGetApprovedPaymentReceiptForms), cause)
}

// ErrPurchaseOrderStatusChangeDenied creates an error for denied purchase order status change
func ErrPurchaseOrderStatusChangeDenied(ctx context.Context, userRole, status string) *AppError {
	template := getErrorMessage(ctx, ErrKeyPurchaseOrderStatusChangeDenied)
	message := fmt.Sprintf(template, userRole, status)
	return NewAppError(ErrorCodeForbidden, message, nil)
}

// ErrPurchaseOrderDeliveryStatusChangeDenied creates an error for denied purchase order delivery status change
func ErrPurchaseOrderDeliveryStatusChangeDenied(ctx context.Context, userRole string) *AppError {
	template := getErrorMessage(ctx, ErrKeyPurchaseOrderDeliveryStatusChangeDenied)
	message := fmt.Sprintf(template, userRole)
	return NewAppError(ErrorCodeForbidden, message, nil)
}

func ErrUnsupportedFileFormat(fileType string) *AppError {
	return NewAppError(ErrorCodeUnsupportedFileFormat, fmt.Sprintf("Định dạng file %s không được hỗ trợ. Định dạng file hợp lệ: .csv, .xlsx, .xls", fileType), nil)
}

func ErrEmptyDataFile() *AppError {
	return NewAppError(ErrorCodeEmptyDataFile, "File không có dữ liệu", nil)
}
