package models

// FileType constants for different Excel file types
const (
	FileTypeRevenueExpense   = "revenue_expense"
	FileTypeInventoryTracker = "inventory_tracker"
)

type ExportFile struct {
	Content     []byte `json:"-"`
	DownloadURL string `json:"download_url"`
}
