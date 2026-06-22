package models

import "encoding/json"

// ReconciliationRequestItemStatus is the per-row state in the reconciliation
// child-item state machine (epic #38).
//
//	in_progress -> ready -> approved -> applied
//
// with an escape hatch: approved -> in_progress on a staff edit. Once applied a
// row is immutable.
type ReconciliationRequestItemStatus string

const (
	// ReconciliationRequestItemStatusInProgress is the initial status; the staff
	// member is still entering/adjusting their counts.
	ReconciliationRequestItemStatusInProgress ReconciliationRequestItemStatus = "in_progress"
	// ReconciliationRequestItemStatusReady means staff marked the row ready for
	// admin/accountant review.
	ReconciliationRequestItemStatusReady ReconciliationRequestItemStatus = "ready"
	// ReconciliationRequestItemStatusApproved means an admin/accountant approved
	// the row. A subsequent staff edit resets it to in_progress.
	ReconciliationRequestItemStatusApproved ReconciliationRequestItemStatus = "approved"
	// ReconciliationRequestItemStatusApplied is terminal/immutable; set during
	// the atomic stage-move when the parent submission is approved.
	ReconciliationRequestItemStatusApplied ReconciliationRequestItemStatus = "applied"
)

// IsValid reports whether s is a recognized status value.
func (s ReconciliationRequestItemStatus) IsValid() bool {
	switch s {
	case ReconciliationRequestItemStatusInProgress,
		ReconciliationRequestItemStatusReady,
		ReconciliationRequestItemStatusApproved,
		ReconciliationRequestItemStatusApplied:
		return true
	default:
		return false
	}
}

// ReconciliationRequestItem is one staff/batch contribution to an in-flight
// reconciliation. Payload holds COUNTED quantities only, in the legacy reconcile
// shape {"items":[{"inventory_item_id":<id>,"quantity":<counted>}]}; the baseline
// lives in ReconciliationSnapshot. At apply time the active rows are summed by
// inventory_item_id into the finalized synthesized payload on the parent
// inventory_submissions row.
//
// The contributing staff member is recorded via Base.CreatedBy (user email),
// consistent with every other table in this codebase.
type ReconciliationRequestItem struct {
	Base
	SubmissionID uint                            `json:"submission_id" gorm:"not null"`
	Submission   *InventorySubmission            `json:"submission,omitempty" gorm:"foreignKey:SubmissionID" validate:"-"`
	Payload      json.RawMessage                 `json:"payload" gorm:"serializer:json;type:jsonb;not null"`
	Status       ReconciliationRequestItemStatus `json:"status" gorm:"type:varchar(20);not null;default:in_progress"`
}

// TableName pins the table name to the migration-created table.
func (ReconciliationRequestItem) TableName() string {
	return "reconciliation_request_items"
}
