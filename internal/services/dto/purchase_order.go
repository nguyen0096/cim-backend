package dto

type UpdatePurchaseOrderDeliveryStatusRequest struct {
	PurchaseOrderID uint `json:"purchase_order_id" validate:"required" param:"id"`
	Items           []struct {
		ID               uint `json:"id" validate:"required"`
		ReceivedQuantity int  `json:"received_quantity" validate:"required,min=1"`
	} `json:"items" validate:"required,min=1,dive"`
}
