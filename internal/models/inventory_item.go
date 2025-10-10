package models

import "time"

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
