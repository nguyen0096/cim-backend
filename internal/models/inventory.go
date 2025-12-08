package models

import "github.com/shopspring/decimal"

// InventoryStatus represents the status of inventory
type InventoryStatus string

const (
	InventoryStatusActive   InventoryStatus = "active"
	InventoryStatusInactive InventoryStatus = "inactive"
)

// Inventory represents a warehouse or storage location
type Inventory struct {
	Base
	Name                    string           `json:"name" gorm:"not null;unique"`
	Description             string           `json:"description"`
	Location                string           `json:"location" gorm:"not null"`
	Status                  InventoryStatus  `json:"status" gorm:"default:active"`
	RestrictedDeliveryHours string           `json:"restricted_delivery_hours"`
	Items                   []*InventoryItem `json:"items,omitempty" gorm:"foreignKey:InventoryID" validate:"-"`
}

// GetTotalItems returns the total number of items in this inventory
func (i *Inventory) GetTotalItems() int {
	return len(i.Items)
}

type InventoryItemTransactionReport struct {
	Report
	*InventoryItem

	StartQuantity decimal.Decimal         `json:"start_quantity"`
	Transactions  []*InventoryTransaction `json:"transactions,omitempty"`

	// computed fields

	PurchaseQuantity decimal.Decimal         `json:"total_purchase"`
	SoldQuantity     decimal.Decimal         `json:"total_sold"`
	TransferQuantity decimal.Decimal         `json:"total_transfer"`
	EndQuantity      decimal.Decimal         `json:"end_quantity"`
	ChangeByDay      map[int]decimal.Decimal `json:"change_by_day"`
}

type InventoryTransactionReport struct {
	Report
	*Inventory

	Items []*InventoryItemTransactionReport `json:"items,omitempty"`
}
