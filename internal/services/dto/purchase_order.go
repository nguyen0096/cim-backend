package dto

import "import-export-backend/internal/models"

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
}
