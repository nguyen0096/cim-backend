package models

// Product represents a product
type Product struct {
	Base
	Name           string           `json:"name" gorm:"not null"`
	Description    string           `json:"description"`
	ProductType    string           `json:"product_type" gorm:"type:varchar(20)"`
	Status         string           `json:"status" gorm:"default:active;check:status IN ('active', 'inactive')"`
	Suppliers      []*Supplier      `json:"suppliers,omitempty" gorm:"many2many:product_suppliers;" validate:"-"`
	InventoryItems []*InventoryItem `json:"inventory_items,omitempty" gorm:"foreignKey:ProductID" validate:"-"`
}
