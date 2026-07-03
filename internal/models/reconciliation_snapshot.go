package models

import "github.com/shopspring/decimal"

// ReconciliationSnapshot captures the per-item baseline quantity when a
// reconciliation is initiated. It is the sole source of truth for prev_quantity.
// One live row per (SubmissionID, InventoryItemID).
type ReconciliationSnapshot struct {
	Base
	SubmissionID    uint                 `json:"submission_id" gorm:"not null"`
	Submission      *InventorySubmission `json:"submission,omitempty" gorm:"foreignKey:SubmissionID" validate:"-"`
	InventoryItemID uint                 `json:"inventory_item_id" gorm:"not null"`
	PrevQuantity    decimal.Decimal      `json:"prev_quantity" gorm:"type:numeric;not null"`
}

// TableName pins the table name to the migration-created table.
func (ReconciliationSnapshot) TableName() string {
	return "reconciliation_snapshots"
}
