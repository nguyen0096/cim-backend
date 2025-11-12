package models

import (
	"cim-backend/pkg"
	"time"
)

type PurchaseOrderStatus string

const (
	PurchaseOrderStatusOrderPlaced        PurchaseOrderStatus = "order_placed"
	PurchaseOrderStatusPartiallyDelivered PurchaseOrderStatus = "partially_delivered"
	PurchaseOrderStatusFullyDelivered     PurchaseOrderStatus = "fully_delivered"
	PurchaseOrderStatusCompleted          PurchaseOrderStatus = "completed"
	PurchaseOrderStatusCancelled          PurchaseOrderStatus = "cancelled"
)

var AllPurchaseOrderStatuses = []PurchaseOrderStatus{
	PurchaseOrderStatusOrderPlaced,
	PurchaseOrderStatusPartiallyDelivered,
	PurchaseOrderStatusFullyDelivered,
	PurchaseOrderStatusCompleted,
	PurchaseOrderStatusCancelled,
}

// PurchaseOrder represents a purchase order
// @Description Purchase order entity with items and status tracking
type PurchaseOrder struct {
	Base
	OrderNumber       string               `json:"order_number" gorm:"unique;not null" example:"PO-2023-001"`
	InventoryID       *uint                `json:"inventory_id" gorm:"not null" validate:"required"`
	Inventory         *Inventory           `json:"inventory,omitempty" gorm:"foreignKey:InventoryID" validate:"-"`
	Status            PurchaseOrderStatus  `json:"status" gorm:"default:order_placed;check:status IN ('order_placed', 'partially_delivered', 'fully_delivered', 'completed', 'cancelled')" example:"order_placed"`
	TotalAmount       float64              `json:"total_amount" gorm:"-" example:"999.99"` // Calculated field, not stored in DB
	Notes             string               `json:"notes" example:"Purchase order notes"`
	ConfirmedAt       *time.Time           `json:"confirmed_at,omitempty" gorm:"column:confirmed_at" example:"2023-01-01T12:00:00Z"`
	ConfirmationNotes string               `json:"confirmation_notes,omitempty" gorm:"column:confirmation_notes" example:"Purchase order confirmed with supplier"`
	Items             []*PurchaseOrderItem `json:"items" gorm:"foreignKey:PurchaseOrderID" validate:"required,min=1,dive"`
}

// CalculateTotalAmount calculates the total amount of a purchase order based on its items
func (po *PurchaseOrder) CalculateTotalAmount() float64 {
	var total float64
	for _, item := range po.Items {
		if item != nil {
			item.TotalAmount = item.CalculateTotalAmount()
			total += item.TotalAmount
		}
	}
	po.TotalAmount = total
	return total
}

// UpdateStatus iterates over the items and updates the status of the purchase order.
// If all items are delivered, the status of the purchase order is updated to fully delivered.
// If some items are delivered, the status of the purchase order is updated to partially delivered.
// If no items are delivered, the status of the purchase order is updated to order placed.
func (po *PurchaseOrder) UpdateStatus() error {
	if len(po.Items) == 0 {
		return pkg.ErrPurchaseOrderNoItems()
	}

	var hasAwaitingDeliveryItem = false
	var hasPartiallyDelieveredItem = false
	var hasDeliveredItem = false
	for _, item := range po.Items {
		if item != nil {
			switch item.Status {
			case PurchaseOrderItemStatusAwaitingDelivery:
				hasAwaitingDeliveryItem = true
			case PurchaseOrderItemStatusPartiallyDelivered:
				hasPartiallyDelieveredItem = true
			case PurchaseOrderItemStatusDelivered:
				hasDeliveredItem = true
			}
		}
	}

	// order_placed -> partially_delivered
	if po.Status == PurchaseOrderStatusOrderPlaced &&
		(hasPartiallyDelieveredItem || hasDeliveredItem) {
		po.Status = PurchaseOrderStatusPartiallyDelivered
	}

	// fully_delivered -> partially_delivered (when quantity increases and received < ordered)
	if po.Status == PurchaseOrderStatusFullyDelivered &&
		(hasAwaitingDeliveryItem || hasPartiallyDelieveredItem) {
		po.Status = PurchaseOrderStatusPartiallyDelivered
	}

	// partially_delivered -> fully_delivered
	if po.Status == PurchaseOrderStatusPartiallyDelivered &&
		!hasAwaitingDeliveryItem && !hasPartiallyDelieveredItem {
		po.Status = PurchaseOrderStatusFullyDelivered
	}

	return nil
}
