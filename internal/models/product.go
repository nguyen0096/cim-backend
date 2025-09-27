package models

import (
	"github.com/google/uuid"
)

// Product represents a product
type Product struct {
	Base
	Name        string     `json:"name" gorm:"not null"`
	Description string     `json:"description"`
	SKU         string     `json:"sku" gorm:"unique"`
	SupplierID  uuid.UUID  `json:"supplier_id"`
	Supplier    *Supplier  `json:"supplier,omitempty" gorm:"foreignKey:SupplierID" validate:"-"`
	UnitPrice   float64    `json:"unit_price" gorm:"type:decimal(13,2)"`
	UnitType    string     `json:"unit_type" gorm:"type:varchar(32)"`
	Status      string     `json:"status" gorm:"default:active;check:status IN ('active', 'inactive', 'discontinued')"`
	Inventory   *Inventory `json:"inventory,omitempty" gorm:"foreignKey:ProductID"`
}
