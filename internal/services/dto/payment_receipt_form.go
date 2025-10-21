package dto

import (
	"time"

	"cim-backend/internal/models"
)

// PaymentReceiptFormPayload represents the payload for creating a payment receipt form
type PaymentReceiptFormPayload struct {
	FullName       string  `json:"full_name" validate:"required"`
	Date           string  `json:"date" validate:"required"`
	Department     string  `json:"department" validate:"required"`
	PaymentDetails string  `json:"payment_details" validate:"required"`
	TotalAmount    float64 `json:"total_amount" validate:"required,gt=0"`
	AmountInWords  string  `json:"amount_in_words" validate:"required"`
}

// ToPaymentReceiptForm converts the payload to a PaymentReceiptForm model
func (p *PaymentReceiptFormPayload) ToPaymentReceiptForm() (*PaymentReceiptFormModel, error) {
	date, err := time.Parse("2006-01-02", p.Date)
	if err != nil {
		return nil, err
	}

	return &PaymentReceiptFormModel{
		FullName:    p.FullName,
		Date:        date,
		Department:  p.Department,
		Details:     p.PaymentDetails,
		TotalAmount: p.TotalAmount,
		Status:      models.PaymentReceiptFormStatusPending,
	}, nil
}

// PaymentReceiptFormModel represents the internal model for payment receipt form
type PaymentReceiptFormModel struct {
	FullName    string                          `json:"full_name"`
	Date        time.Time                       `json:"date"`
	Department  string                          `json:"department"`
	Details     string                          `json:"details"`
	TotalAmount float64                         `json:"total_amount"`
	Status      models.PaymentReceiptFormStatus `json:"status"`
}
