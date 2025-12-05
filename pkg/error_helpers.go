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
	// Quantity Error Keys

	ErrKeyQuantityHavingMoreDecimalPlacesThanProductUnit = "quantity_having_more_decimal_places_than_product_unit"

	// Purchase Order Error Keys

	ErrKeyPurchaseOrderNoItems                      = "purchase_order_no_items"
	ErrKeyPurchaseOrderNoApprovedPaymentReceipt     = "purchase_order_no_approved_payment_receipt"
	ErrKeyPurchaseOrderNotFound                     = "purchase_order_not_found"
	ErrKeyFailedToFetchPurchaseOrder                = "failed_to_fetch_purchase_order"
	ErrKeyCannotEditPurchaseOrderWithStatus         = "cannot_edit_purchase_order_with_status"
	ErrKeyCannotDeleteItemWithReceivedQuantity      = "cannot_delete_item_with_received_quantity"
	ErrKeyReceivedQuantityGreaterThanUpdated        = "received_quantity_greater_than_updated"
	ErrKeyFailedToSavePurchaseOrderItem             = "failed_to_save_purchase_order_item"
	ErrKeyFailedToDeletePurchaseOrderItems          = "failed_to_delete_purchase_order_items"
	ErrKeyFailedToReloadPurchaseOrderItems          = "failed_to_reload_purchase_order_items"
	ErrKeyFailedToUpdatePurchaseOrder               = "failed_to_update_purchase_order"
	ErrKeyFailedToUpdatePurchaseOrderStatus         = "failed_to_update_purchase_order_status"
	ErrKeyFailedToDeletePurchaseOrder               = "failed_to_delete_purchase_order"
	ErrKeyFailedToUpdatePurchaseOrderDeliveryStatus = "failed_to_update_purchase_order_delivery_status"
	ErrKeyFailedToGetApprovedPaymentReceiptForms    = "failed_to_get_approved_payment_receipt_forms"
	ErrKeyPurchaseOrderStatusChangeDenied           = "purchase_order_status_change_denied"
	ErrKeyPurchaseOrderDeliveryStatusChangeDenied   = "purchase_order_delivery_status_change_denied"

	// Inventory Error Keys

	ErrKeyInventoryItemNotFound  = "inventory_item_not_found"
	ErrKeyOptimisticLockConflict = "optimistic_lock_conflict"

	// Unit Error Keys

	ErrKeyUnitAlreadyExists                 = "unit_already_exists"
	ErrKeyFailedToCheckProductReferences    = "failed_to_check_product_references"
	ErrKeyUnitIDRequired                    = "unit_id_required"
	ErrKeyCannotDeleteUnitProductsReference = "cannot_delete_unit_products_reference"
	ErrKeyFailedToDeleteUnit                = "failed_to_delete_unit"
	ErrKeyUnitIncompatibleWithProduct       = "unit_incompatible_with_product"
	ErrKeyFailedToGetBaseUnit               = "failed_to_get_base_unit"
	ErrKeyFailedToConvertQuantityToBaseUnit = "failed_to_convert_quantity_to_base_unit"

	// File Error Keys

	ErrKeyUnsupportedFileFormat = "unsupported_file_format"
	ErrKeyEmptyDataFile         = "empty_data_file"

	// Revenue Expense Error Keys

	ErrKeyInvalidRequestBody                  = "invalid_request_body"
	ErrKeyValidationFailed                    = "validation_failed"
	ErrKeyFailedToFinalizeRevenueExpense      = "failed_to_finalize_revenue_expense"
	ErrKeyFailedToSetLastFinalizedDate        = "failed_to_set_last_finalized_date"
	ErrKeyFailedToGetRevenueExpenseSettings   = "failed_to_get_revenue_expense_settings"
	ErrKeyRevenueExpenseSettingsNotConfigured = "revenue_expense_settings_not_configured"
	ErrKeyFailedToParseRevenueExpenseSettings = "failed_to_parse_revenue_expense_settings"
	ErrKeyFilePathNotFoundInSettings          = "file_path_not_found_in_settings"
	ErrKeySheetNameNotFoundInSettings         = "sheet_name_not_found_in_settings"
	ErrKeyFailedToQueryPaymentReceiptForms    = "failed_to_query_payment_receipt_forms"
	ErrKeyInvalidGoogleSheetsURL              = "invalid_google_sheets_url"
	ErrKeyServiceAccountNotConfigured         = "service_account_not_configured"
	ErrKeyFailedToInitializeGoogleSheetsRepo  = "failed_to_initialize_google_sheets_repo"
	ErrKeyFailedToGetLastTransactionDate      = "failed_to_get_last_transaction_date"
	ErrKeyFailedToAddNewDateRowGoogleSheets   = "failed_to_add_new_date_row_google_sheets"
	ErrKeyFailedToAddExpensesToGoogleSheets   = "failed_to_add_expenses_to_google_sheets"
	ErrKeyFailedToInitializeExcelRepo         = "failed_to_initialize_excel_repo"
	ErrKeyFailedToAddNewDateRowExcel          = "failed_to_add_new_date_row_excel"
	ErrKeyFailedToAddExpensesToExcel          = "failed_to_add_expenses_to_excel"

	// Unit Error Keys
	ErrKeyUnitConversionAlreadyExists = "unit_conversion_already_exists"
)

// ErrorMessages maps error keys to multilingual messages
var ErrorMessages = map[string]ErrorMessage{
	// Quantity Errors
	ErrKeyQuantityHavingMoreDecimalPlacesThanProductUnit: {
		EN: "Quantity has %d decimal places, more than allowed %d decimal places by product unit %s",
		VI: "Số lượng có %d chữ số thập phân, nhiều hơn quy định %d chữ số thập phân của đơn vị %s",
	},

	// Purchase Order Errors
	ErrKeyPurchaseOrderNoItems: {
		EN: "Purchase order has no items",
		VI: "Đơn hàng không có sản phẩm",
	},
	ErrKeyPurchaseOrderNoApprovedPaymentReceipt: {
		EN: "Cannot complete purchase order: no approved payment receipt form found",
		VI: "Không thể hoàn thành đơn hàng: không tìm thấy phiếu thu chi đã được duyệt",
	},
	ErrKeyPurchaseOrderNotFound: {
		EN: "Purchase order with ID %d not found",
		VI: "Không tìm thấy đơn hàng với ID %d",
	},
	ErrKeyFailedToFetchPurchaseOrder: {
		EN: "Failed to fetch purchase order",
		VI: "Không thể lấy thông tin đơn hàng",
	},
	ErrKeyCannotEditPurchaseOrderWithStatus: {
		EN: "Cannot edit purchase order with status %s",
		VI: "Không thể chỉnh sửa đơn hàng với trạng thái %s",
	},
	ErrKeyCannotDeleteItemWithReceivedQuantity: {
		EN: "Cannot delete item with received quantity %s",
		VI: "Không thể xóa sản phẩm với số lượng đã nhận %s",
	},
	ErrKeyReceivedQuantityGreaterThanUpdated: {
		EN: "Received quantity (%s) for product %d from supplier %d is greater than updated quantity (%s)",
		VI: "Số lượng đã nhận (%s) cho sản phẩm %d từ nhà cung cấp %d lớn hơn số lượng cập nhật (%s)",
	},
	ErrKeyFailedToSavePurchaseOrderItem: {
		EN: "Failed to save purchase order item",
		VI: "Không thể lưu sản phẩm đơn hàng",
	},
	ErrKeyFailedToDeletePurchaseOrderItems: {
		EN: "Failed to delete removed purchase order items",
		VI: "Không thể xóa các sản phẩm đơn hàng đã bị loại bỏ",
	},
	ErrKeyFailedToReloadPurchaseOrderItems: {
		EN: "Failed to reload purchase order items",
		VI: "Không thể tải lại các sản phẩm đơn hàng",
	},
	ErrKeyFailedToUpdatePurchaseOrder: {
		EN: "Failed to update purchase order",
		VI: "Không thể cập nhật đơn hàng",
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
	// Unit Errors
	ErrKeyUnitAlreadyExists: {
		EN: "Unit '%s' already exists for type '%s'",
		VI: "Đơn vị '%s' đã tồn tại cho loại '%s'",
	},
	ErrKeyFailedToCheckProductReferences: {
		EN: "Failed to check product references for unit %d",
		VI: "Không thể kiểm tra tham chiếu sản phẩm cho đơn vị %d",
	},
	ErrKeyUnitIDRequired: {
		EN: "Unit ID is required",
		VI: "ID đơn vị là bắt buộc",
	},
	ErrKeyCannotDeleteUnitProductsReference: {
		EN: "Cannot delete unit: products reference this unit",
		VI: "Không thể xóa đơn vị: có sản phẩm đang tham chiếu đơn vị này",
	},
	ErrKeyFailedToDeleteUnit: {
		EN: "Failed to delete unit %d",
		VI: "Không thể xóa đơn vị %d",
	},
	ErrKeyUnitIncompatibleWithProduct: {
		EN: "Unit %d (base unit %d) is not compatible with product %d (base unit %d)",
		VI: "Đơn vị %d (đơn vị cơ sở %d) không tương thích với sản phẩm %d (đơn vị cơ sở %d)",
	},
	ErrKeyFailedToGetBaseUnit: {
		EN: "Failed to get base unit",
		VI: "Không thể lấy đơn vị cơ sở",
	},
	ErrKeyFailedToConvertQuantityToBaseUnit: {
		EN: "Failed to convert quantity to base unit for unit %d",
		VI: "Không thể chuyển đổi số lượng sang đơn vị cơ sở cho đơn vị %d",
	},
	// File Errors
	ErrKeyUnsupportedFileFormat: {
		EN: "File format %s is not supported. Valid file formats: .csv, .xlsx, .xls",
		VI: "Định dạng file %s không được hỗ trợ. Định dạng file hợp lệ: .csv, .xlsx, .xls",
	},
	ErrKeyEmptyDataFile: {
		EN: "File has no data",
		VI: "File không có dữ liệu",
	},
	// Revenue Expense Errors
	ErrKeyInvalidRequestBody: {
		EN: "Invalid request format",
		VI: "Định dạng yêu cầu không hợp lệ",
	},
	ErrKeyValidationFailed: {
		EN: "Validation failed",
		VI: "Dữ liệu không hợp lệ",
	},
	ErrKeyFailedToFinalizeRevenueExpense: {
		EN: "Failed to finalize revenue expense",
		VI: "Không thể hoàn tất thu chi",
	},
	ErrKeyFailedToSetLastFinalizedDate: {
		EN: "Failed to set last finalized date",
		VI: "Không thể cập nhật ngày hoàn tất cuối cùng",
	},
	ErrKeyFailedToGetRevenueExpenseSettings: {
		EN: "Failed to get revenue expense settings",
		VI: "Không thể lấy cài đặt thu chi",
	},
	ErrKeyRevenueExpenseSettingsNotConfigured: {
		EN: "Revenue expense settings not configured",
		VI: "Cài đặt thu chi chưa được cấu hình",
	},
	ErrKeyFailedToParseRevenueExpenseSettings: {
		EN: "Failed to parse revenue expense settings",
		VI: "Không thể phân tích cài đặt thu chi",
	},
	ErrKeyFilePathNotFoundInSettings: {
		EN: "File path not found in revenue expense settings",
		VI: "Không tìm thấy đường dẫn file trong cài đặt thu chi",
	},
	ErrKeySheetNameNotFoundInSettings: {
		EN: "Sheet name not found in revenue expense settings",
		VI: "Không tìm thấy tên sheet trong cài đặt thu chi",
	},
	ErrKeyFailedToQueryPaymentReceiptForms: {
		EN: "Failed to query payment receipt forms for date %s",
		VI: "Không thể truy vấn phiếu thu chi cho ngày %s",
	},
	ErrKeyInvalidGoogleSheetsURL: {
		EN: "Invalid Google Sheets URL",
		VI: "URL Google Sheets không hợp lệ",
	},
	ErrKeyServiceAccountNotConfigured: {
		EN: "Service account file path not configured",
		VI: "Đường dẫn file service account chưa được cấu hình",
	},
	ErrKeyFailedToInitializeGoogleSheetsRepo: {
		EN: "Failed to initialize Google Sheets repository",
		VI: "Không thể khởi tạo repository Google Sheets",
	},
	ErrKeyFailedToGetLastTransactionDate: {
		EN: "Failed to get last transaction date",
		VI: "Không thể lấy ngày giao dịch cuối cùng",
	},
	ErrKeyFailedToAddNewDateRowGoogleSheets: {
		EN: "Failed to add new date row to Google Sheets",
		VI: "Không thể thêm dòng ngày mới vào Google Sheets",
	},
	ErrKeyFailedToAddExpensesToGoogleSheets: {
		EN: "Failed to add expenses to Google Sheets",
		VI: "Không thể thêm chi phí vào Google Sheets",
	},
	ErrKeyFailedToInitializeExcelRepo: {
		EN: "Failed to initialize Excel repository",
		VI: "Không thể khởi tạo repository Excel",
	},
	ErrKeyFailedToAddNewDateRowExcel: {
		EN: "Failed to add new date row to Excel",
		VI: "Không thể thêm dòng ngày mới vào Excel",
	},
	ErrKeyFailedToAddExpensesToExcel: {
		EN: "Failed to add expenses to Excel",
		VI: "Không thể thêm chi phí vào Excel",
	},

	// Unit Error Keys

	ErrKeyUnitConversionAlreadyExists: {
		EN: "Unit conversion from %d to %d already exists",
		VI: "Đã tồn tại chuyển đổi đơn vị từ %d đến %d",
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

const MsgErrValidation = "Dữ liệu không hợp lệ"

// ErrValidation creates an error for validation failures
func ErrValidation(message string, cause error) *AppError {
	err := NewAppError(
		ErrorCodeValidation,
		MsgErrValidation,
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

// ErrPurchaseOrderNotFound creates an error for purchase order not found
func ErrPurchaseOrderNotFound(ctx context.Context, id uint) *AppError {
	template := getErrorMessage(ctx, ErrKeyPurchaseOrderNotFound)
	message := fmt.Sprintf(template, id)
	return NewAppError(ErrorCodeNotFound, message, nil)
}

// ErrFailedToFetchPurchaseOrder creates an error for failed to fetch purchase order
func ErrFailedToFetchPurchaseOrder(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToFetchPurchaseOrder)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrCannotEditPurchaseOrderWithStatus creates an error when trying to edit purchase order with invalid status
func ErrCannotEditPurchaseOrderWithStatus(ctx context.Context, status string) *AppError {
	template := getErrorMessage(ctx, ErrKeyCannotEditPurchaseOrderWithStatus)
	message := fmt.Sprintf(template, status)
	return NewAppError(ErrorCodeValidation, message, nil)
}

// ErrCannotDeleteItemWithReceivedQuantity creates an error when trying to delete item with received quantity
func ErrCannotDeleteItemWithReceivedQuantity(ctx context.Context, receivedQuantity decimal.Decimal) *AppError {
	template := getErrorMessage(ctx, ErrKeyCannotDeleteItemWithReceivedQuantity)
	message := fmt.Sprintf(template, receivedQuantity.String())
	return NewAppError(ErrorCodeValidation, message, nil)
}

// ErrReceivedQuantityGreaterThanUpdated creates an error when received quantity is greater than updated quantity
func ErrReceivedQuantityGreaterThanUpdated(ctx context.Context, receivedQuantity decimal.Decimal, productID, supplierID uint, updatedQuantity decimal.Decimal) *AppError {
	template := getErrorMessage(ctx, ErrKeyReceivedQuantityGreaterThanUpdated)
	message := fmt.Sprintf(template, receivedQuantity.String(), productID, supplierID, updatedQuantity.String())
	return NewAppError(ErrorCodeValidation, message, nil)
}

// ErrFailedToSavePurchaseOrderItem creates an error for failed to save purchase order item
func ErrFailedToSavePurchaseOrderItem(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToSavePurchaseOrderItem)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrFailedToDeletePurchaseOrderItems creates an error for failed to delete purchase order items
func ErrFailedToDeletePurchaseOrderItems(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToDeletePurchaseOrderItems)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrFailedToReloadPurchaseOrderItems creates an error for failed to reload purchase order items
func ErrFailedToReloadPurchaseOrderItems(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToReloadPurchaseOrderItems)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrFailedToUpdatePurchaseOrder creates an error for failed purchase order update
func ErrFailedToUpdatePurchaseOrder(ctx context.Context, cause error) *AppError {
	return NewAppError(ErrorCodeInternal, getErrorMessage(ctx, ErrKeyFailedToUpdatePurchaseOrder), cause)
}

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

func ErrUnsupportedFileFormat(ctx context.Context, fileType string) *AppError {
	template := getErrorMessage(ctx, ErrKeyUnsupportedFileFormat)
	message := fmt.Sprintf(template, fileType)
	return NewAppError(ErrorCodeUnsupportedFileFormat, message, nil)
}

func ErrEmptyDataFile(ctx context.Context) *AppError {
	message := getErrorMessage(ctx, ErrKeyEmptyDataFile)
	return NewAppError(ErrorCodeEmptyDataFile, message, nil)
}

// ErrFailedToCheckProductReferences creates an error for failed product reference check
func ErrFailedToCheckProductReferences(ctx context.Context, unitID uint, cause error) *AppError {
	template := getErrorMessage(ctx, ErrKeyFailedToCheckProductReferences)
	message := fmt.Sprintf(template, unitID)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrUnitIDRequired creates an error for missing unit ID
func ErrUnitIDRequired(ctx context.Context) *AppError {
	message := getErrorMessage(ctx, ErrKeyUnitIDRequired)
	return NewAppError(ErrorCodeValidation, message, nil)
}

// ErrCannotDeleteUnitProductsReference creates an error when trying to delete a unit that has products
func ErrCannotDeleteUnitProductsReference(ctx context.Context) *AppError {
	message := getErrorMessage(ctx, ErrKeyCannotDeleteUnitProductsReference)
	return NewAppError(ErrorCodeValidation, message, nil)
}

// ErrFailedToDeleteUnit creates an error for failed unit deletion
func ErrFailedToDeleteUnit(ctx context.Context, unitID uint, cause error) *AppError {
	template := getErrorMessage(ctx, ErrKeyFailedToDeleteUnit)
	message := fmt.Sprintf(template, unitID)
	return NewAppError(ErrorCodeInternal, message, cause)
}

func ErrUnitAlreadyExists(ctx context.Context, unitName, unitType string) *AppError {
	template := getErrorMessage(ctx, ErrKeyUnitAlreadyExists)
	message := fmt.Sprintf(template, unitName, unitType)
	return NewAppError(ErrorCodeDuplicate, message, nil)
}

// ErrUnitIncompatibleWithProduct creates an error when unit is not compatible with product
func ErrUnitIncompatibleWithProduct(ctx context.Context, unitID, unitBaseUnitID, productID, productBaseUnitID uint) *AppError {
	template := getErrorMessage(ctx, ErrKeyUnitIncompatibleWithProduct)
	message := fmt.Sprintf(template, unitID, unitBaseUnitID, productID, productBaseUnitID)
	return NewAppError(ErrorCodeValidation, message, nil)
}

// ErrFailedToGetBaseUnit creates an error for failed to get base unit
func ErrFailedToGetBaseUnit(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToGetBaseUnit)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrFailedToConvertQuantityToBaseUnit creates an error for failed to convert quantity to base unit
func ErrFailedToConvertQuantityToBaseUnit(ctx context.Context, unitID uint, cause error) *AppError {
	template := getErrorMessage(ctx, ErrKeyFailedToConvertQuantityToBaseUnit)
	message := fmt.Sprintf(template, unitID)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// Revenue Expense Error Helpers

// ErrInvalidRequestBodyI18n creates an i18n-aware error for invalid request body
func ErrInvalidRequestBodyI18n(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyInvalidRequestBody)
	return NewAppError(ErrorCodeInvalidRequestBody, message, cause)
}

// ErrValidationI18n creates an i18n-aware error for validation failures
func ErrValidationI18n(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyValidationFailed)
	return NewAppError(ErrorCodeValidation, message, cause)
}

// ErrFailedToFinalizeRevenueExpense creates an error for failed revenue expense finalization
func ErrFailedToFinalizeRevenueExpense(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToFinalizeRevenueExpense)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrFailedToSetLastFinalizedDate creates an error for failed to set last finalized date
func ErrFailedToSetLastFinalizedDate(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToSetLastFinalizedDate)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrFailedToGetRevenueExpenseSettings creates an error for failed to get revenue expense settings
func ErrFailedToGetRevenueExpenseSettings(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToGetRevenueExpenseSettings)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrRevenueExpenseSettingsNotConfigured creates an error for revenue expense settings not configured
func ErrRevenueExpenseSettingsNotConfigured(ctx context.Context) *AppError {
	message := getErrorMessage(ctx, ErrKeyRevenueExpenseSettingsNotConfigured)
	return NewAppError(ErrorCodeInternal, message, nil)
}

// ErrFailedToParseRevenueExpenseSettings creates an error for failed to parse revenue expense settings
func ErrFailedToParseRevenueExpenseSettings(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToParseRevenueExpenseSettings)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrFilePathNotFoundInSettings creates an error for file path not found in settings
func ErrFilePathNotFoundInSettings(ctx context.Context) *AppError {
	message := getErrorMessage(ctx, ErrKeyFilePathNotFoundInSettings)
	return NewAppError(ErrorCodeInternal, message, nil)
}

// ErrSheetNameNotFoundInSettings creates an error for sheet name not found in settings
func ErrSheetNameNotFoundInSettings(ctx context.Context) *AppError {
	message := getErrorMessage(ctx, ErrKeySheetNameNotFoundInSettings)
	return NewAppError(ErrorCodeInternal, message, nil)
}

// ErrFailedToQueryPaymentReceiptForms creates an error for failed to query payment receipt forms
func ErrFailedToQueryPaymentReceiptForms(ctx context.Context, date string, cause error) *AppError {
	template := getErrorMessage(ctx, ErrKeyFailedToQueryPaymentReceiptForms)
	message := fmt.Sprintf(template, date)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrInvalidGoogleSheetsURL creates an error for invalid Google Sheets URL
func ErrInvalidGoogleSheetsURL(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyInvalidGoogleSheetsURL)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrServiceAccountNotConfigured creates an error for service account not configured
func ErrServiceAccountNotConfigured(ctx context.Context) *AppError {
	message := getErrorMessage(ctx, ErrKeyServiceAccountNotConfigured)
	return NewAppError(ErrorCodeInternal, message, nil)
}

// ErrFailedToInitializeGoogleSheetsRepo creates an error for failed to initialize Google Sheets repository
func ErrFailedToInitializeGoogleSheetsRepo(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToInitializeGoogleSheetsRepo)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrFailedToGetLastTransactionDate creates an error for failed to get last transaction date
func ErrFailedToGetLastTransactionDate(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToGetLastTransactionDate)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrFailedToAddNewDateRowGoogleSheets creates an error for failed to add new date row to Google Sheets
func ErrFailedToAddNewDateRowGoogleSheets(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToAddNewDateRowGoogleSheets)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrFailedToAddExpensesToGoogleSheets creates an error for failed to add expenses to Google Sheets
func ErrFailedToAddExpensesToGoogleSheets(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToAddExpensesToGoogleSheets)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrFailedToInitializeExcelRepo creates an error for failed to initialize Excel repository
func ErrFailedToInitializeExcelRepo(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToInitializeExcelRepo)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrFailedToAddNewDateRowExcel creates an error for failed to add new date row to Excel
func ErrFailedToAddNewDateRowExcel(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToAddNewDateRowExcel)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// ErrFailedToAddExpensesToExcel creates an error for failed to add expenses to Excel
func ErrFailedToAddExpensesToExcel(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToAddExpensesToExcel)
	return NewAppError(ErrorCodeInternal, message, cause)
}

func ErrUnitConversionAlreadyExists(ctx context.Context, fromUnitID, toUnitID uint) *AppError {
	template := getErrorMessage(ctx, ErrKeyUnitConversionAlreadyExists)
	message := fmt.Sprintf(template, fromUnitID, toUnitID)
	return NewAppError(ErrorCodeDuplicate, message, nil)
}

func ErrQuantityHavingMoreDecimalPlacesThanProductUnit(ctx context.Context,
	quantityDecimalPlaces int,
	productUnitDecimalPlaces int,
	productUnitName string,
) *AppError {
	template := getErrorMessage(ctx, ErrKeyQuantityHavingMoreDecimalPlacesThanProductUnit)
	message := fmt.Sprintf(template, quantityDecimalPlaces, productUnitDecimalPlaces, productUnitName)
	return NewAppError(ErrorCodeValidation, message, nil)
}
