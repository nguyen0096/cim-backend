package dto

import (
	"strings"
	"time"

	"cim-backend/internal/models"
	"cim-backend/pkg"
)

type PaymentReceiptFormListRequest struct {
	models.ListParams
	PurchaseOrderID uint                              `json:"purchase_order_id" query:"purchase_order_id"`
	InventoryID     uint                              `json:"inventory_id" query:"inventory_id"`
	Date            string                            `json:"date" query:"date"`
	Statuses        []models.PaymentReceiptFormStatus `json:"statuses" query:"statuses"`
}

// PaymentReceiptFormPayload represents the payload for creating a payment receipt form
type PaymentReceiptFormPayload struct {
	InventoryID     *uint                           `json:"inventory_id,omitempty"`
	FormNumber      *string                         `json:"form_number,omitempty"`
	PurchaseOrderID uint                            `json:"purchase_order_id"`
	FullName        string                          `json:"full_name" validate:"required"`
	Date            string                          `json:"date"`
	Department      string                          `json:"department"`
	Details         string                          `json:"details" validate:"required"`
	TotalAmount     float64                         `json:"total_amount" validate:"required"`
	AmountInWords   string                          `json:"amount_in_words"`
	Status          models.PaymentReceiptFormStatus `json:"status" validate:"omitempty,oneof=pending submitted approved rejected"`
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

	if p.InventoryID != nil {
		model.PurchaseOrder = &models.PurchaseOrder{
			InventoryID: p.InventoryID,
		}
	}

	if p.Date == "" {
		return model, nil
	}
	if strings.Contains(p.Date, "T") {
		date, err := time.Parse(time.RFC3339, p.Date)
		if err != nil {
			return nil, pkg.NewAppError(pkg.ErrorCodeValidation, "Invalid date format. Expected RFC3339 format", err)
		}
		model.Date = date
	} else {
		date, err := time.Parse("2006-01-02", p.Date)
		if err != nil {
			return nil, pkg.NewAppError(pkg.ErrorCodeValidation, "Invalid date format. Expected YYYY-MM-DD format", err)
		}
		model.Date = date
	}

	return model, nil
}
