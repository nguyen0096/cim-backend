package dto

import "cim-backend/internal/models"

// ReconcileInventoryRequest represents the request for confirming inventory
type ReconcileInventoryRequest struct {
	InventoryID uint            `json:"inventory_id" validate:"required" param:"id"`
	Items       []ReconcileItem `json:"items" validate:"required,min=1,dive"`
}

func (d ReconcileInventoryRequest) GetItemIDs() []uint {
	itemIDs := make([]uint, len(d.Items))
	for i, item := range d.Items {
		itemIDs[i] = item.InventoryItemID
	}
	return itemIDs
}

// InventoryItemQuantity represents a single inventory item disposal
type ReconcileItem struct {
	InventoryItemID uint                  `json:"inventory_item_id" validate:"required"`
	InventoryItem   *models.InventoryItem `json:"inventory_item,omitempty" validate:"-"`
	ActualQuantity  *int                  `json:"actual_quantity" validate:"required,min=0"`
	PrevQuantity    int                   `json:"prev_quantity,omitempty" validate:"required,min=1"`
}

// DisposeInventoryRequest represents the request for disposing inventory items
type DisposeInventoryRequest struct {
	InventoryID uint          `json:"inventory_id" validate:"required" param:"id"`
	Items       []DisposeItem `json:"items" validate:"required,min=1,dive"`
}

func (d DisposeInventoryRequest) GetItemIDs() []uint {
	itemIDs := make([]uint, len(d.Items))
	for i, item := range d.Items {
		itemIDs[i] = item.InventoryItemID
	}
	return itemIDs
}

// InventoryItemQuantity represents a single inventory item disposal
type DisposeItem struct {
	InventoryItemID uint                  `json:"inventory_item_id" validate:"required"`
	InventoryItem   *models.InventoryItem `json:"inventory_item,omitempty" validate:"-"`
	Quantity        *int                  `json:"quantity" validate:"required,min=1"`
	PrevQuantity    int                   `json:"prev_quantity,omitempty" validate:"required,min=1"`
}

// TransferInventoryRequest represents the request for transferring inventory items between inventories
type TransferInventoryRequest struct {
	SourceInventoryID      uint           `json:"source_inventory_id" validate:"required"`
	DestinationInventoryID uint           `json:"destination_inventory_id" validate:"required"`
	Items                  []TransferItem `json:"items" validate:"required,min=1,dive"`
}

func (d TransferInventoryRequest) GetItemIDs() []uint {
	itemIDs := make([]uint, len(d.Items))
	for i, item := range d.Items {
		itemIDs[i] = item.InventoryItemID
	}
	return itemIDs
}

// TransferItem represents a single inventory item to transfer
type TransferItem struct {
	InventoryItemID uint                  `json:"inventory_item_id" validate:"required"`
	InventoryItem   *models.InventoryItem `json:"inventory_item,omitempty" validate:"-"`
	Quantity        int                   `json:"quantity" validate:"required,min=1"`
}
