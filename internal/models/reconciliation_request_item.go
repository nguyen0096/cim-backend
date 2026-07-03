package models

import "encoding/json"

// ReconciliationRequestItemStatus is the per-session readiness state of a
// reconciliation child-item row.
type ReconciliationRequestItemStatus string

const (
	// ReconciliationRequestItemStatusInProgress: staff are still entering counts.
	ReconciliationRequestItemStatusInProgress ReconciliationRequestItemStatus = "in_progress"
	// ReconciliationRequestItemStatusReadyForReview: the staff member marked their
	// own count session done.
	ReconciliationRequestItemStatusReadyForReview ReconciliationRequestItemStatus = "ready_for_review"
)

// IsValid reports whether s is a recognized status value.
func (s ReconciliationRequestItemStatus) IsValid() bool {
	switch s {
	case ReconciliationRequestItemStatusInProgress, ReconciliationRequestItemStatusReadyForReview:
		return true
	default:
		return false
	}
}

// ReconciliationRequestItem is one staff contribution to an in-flight
// reconciliation. Payload holds counted quantities only; the baseline lives in
// ReconciliationSnapshot.
type ReconciliationRequestItem struct {
	Base
	SubmissionID uint                 `json:"submission_id" gorm:"not null"`
	Submission   *InventorySubmission `json:"submission,omitempty" gorm:"foreignKey:SubmissionID" validate:"-"`
	// Label is the row-level identifier for this count session, distinct from the
	// per-count labels inside Payload. Blank means no label; required once the user
	// has a 2nd row, and distinct per (submission, created_by).
	Label   string                          `json:"label" gorm:"type:varchar(255);not null;default:''"`
	Payload json.RawMessage                 `json:"payload" gorm:"serializer:json;type:jsonb;not null" swaggertype:"object"`
	Status  ReconciliationRequestItemStatus `json:"status" gorm:"type:varchar(20);not null;default:in_progress"`
}

// TableName pins the table name to the migration-created table.
func (ReconciliationRequestItem) TableName() string {
	return "reconciliation_request_items"
}
