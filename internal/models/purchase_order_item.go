package models

type PurchaseOrderItemStatus string

const (
	PurchaseOrderItemStatusAwaitingDelivery   PurchaseOrderItemStatus = "awaiting_delivery"
	PurchaseOrderItemStatusPartiallyDelivered PurchaseOrderItemStatus = "partially_delivered"
	PurchaseOrderItemStatusDelivered          PurchaseOrderItemStatus = "delivered"
	PurchaseOrderItemStatusCancelled          PurchaseOrderItemStatus = "cancelled"
)

// PurchaseOrderItem represents an item in a purchase order
type PurchaseOrderItem struct {
	Base
	PurchaseOrderID  *uint                   `json:"purchase_order_id" gorm:"uniqueIndex:idx_product_supplier_po"`
	PurchaseOrder    *PurchaseOrder          `json:"purchase_order,omitempty" gorm:"foreignKey:PurchaseOrderID"`
	ProductID        *uint                   `json:"product_id" gorm:"not null;uniqueIndex:idx_product_supplier_po" validate:"required"`
	Product          *Product                `json:"product,omitempty" gorm:"foreignKey:ProductID"`
	SupplierID       *uint                   `json:"supplier_id" gorm:"not null;uniqueIndex:idx_product_supplier_po" validate:"required"`
	Supplier         *Supplier               `json:"supplier,omitempty" gorm:"foreignKey:SupplierID"`
	UnitID           *uint                   `json:"unit_id" gorm:"not null" validate:"required"`
	Unit             *Unit                   `json:"unit,omitempty" gorm:"foreignKey:UnitID"`
	UnitPrice        float64                 `json:"unit_price" gorm:"type:decimal(13,2)" validate:"min=0"`
	Quantity         int                     `json:"quantity" gorm:"not null" validate:"required,min=1"`
	ReceivedQuantity int                     `json:"received_quantity" gorm:"default:0"`
	Status           PurchaseOrderItemStatus `json:"status" gorm:"default:awaiting_delivery;check:status IN ('awaiting_delivery', 'partially_delivered', 'delivered', 'cancelled')" example:"delivering"`

	// Display fields, not stored in DB

	TotalAmount float64 `json:"total_amount" gorm:"-"` // Calculated field, not stored in DB
}

// CalculateItemTotalPrice calculates the total price for a purchase order item
func (poi *PurchaseOrderItem) CalculateTotalAmount() float64 {
	poi.TotalAmount = float64(poi.Quantity) * poi.UnitPrice
	return poi.TotalAmount
}

func (poi *PurchaseOrderItem) UpdateStatus() {
	// delivered -> partially_delivered (when quantity increases and received < quantity)
	if poi.Status == PurchaseOrderItemStatusDelivered &&
		poi.ReceivedQuantity < poi.Quantity {
		poi.Status = PurchaseOrderItemStatusPartiallyDelivered
	}

	// awaiting_delivery -> partially_delivered
	if poi.Status == PurchaseOrderItemStatusAwaitingDelivery &&
		poi.ReceivedQuantity > 0 {
		poi.Status = PurchaseOrderItemStatusPartiallyDelivered
	}

	// partially_delivered -> delivered
	if poi.Status == PurchaseOrderItemStatusPartiallyDelivered &&
		poi.ReceivedQuantity == poi.Quantity {
		poi.Status = PurchaseOrderItemStatusDelivered
	}
}
