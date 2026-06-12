package dto

// InventoryInOutExportRequest is the request payload for the inventory in/out
// Excel export endpoint. Mirrors the InventoryTimelineRequest filter shape.
type InventoryInOutExportRequest struct {
	InventoryID uint   `param:"id" validate:"required"`
	StartDate   string `query:"start_date" validate:"required"`
	EndDate     string `query:"end_date" validate:"required"`
	// IgnoreMissingSellingPrice bypasses the missing-selling-price precondition:
	// the export proceeds and uncomputable values render as "-".
	IgnoreMissingSellingPrice bool `query:"ignore_missing_selling_price"`
}

// InventoryInOutExportResponse is returned on success: a presigned download URL
// for the generated xlsx and the filename the client should suggest.
type InventoryInOutExportResponse struct {
	DownloadURL string `json:"download_url"`
	Filename    string `json:"filename"`
}

// MissingSellingPricePO identifies a PO that blocks export because no selling
// price (PO-item override or product default) is set for one of its items.
type MissingSellingPricePO struct {
	POID     uint   `json:"po_id"`
	PONumber string `json:"po_number"`
}
