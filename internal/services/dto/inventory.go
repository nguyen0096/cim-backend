package dto

import "cim-backend/internal/models"

var (
	_ models.IDGetter = (*QuantityItem)(nil)
)

// QuantityItem represents a single inventory item quantity
type QuantityItem struct {
	InventoryItemID uint                  `json:"inventory_item_id" validate:"required"`
	InventoryItem   *models.InventoryItem `json:"inventory_item,omitempty" validate:"-"`
	ProductName     string                `json:"product_name,omitempty" validate:"-"`
	Quantity        *int                  `json:"quantity" validate:"required,min=0"`
	PrevQuantity    int                   `json:"prev_quantity,omitempty" validate:"required,min=1"`
}

func (d QuantityItem) GetID() uint {
	return d.InventoryItemID
}

// ReconcileInventoryRequest represents the request for confirming inventory
type ReconcileInventoryRequest struct {
	InventoryID uint           `json:"inventory_id" validate:"required" param:"id"`
	Items       []QuantityItem `json:"items" validate:"required,min=1,dive"`
}

func (d ReconcileInventoryRequest) GetItemIDs() []uint {
	itemIDs := make([]uint, len(d.Items))
	for i, item := range d.Items {
		itemIDs[i] = item.InventoryItemID
	}
	return itemIDs
}

// DisposeInventoryRequest represents the request for disposing inventory items
type DisposeInventoryRequest struct {
	InventoryID uint           `json:"inventory_id" validate:"required" param:"id"`
	Items       []QuantityItem `json:"items" validate:"required,min=1,dive"`
}

func (d DisposeInventoryRequest) GetItemIDs() []uint {
	itemIDs := make([]uint, len(d.Items))
	for i, item := range d.Items {
		itemIDs[i] = item.InventoryItemID
	}
	return itemIDs
}

// TransferInventoryRequest represents the request for transferring inventory items between inventories
type TransferInventoryRequest struct {
	SourceInventoryID      uint           `json:"source_inventory_id" validate:"required"`
	DestinationInventoryID uint           `json:"destination_inventory_id" validate:"required"`
	Items                  []QuantityItem `json:"items" validate:"required,min=1,dive"`
}

func (d TransferInventoryRequest) GetItemIDs() []uint {
	itemIDs := make([]uint, len(d.Items))
	for i, item := range d.Items {
		itemIDs[i] = item.InventoryItemID
	}
	return itemIDs
}
