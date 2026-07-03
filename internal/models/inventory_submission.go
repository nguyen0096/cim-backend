package models

import (
	"encoding/json"
	"time"
)

// ReconcileLifecycleStatus is the submission-level reconciliation lifecycle.
// Only initiated reconciles carry it; every other submission leaves it empty.
//
//	open       -> staff may create/edit/delete their child count rows.
//	closed     -> staff locked out; admin/accountant may still edit child rows.
//	processing -> transient, during the Start-Processing apply.
//	processed  -> terminal: applied (stock mutated), immutable.
//	canceled   -> terminal: abandoned with no inventory mutation.
type ReconcileLifecycleStatus string

const (
	ReconcileLifecycleStatusOpen       ReconcileLifecycleStatus = "open"
	ReconcileLifecycleStatusClosed     ReconcileLifecycleStatus = "closed"
	ReconcileLifecycleStatusProcessing ReconcileLifecycleStatus = "processing"
	ReconcileLifecycleStatusProcessed  ReconcileLifecycleStatus = "processed"
	ReconcileLifecycleStatusCanceled   ReconcileLifecycleStatus = "canceled"
)

// IsValid reports whether s is a recognized reconcile lifecycle status.
func (s ReconcileLifecycleStatus) IsValid() bool {
	switch s {
	case ReconcileLifecycleStatusOpen,
		ReconcileLifecycleStatusClosed,
		ReconcileLifecycleStatusProcessing,
		ReconcileLifecycleStatusProcessed,
		ReconcileLifecycleStatusCanceled:
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
	// ReconcileStatus is the reconciliation lifecycle status; set only for
	// initiated reconciles, empty otherwise.
	ReconcileStatus ReconcileLifecycleStatus `json:"reconcile_status,omitempty" gorm:"type:varchar(20)"`
	// ProcessedAt is the instant a consuming submission's processing completed;
	// nil while pending/failed.
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
}

// IsActiveReconcile reports whether s is a reconcile not yet start-processed.
func (s InventorySubmission) IsActiveReconcile() bool {
	return s.SubmissionType == InventorySubmissionTypeReconcile &&
		s.ProcessingStatus == InventorySubmissionStatusPending &&
		(s.ReconcileStatus == ReconcileLifecycleStatusOpen ||
			s.ReconcileStatus == ReconcileLifecycleStatusClosed)
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

		if marshaler, ok := err.(json.Marshaler); ok {
			errorObjects = append(errorObjects, marshaler)
		} else {
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
