package models

import "github.com/shopspring/decimal"

// SaleOrderItem represents an item in a sale order
type SaleOrderItem struct {
	Base
	SaleOrderID *uint       `json:"sale_order_id" gorm:"not null"`
	SaleOrder   *SaleOrder  `json:"sale_order,omitempty" gorm:"foreignKey:SaleOrderID"`
	MenuItems   []*MenuItem `json:"menu_items,omitempty" gorm:"many2many:sale_order_item_menu_items;" validate:"-"`

	// Display fields, not stored in DB
	TotalPrice decimal.Decimal `json:"total_price" gorm:"-"`
}

// CalculateTotalPrice calculates the total price for a sale order item
// Note: This is a placeholder implementation. The actual calculation logic
// should be implemented based on business requirements (e.g., menu item prices)
func (soi *SaleOrderItem) CalculateTotalPrice() decimal.Decimal {
	// TODO: Implement actual price calculation based on menu items
	// For now, return zero as placeholder
	soi.TotalPrice = decimal.Zero
	return decimal.Zero
}
