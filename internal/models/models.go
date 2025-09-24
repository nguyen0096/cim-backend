package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Supplier represents a supplier
type Supplier struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name         string    `json:"name" gorm:"not null"`
	ContactEmail string    `json:"contact_email"`
	ContactPhone string    `json:"contact_phone"`
	Address      string    `json:"address"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Product represents a product
type Product struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name        string     `json:"name" gorm:"not null"`
	Description string     `json:"description"`
	SKU         string     `json:"sku" gorm:"unique"`
	SupplierID  uuid.UUID  `json:"supplier_id"`
	Supplier    Supplier   `json:"supplier" gorm:"foreignKey:SupplierID"`
	UnitPrice   float64    `json:"unit_price" gorm:"type:decimal(10,2)"`
	Status      string     `json:"status" gorm:"default:active;check:status IN ('active', 'inactive', 'discontinued')"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Inventory   *Inventory `json:"inventory,omitempty" gorm:"foreignKey:ProductID"`
}

// Inventory represents inventory for a product
type Inventory struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ProductID    uuid.UUID `json:"product_id" gorm:"unique;not null"`
	Product      Product   `json:"product" gorm:"foreignKey:ProductID"`
	Quantity     int       `json:"quantity" gorm:"default:0"`
	ReorderLevel int       `json:"reorder_level" gorm:"default:0"`
	Location     string    `json:"location"`
	LastUpdated  time.Time `json:"last_updated"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PurchaseOrder represents a purchase order
type PurchaseOrder struct {
	ID           uuid.UUID             `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrderNumber  string                `json:"order_number" gorm:"unique;not null"`
	SupplierID   uuid.UUID             `json:"supplier_id"`
	Supplier     Supplier              `json:"supplier" gorm:"foreignKey:SupplierID"`
	Status       string                `json:"status" gorm:"default:pending;check:status IN ('pending', 'approved', 'received', 'cancelled')"`
	TotalAmount  float64               `json:"total_amount" gorm:"type:decimal(10,2)"`
	Notes        string                `json:"notes"`
	CreatedBy    string                `json:"created_by"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	Items        []PurchaseOrderItem   `json:"items" gorm:"foreignKey:PurchaseOrderID"`
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

// InventoryTransaction represents an inventory transaction
type InventoryTransaction struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ProductID      uuid.UUID `json:"product_id"`
	Product        Product   `json:"product" gorm:"foreignKey:ProductID"`
	TransactionType string   `json:"transaction_type" gorm:"not null;check:transaction_type IN ('purchase', 'sale', 'adjustment', 'return')"`
	Quantity       int       `json:"quantity" gorm:"not null"`
	ReferenceID    uuid.UUID `json:"reference_id"`
	ReferenceType  string    `json:"reference_type"`
	Notes          string    `json:"notes"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

// Order represents a customer order
type Order struct {
	ID           uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrderNumber  string     `json:"order_number" gorm:"unique;not null"`
	CustomerName string     `json:"customer_name"`
	CustomerEmail string    `json:"customer_email"`
	Status       string     `json:"status" gorm:"default:pending;check:status IN ('pending', 'processing', 'completed', 'cancelled')"`
	TotalAmount  float64    `json:"total_amount" gorm:"type:decimal(10,2)"`
	Notes        string     `json:"notes"`
	CreatedBy    string     `json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Items        []OrderItem `json:"items" gorm:"foreignKey:OrderID"`
}

// OrderItem represents an item in an order
type OrderItem struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrderID   uuid.UUID `json:"order_id"`
	Order     Order     `json:"order" gorm:"foreignKey:OrderID"`
	ProductID uuid.UUID `json:"product_id"`
	Product   Product   `json:"product" gorm:"foreignKey:ProductID"`
	Quantity  int       `json:"quantity" gorm:"not null"`
	UnitPrice float64   `json:"unit_price" gorm:"type:decimal(10,2)"`
	TotalPrice float64  `json:"total_price" gorm:"type:decimal(10,2)"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Supplier) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

func (p *Product) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (i *Inventory) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}

func (po *PurchaseOrder) BeforeCreate(tx *gorm.DB) error {
	if po.ID == uuid.Nil {
		po.ID = uuid.New()
	}
	return nil
}

func (poi *PurchaseOrderItem) BeforeCreate(tx *gorm.DB) error {
	if poi.ID == uuid.Nil {
		poi.ID = uuid.New()
	}
	return nil
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
