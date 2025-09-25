package models

import (
	"time"

	"github.com/google/uuid"
)

// PurchaseOrder represents a purchase order
type PurchaseOrder struct {
	ID          uuid.UUID           `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrderNumber string              `json:"order_number" gorm:"unique;not null"`
	SupplierID  uuid.UUID           `json:"supplier_id"`
	Supplier    Supplier            `json:"supplier" gorm:"foreignKey:SupplierID"`
	Status      string              `json:"status" gorm:"default:pending;check:status IN ('pending', 'approved', 'received', 'cancelled')"`
	TotalAmount float64             `json:"total_amount" gorm:"type:decimal(10,2)"`
	Notes       string              `json:"notes"`
	CreatedBy   string              `json:"created_by"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	Items       []PurchaseOrderItem `json:"items" gorm:"foreignKey:PurchaseOrderID"`
}

// PurchaseOrderItem represents an item in a purchase order
type PurchaseOrderItem struct {
	ID               uuid.UUID     `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	PurchaseOrderID  uuid.UUID     `json:"purchase_order_id"`
	PurchaseOrder    PurchaseOrder `json:"purchase_order" gorm:"foreignKey:PurchaseOrderID"`
	ProductID        uuid.UUID     `json:"product_id"`
	Product          Product       `json:"product" gorm:"foreignKey:ProductID"`
	Quantity         int           `json:"quantity" gorm:"not null"`
	UnitPrice        float64       `json:"unit_price" gorm:"type:decimal(10,2)"`
	TotalPrice       float64       `json:"total_price" gorm:"type:decimal(10,2)"`
	ReceivedQuantity int           `json:"received_quantity" gorm:"default:0"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}
