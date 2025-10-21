package dto

import (
	"time"

	"cim-backend/internal/models"
)

// PaymentReceiptFormPayload represents the payload for creating a payment receipt form
type PaymentReceiptFormPayload struct {
	PurchaseOrderID uint    `json:"purchase_order_id"`
	FullName        string  `json:"full_name"`
	Date            string  `json:"date"`
	Department      string  `json:"department"`
	Details         string  `json:"details"`
	TotalAmount     float64 `json:"total_amount"`
	AmountInWords   string  `json:"amount_in_words"`
}

// ToPaymentReceiptForm converts the payload to a PaymentReceiptForm model
func (p *PaymentReceiptFormPayload) ToPaymentReceiptForm() (*models.PaymentReceiptForm, error) {
	model := &models.PaymentReceiptForm{
		PurchaseOrderID: p.PurchaseOrderID,
		FullName:        p.FullName,
		Department:      p.Department,
		Details:         p.Details,
		TotalAmount:     p.TotalAmount,
		Status:          models.PaymentReceiptFormStatusPending,
	}

	date, err := time.Parse("2006-01-02", p.Date)
	if err == nil {
		model.Date = date
	}

	return model, nil
}
