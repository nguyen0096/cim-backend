package models

import "time"

type PaymentReceiptFormStatus string

const (
	PaymentReceiptFormStatusPending   PaymentReceiptFormStatus = "pending"
	PaymentReceiptFormStatusSubmitted PaymentReceiptFormStatus = "submitted"
)

type PaymentReceiptForm struct {
	Base
	Location    string                   `json:"location" gorm:"not null"`
	Date        time.Time                `json:"date" gorm:"not null"`
	FullName    string                   `json:"full_name" gorm:"not null"`
	Department  string                   `json:"department" gorm:"not null"`
	Details     string                   `json:"details"`
	TotalAmount float64                  `json:"total_amount" gorm:"not null;check:total_amount > 0"`
	Status      PaymentReceiptFormStatus `json:"status" gorm:"default:pending;check:status IN ('pending', 'submitted')"`
}
