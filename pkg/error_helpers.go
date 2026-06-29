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

	ErrKeyInventoryItemNotFound          = "inventory_item_not_found"
	ErrKeyOptimisticLockConflict         = "optimistic_lock_conflict"
	ErrKeyNoTransactionsInReportPeriod   = "no_transactions_in_report_period"
	ErrKeyActivePendingReconcileConflict = "active_pending_reconcile_conflict"

	// Reconciliation Request Item Error Keys (epic #38, Part 4)

	ErrKeyReconItemNotFound                 = "recon_item_not_found"
	ErrKeyReconParentNotFound               = "recon_parent_not_found"
	ErrKeyReconParentNotInitiated           = "recon_parent_not_initiated"
	ErrKeyReconParentNotInFlight            = "recon_parent_not_in_flight"
	ErrKeyReconItemMissingQuantity          = "recon_item_missing_quantity"
	ErrKeyReconItemNotOwned                 = "recon_item_not_owned"
	ErrKeyReconItemNotInParent              = "recon_item_not_in_parent"
	ErrKeyReconItemImmutable                = "recon_item_immutable"
	ErrKeyReconItemCannotDeleteStatus       = "recon_item_cannot_delete_status"
	ErrKeyReconItemInvalidTransition        = "recon_item_invalid_transition"
	ErrKeyReconItemCountExceedsBaseline     = "recon_item_count_exceeds_baseline"
	ErrKeyReconItemAggregateExceedsBaseline = "recon_item_aggregate_exceeds_baseline"
	ErrKeyReconItemNoSnapshotBaseline       = "recon_item_no_snapshot_baseline"
	ErrKeyReconItemNegativeQuantity         = "recon_item_negative_quantity"
	ErrKeyReconItemDuplicateLine            = "recon_item_duplicate_line"

	// Per-count label keys (issue #73): distinguishing multiple counts of the same
	// inventory item within a reconciliation submission.

	ErrKeyReconItemLabelRequiredForDuplicate = "recon_item_label_required_for_duplicate"
	ErrKeyReconItemLabelConflict             = "recon_item_label_conflict"
	ErrKeyReconItemLabelTooLong              = "recon_item_label_too_long"

	// Row-level (count-session) label keys (issue #73): the label on the
	// reconciliation_request_items row that identifies a staff user's count session.

	ErrKeyReconRowLabelRequired = "recon_row_label_required"
	ErrKeyReconRowLabelConflict = "recon_row_label_conflict"
	ErrKeyReconRowLabelTooLong  = "recon_row_label_too_long"

	// Reconciliation management Error Keys (epic #38, Part 6 redesign)

	ErrKeyReconSubmissionClosed           = "recon_submission_closed"
	ErrKeyReconInvalidLifecycleTransition = "recon_invalid_lifecycle_transition"
	ErrKeyReconDriftWarning               = "recon_drift_warning"
	ErrKeyReconCannotRejectInLifecycle    = "recon_cannot_reject_in_lifecycle"

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
	// Reconciliation Request Item Errors (epic #38, Part 4)
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
	ErrKeyReconItemCountExceedsBaseline: {
		EN: "Counted quantity %s for inventory item %d exceeds the snapshot baseline %s",
		VI: "Số lượng đếm được %s cho sản phẩm %d vượt quá số liệu nền %s",
	},
	ErrKeyReconItemAggregateExceedsBaseline: {
		EN: "Total counted quantity %s for inventory item %d across all staff submissions exceeds the snapshot baseline %s",
		VI: "Tổng số lượng đếm được %s cho sản phẩm %d trên tất cả các phiếu kiểm kê của nhân viên vượt quá số liệu nền %s",
	},
	ErrKeyReconItemNoSnapshotBaseline: {
		EN: "Inventory item %d has no snapshot baseline for this reconciliation",
		VI: "Sản phẩm %d không có số liệu nền cho phiếu kiểm kê này",
	},
	ErrKeyReconItemNegativeQuantity: {
		EN: "Counted quantity for inventory item %d must not be negative",
		VI: "Số lượng đếm được cho sản phẩm %d không được âm",
	},
	ErrKeyReconItemDuplicateLine: {
		EN: "Inventory item %d appears more than once in the reconciliation item payload",
		VI: "Sản phẩm %d xuất hiện nhiều lần trong mục kiểm kê",
	},
	// Per-count labels (issue #73)
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
	// Row-level (count-session) labels (issue #73)
	ErrKeyReconRowLabelRequired: {
		EN: "A label is required for this count session: you already have another count in this reconciliation, so each one needs a label to tell them apart",
		VI: "Cần nhập nhãn cho lần kiểm đếm này: bạn đã có một lần kiểm đếm khác trong phiếu kiểm kê này, nên mỗi lần cần một nhãn để phân biệt",
	},
	ErrKeyReconRowLabelConflict: {
		EN: "Label \"%s\" is already used by another of your count sessions in this reconciliation; use a distinct label",
		VI: "Nhãn \"%s\" đã được dùng cho một lần kiểm đếm khác của bạn trong phiếu kiểm kê này; hãy dùng nhãn khác",
	},
	ErrKeyReconRowLabelTooLong: {
		EN: "Count-session label is too long (max %d characters)",
		VI: "Nhãn lần kiểm đếm quá dài (tối đa %d ký tự)",
	},
	// Reconciliation management (epic #38, Part 6 redesign)
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

// ErrActivePendingReconcileConflict is the domain conflict for the
// one-active-pending-reconcile guard (#38 P3). Both the service pre-check and the
// repo unique-violation translator return it. It defers localization to the error
// handler (MessageKey + inventoryID) so the language-agnostic repo layer can raise
// it; Message is the English fallback.
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

// ErrDuplicateOrderNumber is the domain error the purchase-order repository
// returns when an INSERT violates the order_number unique constraint (#84 race).
// It lets the repository translate the DB-specific 23505 / constraint-name detail
// into a typed signal the service can match (via IsErrorCode /
// ErrorCodeDuplicateOrderNumber) to decide a regenerate-and-retry, without the
// service knowing any SQLSTATE codes or constraint names. The original DB error
// is preserved as the cause.
func ErrDuplicateOrderNumber(cause error) *AppError {
	return NewAppError(ErrorCodeDuplicateOrderNumber, "duplicate purchase order number", cause)
}

func ErrReconcileValidationFailed(message string) *AppError {
	return NewAppError(ErrorCodeReconcileValidationFailed, message, nil)
}

// --- Reconciliation request item domain errors (epic #38, Part 4) ---
// Each maps a child-item rejection to a localized message + the correct HTTP
// status via its ErrorCode, mirroring the Part 3 layering (no raw errors leak).

// newKeyedValidationError builds a 400/validation AppError that carries its
// catalog key so the error handler can expose a stable, language-independent
// "key" field for frontend routing (issue #42). It also resolves Message eagerly
// (so the static fallback and any json.Marshal path stay populated) while setting
// MessageKey/MessageArgs so the handler re-localizes per request language.
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

// ErrReconParentNotInitiated is a 400/validation: the parent is not a reconcile
// started via initiate (no snapshot baseline), so child items cannot be filed.
func ErrReconParentNotInitiated(ctx context.Context, submissionID uint) *AppError {
	return NewAppError(ErrorCodeValidation,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconParentNotInitiated), submissionID), nil)
}

// ErrReconParentNotInFlight is a 409/conflict: the parent reconciliation has left
// the in-flight (approval pending) state — it was rejected/canceled or already
// approved/applied — so its child items can no longer be created/edited/deleted.
func ErrReconParentNotInFlight(ctx context.Context, submissionID uint, status string) *AppError {
	return NewAppError(ErrorCodeConflict,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconParentNotInFlight), submissionID, status), nil)
}

// ErrReconItemMissingQuantity is a 400/validation: a counted line omitted the
// quantity entirely (distinct from an explicit zero count).
func ErrReconItemMissingQuantity(ctx context.Context, itemID uint) *AppError {
	return newKeyedValidationError(ctx, ErrKeyReconItemMissingQuantity, itemID)
}

// ErrReconItemNotOwned is a 403: a staff user tried to touch another user's row.
func ErrReconItemNotOwned(ctx context.Context) *AppError {
	return NewAppError(ErrorCodeForbidden, getErrorMessage(ctx, ErrKeyReconItemNotOwned), nil)
}

// ErrReconItemNotInParent is a 404/validation: the item exists but under a
// different parent submission than the path-scoped one.
func ErrReconItemNotInParent(ctx context.Context, itemID, submissionID uint) *AppError {
	return NewAppError(ErrorCodeNotFound,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemNotInParent), itemID, submissionID), nil)
}

// ErrReconItemImmutable is a 409/conflict: the row is applied and immutable.
func ErrReconItemImmutable(ctx context.Context, itemID uint) *AppError {
	return NewAppError(ErrorCodeConflict,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemImmutable), itemID), nil)
}

// ErrReconItemCannotDeleteStatus is a 409/conflict: only in_progress/ready rows
// may be soft-deleted.
func ErrReconItemCannotDeleteStatus(ctx context.Context, itemID uint, status string) *AppError {
	return NewAppError(ErrorCodeConflict,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemCannotDeleteStatus), itemID, status), nil)
}

// ErrReconItemInvalidTransition is a 409/conflict for an illegal status move.
func ErrReconItemInvalidTransition(ctx context.Context, from, to string) *AppError {
	return NewAppError(ErrorCodeConflict,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemInvalidTransition), from, to), nil)
}

// ErrReconItemCountExceedsBaseline is a 400/validation for the S2 rule:
// counted > snapshot baseline is rejected (no positive-adjustment mechanism).
func ErrReconItemCountExceedsBaseline(ctx context.Context, itemID uint, counted, baseline decimal.Decimal) *AppError {
	return NewAppError(ErrorCodeValidation,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemCountExceedsBaseline), counted.String(), itemID, baseline.String()), nil)
}

// ErrReconItemAggregateExceedsBaseline is a 400/validation for the cross-row S2
// rule: the SUM of counted quantities for one inventory item across ALL live
// (non-deleted) staff child rows of the same parent reconcile — which are summed
// by item at synthesis — must not exceed the snapshot baseline. This generalizes
// the per-row ErrReconItemCountExceedsBaseline so two rows of 80 against a
// baseline of 100 cannot each pass per-row yet sum to 160.
func ErrReconItemAggregateExceedsBaseline(ctx context.Context, itemID uint, total, baseline decimal.Decimal) *AppError {
	return NewAppError(ErrorCodeValidation,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemAggregateExceedsBaseline), total.String(), itemID, baseline.String()), nil)
}

// ErrReconItemNoSnapshotBaseline is a 400/validation: a counted item has no
// snapshot row (e.g. an item added after initiate), so there is no baseline.
func ErrReconItemNoSnapshotBaseline(ctx context.Context, itemID uint) *AppError {
	return NewAppError(ErrorCodeValidation,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemNoSnapshotBaseline), itemID), nil)
}

// ErrReconItemNegativeQuantity is a 400/validation for a negative counted qty.
func ErrReconItemNegativeQuantity(ctx context.Context, itemID uint) *AppError {
	return NewAppError(ErrorCodeValidation,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemNegativeQuantity), itemID), nil)
}

// ErrReconItemDuplicateLine is a 400/validation: the same inventory item appears
// twice in one child payload (would make the per-item baseline check ambiguous).
func ErrReconItemDuplicateLine(ctx context.Context, itemID uint) *AppError {
	return NewAppError(ErrorCodeValidation,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconItemDuplicateLine), itemID), nil)
}

// ErrReconItemLabelRequiredForDuplicate is a 400/validation for the issue #73 rule:
// once an inventory item already has another live count in the submission, a
// further count must carry a non-empty label so the two contributions can be told
// apart in review (synthesis sums them by item, erasing the distinction otherwise).
func ErrReconItemLabelRequiredForDuplicate(ctx context.Context, itemID uint) *AppError {
	return newKeyedValidationError(ctx, ErrKeyReconItemLabelRequiredForDuplicate, itemID)
}

// ErrReconItemLabelConflict is a 400/validation for the issue #73 rule: the label
// supplied for a count collides with a label already used by another live count of
// the same inventory item in this submission (labels must be distinct per item).
func ErrReconItemLabelConflict(ctx context.Context, itemID uint, label string) *AppError {
	return newKeyedValidationError(ctx, ErrKeyReconItemLabelConflict, itemID, label)
}

// ErrReconItemLabelTooLong is a 400/validation: a count label exceeds the
// app-validated maximum length in RUNES (issue #73; the JSONB payload has no
// length constraint, so the cap is enforced here).
func ErrReconItemLabelTooLong(ctx context.Context, itemID uint, maxLen int) *AppError {
	return newKeyedValidationError(ctx, ErrKeyReconItemLabelTooLong, itemID, maxLen)
}

// ErrReconRowLabelRequired is a 400/validation for the issue #73 ROW-level rule: a
// count session (reconciliation_request_items row) needs a label once its owner
// already has another live row in the submission (the first/only row may be blank).
func ErrReconRowLabelRequired(ctx context.Context) *AppError {
	return newKeyedValidationError(ctx, ErrKeyReconRowLabelRequired)
}

// ErrReconRowLabelConflict is a 400/validation for the issue #73 ROW-level rule:
// the row label collides with a label already used by another of the owner's live
// rows in this submission (row labels must be distinct per (submission, user)).
func ErrReconRowLabelConflict(ctx context.Context, label string) *AppError {
	return newKeyedValidationError(ctx, ErrKeyReconRowLabelConflict, label)
}

// ErrReconRowLabelTooLong is a 400/validation: a row (count-session) label exceeds
// the app-validated maximum length in RUNES (issue #73).
func ErrReconRowLabelTooLong(ctx context.Context, maxLen int) *AppError {
	return newKeyedValidationError(ctx, ErrKeyReconRowLabelTooLong, maxLen)
}

// --- Reconciliation management domain errors (epic #38, Part 6 redesign) ---

// ErrReconSubmissionClosed is a 409/conflict: a staff member tried to edit a
// child row of a reconciliation that an admin/accountant has already closed (or
// that is processing/processed). Staff are locked out once closed.
func ErrReconSubmissionClosed(ctx context.Context, submissionID uint, status string) *AppError {
	return NewAppError(ErrorCodeConflict,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconSubmissionClosed), submissionID, status), nil)
}

// ErrReconInvalidLifecycleTransition is a 409/conflict for an illegal
// close/reopen/start-processing transition (e.g. close on an already-closed
// submission, start-processing on an open one).
func ErrReconInvalidLifecycleTransition(ctx context.Context, submissionID uint, from, to string) *AppError {
	return NewAppError(ErrorCodeConflict,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconInvalidLifecycleTransition), submissionID, from, to), nil)
}

// ErrReconCannotRejectInLifecycle is a 409/conflict: a legacy reject was attempted
// on an initiated reconcile that has already left the staff-editable `open` state
// (it is closed/processing/processed). Once closed, the admin must reopen before a
// reject is meaningful; once processing/processed the apply has consumed stock and
// the reconcile is terminal, so a reject can no longer flip it without corrupting
// the applied consuming inventory transactions. Evaluated under the parent FOR
// UPDATE lock on the freshly-read status, so it cannot race StartProcessing.
func ErrReconCannotRejectInLifecycle(ctx context.Context, submissionID uint, status string) *AppError {
	return NewAppError(ErrorCodeConflict,
		fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconCannotRejectInLifecycle), submissionID, status), nil)
}

// ReconDriftWarning renders one warning-shaped line for a consuming submission
// that processed during the reconcile window (locked decision Q8). It is a plain
// localized string, not an error — Start Processing returns these in the result's
// Warnings list after rolling back.
func ReconDriftWarning(ctx context.Context, submissionID uint, submissionType, processedAt string) string {
	return fmt.Sprintf(getErrorMessage(ctx, ErrKeyReconDriftWarning), submissionID, submissionType, processedAt)
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
