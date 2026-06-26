package models

import "github.com/gabriel-vasile/mimetype"

// FileType constants for different Excel file types
const (
	FileTypeRevenueExpense   = "revenue_expense"
	FileTypeInventoryTracker = "inventory_tracker"
)

type ExportFile struct {
	Content     []byte        `json:"-"`
	FileType    mimetype.MIME `json:"file_type" swaggertype:"string"`
	DownloadURL string        `json:"download_url"`
}
