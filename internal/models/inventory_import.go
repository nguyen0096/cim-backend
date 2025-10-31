package models

// InventoryImportDTO represents a single product/PO row from the inventory spreadsheet
// Each row contains product information and daily quantities for the month
type InventoryImportDTO struct {
	ProductName     string          `json:"product_name"`
	Unit            string          `json:"unit"`
	Price           float64         `json:"price"`
	InitialQuantity float64         `json:"initial_quantity"`
	QuantityByDay   map[int]float64 `json:"quantity_by_day"` // map[day]quantity, day is 1-31
}

// InventoryImportSheetDTO represents all data from an inventory sheet
type InventoryImportSheetDTO struct {
	Items []InventoryImportDTO `json:"items"`
}
