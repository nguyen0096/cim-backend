package dto

import (
	"time"

	"cim-backend/internal/models"
)

type PaymentReceiptFormListRequest struct {
	models.ListParams
	PurchaseOrderID uint                              `json:"purchase_order_id" query:"purchase_order_id"`
	Date            string                            `json:"date" query:"date"`
	Statuses        []models.PaymentReceiptFormStatus `json:"statuses" query:"statuses"`
}

// PaymentReceiptFormPayload represents the payload for creating a payment receipt form
type PaymentReceiptFormPayload struct {
	FormNumber      string                          `json:"form_number,omitempty"`
	PurchaseOrderID uint                            `json:"purchase_order_id"`
	FullName        string                          `json:"full_name"`
	Date            string                          `json:"date"`
	Department      string                          `json:"department"`
	Details         string                          `json:"details"`
	TotalAmount     float64                         `json:"total_amount"`
	AmountInWords   string                          `json:"amount_in_words"`
	Status          models.PaymentReceiptFormStatus `json:"status" validate:"required,oneof=pending submitted approved rejected"`
}

// ToPaymentReceiptForm converts the payload to a PaymentReceiptForm model
func (p *PaymentReceiptFormPayload) ToPaymentReceiptForm() (*models.PaymentReceiptForm, error) {
	model := &models.PaymentReceiptForm{
		FormNumber:      p.FormNumber,
		PurchaseOrderID: p.PurchaseOrderID,
		FullName:        p.FullName,
		Department:      p.Department,
		Details:         p.Details,
		TotalAmount:     p.TotalAmount,
		Status:          p.Status,
	}

	date, err := time.Parse("2006-01-02", p.Date)
	if err == nil {
		model.Date = date
	}

	return model, nil
}
