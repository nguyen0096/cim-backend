package models

import (
	"github.com/google/uuid"
)

// PurchaseOrder represents a purchase order
type PurchaseOrder struct {
	Base
	OrderNumber string              `json:"order_number" gorm:"unique;not null"`
	Status      string              `json:"status" gorm:"default:pending;check:status IN ('pending', 'approved', 'received', 'cancelled')"`
	TotalAmount float64             `json:"total_amount" gorm:"type:decimal(10,2)"`
	Notes       string              `json:"notes"`
	Items       []PurchaseOrderItem `json:"items" gorm:"foreignKey:PurchaseOrderID"`
}

// PurchaseOrderItem represents an item in a purchase order
type PurchaseOrderItem struct {
	Base
	PurchaseOrderID  uuid.UUID     `json:"purchase_order_id"`
	PurchaseOrder    PurchaseOrder `json:"purchase_order" gorm:"foreignKey:PurchaseOrderID"`
	ProductID        uuid.UUID     `json:"product_id"`
	Product          Product       `json:"product" gorm:"foreignKey:ProductID"`
	Quantity         int           `json:"quantity" gorm:"not null"`
	UnitPrice        float64       `json:"unit_price" gorm:"type:decimal(10,2)"`
	TotalPrice       float64       `json:"total_price" gorm:"type:decimal(10,2)"`
	ReceivedQuantity int           `json:"received_quantity" gorm:"default:0"`
}
