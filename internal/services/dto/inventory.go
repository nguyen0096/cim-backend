package dto

// ReconcileInventoryRequest represents the request for confirming inventory
type ReconcileInventoryRequest struct {
	InventoryID uint            `json:"inventory_id" validate:"required" param:"id"`
	Items       []ReconcileItem `json:"items" validate:"required,min=1,dive"`
}

// InventoryItemQuantity represents a single inventory item disposal
type ReconcileItem struct {
	InventoryItemID uint `json:"inventory_item_id" validate:"required"`
	ActualQuantity  int  `json:"actual_quantity" validate:"required,min=1"`
	PrevQuantity    int  `json:"prev_quantity" validate:"required,min=1"`
}

// DisposeInventoryRequest represents the request for disposing inventory items
type DisposeInventoryRequest struct {
	InventoryID uint          `json:"inventory_id" validate:"required" param:"id"`
	Items       []DisposeItem `json:"items" validate:"required,min=1,dive"`
}

// InventoryItemQuantity represents a single inventory item disposal
type DisposeItem struct {
	InventoryItemID uint `json:"inventory_item_id" validate:"required"`
	Quantity        int  `json:"quantity" validate:"required,min=1"`
	PrevQuantity    int  `json:"prev_quantity" validate:"required,min=1"`
}

// LastPurchasePriceResponse represents the last purchase price for a product-supplier combination
type LastPurchasePriceResponse struct {
	ProductID        uint    `json:"product_id"`
	SupplierID       uint    `json:"supplier_id"`
	LastPrice        float64 `json:"last_price"`
	LastPurchaseDate string  `json:"last_purchase_date"`
}

// LastPurchasePriceMap represents nested map of product_id -> supplier_id -> last_price
type LastPurchasePriceMap map[uint]map[uint]float64
