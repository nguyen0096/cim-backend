package pkg

import "fmt"

// FileCategory represents different types of uploaded files
type FileCategory string

const (
	// FileCategoryPurchaseOrder represents purchase order import files
	FileCategoryPurchaseOrder FileCategory = "purchase_order"
	// Future categories can be added here:
	// FileCategoryProduct FileCategory = "product"
	// FileCategoryInvoice FileCategory = "invoice"
)

// FileCategorySubdirs maps file categories to their subdirectory names
var FileCategorySubdirs = map[FileCategory]string{
	FileCategoryPurchaseOrder: "purchase-orders",
	// Future mappings...
}

// GetSubdirectory returns the subdirectory for a file category
func (fc FileCategory) GetSubdirectory() (string, error) {
	subdir, ok := FileCategorySubdirs[fc]
	if !ok {
		return "", fmt.Errorf("unknown file category: %s", fc)
	}
	return subdir, nil
}

