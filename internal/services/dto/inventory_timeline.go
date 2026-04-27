package dto

type InventoryTimelineRequest struct {
	InventoryID uint   `param:"id" validate:"required"`
	StartDate   string `query:"start_date" validate:"required"`
	EndDate     string `query:"end_date" validate:"required"`
	ProductIDs  []uint `query:"product_ids"`
}

type TimelineTransaction struct {
	TransactionID   uint     `json:"transaction_id"`
	TransactionType string   `json:"transaction_type"`
	Date            string   `json:"date"`
	Quantity        float64  `json:"quantity"`
	SellingPrice    *float64 `json:"selling_price,omitempty"`
	CostPrice       *float64 `json:"cost_price,omitempty"`
	Notes           string   `json:"notes,omitempty"`
	POID            *uint    `json:"po_id,omitempty"`
}

type TimelinePurchaseOrder struct {
	POID             uint     `json:"po_id"`
	PONumber         string   `json:"po_number"`
	DeliveryDate     *string  `json:"delivery_date"`
	DeliveryStatus   string   `json:"delivery_status"`
	PaymentDate      *string  `json:"payment_date"`
	PaymentStatus    string   `json:"payment_status"`
	QuantityOrdered  float64  `json:"quantity_ordered"`
	QuantityReceived float64  `json:"quantity_received"`
	SellingPrice     *float64 `json:"selling_price"`
}

type TimelineProductMetrics struct {
	TotalPurchased   float64 `json:"total_purchased"`
	TotalSold        float64 `json:"total_sold"`
	TotalDisposed    float64 `json:"total_disposed"`
	TotalTransferIn  float64 `json:"total_transfer_in"`
	TotalTransferred float64 `json:"total_transferred"`
	TotalRevenue     float64 `json:"total_revenue"`
}

type ProductTimeline struct {
	ProductID      uint                    `json:"product_id"`
	ProductName    string                  `json:"product_name"`
	ProductUnit    string                  `json:"product_unit"`
	BeginningStock float64                 `json:"beginning_stock"`
	EndingStock    float64                 `json:"ending_stock"`
	PurchaseOrders []TimelinePurchaseOrder `json:"purchase_orders"`
	Transactions   []TimelineTransaction   `json:"transactions"`
	Metrics        TimelineProductMetrics  `json:"metrics"`
}

type InventoryTimelineResponse struct {
	Products []ProductTimeline `json:"products"`
}
