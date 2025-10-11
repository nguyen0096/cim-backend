package models

import "import-export-backend/pkg"

type PurchaseOrderStatus string

const (
	PurchaseOrderStatusOrderPlaced        PurchaseOrderStatus = "order_placed"
	PurchaseOrderStatusPartiallyDelivered PurchaseOrderStatus = "partially_delivered"
	PurchaseOrderStatusFullyDelivered     PurchaseOrderStatus = "fully_delivered"
	PurchaseOrderStatusCompleted          PurchaseOrderStatus = "completed"
	PurchaseOrderStatusCancelled          PurchaseOrderStatus = "cancelled"
)

// PurchaseOrder represents a purchase order
// @Description Purchase order entity with items and status tracking
type PurchaseOrder struct {
	Base
	OrderNumber string               `json:"order_number" gorm:"unique;not null" example:"PO-2023-001"`
	Status      PurchaseOrderStatus  `json:"status" gorm:"default:order_placed;check:status IN ('order_placed', 'partially_delivered', 'fully_delivered', 'completed', 'cancelled')" example:"order_placed"`
	TotalAmount float64              `json:"total_amount" gorm:"-" example:"999.99"` // Calculated field, not stored in DB
	Notes       string               `json:"notes" example:"Purchase order notes"`
	Items       []*PurchaseOrderItem `json:"items" gorm:"foreignKey:PurchaseOrderID" validate:"required,min=1,dive"`
}

// CalculateTotalAmount calculates the total amount of a purchase order based on its items
func (po *PurchaseOrder) CalculateTotalAmount() float64 {
	var total float64
	for _, item := range po.Items {
		if item != nil {
			item.TotalPrice = item.CalculateTotalPrice()
			total += item.TotalPrice
		}
	}
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

	var hasUndeliveredItem = false
	var hasDeliveredItem = false
	for _, item := range po.Items {
		if item != nil {
			if item.Status == PurchaseOrderItemStatusDelivered {
				hasDeliveredItem = true
			} else {
				hasUndeliveredItem = true
			}
		}
	}

	if hasUndeliveredItem && hasDeliveredItem {
		po.Status = PurchaseOrderStatusPartiallyDelivered
	} else if hasDeliveredItem {
		po.Status = PurchaseOrderStatusFullyDelivered
	} else {
		po.Status = PurchaseOrderStatusOrderPlaced
	}

	return nil
}
