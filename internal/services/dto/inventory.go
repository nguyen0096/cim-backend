package dto

// ReconcileInventoryRequest represents the request for confirming inventory
type ReconcileInventoryRequest struct {
	InventoryID uint                    `json:"inventory_id" validate:"required" param:"id"`
	Items       []InventoryItemQuantity `json:"items" validate:"required,min=1,dive"`
}

// DisposeItemsRequest represents the request for disposing inventory items
type DisposeItemsRequest struct {
	InventoryID uint                    `json:"inventory_id" validate:"required" param:"id"`
	Items       []InventoryItemQuantity `json:"items" validate:"required,min=1,dive"`
}

// InventoryItemQuantity represents a single inventory item disposal
type InventoryItemQuantity struct {
	InventoryItemID uint `json:"inventory_item_id" validate:"required"`
	Quantity        int  `json:"quantity" validate:"required,min=1"`
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
