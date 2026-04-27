package models

import "github.com/shopspring/decimal"

// POItemSellingPrice links a purchase order item to its selling price.
// SellingPrice (the override value) is NULL until the user explicitly overrides it.
// Resolution order: SellingPrice (if non-nil) → SellingPriceRef.Price (via SellingPriceID) → none.
type POItemSellingPrice struct {
	Base
	PurchaseOrderItemID uint             `json:"purchase_order_item_id" gorm:"uniqueIndex;not null"`
	PurchaseOrderItem   *PurchaseOrderItem `json:"purchase_order_item,omitempty" gorm:"foreignKey:PurchaseOrderItemID" validate:"-"`
	SellingPrice        *decimal.Decimal `json:"selling_price" gorm:"type:numeric(13,2)"`
	SellingPriceID      *uint            `json:"selling_price_id"`
	SellingPriceRef     *SellingPrice    `json:"selling_price_ref,omitempty" gorm:"foreignKey:SellingPriceID" validate:"-"`
}

// TableName overrides GORM's default (po_item_selling_prices) to match the
// migration-created table name purchase_order_item_selling_prices.
func (POItemSellingPrice) TableName() string {
	return "purchase_order_item_selling_prices"
}

// GetEffectiveSellingPrice returns the effective selling price.
// Returns SellingPrice if non-nil (user override), else SellingPriceRef.Price if loaded, else nil.
func (p *POItemSellingPrice) GetEffectiveSellingPrice() *decimal.Decimal {
	if p == nil {
		return nil
	}
	if p.SellingPrice != nil {
		return p.SellingPrice
	}
	if p.SellingPriceRef != nil {
		return &p.SellingPriceRef.Price
	}
	return nil
}
