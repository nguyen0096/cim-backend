package models

import "time"

// PurchaseOrderReceipt records an applied receive submission by idempotency key,
// making the receive endpoint safe against duplicate submits.
type PurchaseOrderReceipt struct {
	ID              uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	IdempotencyKey  string    `json:"idempotency_key" gorm:"column:idempotency_key;not null;uniqueIndex:idx_purchase_order_receipts_po_key,priority:2"`
	PurchaseOrderID uint      `json:"purchase_order_id" gorm:"column:purchase_order_id;not null;uniqueIndex:idx_purchase_order_receipts_po_key,priority:1"`
	CreatedAt       time.Time `json:"created_at"`
}

func (PurchaseOrderReceipt) TableName() string {
	return "purchase_order_receipts"
}
