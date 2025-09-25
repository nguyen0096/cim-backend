package models

import (
	"github.com/google/uuid"
)

type PurchaseOrderStatus string

const (
	PurchaseOrderStatusPending   PurchaseOrderStatus = "order_placed"
	PurchaseOrderStatusApproved  PurchaseOrderStatus = "partially_completed"
	PurchaseOrderStatusReceived  PurchaseOrderStatus = "completed"
	PurchaseOrderStatusCancelled PurchaseOrderStatus = "cancelled"
)

// PurchaseOrder represents a purchase order
type PurchaseOrder struct {
	Base
	OrderNumber string              `json:"order_number" gorm:"unique;not null"`
	Status      PurchaseOrderStatus `json:"status" gorm:"default:order_placed;check:status IN ('order_placed', 'partially_completed', 'completed', 'cancelled')"`
	TotalAmount float64             `json:"total_amount" gorm:"type:decimal(10,2)"`
	Notes       string              `json:"notes"`
	Items       []PurchaseOrderItem `json:"items" gorm:"foreignKey:PurchaseOrderID" validate:"required,min=1,dive"`
}

// PurchaseOrderItem represents an item in a purchase order
type PurchaseOrderItem struct {
	Base
	PurchaseOrderID  uuid.UUID      `json:"purchase_order_id"`
	PurchaseOrder    *PurchaseOrder `json:"purchase_order,omitempty" gorm:"foreignKey:PurchaseOrderID" validate:"-"`
	ProductID        *uuid.UUID     `json:"product_id" validate:"required"`
	Product          *Product       `json:"product,omitempty" gorm:"foreignKey:ProductID" validate:"-"`
	Quantity         int            `json:"quantity" gorm:"not null" validate:"required,min=1"`
	TotalPrice       float64        `json:"total_price" gorm:"type:decimal(10,2)"`
	ReceivedQuantity int            `json:"received_quantity" gorm:"default:0"`
}
