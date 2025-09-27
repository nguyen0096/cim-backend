package models

type PurchaseOrderStatus string

const (
	PurchaseOrderStatusOrderPlaced        PurchaseOrderStatus = "order_placed"
	PurchaseOrderStatusPartiallyDelivered PurchaseOrderStatus = "partially_delivered"
	PurchaseOrderStatusFullyDelivered     PurchaseOrderStatus = "fully_delivered"
	PurchaseOrderStatusCompleted          PurchaseOrderStatus = "completed"
	PurchaseOrderStatusCancelled          PurchaseOrderStatus = "cancelled"
)

// PurchaseOrder represents a purchase order
type PurchaseOrder struct {
	Base
	OrderNumber string               `json:"order_number" gorm:"unique;not null"`
	Status      PurchaseOrderStatus  `json:"status" gorm:"default:order_placed;check:status IN ('order_placed', 'partially_delivered', 'fully_delivered', 'completed', 'cancelled')"`
	TotalAmount float64              `json:"total_amount" gorm:"-"` // Calculated field, not stored in DB
	Notes       string               `json:"notes"`
	Items       []*PurchaseOrderItem `json:"items" gorm:"foreignKey:PurchaseOrderID" validate:"required,min=1,dive"`
}

// PurchaseOrderItem represents an item in a purchase order
type PurchaseOrderItem struct {
	Base
	PurchaseOrderID  *uint          `json:"purchase_order_id"`
	PurchaseOrder    *PurchaseOrder `json:"purchase_order,omitempty" gorm:"foreignKey:PurchaseOrderID" validate:"-"`
	ProductID        *uint          `json:"product_id" validate:"required"`
	Product          *Product       `json:"product,omitempty" gorm:"foreignKey:ProductID" validate:"-"`
	Quantity         int            `json:"quantity" gorm:"not null" validate:"required,min=1"`
	TotalPrice       float64        `json:"total_price" gorm:"-"` // Calculated field, not stored in DB
	ReceivedQuantity int            `json:"received_quantity" gorm:"default:0"`
}

// CalculateItemTotalPrice calculates the total price for a purchase order item
func (poi *PurchaseOrderItem) CalculateTotalPrice() float64 {
	if poi.Product != nil {
		return float64(poi.Quantity) * poi.Product.UnitPrice
	}
	return 0
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
