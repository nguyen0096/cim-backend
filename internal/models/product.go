package models

// Product represents a product
type Product struct {
	Base
	Name           string           `json:"name" gorm:"not null"`
	Description    string           `json:"description"`
	ProductType    string           `json:"product_type" gorm:"type:varchar(20)"`
	UnitID         uint             `json:"unit_id" gorm:"not null"`
	Unit           *Unit            `json:"unit,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Status         string           `json:"status" gorm:"default:active;check:status IN ('active', 'inactive')"`
	ProductImage   []byte           `json:"product_image" gorm:"type:bytea"`
	Suppliers      []*Supplier      `json:"suppliers,omitempty" gorm:"many2many:product_suppliers;" validate:"-"`
	InventoryItems []*InventoryItem `json:"inventory_items,omitempty" gorm:"foreignKey:ProductID" validate:"-"`
}
