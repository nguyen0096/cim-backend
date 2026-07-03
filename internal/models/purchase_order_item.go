package models

import "github.com/shopspring/decimal"

type PurchaseOrderItemStatus string

const (
	PurchaseOrderItemStatusAwaitingDelivery   PurchaseOrderItemStatus = "awaiting_delivery"
	PurchaseOrderItemStatusPartiallyDelivered PurchaseOrderItemStatus = "partially_delivered"
	PurchaseOrderItemStatusDelivered          PurchaseOrderItemStatus = "delivered"
	PurchaseOrderItemStatusOverDelivered      PurchaseOrderItemStatus = "over_delivered"
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
	Quantity         decimal.Decimal         `json:"quantity" gorm:"type:decimal(10,2);not null" validate:"required"`
	ReceivedQuantity decimal.Decimal         `json:"received_quantity" gorm:"type:decimal(10,2);default:0"`
	Status           PurchaseOrderItemStatus `json:"status" gorm:"default:awaiting_delivery;check:status IN ('awaiting_delivery', 'partially_delivered', 'delivered', 'over_delivered', 'cancelled')" example:"delivering"`

	// Relationships
	POItemSellingPrice *POItemSellingPrice `json:"po_item_selling_price,omitempty" gorm:"foreignKey:PurchaseOrderItemID"`

	// Display fields, not stored in DB

	TotalAmount decimal.Decimal `json:"total_amount" gorm:"-"`
}

// CalculateTotalAmount computes and stores the item's total amount.
func (poi *PurchaseOrderItem) CalculateTotalAmount() decimal.Decimal {
	poi.TotalAmount = poi.Quantity.Mul(decimal.NewFromFloat(poi.UnitPrice))
	return poi.TotalAmount
}

func (poi *PurchaseOrderItem) UpdateStatus() {
	if poi.ReceivedQuantity.Equal(decimal.Zero) {
		if poi.Status == PurchaseOrderItemStatusPartiallyDelivered || poi.Status == PurchaseOrderItemStatusDelivered || poi.Status == PurchaseOrderItemStatusOverDelivered {
			poi.Status = PurchaseOrderItemStatusAwaitingDelivery
		}
		return
	}

	if poi.ReceivedQuantity.GreaterThan(poi.Quantity) {
		poi.Status = PurchaseOrderItemStatusOverDelivered
		return
	}

	if poi.ReceivedQuantity.Equal(poi.Quantity) {
		if poi.Status == PurchaseOrderItemStatusAwaitingDelivery || poi.Status == PurchaseOrderItemStatusPartiallyDelivered || poi.Status == PurchaseOrderItemStatusOverDelivered {
			poi.Status = PurchaseOrderItemStatusDelivered
		}
		return
	}

	if poi.ReceivedQuantity.GreaterThan(decimal.Zero) && poi.ReceivedQuantity.LessThan(poi.Quantity) {
		if poi.Status == PurchaseOrderItemStatusAwaitingDelivery || poi.Status == PurchaseOrderItemStatusDelivered || poi.Status == PurchaseOrderItemStatusOverDelivered {
			poi.Status = PurchaseOrderItemStatusPartiallyDelivered
		}
		return
	}
}

// CompatifyUnit backfills the unit field on POIs that predate it.
func (poi *PurchaseOrderItem) CompatifyUnit(compatibleUnitID uint) {
	if poi.Unit == nil {
		poi.UnitID = &compatibleUnitID
		return
	}
}
