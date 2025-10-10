package dto

// ConfirmInventoryRequest represents the request for confirming inventory
type ConfirmInventoryRequest struct {
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
