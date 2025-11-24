package services

// Excel cell positions for purchase order import
const (
	// Cell positions
	POImportInventoryNameCell = "A1"
	POImportDateCell          = "E1"
	POImportSupplierNameCell  = "A3"
	POImportHeaderRow         = 4
	POImportDataStartRow      = 5

	// Date format
	POImportDatePrefix = "NGÀY:"
	POImportDateFormat = "02/01/2006" // DD/MM/YYYY

	// End marker
	POImportEndMarker = "TỔNG CỘNG"
)

// Column indices (0-based) for purchase order import
const (
	POImportColNumber      = 0 // STT
	POImportColProductName = 1 // DIỄN GIẢI
	POImportColUnit        = 2 // ĐVT
	POImportColQuantity    = 3 // SỐ LƯỢNG
	POImportColUnitPrice   = 4 // ĐƠN GIÁ
	POImportColTotal       = 5 // TT (ignored)
)

// Header mappings for validation (Vietnamese to field names)
var POImportHeaderMap = map[int]string{
	POImportColNumber:      "STT",
	POImportColProductName: "DIỄN GIẢI",
	POImportColUnit:        "ĐVT",
	POImportColQuantity:    "SỐ LƯỢNG",
	POImportColUnitPrice:   "ĐƠN GIÁ",
	POImportColTotal:       "TT",
}

// Expected header names for validation
var POImportExpectedHeaders = []string{
	"STT",
	"DIỄN GIẢI",
	"ĐVT",
	"SỐ LƯỢNG",
	"ĐƠN GIÁ",
	"TT",
}
