package models

import "time"

// PurchaseOrderReceipt records an applied receive submission by idempotency key,
// making the receive endpoint safe against duplicate submits.
type PurchaseOrderReceipt struct {
	ID              uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	IdempotencyKey  string    `json:"idempotency_key" gorm:"column:idempotency_key;not null;uniqueIndex"`
	PurchaseOrderID uint      `json:"purchase_order_id" gorm:"column:purchase_order_id;not null"`
	CreatedAt       time.Time `json:"created_at"`
}

func (PurchaseOrderReceipt) TableName() string {
	return "purchase_order_receipts"
}
