package models

import "time"

type PaymentReceiptFormStatus string

const (
	PaymentReceiptFormStatusPending   PaymentReceiptFormStatus = "pending"
	PaymentReceiptFormStatusSubmitted PaymentReceiptFormStatus = "submitted"
	PaymentReceiptFormStatusApproved  PaymentReceiptFormStatus = "approved"
	PaymentReceiptFormStatusRejected  PaymentReceiptFormStatus = "rejected"
)

type PaymentReceiptForm struct {
	Base
	FormNumber      *string                  `json:"form_number,omitempty" gorm:"unique" example:"20250101-1-1"`
	PurchaseOrderID uint                     `json:"purchase_order_id" gorm:"not null"`
	PurchaseOrder   *PurchaseOrder           `json:"purchase_order,omitempty" gorm:"foreignKey:PurchaseOrderID"`
	Date            time.Time                `json:"date"`
	FullName        string                   `json:"full_name"`
	Department      string                   `json:"department"`
	Details         string                   `json:"details"`
	TotalAmount     float64                  `json:"total_amount"`
	Status          PaymentReceiptFormStatus `json:"status" gorm:"default:pending;check:status IN ('pending', 'submitted', 'approved', 'rejected')"`
}
