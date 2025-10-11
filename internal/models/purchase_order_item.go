package models

type PurchaseOrderItemStatus string

const (
	PurchaseOrderItemStatusDelivering PurchaseOrderItemStatus = "delivering"
	PurchaseOrderItemStatusDelivered  PurchaseOrderItemStatus = "delivered"
	PurchaseOrderItemStatusCancelled  PurchaseOrderItemStatus = "cancelled"
)

// PurchaseOrderItem represents an item in a purchase order
type PurchaseOrderItem struct {
	Base
	PurchaseOrderID  *uint                   `json:"purchase_order_id"`
	PurchaseOrder    *PurchaseOrder          `json:"purchase_order,omitempty" gorm:"foreignKey:PurchaseOrderID"`
	ProductID        *uint                   `json:"product_id" validate:"required"`
	Product          *Product                `json:"product,omitempty" gorm:"foreignKey:ProductID"`
	SupplierID       *uint                   `json:"supplier_id" validate:"required"`
	Supplier         *Supplier               `json:"supplier,omitempty" gorm:"foreignKey:SupplierID"`
	UnitPrice        float64                 `json:"unit_price" gorm:"type:decimal(13,2)" validate:"min=0"`
	Quantity         int                     `json:"quantity" gorm:"not null" validate:"required,min=1"`
	TotalPrice       float64                 `json:"total_price" gorm:"-"` // Calculated field, not stored in DB
	ReceivedQuantity int                     `json:"received_quantity" gorm:"default:0"`
	Status           PurchaseOrderItemStatus `json:"status" gorm:"default:delivering;check:status IN ('delivering', 'delivered', 'cancelled')" example:"delivering"`
}

// CalculateItemTotalPrice calculates the total price for a purchase order item
func (poi *PurchaseOrderItem) CalculateTotalPrice() float64 {
	return float64(poi.Quantity) * poi.UnitPrice
}

func (poi *PurchaseOrderItem) UpdateStatus() {
	if poi.Status == PurchaseOrderItemStatusDelivering &&
		poi.ReceivedQuantity == poi.Quantity {
		poi.Status = PurchaseOrderItemStatusDelivered
	}
}
