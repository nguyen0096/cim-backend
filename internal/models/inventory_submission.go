package models

import "encoding/json"

type SubmissionProcessingStatus string

const (
	InventorySubmissionStatusPending   SubmissionProcessingStatus = "pending"
	InventorySubmissionStatusFailed    SubmissionProcessingStatus = "failed"
	InventorySubmissionStatusCompleted SubmissionProcessingStatus = "completed"
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
	Payload          json.RawMessage            `json:"payload" gorm:"serializer:json"`
	Reason           string                     `json:"reason,omitempty"`
	Error            json.RawMessage            `json:"error,omitempty" gorm:"serializer:json"`
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
