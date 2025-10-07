package models

type PurchaseOrderStatus string

const (
	PurchaseOrderStatusOrderPlaced        PurchaseOrderStatus = "order_placed"
	PurchaseOrderStatusPartiallyDelivered PurchaseOrderStatus = "partially_delivered"
	PurchaseOrderStatusFullyDelivered     PurchaseOrderStatus = "fully_delivered"
	PurchaseOrderStatusCompleted          PurchaseOrderStatus = "completed"
	PurchaseOrderStatusCancelled          PurchaseOrderStatus = "cancelled"
)

type PurchaseOrderItemStatus string

const (
	PurchaseOrderItemStatusDelivering PurchaseOrderItemStatus = "delivering"
	PurchaseOrderItemStatusDelivered  PurchaseOrderItemStatus = "delivered"
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

// PurchaseOrderItem represents an item in a purchase order
type PurchaseOrderItem struct {
	Base
	PurchaseOrderID  *uint                   `json:"purchase_order_id"`
	PurchaseOrder    *PurchaseOrder          `json:"purchase_order,omitempty" gorm:"foreignKey:PurchaseOrderID" validate:"-"`
	ProductID        *uint                   `json:"product_id" validate:"required"`
	Product          *Product                `json:"product,omitempty" gorm:"foreignKey:ProductID" validate:"-"`
	UnitPrice        float64                 `json:"unit_price" gorm:"type:decimal(13,2)" validate:"required,min=0"`
	Quantity         int                     `json:"quantity" gorm:"not null" validate:"required,min=1"`
	TotalPrice       float64                 `json:"total_price" gorm:"-"` // Calculated field, not stored in DB
	ReceivedQuantity int                     `json:"received_quantity" gorm:"default:0"`
	Status           PurchaseOrderItemStatus `json:"status" gorm:"default:delivering;check:status IN ('delivering', 'delivered')" example:"delivering"`
}

// CalculateItemTotalPrice calculates the total price for a purchase order item
func (poi *PurchaseOrderItem) CalculateTotalPrice() float64 {
	return float64(poi.Quantity) * poi.UnitPrice
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

// UpdatePurchaseOrderItemStatusResponse represents the response for updating purchase order item status
type UpdatePurchaseOrderItemStatusResponse struct {
	ItemStatus  PurchaseOrderItemStatus `json:"item_status" example:"delivered"`
	OrderStatus PurchaseOrderStatus     `json:"order_status" example:"fully_delivered"`
}
