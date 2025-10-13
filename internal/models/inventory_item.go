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

// @todo: add unique key: inventory_id + product_id + supplier_id + deleted_at

// InventoryItem represents an item in an inventory
type InventoryItem struct {
	Base
	InventoryID                uint                    `json:"inventory_id" gorm:"index:idx_inventory_items_unique,unique;not null"`
	Inventory                  *Inventory              `json:"inventory,omitempty" gorm:"foreignKey:InventoryID" validate:"-"`
	ProductID                  uint                    `json:"product_id" gorm:"index:idx_inventory_items_unique,unique;not null"`
	Product                    *Product                `json:"product,omitempty" gorm:"foreignKey:ProductID" validate:"-"`
	SupplierID                 uint                    `json:"supplier_id"`
	Supplier                   *Supplier               `json:"supplier,omitempty" gorm:"foreignKey:SupplierID" validate:"-"`
	Unit                       string                  `json:"unit" gorm:"type:varchar(20)"`
	Quantity                   int                     `json:"quantity" gorm:"default:0"`
	Status                     InventoryItemStatus     `json:"status" gorm:"default:active"`
	ConsumingTransactionID     uint                    `json:"consuming_transaction_id" validate:"-"`
	ActivePurchaseTransactions []*InventoryTransaction `json:"active_purchase_transactions,omitempty"`
}

// ValidateActivePurchaseTransactions validates if transaction quantities are reflected
// in the inventory item quantity correctly. Since inventory items are consumed in FIFO order,
// it's expected that ConsumingTransactionID points to the oldest transaction ID that
// is currently being consumed (still has un-consumed quantity).
func (ii *InventoryItem) ValidateActivePurchaseTransactions() error {
	if len(ii.ActivePurchaseTransactions) == 0 {
		return pkg.NewAppError(pkg.ErrorCodeValidation, fmt.Sprintf("no active purchase transactions found for inventory item %d", ii.ID), nil)
	}

	if ii.ConsumingTransactionID != 0 &&
		ii.ActivePurchaseTransactions[0].ID != ii.ConsumingTransactionID {
		return pkg.ErrBadInventoryItemState(fmt.Sprintf("consuming transaction ID %d does not match the first active transaction ID %d", ii.ConsumingTransactionID, ii.ActivePurchaseTransactions[0].ID), nil)
	}

	var totalTransactionQuantity int
	for _, transaction := range ii.ActivePurchaseTransactions {
		totalTransactionQuantity += transaction.Quantity - transaction.ConsumedQuantity
	}

	if totalTransactionQuantity != ii.Quantity {
		return pkg.ErrBadInventoryItemState(
			fmt.Sprintf("total transaction quantity does not match inventory item quantity for inventory item %d. Item quantity: %d, Total transaction quantity: %d",
				ii.ID, ii.Quantity, totalTransactionQuantity), nil)
	}

	return nil
}
