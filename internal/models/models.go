package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"import-export-backend/pkg"
)

type Base struct {
	ID        uint           `json:"id" gorm:"primaryKey;autoIncrement"`
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

// Order represents a customer order
type Order struct {
	ID            uint        `json:"id" gorm:"primaryKey;autoIncrement"`
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
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderID    uint      `json:"order_id"`
	Order      Order     `json:"order" gorm:"foreignKey:OrderID"`
	ProductID  uint      `json:"product_id"`
	Product    Product   `json:"product" gorm:"foreignKey:ProductID"`
	Quantity   int       `json:"quantity" gorm:"not null"`
	UnitPrice  float64   `json:"unit_price" gorm:"type:decimal(10,2)"`
	TotalPrice float64   `json:"total_price" gorm:"type:decimal(10,2)"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (it *InventoryTransaction) BeforeCreate(tx *gorm.DB) error {
	return nil
}

func (o *Order) BeforeCreate(tx *gorm.DB) error {
	return nil
}

func (oi *OrderItem) BeforeCreate(tx *gorm.DB) error {
	return nil
}
