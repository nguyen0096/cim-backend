package models

import "github.com/shopspring/decimal"

type SaleOrderStatus string

const (
	SaleOrderStatusOrdered   SaleOrderStatus = "ordered"
	SaleOrderStatusServed    SaleOrderStatus = "served"
	SaleOrderStatusCompleted SaleOrderStatus = "completed"
	SaleOrderStatusCancelled SaleOrderStatus = "cancelled"
)

var AllSaleOrderStatuses = []SaleOrderStatus{
	SaleOrderStatusOrdered,
	SaleOrderStatusServed,
	SaleOrderStatusCompleted,
	SaleOrderStatusCancelled,
}

// SaleOrder represents a sale order
// @Description Sale order entity with items and status tracking
type SaleOrder struct {
	Base
	PreviousOrderID *uint            `json:"previous_order_id,omitempty" gorm:"column:previous_order_id"`
	PreviousOrder   *SaleOrder       `json:"previous_order,omitempty" gorm:"foreignKey:PreviousOrderID"`
	IsLatest        bool             `json:"is_latest" gorm:"column:is_latest;default:true"`
	CustomerID      string           `json:"customer_id" gorm:"column:customer_id;not null;size:26" validate:"required"`
	Tag             int              `json:"tag" gorm:"default:0"`
	OrderNumber     string           `json:"order_number" gorm:"not null" example:"SO-2023-001"`
	InventoryID     *uint            `json:"inventory_id" gorm:"not null" validate:"required"`
	Inventory       *Inventory       `json:"inventory,omitempty" gorm:"foreignKey:InventoryID" validate:"-"`
	Status          SaleOrderStatus  `json:"status" gorm:"default:ordered;check:status IN ('ordered', 'served', 'completed', 'cancelled')" example:"ordered"`
	Notes           string           `json:"notes" example:"Sale order notes"`
	Items           []*SaleOrderItem `json:"items" gorm:"foreignKey:SaleOrderID" validate:"required,min=1,dive"`

	// Display fields, not stored in DB
	TotalPrice decimal.Decimal `json:"total_price" gorm:"-" example:"999.99"`
}

// CalculateTotalPrice calculates the total price of a sale order based on its items
func (so *SaleOrder) CalculateTotalPrice() decimal.Decimal {
	total := decimal.Zero
	for _, item := range so.Items {
		if item != nil {
			item.TotalPrice = item.CalculateTotalPrice()
			total = total.Add(item.TotalPrice)
		}
	}
	so.TotalPrice = total
	return total
}
