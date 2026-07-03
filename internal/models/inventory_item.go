package models

import (
	"cim-backend/pkg"
	"fmt"

	"github.com/shopspring/decimal"
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
	Quantity               decimal.Decimal         `json:"quantity" gorm:"type:decimal(10,2);default:0"`
	UnitID                 uint                    `json:"unit_id" gorm:"not null"`
	Unit                   *Unit                   `json:"unit,omitempty" gorm:"foreignKey:UnitID"`
	Status                 InventoryItemStatus     `json:"status" gorm:"default:active"`
	ConsumingTransactionID uint                    `json:"consuming_transaction_id" validate:"-"`
	ConsumableTransactions []*InventoryTransaction `json:"active_purchase_transactions,omitempty"`
}

// ValidateActivePurchaseTransactions checks the consumable transaction quantities
// sum to the inventory item quantity.
func (ii *InventoryItem) ValidateActivePurchaseTransactions() error {
	if ii.Quantity.IsZero() && len(ii.ConsumableTransactions) == 0 {
		return nil
	}

	if !ii.Quantity.IsZero() && len(ii.ConsumableTransactions) == 0 ||
		ii.Quantity.IsZero() && len(ii.ConsumableTransactions) != 0 {
		return pkg.NewAppError(pkg.ErrorCodeValidation, fmt.Sprintf("inventory item %d has invalid quantity and active purchase transactions", ii.ID), nil)
	}

	totalTransactionQuantity := decimal.Zero
	for _, transaction := range ii.ConsumableTransactions {
		totalTransactionQuantity = totalTransactionQuantity.Add(transaction.Quantity.Sub(transaction.ConsumedQuantity))
	}

	if !totalTransactionQuantity.Equal(ii.Quantity) {
		return pkg.ErrBadInventoryItemState(
			fmt.Sprintf("total transaction quantity does not match inventory item quantity for inventory item %d. Item quantity: %s, Total transaction quantity: %s",
				ii.ID, ii.Quantity.String(), totalTransactionQuantity.String()), nil)
	}

	return nil
}

func (i *InventoryItem) IsNew() bool {
	return i.Base.ID == 0
}

// InventoryItemChange represents a change to an inventory item.
type InventoryItemChange struct {
	*InventoryItem
	OriginalQuantity decimal.Decimal
}
