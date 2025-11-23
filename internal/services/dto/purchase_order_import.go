package dto

import (
	"cim-backend/pkg"
	"time"
)

// UploadPurchaseOrderFileResponse represents the response from the upload API
type UploadPurchaseOrderFileResponse struct {
	FileUID       string                  `json:"file_uid"`
	FileName      string                  `json:"file_name"`
	FileExtension string                  `json:"file_extension"`
	UploadedAt    time.Time               `json:"uploaded_at"`
	Sheets        []SheetValidationResult `json:"sheets"`
}

// SheetValidationResult represents the validation result for a single sheet
type SheetValidationResult struct {
	SheetName string          `json:"sheet_name"`
	IsValid   bool            `json:"is_valid"`
	Preview   *SheetPreview   `json:"preview,omitempty"`
	Error     *pkg.BatchError `json:"error,omitempty"`
}

// SheetPreview contains preview data for a valid sheet
type SheetPreview struct {
	InventoryName string `json:"inventory_name"`
	OrderDate     string `json:"order_date"` // Format: YYYY-MM-DD
	SupplierName  string `json:"supplier_name"`
	ItemsCount    int    `json:"items_count"`
}

// ProcessImportRequest represents the request to process an uploaded file
type ProcessImportRequest struct {
	SheetName string `json:"sheet_name" validate:"required"`
}
