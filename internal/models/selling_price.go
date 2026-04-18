package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// SellingPrice represents a selling price entry in the price ledger for a product.
// If InventoryID is nil, it's a global price. If set, it's inventory-specific.
type SellingPrice struct {
	Base
	ProductID     uint            `json:"product_id" gorm:"not null"`
	Product       *Product        `json:"product,omitempty" gorm:"foreignKey:ProductID" validate:"-"`
	InventoryID   *uint           `json:"inventory_id"`
	Inventory     *Inventory      `json:"inventory,omitempty" gorm:"foreignKey:InventoryID" validate:"-"`
	Price         decimal.Decimal `json:"price" gorm:"type:numeric(13,2);not null"`
	EffectiveFrom time.Time       `json:"effective_from" gorm:"type:date;not null"`
	Notes         string          `json:"notes"`
}
