package models

import (
	"cim-backend/pkg"
	"fmt"
)

// InventoryItemStatus represents the status of an inventory item
type InventoryItemStatus string

const (
	InventoryItemStatusActive   InventoryItemStatus = "active"
	InventoryItemStatusInactive InventoryItemStatus = "inactive"
)

// InventoryItem represents an item in an inventory
type InventoryItem struct {
	Base
	InventoryID            uint                    `json:"inventory_id" gorm:"index:idx_inventory_items_unique,unique;not null"`
	Inventory              *Inventory              `json:"inventory,omitempty" gorm:"foreignKey:InventoryID" validate:"-"`
	ProductID              uint                    `json:"product_id" gorm:"index:idx_inventory_items_unique,unique;not null"`
	Product                *Product                `json:"product,omitempty" gorm:"foreignKey:ProductID" validate:"-"`
	Quantity               int                     `json:"quantity" gorm:"default:0"`
	Status                 InventoryItemStatus     `json:"status" gorm:"default:active"`
	ConsumingTransactionID uint                    `json:"consuming_transaction_id" validate:"-"`
	ConsumableTransactions []*InventoryTransaction `json:"active_purchase_transactions,omitempty"`
}

// ValidateActivePurchaseTransactions validates if transaction quantities are reflected
// in the inventory item quantity correctly. Since inventory items are consumed in FIFO order,
// it's expected that ConsumingTransactionID points to the oldest transaction ID that
// is currently being consumed (still has un-consumed quantity).
func (ii *InventoryItem) ValidateActivePurchaseTransactions() error {
	if ii.Quantity == 0 && len(ii.ConsumableTransactions) == 0 {
		return nil
	}

	if ii.Quantity != 0 && len(ii.ConsumableTransactions) == 0 ||
		ii.Quantity == 0 && len(ii.ConsumableTransactions) != 0 {
		return pkg.NewAppError(pkg.ErrorCodeValidation, fmt.Sprintf("inventory item %d has invalid quantity and active purchase transactions", ii.ID), nil)
	}

	var totalTransactionQuantity int
	for _, transaction := range ii.ConsumableTransactions {
		totalTransactionQuantity += transaction.Quantity - transaction.ConsumedQuantity
	}

	if totalTransactionQuantity != ii.Quantity {
		return pkg.ErrBadInventoryItemState(
			fmt.Sprintf("total transaction quantity does not match inventory item quantity for inventory item %d. Item quantity: %d, Total transaction quantity: %d",
				ii.ID, ii.Quantity, totalTransactionQuantity), nil)
	}

	return nil
}

func (i *InventoryItem) IsNew() bool {
	return i.Base.ID == 0
}

func (i *InventoryItem) GetID() uint {
	return i.Base.ID
}

// InventoryItemChange represents a change to an inventory item.
type InventoryItemChange struct {
	*InventoryItem
	OriginalQuantity int
}

func (i *InventoryItemChange) GetID() uint {
	return i.ID
}
