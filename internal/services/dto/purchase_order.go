package dto

import "cim-backend/internal/models"

// UpdatePurchaseOrderItemStatusResponse represents the response for updating purchase order item status
type UpdatePurchaseOrderItemStatusResponse struct {
	ItemStatus  models.PurchaseOrderItemStatus `json:"item_status" example:"delivered"`
	OrderStatus models.PurchaseOrderStatus     `json:"order_status" example:"fully_delivered"`
}

type UpdatePurchaseOrderDeliveryStatusRequest struct {
	PurchaseOrderID uint `json:"purchase_order_id" validate:"required" param:"id"`
	Items           []struct {
		ID               uint `json:"id" validate:"required"`
		ReceivedQuantity int  `json:"received_quantity" validate:"required,min=1"`
	} `json:"items" validate:"required,min=1,dive"`
	ConfirmationNotes string `json:"confirmation_notes"`
}

// PriceHistory represents a single price entry with date
type PriceHistory struct {
	Price        float64 `json:"price"`
	PurchaseDate string  `json:"purchase_date"`
}

// LastPurchasePriceResponse represents the last purchase price for a product-supplier combination
type LastPurchasePriceResponse struct {
	ProductID        uint    `json:"product_id"`
	SupplierID       uint    `json:"supplier_id"`
	LastPrice        float64 `json:"last_price"`
	LastPurchaseDate string  `json:"last_purchase_date"`
}

// LastPurchasePriceMap represents nested map of product_id -> supplier_id -> slice of latest prices
type LastPurchasePriceMap map[uint]map[uint][]PriceHistory

// UpdatePurchaseOrderRequest represents the request to update a purchase order
type UpdatePurchaseOrderRequest struct {
	InventoryID *uint                            `json:"inventory_id" validate:"required"`
	Notes       string                           `json:"notes"`
	Items       []UpdatePurchaseOrderItemRequest `json:"items" validate:"required,min=1,dive"`
}

// UpdatePurchaseOrderItemRequest represents an item in the update purchase order request
type UpdatePurchaseOrderItemRequest struct {
	SupplierID *uint   `json:"supplier_id" validate:"required"`
	ProductID  *uint   `json:"product_id" validate:"required"`
	Quantity   int     `json:"quantity" validate:"required,min=1"`
	UnitPrice  float64 `json:"unit_price" validate:"min=0"`
}
