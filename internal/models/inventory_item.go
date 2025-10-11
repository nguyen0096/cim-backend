package models

import (
	"fmt"
	"import-export-backend/pkg"
	"time"
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
	InventoryID                uint                    `json:"inventory_id" gorm:"index:idx_inventory_items_unique,unique;not null"`
	Inventory                  *Inventory              `json:"inventory,omitempty" gorm:"foreignKey:InventoryID" validate:"-"`
	ProductID                  uint                    `json:"product_id" gorm:"index:idx_inventory_items_unique,unique;not null"`
	Product                    *Product                `json:"product,omitempty" gorm:"foreignKey:ProductID" validate:"-"`
	SupplierID                 uint                    `json:"supplier_id"`
	Supplier                   *Supplier               `json:"supplier,omitempty" gorm:"foreignKey:SupplierID" validate:"-"`
	UnitType                   string                  `json:"unit_type" gorm:"type:varchar(20)"`
	Quantity                   int                     `json:"quantity" gorm:"default:0"`
	Status                     InventoryItemStatus     `json:"status" gorm:"default:active"`
	LatestActivePurchaseAt     time.Time               `json:"latest_active_purchase_at" validate:"-"`
	ActivePurchaseTransactions []*InventoryTransaction `json:"active_purchase_transactions,omitempty"`
}

// ValidateActivePurchaseTransactions validates if transaction quantities are reflected
// in the inventory item quantity correctly. Since inventory items are consumed in FIFO order,
// it's expected that LatestActivePurchaseAt is the oldest transaction's created_at time that
// still has un-consumed quantity.
func (ii *InventoryItem) ValidateActivePurchaseTransactions() error {
	if len(ii.ActivePurchaseTransactions) == 0 {
		return pkg.NewAppError(pkg.ErrorCodeValidation, fmt.Sprintf("no active purchase transactions found for inventory item %d", ii.ID), nil)
	}

	// Calculate total quantity from all transactions
	// For the oldest transaction, we need to subtract the consumed quantity
	totalTransactionQuantity := ii.ActivePurchaseTransactions[0].Quantity - ii.ActivePurchaseTransactions[0].ConsumedQuantity
	for _, transaction := range ii.ActivePurchaseTransactions[1:] {
		totalTransactionQuantity += transaction.Quantity
	}

	if totalTransactionQuantity != ii.Quantity {
		return pkg.ErrBadInventoryItemState(
			fmt.Sprintf("total transaction quantity does not match inventory item quantity for inventory item %d. Item quantity: %d, Total transaction quantity: %d",
				ii.ID, ii.Quantity, totalTransactionQuantity), nil)
	}

	return nil
}
