package models

import (
	"encoding/json"
	"time"
)

// ReconcileLifecycleStatus is the SUBMISSION-level reconciliation lifecycle
// (epic #38, Part 6 redesign, locked decision Q3). It is a status OF
// inventory_submissions (not a separate boolean and not an approval status):
// only initiated reconciles carry it; every other submission leaves it empty.
//
//	open       -> staff may freely create/edit/delete their child count rows.
//	closed      -> staff are LOCKED out (immutable to staff); admin/accountant
//	               may still edit child rows. The admin reviews, then starts
//	               processing (or reopens to let staff edit again).
//	processing  -> transient: set at the start of the atomic Start-Processing
//	               apply transaction.
//	processed   -> terminal: the reconcile has been applied (consuming
//	               transactions created, stock mutated). Immutable to everyone.
//
// `open` is the only editable staff state; `close` is the only gate (the
// per-row ready/approved/applied states were removed in this redesign).
type ReconcileLifecycleStatus string

const (
	ReconcileLifecycleStatusOpen       ReconcileLifecycleStatus = "open"
	ReconcileLifecycleStatusClosed     ReconcileLifecycleStatus = "closed"
	ReconcileLifecycleStatusProcessing ReconcileLifecycleStatus = "processing"
	ReconcileLifecycleStatusProcessed  ReconcileLifecycleStatus = "processed"
)

// IsValid reports whether s is a recognized reconcile lifecycle status.
func (s ReconcileLifecycleStatus) IsValid() bool {
	switch s {
	case ReconcileLifecycleStatusOpen,
		ReconcileLifecycleStatusClosed,
		ReconcileLifecycleStatusProcessing,
		ReconcileLifecycleStatusProcessed:
		return true
	default:
		return false
	}
}

type SubmissionProcessingStatus string

const (
	InventorySubmissionStatusPending   SubmissionProcessingStatus = "pending"
	InventorySubmissionStatusFailed    SubmissionProcessingStatus = "failed"
	InventorySubmissionStatusCompleted SubmissionProcessingStatus = "completed"
	InventorySubmissionStatusCanceled  SubmissionProcessingStatus = "canceled"
)

type SubmissionApprovalStatus string

const (
	InventorySubmissionApprovalStatusPending  SubmissionApprovalStatus = "pending"
	InventorySubmissionApprovalStatusApproved SubmissionApprovalStatus = "approved"
	InventorySubmissionApprovalStatusRejected SubmissionApprovalStatus = "rejected"
)

type SubmissionType string

const (
	InventorySubmissionTypeReconcile SubmissionType = "reconcile"
	InventorySubmissionTypeDispose   SubmissionType = "dispose"
	InventorySubmissionTypeTransfer  SubmissionType = "transfer"
)

type InventorySubmissionAction string

const (
	InventorySubmissionActionApprove InventorySubmissionAction = "approve"
	InventorySubmissionActionReject  InventorySubmissionAction = "reject"
)

// InventorySubmission represents a pending inventory operation
type InventorySubmission struct {
	Base
	InventoryID      uint                       `json:"inventory_id"`
	Inventory        *Inventory                 `json:"inventory,omitempty" gorm:"foreignKey:InventoryID"`
	SubmissionType   SubmissionType             `json:"submission_type" gorm:"not null"`
	ProcessingStatus SubmissionProcessingStatus `json:"processing_status" gorm:"default:pending"`
	ApprovalStatus   SubmissionApprovalStatus   `json:"approval_status" gorm:"default:pending"`
	Payload          json.RawMessage            `json:"payload" gorm:"serializer:json;type:jsonb" swaggertype:"object"`
	Reason           string                     `json:"reason,omitempty"`
	Error            json.RawMessage            `json:"error,omitempty" gorm:"serializer:json" swaggertype:"object"`
	// ReconcileStatus is the reconciliation lifecycle status (epic #38, Part 6).
	// Set only for initiated reconciles (open at initiate); empty for every other
	// submission type/flow. Drives the staff-immutability guard and the
	// close/reopen/start-processing transitions.
	ReconcileStatus ReconcileLifecycleStatus `json:"reconcile_status,omitempty" gorm:"type:varchar(20)"`
	// ProcessedAt is the precise instant a CONSUMING submission's processing
	// completed (epic #38, Part 6, locked decision Q6). It is the authoritative
	// window bound for the Start-Processing drift re-check (a sibling consuming
	// submission with processed_at inside [snapshot_capture, now] is drift). Set
	// when processing completes; nil while pending/failed.
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
}

// MarshalErrors marshals errors to JSON
func MarshalErrors(errors []error) (json.RawMessage, error) {
	if len(errors) == 0 {
		return nil, nil
	}

	errorObjects := make([]interface{}, 0, len(errors))
	for _, err := range errors {
		if err == nil {
			continue
		}

		// Check if the error implements json.Marshaler
		if marshaler, ok := err.(json.Marshaler); ok {
			errorObjects = append(errorObjects, marshaler)
		} else {
			// Otherwise, marshal it to {"message": error.Error()}
			errorObjects = append(errorObjects, map[string]string{
				"message": err.Error(),
			})
		}
	}

	if len(errorObjects) == 0 {
		return nil, nil
	}

	return json.Marshal(errorObjects)
}
