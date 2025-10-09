package models

// InventoryItemStatus represents the status of an inventory item
type InventoryItemStatus string

const (
	InventoryItemStatusActive   InventoryItemStatus = "active"
	InventoryItemStatusInactive InventoryItemStatus = "inactive"
)

// InventoryItem represents an item in an inventory
type InventoryItem struct {
	Base
	InventoryID uint                `json:"inventory_id" gorm:"index:idx_inventory_items_unique,unique;not null"`
	Inventory   *Inventory          `json:"inventory,omitempty" gorm:"foreignKey:InventoryID" validate:"-"`
	ProductID   uint                `json:"product_id" gorm:"index:idx_inventory_items_unique,unique;not null"`
	Product     *Product            `json:"product,omitempty" gorm:"foreignKey:ProductID" validate:"-"`
	SupplierID  uint                `json:"supplier_id"`
	Supplier    *Supplier           `json:"supplier,omitempty" gorm:"foreignKey:SupplierID" validate:"-"`
	UnitPrice   float64             `json:"unit_price" gorm:"type:decimal(13,2)"`
	UnitType    string              `json:"unit_type" gorm:"type:varchar(20)"`
	Quantity    int                 `json:"quantity" gorm:"default:0"`
	Status      InventoryItemStatus `json:"status" gorm:"default:active"`
}

// CalculateTotalValue calculates the total value of this inventory item
func (ii *InventoryItem) CalculateTotalValue() float64 {
	return float64(ii.Quantity) * ii.UnitPrice
}
