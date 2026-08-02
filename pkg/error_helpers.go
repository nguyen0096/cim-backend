package pkg

import (
	"context"
	"fmt"
	"strings"

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
	ErrKeyCannotReceivePurchaseOrderWithStatus      = "cannot_receive_purchase_order_with_status"
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

	ErrKeyInventoryItemNotFound          = "inventory_item_not_found"
	ErrKeyOptimisticLockConflict         = "optimistic_lock_conflict"
	ErrKeyNoTransactionsInReportPeriod   = "no_transactions_in_report_period"
	ErrKeyActivePendingReconcileConflict = "active_pending_reconcile_conflict"

	// Reconciliation Request Item Error Keys

	ErrKeyReconItemNotFound           = "recon_item_not_found"
	ErrKeyReconParentNotFound         = "recon_parent_not_found"
	ErrKeyReconParentNotInitiated     = "recon_parent_not_initiated"
	ErrKeyReconParentNotInFlight      = "recon_parent_not_in_flight"
	ErrKeyReconItemMissingQuantity    = "recon_item_missing_quantity"
	ErrKeyReconItemNotOwned           = "recon_item_not_owned"
	ErrKeyReconItemNotInParent        = "recon_item_not_in_parent"
	ErrKeyReconItemImmutable          = "recon_item_immutable"
	ErrKeyReconItemCannotDeleteStatus = "recon_item_cannot_delete_status"
	ErrKeyReconItemInvalidTransition  = "recon_item_invalid_transition"
	ErrKeyReconItemNoSnapshotBaseline = "recon_item_no_snapshot_baseline"
	ErrKeyReconItemNegativeQuantity   = "recon_item_negative_quantity"
	ErrKeyReconItemDuplicateLine      = "recon_item_duplicate_line"

	// Per-count label keys

	ErrKeyReconItemLabelRequiredForDuplicate = "recon_item_label_required_for_duplicate"
	ErrKeyReconItemLabelConflict             = "recon_item_label_conflict"
	ErrKeyReconItemLabelTooLong              = "recon_item_label_too_long"

	// Row-level (count-session) label keys

	ErrKeyReconRowLabelRequired = "recon_row_label_required"
	ErrKeyReconRowLabelConflict = "recon_row_label_conflict"
	ErrKeyReconRowLabelTooLong  = "recon_row_label_too_long"

	// Reconciliation management Error Keys

	ErrKeyReconSubmissionClosed              = "recon_submission_closed"
	ErrKeyReconInvalidLifecycleTransition    = "recon_invalid_lifecycle_transition"
	ErrKeyReconDriftWarning                  = "recon_drift_warning"
	ErrKeyReconCannotRejectInLifecycle       = "recon_cannot_reject_in_lifecycle"
	ErrKeyReconNotReadySessionWarning        = "recon_not_ready_session_warning"
	ErrKeyReconNotReadySessionWarningNoLabel = "recon_not_ready_session_warning_no_label"

	// Unit Error Keys

	ErrKeyUnitAlreadyExists                 = "unit_already_exists"
	ErrKeyFailedToCheckProductReferences    = "failed_to_check_product_references"
	ErrKeyUnitIDRequired                    = "unit_id_required"
	ErrKeyCannotDeleteUnitProductsReference = "cannot_delete_unit_products_reference"
	ErrKeyFailedToDeleteUnit                = "failed_to_delete_unit"
	ErrKeyUnitIncompatibleWithProduct       = "unit_incompatible_with_product"
	ErrKeyFailedToGetBaseUnit               = "failed_to_get_base_unit"
	ErrKeyFailedToConvertQuantityToBaseUnit = "failed_to_convert_quantity_to_base_unit"

	// Product Error Keys

	ErrKeyFailedToCreateProduct = "failed_to_create_product"

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

	// Initial Stock Import Error Keys (request level)

	ErrKeyInitialStockFileRequired       = "initial_stock_file_required"
	ErrKeyInitialStockInvalidFileType    = "initial_stock_invalid_file_type"
	ErrKeyInitialStockFileTooLarge       = "initial_stock_file_too_large"
	ErrKeyInitialStockEmptyFile          = "initial_stock_empty_file"
	ErrKeyInitialStockInvalidDryRun      = "initial_stock_invalid_dry_run"
	ErrKeyInitialStockInventoryRequired  = "initial_stock_inventory_required"
	ErrKeyInitialStockSheetNameRequired  = "initial_stock_sheet_name_required"
	ErrKeyInitialStockSheetNotFound      = "initial_stock_sheet_not_found"
	ErrKeyInitialStockHeaderMismatch     = "initial_stock_header_mismatch"
	ErrKeyInitialStockNoDataRows         = "initial_stock_no_data_rows"
	ErrKeyInitialStockParseFailed        = "initial_stock_parse_failed"
	ErrKeyInitialStockInventoryNotFound  = "initial_stock_inventory_not_found"
	ErrKeyInitialStockAlreadyImported    = "initial_stock_already_imported"
	ErrKeyInitialStockReconcileOpen      = "initial_stock_reconcile_open"
	ErrKeyInitialStockKeyPayloadMismatch = "initial_stock_key_payload_mismatch"
	ErrKeyInitialStockKeyTooLong         = "initial_stock_key_too_long"

	// Initial Stock Import row-level keys. Always rendered Vietnamese: BatchError
	// never localizes Locations, and the frontend owns only the row label.

	ErrKeyInitialStockRowNameRequired       = "initial_stock_row_name_required"
	ErrKeyInitialStockRowDuplicateName      = "initial_stock_row_duplicate_name"
	ErrKeyInitialStockRowQuantityInvalid    = "initial_stock_row_quantity_invalid"
	ErrKeyInitialStockRowQuantityNegative   = "initial_stock_row_quantity_negative"
	ErrKeyInitialStockRowQuantityScale      = "initial_stock_row_quantity_scale"
	ErrKeyInitialStockRowUnitRequired       = "initial_stock_row_unit_required"
	ErrKeyInitialStockRowUnitMismatch       = "initial_stock_row_unit_mismatch"
	ErrKeyInitialStockRowUnitSoftDeleted    = "initial_stock_row_unit_soft_deleted"
	ErrKeyInitialStockRowUnitAmbiguous      = "initial_stock_row_unit_ambiguous"
	ErrKeyInitialStockRowProductAmbiguous   = "initial_stock_row_product_ambiguous"
	ErrKeyInitialStockRowProductDeleted     = "initial_stock_row_product_deleted"
	ErrKeyInitialStockRowItemSoftDeleted    = "initial_stock_row_item_soft_deleted"
	ErrKeyInitialStockRowItemInactive       = "initial_stock_row_item_inactive"
	ErrKeyInitialStockRowProductTypeTooLong = "initial_stock_row_product_type_too_long"
	ErrKeyInitialStockRowNameTooLong        = "initial_stock_row_name_too_long"
	ErrKeyInitialStockRowUnitTooLong        = "initial_stock_row_unit_too_long"
	ErrKeyInitialStockRowQuantityTooLarge   = "initial_stock_row_quantity_too_large"
	ErrKeyInitialStockRowResultTooLarge     = "initial_stock_row_result_too_large"

	// Initial Stock Import row-level warning keys. Same catalogue and same Vietnamese
	// rendering as the row errors; a warned row still imports.

	WarnKeyInitialStockRowProductTypeIgnored           = "initial_stock_row_product_type_ignored"
	WarnKeyInitialStockRowProductTypeIgnoredNoType     = "initial_stock_row_product_type_ignored_no_type"
	WarnKeyInitialStockRowProductTypeIgnoredUnreadable = "initial_stock_row_product_type_ignored_unreadable"
	WarnKeyInitialStockRowProductTypeUnreadable        = "initial_stock_row_product_type_unreadable"

	// Selling Price Error Keys

	ErrKeySellingPriceInventorySpecificUnsupported  = "selling_price_inventory_specific_unsupported"
	ErrKeySellingPriceInvalidEndEffectiveFromFormat = "selling_price_invalid_end_effective_from_format"
	ErrKeySellingPriceMoveEarliestNoTakeover        = "selling_price_move_earliest_no_takeover"
	ErrKeySellingPriceDeleteNoTakeover              = "selling_price_delete_no_takeover"
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
	ErrKeyCannotReceivePurchaseOrderWithStatus: {
		EN: "Cannot receive purchase order with status %s",
		VI: "Không thể nhận hàng cho đơn hàng với trạng thái %s",
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
	ErrKeyNoTransactionsInReportPeriod: {
		EN: "No transactions found for the selected period",
		VI: "Không tìm thấy giao dịch nào trong khoảng thời gian đã chọn",
	},
	ErrKeyActivePendingReconcileConflict: {
		EN: "Inventory %d already has a pending reconcile; resolve it before creating another",
		VI: "Kho %d đang có một phiếu kiểm kê chờ xử lý; vui lòng hoàn tất phiếu đó trước khi tạo phiếu mới",
	},
	// Reconciliation Request Item Errors
	ErrKeyReconItemNotFound: {
		EN: "Reconciliation item %d not found",
		VI: "Không tìm thấy mục kiểm kê %d",
	},
	ErrKeyReconParentNotFound: {
		EN: "Reconciliation submission %d not found",
		VI: "Không tìm thấy phiếu kiểm kê %d",
	},
	ErrKeyReconParentNotInitiated: {
		EN: "Submission %d is not an initiated reconciliation; it has no snapshot baseline for child items",
		VI: "Phiếu %d không phải là phiếu kiểm kê đã khởi tạo; không có số liệu nền (snapshot) cho các mục kiểm kê",
	},
	ErrKeyReconParentNotInFlight: {
		EN: "Reconciliation submission %d is no longer in progress (status %s); its items can no longer be modified",
		VI: "Phiếu kiểm kê %d không còn đang xử lý (trạng thái %s); không thể chỉnh sửa các mục của phiếu",
	},
	ErrKeyReconItemMissingQuantity: {
		EN: "Counted quantity is required for inventory item %d",
		VI: "Số lượng đếm được là bắt buộc cho sản phẩm %d",
	},
	ErrKeyReconItemNotOwned: {
		EN: "You can only modify reconciliation items you created",
		VI: "Bạn chỉ có thể chỉnh sửa các mục kiểm kê do chính bạn tạo",
	},
	ErrKeyReconItemNotInParent: {
		EN: "Reconciliation item %d does not belong to submission %d",
		VI: "Mục kiểm kê %d không thuộc phiếu %d",
	},
	ErrKeyReconItemImmutable: {
		EN: "Reconciliation item %d is applied and can no longer be modified",
		VI: "Mục kiểm kê %d đã được áp dụng và không thể chỉnh sửa",
	},
	ErrKeyReconItemCannotDeleteStatus: {
		EN: "Reconciliation item %d cannot be deleted in status %s; only in_progress or ready items may be deleted",
		VI: "Không thể xóa mục kiểm kê %d ở trạng thái %s; chỉ có thể xóa mục ở trạng thái in_progress hoặc ready",
	},
	ErrKeyReconItemInvalidTransition: {
		EN: "Invalid reconciliation item status transition from %s to %s",
		VI: "Chuyển trạng thái mục kiểm kê không hợp lệ từ %s sang %s",
	},
	ErrKeyReconItemNoSnapshotBaseline: {
		EN: "Product \"%s\" has no quantity recorded at the start of reconciliation for this reconciliation",
		VI: "Sản phẩm «%s» không có số lượng ghi nhận tại thời điểm bắt đầu đối soát cho phiếu kiểm kê này",
	},
	ErrKeyReconItemNegativeQuantity: {
		EN: "Counted quantity for inventory item %d must not be negative",
		VI: "Số lượng đếm được cho sản phẩm %d không được âm",
	},
	ErrKeyReconItemDuplicateLine: {
		EN: "Inventory item %d appears more than once in the reconciliation item payload",
		VI: "Sản phẩm %d xuất hiện nhiều lần trong mục kiểm kê",
	},
	// Per-count labels
	ErrKeyReconItemLabelRequiredForDuplicate: {
		EN: "Inventory item %d is already counted in this reconciliation; this additional count needs a non-empty label to tell it apart",
		VI: "Sản phẩm %d đã được đếm trong phiếu kiểm kê này; lần đếm thêm này cần một nhãn không để trống để phân biệt",
	},
	ErrKeyReconItemLabelConflict: {
		EN: "Label \"%[2]s\" is already used by another count of inventory item %[1]d in this reconciliation; use a distinct label",
		VI: "Nhãn \"%[2]s\" đã được dùng cho một lần đếm khác của sản phẩm %[1]d trong phiếu kiểm kê này; hãy dùng nhãn khác",
	},
	ErrKeyReconItemLabelTooLong: {
		EN: "Label for inventory item %d is too long (max %d characters)",
		VI: "Nhãn cho sản phẩm %d quá dài (tối đa %d ký tự)",
	},
	// Row-level (count-session) labels
	ErrKeyReconRowLabelRequired: {
		EN: "A label is required for this count session: you already have an unlabelled count in this reconciliation, so this one needs a label to tell them apart",
		VI: "Cần nhập nhãn cho lần kiểm đếm này: bạn đã có một lần kiểm đếm chưa đặt nhãn trong phiếu kiểm kê này, nên lần này cần một nhãn để phân biệt",
	},
	ErrKeyReconRowLabelConflict: {
		EN: "Label \"%s\" is already used by another of your count sessions in this reconciliation; use a distinct label",
		VI: "Nhãn \"%s\" đã được dùng cho một lần kiểm đếm khác của bạn trong phiếu kiểm kê này; hãy dùng nhãn khác",
	},
	ErrKeyReconRowLabelTooLong: {
		EN: "Count-session label is too long (max %d characters)",
		VI: "Nhãn lần kiểm đếm quá dài (tối đa %d ký tự)",
	},
	// Reconciliation management
	ErrKeyReconSubmissionClosed: {
		EN: "Reconciliation submission %d is closed (status %s); staff can no longer edit its items",
		VI: "Phiếu kiểm kê %d đã được đóng (trạng thái %s); nhân viên không thể chỉnh sửa các mục của phiếu",
	},
	ErrKeyReconInvalidLifecycleTransition: {
		EN: "Reconciliation submission %d cannot move from status %s to %s",
		VI: "Phiếu kiểm kê %d không thể chuyển từ trạng thái %s sang %s",
	},
	ErrKeyReconDriftWarning: {
		EN: "A stock-consuming submission (#%d, type %s) was processed at %s during this reconciliation; the counted baseline is no longer valid",
		VI: "Một phiếu làm giảm tồn kho (#%d, loại %s) đã được xử lý lúc %s trong thời gian kiểm kê; số liệu nền đã đếm không còn hợp lệ",
	},
	ErrKeyReconCannotRejectInLifecycle: {
		EN: "Reconciliation submission %d can no longer be rejected (lifecycle status %s); it must be reopened first, and a processed reconciliation is final",
		VI: "Phiếu kiểm kê %d không thể bị từ chối nữa (trạng thái %s); cần mở lại trước, và phiếu kiểm kê đã xử lý là kết quả cuối cùng",
	},
	ErrKeyReconNotReadySessionWarning: {
		EN: "Count session #%d (label %q, by %s) was still in progress, not yet marked ready for review, when this reconciliation was closed",
		VI: "Phiên kiểm đếm #%d (nhãn %q, bởi %s) vẫn đang thực hiện, chưa được đánh dấu sẵn sàng để duyệt, khi phiếu kiểm kê này được đóng",
	},
	ErrKeyReconNotReadySessionWarningNoLabel: {
		EN: "Count session #%d (by %s) was still in progress, not yet marked ready for review, when this reconciliation was closed",
		VI: "Phiên kiểm đếm #%d (bởi %s) vẫn đang thực hiện, chưa được đánh dấu sẵn sàng để duyệt, khi phiếu kiểm kê này được đóng",
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
	// Product Errors
	ErrKeyFailedToCreateProduct: {
		EN: "Failed to create product",
		VI: "Không thể tạo sản phẩm",
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

	// Selling Price Errors
	ErrKeySellingPriceInventorySpecificUnsupported: {
		EN: "inventory-specific selling price is not supported yet",
		VI: "giá bán riêng theo kho chưa được hỗ trợ",
	},
	ErrKeySellingPriceInvalidEndEffectiveFromFormat: {
		EN: "invalid end_effective_from date format, expected YYYY-MM-DD",
		VI: "định dạng ngày end_effective_from không hợp lệ, yêu cầu YYYY-MM-DD",
	},
	ErrKeySellingPriceMoveEarliestNoTakeover: {
		EN: "cannot move the earliest selling price later: the vacated window would have no selling price to take over",
		VI: "không thể dời giá bán sớm nhất sang ngày muộn hơn: khoảng thời gian bị bỏ trống sẽ không có giá bán nào thay thế",
	},
	ErrKeySellingPriceDeleteNoTakeover: {
		EN: "cannot delete: no previous selling price to take over the vacated window",
		VI: "không thể xóa: không có giá bán trước đó để thay thế cho khoảng thời gian bị bỏ trống",
	},

	// Initial Stock Import Errors (request level)

	ErrKeyInitialStockFileRequired: {
		EN: "A file is required",
		VI: "Vui lòng chọn file",
	},
	ErrKeyInitialStockInvalidFileType: {
		EN: "File must be an .xlsx file",
		VI: "File phải có định dạng .xlsx",
	},
	ErrKeyInitialStockFileTooLarge: {
		EN: "File is too large, the maximum size is %d MB",
		VI: "File quá lớn, kích thước tối đa là %d MB",
	},
	ErrKeyInitialStockEmptyFile: {
		EN: "The uploaded file is empty",
		VI: "File tải lên rỗng",
	},
	ErrKeyInitialStockInvalidDryRun: {
		EN: `dry_run is required and must be exactly "true" or "false"`,
		VI: `dry_run là bắt buộc và phải là "true" hoặc "false"`,
	},
	ErrKeyInitialStockInventoryRequired: {
		EN: "inventory_id is required",
		VI: "inventory_id là bắt buộc",
	},
	ErrKeyInitialStockSheetNameRequired: {
		EN: "sheet_name is required",
		VI: "sheet_name là bắt buộc",
	},
	ErrKeyInitialStockSheetNotFound: {
		EN: "Sheet %q was not found in the uploaded file",
		VI: "Không tìm thấy sheet %q trong file đã tải lên",
	},
	ErrKeyInitialStockHeaderMismatch: {
		EN: "Sheet %q does not have the expected header on row 3 (STT / TÊN / ĐVT / SỐ LƯỢNG)",
		VI: "Sheet %q không có đúng tiêu đề ở dòng 3 (STT / TÊN / ĐVT / SỐ LƯỢNG)",
	},
	ErrKeyInitialStockNoDataRows: {
		EN: "Sheet %q contains no data rows",
		VI: "Sheet %q không có dòng dữ liệu nào",
	},
	ErrKeyInitialStockParseFailed: {
		EN: "The uploaded file could not be read as an .xlsx workbook",
		VI: "Không thể đọc file tải lên dưới dạng workbook .xlsx",
	},
	ErrKeyInitialStockInventoryNotFound: {
		EN: "Inventory %d was not found or is not active",
		VI: "Không tìm thấy kho %d hoặc kho không hoạt động",
	},
	ErrKeyInitialStockAlreadyImported: {
		EN: "Initial stock has already been loaded into inventory %d",
		VI: "Kho %d đã được nạp tồn kho ban đầu",
	},
	ErrKeyInitialStockReconcileOpen: {
		EN: "Inventory %d has a reconciliation in progress. Process or cancel it before loading initial stock",
		VI: "Kho %d đang có phiên kiểm kê. Vui lòng xử lý hoặc hủy phiên kiểm kê trước khi nạp tồn kho ban đầu",
	},
	ErrKeyInitialStockKeyPayloadMismatch: {
		EN: "This Idempotency-Key was already used for a different file or sheet",
		VI: "Idempotency-Key này đã được dùng cho file hoặc sheet khác",
	},
	ErrKeyInitialStockKeyTooLong: {
		EN: "Idempotency-Key is longer than %d characters",
		VI: "Idempotency-Key dài hơn %d ký tự",
	},

	// Initial Stock Import Errors (row level)

	ErrKeyInitialStockRowNameRequired: {
		EN: "Product name is empty",
		VI: "Tên sản phẩm để trống",
	},
	ErrKeyInitialStockRowDuplicateName: {
		EN: "Product name %q appears more than once in this sheet",
		VI: "Tên sản phẩm %q xuất hiện nhiều lần trong sheet này",
	},
	ErrKeyInitialStockRowQuantityInvalid: {
		EN: "Quantity %q is not a number",
		VI: "Số lượng %q không phải là số",
	},
	ErrKeyInitialStockRowQuantityNegative: {
		EN: "Quantity %s must not be negative",
		VI: "Số lượng %s không được âm",
	},
	ErrKeyInitialStockRowQuantityScale: {
		EN: "Quantity has %d decimal places, more than the allowed %d",
		VI: "Số lượng có %d chữ số thập phân, nhiều hơn mức cho phép %d",
	},
	ErrKeyInitialStockRowUnitRequired: {
		EN: "Unit is empty",
		VI: "Đơn vị để trống",
	},
	ErrKeyInitialStockRowUnitMismatch: {
		EN: "Sheet unit %q does not match the unit %q already used for this product",
		VI: "Đơn vị %q trong sheet không khớp với đơn vị %q đang dùng cho sản phẩm này",
	},
	ErrKeyInitialStockRowUnitAmbiguous: {
		EN: "Unit %q matches %d existing units",
		VI: "Đơn vị %q khớp với %d đơn vị đang tồn tại",
	},
	ErrKeyInitialStockRowUnitSoftDeleted: {
		EN: "Unit %q exists but has been deleted (unit id %d); restore it before loading",
		VI: "Đơn vị %q đã tồn tại nhưng đã bị xóa (đơn vị id %d); vui lòng phục hồi trước khi nạp",
	},
	ErrKeyInitialStockRowProductAmbiguous: {
		EN: "Product name %q matches %d existing products",
		VI: "Tên sản phẩm %q khớp với %d sản phẩm đang tồn tại",
	},
	ErrKeyInitialStockRowProductDeleted: {
		EN: "Product %q exists but has been deleted (product id %d); restore it before loading",
		VI: "Sản phẩm %q đã tồn tại nhưng đã bị xóa (sản phẩm id %d); vui lòng phục hồi trước khi nạp",
	},
	ErrKeyInitialStockRowItemSoftDeleted: {
		EN: "This product's inventory item has been deleted (inventory item id %d); restore it before loading",
		VI: "Mục tồn kho của sản phẩm này đã bị xóa (mục tồn kho id %d); vui lòng phục hồi trước khi nạp",
	},
	ErrKeyInitialStockRowItemInactive: {
		EN: "This product's inventory item is inactive (inventory item id %d); activate it before loading",
		VI: "Mục tồn kho của sản phẩm này đang không hoạt động (mục tồn kho id %d); vui lòng kích hoạt trước khi nạp",
	},
	ErrKeyInitialStockRowProductTypeTooLong: {
		EN: "Product type %q is longer than 20 characters",
		VI: "Loại sản phẩm %q dài hơn 20 ký tự",
	},
	ErrKeyInitialStockRowNameTooLong: {
		EN: "Product name is %d characters, longer than the allowed %d",
		VI: "Tên sản phẩm dài %d ký tự, vượt quá mức cho phép %d",
	},
	ErrKeyInitialStockRowUnitTooLong: {
		EN: "Unit %q is %d characters, longer than the allowed %d",
		VI: "Đơn vị %q dài %d ký tự, vượt quá mức cho phép %d",
	},
	ErrKeyInitialStockRowQuantityTooLarge: {
		EN: "Quantity %s is too large; at most %d digits before the decimal point are allowed",
		VI: "Số lượng %s quá lớn; chỉ cho phép tối đa %d chữ số trước dấu thập phân",
	},
	ErrKeyInitialStockRowResultTooLarge: {
		EN: "Adding %s to the current on-hand %s exceeds the maximum storable quantity",
		VI: "Cộng %s vào tồn kho hiện tại %s sẽ vượt quá số lượng tối đa có thể lưu",
	},

	// Initial Stock Import Warnings (row level)

	WarnKeyInitialStockRowProductTypeIgnored: {
		EN: "Product type %q in the sheet is not applied; the existing product keeps its type %q",
		VI: "Loại sản phẩm %q trong sheet không được áp dụng; sản phẩm đã tồn tại giữ loại %q",
	},
	WarnKeyInitialStockRowProductTypeIgnoredNoType: {
		EN: "Product type %q in the sheet is not applied; the existing product keeps no product type",
		VI: "Loại sản phẩm %q trong sheet không được áp dụng; sản phẩm đã tồn tại không có loại sản phẩm",
	},
	WarnKeyInitialStockRowProductTypeIgnoredUnreadable: {
		EN: "Product type %q in the sheet was not applied; the product this row loaded can no longer be read, so its type cannot be shown",
		VI: "Loại sản phẩm %q trong sheet đã không được áp dụng; không còn đọc được sản phẩm mà dòng này đã nạp nên không thể hiển thị loại của nó",
	},
	WarnKeyInitialStockRowProductTypeUnreadable: {
		EN: "The product type cannot be shown: the product this row loaded can no longer be read",
		VI: "Không thể hiển thị loại sản phẩm: không còn đọc được sản phẩm mà dòng này đã nạp",
	},
}

// RowMessage renders a row-level import message in Vietnamese regardless of the
// request language: BatchError.MarshalJSON emits Locations verbatim, and the
// frontend localizes only the row label, not the reason.
func RowMessage(key string, args ...interface{}) string {
	tmpl := GetErrorMessageByLang(key, LangVI)
	if tmpl == "" {
		return key
	}
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}

// ErrInitialStock builds a keyed AppError for the initial-stock tool. Every
// request-level failure carries a MessageKey so the frontend can branch on it
// without parsing prose; Message is the English fallback.
func ErrInitialStock(code ErrorCode, key string, args ...interface{}) *AppError {
	fallback := GetErrorMessageByLang(key, LangEN)
	if len(args) > 0 {
		fallback = fmt.Sprintf(fallback, args...)
	}
	err := NewAppError(code, fallback, nil)
	err.MessageKey = key
	err.MessageArgs = args
	return err
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

// ErrNoTransactionsInReportPeriod creates an error for no transactions in report period
func ErrNoTransactionsInReportPeriod(ctx context.Context) *AppError {
	message := getErrorMessage(ctx, ErrKeyNoTransactionsInReportPeriod)
	return NewAppError(ErrorCodeNotFound, message, nil)
}

// ErrActivePendingReconcileConflict is the domain conflict for the one-active-pending-reconcile guard.
func ErrActivePendingReconcileConflict(inventoryID uint, cause error) *AppError {
	err := NewAppError(
		ErrorCodeActivePendingReconcileConflict,
		fmt.Sprintf(ErrorMessages[ErrKeyActivePendingReconcileConflict].EN, inventoryID),
		cause,
	)
	err.MessageKey = ErrKeyActivePendingReconcileConflict
	err.MessageArgs = []interface{}{inventoryID}
	return err
}

// ErrDuplicateOrderNumber is the domain error for an order_number unique-constraint violation.
func ErrDuplicateOrderNumber(cause error) *AppError {
	return NewAppError(ErrorCodeDuplicateOrderNumber, "duplicate purchase order number", cause)
}

func ErrReconcileValidationFailed(message string) *AppError {
	return NewAppError(ErrorCodeReconcileValidationFailed, message, nil)
}

// --- Reconciliation request item domain errors ---

// newKeyedValidationError builds a validation AppError carrying its catalog key for localization.
func newKeyedValidationError(ctx context.Context, key string, args ...interface{}) *AppError {
	tmpl := getErrorMessage(ctx, key)
	message := tmpl
	if len(args) > 0 {
		message = fmt.Sprintf(tmpl, args...)
	}
	err := NewAppError(ErrorCodeValidation, message, nil)
	err.MessageKey = key
	err.MessageArgs = args
	return err
}

// ErrReconItemNotFound is a 404 for a missing/soft-deleted child item.
func ErrReconItemNotFound(ctx context.Context, itemID uint) *AppError {
	return NewAppError(ErrorCodeNotFound,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemNotFound), itemID), nil)
}

// ErrReconParentNotFound is a 404 for a missing parent submission.
func ErrReconParentNotFound(ctx context.Context, submissionID uint) *AppError {
	return NewAppError(ErrorCodeNotFound,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconParentNotFound), submissionID), nil)
}

// ErrReconParentNotInitiated is a 400/validation: the parent has no snapshot baseline.
func ErrReconParentNotInitiated(ctx context.Context, submissionID uint) *AppError {
	return NewAppError(ErrorCodeValidation,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconParentNotInitiated), submissionID), nil)
}

// ErrReconParentNotInFlight is a 409/conflict: the parent reconciliation is no longer in flight.
func ErrReconParentNotInFlight(ctx context.Context, submissionID uint, status string) *AppError {
	return NewAppError(ErrorCodeConflict,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconParentNotInFlight), submissionID, status), nil)
}

// ErrReconItemMissingQuantity is a 400/validation for a counted line with no quantity.
func ErrReconItemMissingQuantity(ctx context.Context, itemID uint) *AppError {
	return newKeyedValidationError(ctx, ErrKeyReconItemMissingQuantity, itemID)
}

// ErrReconItemNotOwned is a 403: a staff user tried to touch another user's row.
func ErrReconItemNotOwned(ctx context.Context) *AppError {
	return NewAppError(ErrorCodeForbidden, getErrorMessage(ctx, ErrKeyReconItemNotOwned), nil)
}

// ErrReconItemNotInParent is a 404: the item belongs to a different parent submission.
func ErrReconItemNotInParent(ctx context.Context, itemID, submissionID uint) *AppError {
	return NewAppError(ErrorCodeNotFound,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemNotInParent), itemID, submissionID), nil)
}

// ErrReconItemImmutable is a 409/conflict: the row is applied and immutable.
func ErrReconItemImmutable(ctx context.Context, itemID uint) *AppError {
	return NewAppError(ErrorCodeConflict,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemImmutable), itemID), nil)
}

// ErrReconItemCannotDeleteStatus is a 409/conflict: only in_progress/ready rows may be deleted.
func ErrReconItemCannotDeleteStatus(ctx context.Context, itemID uint, status string) *AppError {
	return NewAppError(ErrorCodeConflict,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemCannotDeleteStatus), itemID, status), nil)
}

// ErrReconItemInvalidTransition is a 409/conflict for an illegal status move.
func ErrReconItemInvalidTransition(ctx context.Context, from, to string) *AppError {
	return NewAppError(ErrorCodeConflict,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemInvalidTransition), from, to), nil)
}

// ErrReconItemNoSnapshotBaseline is a 400/validation: the counted item has no snapshot baseline.
func ErrReconItemNoSnapshotBaseline(ctx context.Context, productName string) *AppError {
	return NewAppError(ErrorCodeValidation,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemNoSnapshotBaseline), productName), nil)
}

// ErrReconItemNegativeQuantity is a 400/validation for a negative counted qty.
func ErrReconItemNegativeQuantity(ctx context.Context, itemID uint) *AppError {
	return NewAppError(ErrorCodeValidation,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemNegativeQuantity), itemID), nil)
}

// ErrReconItemDuplicateLine is a 400/validation: the same inventory item appears twice in one payload.
func ErrReconItemDuplicateLine(ctx context.Context, itemID uint) *AppError {
	return NewAppError(ErrorCodeValidation,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemDuplicateLine), itemID), nil)
}

// ErrReconItemLabelRequiredForDuplicate is a 400/validation: a duplicate count needs a distinguishing label.
func ErrReconItemLabelRequiredForDuplicate(ctx context.Context, itemID uint) *AppError {
	return newKeyedValidationError(ctx, ErrKeyReconItemLabelRequiredForDuplicate, itemID)
}

// ErrReconItemLabelConflict is a 400/validation: the label collides with another count of the same item.
func ErrReconItemLabelConflict(ctx context.Context, itemID uint, label string) *AppError {
	return newKeyedValidationError(ctx, ErrKeyReconItemLabelConflict, itemID, label)
}

// ErrReconItemLabelTooLong is a 400/validation: a count label exceeds the maximum length.
func ErrReconItemLabelTooLong(ctx context.Context, itemID uint, maxLen int) *AppError {
	return newKeyedValidationError(ctx, ErrKeyReconItemLabelTooLong, itemID, maxLen)
}

// ErrReconRowLabelRequired is a 400/validation: a count session needs a label when the owner already has an unlabelled one.
func ErrReconRowLabelRequired(ctx context.Context) *AppError {
	return newKeyedValidationError(ctx, ErrKeyReconRowLabelRequired)
}

// ErrReconRowLabelConflict is a 400/validation: the row label collides with another of the owner's rows.
func ErrReconRowLabelConflict(ctx context.Context, label string) *AppError {
	return newKeyedValidationError(ctx, ErrKeyReconRowLabelConflict, label)
}

// ErrReconRowLabelTooLong is a 400/validation: a row (count-session) label exceeds the maximum length.
func ErrReconRowLabelTooLong(ctx context.Context, maxLen int) *AppError {
	return newKeyedValidationError(ctx, ErrKeyReconRowLabelTooLong, maxLen)
}

// --- Reconciliation management domain errors ---

// ErrReconSubmissionClosed is a 409/conflict: staff cannot edit a closed reconciliation.
func ErrReconSubmissionClosed(ctx context.Context, submissionID uint, status string) *AppError {
	return NewAppError(ErrorCodeConflict,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconSubmissionClosed), submissionID, status), nil)
}

// ErrReconInvalidLifecycleTransition is a 409/conflict for an illegal lifecycle transition.
func ErrReconInvalidLifecycleTransition(ctx context.Context, submissionID uint, from, to string) *AppError {
	return NewAppError(ErrorCodeConflict,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconInvalidLifecycleTransition), submissionID, from, to), nil)
}

// ErrReconCannotRejectInLifecycle is a 409/conflict: the reconciliation can no longer be rejected.
func ErrReconCannotRejectInLifecycle(ctx context.Context, submissionID uint, status string) *AppError {
	return NewAppError(ErrorCodeConflict,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconCannotRejectInLifecycle), submissionID, status), nil)
}

// ReconDriftWarning renders a localized warning line for a consuming submission processed during the reconcile window.
func ReconDriftWarning(ctx context.Context, submissionID uint, submissionType, processedAt string) string {
	return fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconDriftWarning), submissionID, submissionType, processedAt)
}

// ReconNotReadySessionsWarning renders one advisory line for a count session
// still in_progress at close time.
func ReconNotReadySessionsWarning(ctx context.Context, sessionID uint, label, createdBy string) string {
	if strings.TrimSpace(label) == "" {
		return fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconNotReadySessionWarningNoLabel), sessionID, createdBy)
	}
	return fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconNotReadySessionWarning), sessionID, label, createdBy)
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

// ErrCannotReceivePurchaseOrderWithStatus creates an error when receiving a purchase order in a terminal status
func ErrCannotReceivePurchaseOrderWithStatus(ctx context.Context, status string) *AppError {
	template := getErrorMessage(ctx, ErrKeyCannotReceivePurchaseOrderWithStatus)
	message := fmt.Sprintf(template, status)
	return NewAppError(ErrorCodeConflict, message, nil)
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

// Product Error Helpers

// ErrFailedToCreateProduct creates an error for failed product creation
func ErrFailedToCreateProduct(ctx context.Context, cause error) *AppError {
	message := getErrorMessage(ctx, ErrKeyFailedToCreateProduct)
	return NewAppError(ErrorCodeInternal, message, cause)
}

// Selling Price Error Helpers

// ErrSellingPriceInventorySpecificUnsupported creates an error when an inventory-scoped selling price is requested
func ErrSellingPriceInventorySpecificUnsupported(ctx context.Context) *AppError {
	return ErrValidation(getErrorMessage(ctx, ErrKeySellingPriceInventorySpecificUnsupported), nil)
}

// ErrSellingPriceInvalidEndEffectiveFromFormat creates an error for a malformed end_effective_from date
func ErrSellingPriceInvalidEndEffectiveFromFormat(ctx context.Context, cause error) *AppError {
	return ErrValidation(getErrorMessage(ctx, ErrKeySellingPriceInvalidEndEffectiveFromFormat), cause)
}

// ErrSellingPriceMoveEarliestNoTakeover creates an error when moving the earliest price later would leave a window with no price to take over
func ErrSellingPriceMoveEarliestNoTakeover(ctx context.Context) *AppError {
	return ErrValidation(getErrorMessage(ctx, ErrKeySellingPriceMoveEarliestNoTakeover), nil)
}

// ErrSellingPriceDeleteNoTakeover creates an error when deleting a price would leave the vacated window with no previous price to take over
func ErrSellingPriceDeleteNoTakeover(ctx context.Context) *AppError {
	return ErrValidation(getErrorMessage(ctx, ErrKeySellingPriceDeleteNoTakeover), nil)
}
