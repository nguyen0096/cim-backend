package models

import "encoding/json"

// ReconciliationRequestItemStatus is the per-row state in the reconciliation
// child-item lifecycle (epic #38).
//
// The lifecycle was collapsed in the Part-6 redesign (locked decision Q1): a
// staff child row has a single editable state, `in_progress`. There is no
// per-row `ready`/`approved`/`applied` gate anymore — the SUBMISSION-level
// lifecycle (open -> closed -> processing -> processed) on inventory_submissions
// is the only gate. Rows become immutable to staff when the parent submission is
// `closed` (enforced by the closed-guard in loadActiveReconcileParent), and the
// whole reconcile is applied atomically at Start Processing.
type ReconciliationRequestItemStatus string

const (
	// ReconciliationRequestItemStatusInProgress is the only status: the staff
	// member's editable counts. Immutability is enforced at the submission level
	// (the parent's closed status), not per row.
	ReconciliationRequestItemStatusInProgress ReconciliationRequestItemStatus = "in_progress"
)

// IsValid reports whether s is a recognized status value.
func (s ReconciliationRequestItemStatus) IsValid() bool {
	switch s {
	case ReconciliationRequestItemStatusInProgress:
		return true
	default:
		return false
	}
}

// ReconciliationRequestItem is one staff/batch contribution to an in-flight
// reconciliation. Payload holds COUNTED quantities only, in the legacy reconcile
// shape {"items":[{"inventory_item_id":<id>,"quantity":<counted>}]}; the baseline
// lives in ReconciliationSnapshot. At Start Processing the active rows are summed
// by inventory_item_id into the finalized synthesized payload on the parent
// inventory_submissions row.
//
// The contributing staff member is recorded via Base.CreatedBy (user email),
// consistent with every other table in this codebase.
type ReconciliationRequestItem struct {
	Base
	SubmissionID uint                            `json:"submission_id" gorm:"not null"`
	Submission   *InventorySubmission            `json:"submission,omitempty" gorm:"foreignKey:SubmissionID" validate:"-"`
	Payload      json.RawMessage                 `json:"payload" gorm:"serializer:json;type:jsonb;not null" swaggertype:"object"`
	Status       ReconciliationRequestItemStatus `json:"status" gorm:"type:varchar(20);not null;default:in_progress"`
}

// TableName pins the table name to the migration-created table.
func (ReconciliationRequestItem) TableName() string {
	return "reconciliation_request_items"
}
