package models

import (
	"cim-backend/pkg"
	"context"
	"time"

	"github.com/shopspring/decimal"
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
	Notes             string               `json:"notes" example:"Purchase order notes"`
	ConfirmedAt       *time.Time           `json:"confirmed_at,omitempty" gorm:"column:confirmed_at" example:"2023-01-01T12:00:00Z"`
	ConfirmationNotes string               `json:"confirmation_notes,omitempty" gorm:"column:confirmation_notes" example:"Purchase order confirmed with supplier"`
	Items             []*PurchaseOrderItem `json:"items" gorm:"foreignKey:PurchaseOrderID" validate:"required,min=1,dive"`

	// Display fields, not stored in DB

	TotalAmount decimal.Decimal `json:"total_amount" gorm:"-" example:"999.99"`
}

// CalculateTotalAmount calculates the total amount of a purchase order based on its items
func (po *PurchaseOrder) CalculateTotalAmount() decimal.Decimal {
	total := decimal.Zero
	for _, item := range po.Items {
		if item != nil {
			item.TotalAmount = item.CalculateTotalAmount()
			total = total.Add(item.TotalAmount)
		}
	}
	po.TotalAmount = total
	return total
}

// UpdateStatus iterates over the items and updates the status of the purchase order.
// If all items are delivered, the status of the purchase order is updated to fully delivered.
// If some items are delivered, the status of the purchase order is updated to partially delivered.
// If no items are delivered, the status of the purchase order is updated to order placed.
func (po *PurchaseOrder) UpdateStatus(ctx context.Context) error {
	if len(po.Items) == 0 {
		return pkg.ErrPurchaseOrderNoItems(ctx)
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
	// Only transition to fully_delivered if there are no awaiting/partially delivered items AND there is at least one delivered item
	if po.Status == PurchaseOrderStatusPartiallyDelivered &&
		!hasAwaitingDeliveryItem && !hasPartiallyDelieveredItem && hasDeliveredItem {
		po.Status = PurchaseOrderStatusFullyDelivered
	}

	return nil
}
