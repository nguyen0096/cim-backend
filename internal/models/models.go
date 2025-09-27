package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"import-export-backend/pkg"
)

type Base struct {
	ID        *uuid.UUID     `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	CreatedBy string         `json:"created_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (b *Base) BeforeCreate(tx *gorm.DB) error {
	// set CreatedBy to the user email
	if tx.Statement.Context == nil {
		return fmt.Errorf("context with user email is required to set CreatedBy")
	}

	userEmail, err := pkg.GetUserEmailFromContext(tx.Statement.Context)
	if err != nil {
		return err
	}
	b.CreatedBy = userEmail
	return nil
}

// Inventory represents inventory for a product
type Inventory struct {
	Base
	ProductID    *uuid.UUID `json:"product_id" gorm:"unique;not null"`
	Product      *Product   `json:"product,omitempty" gorm:"foreignKey:ProductID" validate:"-"`
	Quantity     int       `json:"quantity" gorm:"default:0"`
	ReorderLevel int       `json:"reorder_level" gorm:"default:0"`
	Location     string    `json:"location"`
}

// InventoryTransaction represents an inventory transaction
type InventoryTransaction struct {
	ID              uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ProductID       uuid.UUID `json:"product_id"`
	Product         Product   `json:"product" gorm:"foreignKey:ProductID"`
	TransactionType string    `json:"transaction_type" gorm:"not null;check:transaction_type IN ('purchase', 'sale', 'adjustment', 'return')"`
	Quantity        int       `json:"quantity" gorm:"not null"`
	ReferenceID     uuid.UUID `json:"reference_id"`
	ReferenceType   string    `json:"reference_type"`
	Notes           string    `json:"notes"`
	CreatedBy       string    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
}

// Order represents a customer order
type Order struct {
	ID            uuid.UUID   `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrderNumber   string      `json:"order_number" gorm:"unique;not null"`
	CustomerName  string      `json:"customer_name"`
	CustomerEmail string      `json:"customer_email"`
	Status        string      `json:"status" gorm:"default:pending;check:status IN ('pending', 'processing', 'completed', 'cancelled')"`
	TotalAmount   float64     `json:"total_amount" gorm:"type:decimal(10,2)"`
	Notes         string      `json:"notes"`
	CreatedBy     string      `json:"created_by"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
	Items         []OrderItem `json:"items" gorm:"foreignKey:OrderID"`
}

// OrderItem represents an item in an order
type OrderItem struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrderID    uuid.UUID `json:"order_id"`
	Order      Order     `json:"order" gorm:"foreignKey:OrderID"`
	ProductID  uuid.UUID `json:"product_id"`
	Product    Product   `json:"product" gorm:"foreignKey:ProductID"`
	Quantity   int       `json:"quantity" gorm:"not null"`
	UnitPrice  float64   `json:"unit_price" gorm:"type:decimal(10,2)"`
	TotalPrice float64   `json:"total_price" gorm:"type:decimal(10,2)"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (it *InventoryTransaction) BeforeCreate(tx *gorm.DB) error {
	if it.ID == uuid.Nil {
		it.ID = uuid.New()
	}
	return nil
}

func (o *Order) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

func (oi *OrderItem) BeforeCreate(tx *gorm.DB) error {
	if oi.ID == uuid.Nil {
		oi.ID = uuid.New()
	}
	return nil
}
