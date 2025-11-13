package dto

import (
	"cim-backend/internal/models"

	"github.com/shopspring/decimal"
)

var (
	_ models.IDGetter = (*QuantityItem)(nil)
)

// QuantityItem represents a single inventory item quantity
type QuantityItem struct {
	InventoryItemID uint             `json:"inventory_item_id" validate:"required"`
	Quantity        *decimal.Decimal `json:"quantity" validate:"required"`
	PrevQuantity    decimal.Decimal  `json:"prev_quantity,omitempty" validate:"required"`

	// Response fields

	InventoryItem   *models.InventoryItem `json:"inventory_item,omitempty" validate:"-"`
	ProductName     string                `json:"product_name,omitempty" validate:"-"`
	CurrentQuantity decimal.Decimal       `json:"current_quantity,omitempty" validate:"-"`
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
