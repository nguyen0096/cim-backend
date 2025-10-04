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
	InventoryID   uint                `json:"inventory_id" gorm:"not null"`
	Inventory     *Inventory          `json:"inventory,omitempty" gorm:"foreignKey:InventoryID" validate:"-"`
	ProductID     uint                `json:"product_id" gorm:"not null"`
	Product       *Product            `json:"product,omitempty" gorm:"foreignKey:ProductID" validate:"-"`
	Quantity      int                 `json:"quantity" gorm:"default:0"`
	ReorderLevel  int                 `json:"reorder_level" gorm:"default:0"`
	MaxStockLevel int                 `json:"max_stock_level" gorm:"default:0"`
	Status        InventoryItemStatus `json:"status" gorm:"default:active"`
	LastCounted   *time.Time          `json:"last_counted"`
	AverageCost   float64             `json:"average_cost" gorm:"type:decimal(13,2);default:0"`
	TotalValue    float64             `json:"total_value" gorm:"-"` // Calculated: Quantity * AverageCost
	Notes         string              `json:"notes"`
}

// CalculateTotalValue calculates the total inventory value
func (ii *InventoryItem) CalculateTotalValue() float64 {
	return float64(ii.Quantity) * ii.AverageCost
}

// IsLowStock checks if inventory item is below reorder level
func (ii *InventoryItem) IsLowStock() bool {
	return ii.Quantity <= ii.ReorderLevel
}

// IsOverstocked checks if inventory item exceeds max stock level
func (ii *InventoryItem) IsOverstocked() bool {
	return ii.MaxStockLevel > 0 && ii.Quantity > ii.MaxStockLevel
}
