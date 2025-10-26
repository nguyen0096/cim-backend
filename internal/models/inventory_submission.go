package models

import "encoding/json"

type InventorySubmissionStatus string

const (
	InventorySubmissionStatusPending   InventorySubmissionStatus = "pending"
	InventorySubmissionStatusFailed    InventorySubmissionStatus = "failed"
	InventorySubmissionStatusCompleted InventorySubmissionStatus = "completed"
)

type InventorySubmissionApprovalStatus string

const (
	InventorySubmissionApprovalStatusPending  InventorySubmissionApprovalStatus = "pending"
	InventorySubmissionApprovalStatusApproved InventorySubmissionApprovalStatus = "approved"
	InventorySubmissionApprovalStatusRejected InventorySubmissionApprovalStatus = "rejected"
)

type InventorySubmissionType string

const (
	InventorySubmissionTypeReconcile InventorySubmissionType = "reconcile"
	InventorySubmissionTypeDispose   InventorySubmissionType = "dispose"
	InventorySubmissionTypeTransfer  InventorySubmissionType = "transfer"
)

type InventorySubmissionAction string

const (
	InventorySubmissionActionApprove InventorySubmissionAction = "approve"
	InventorySubmissionActionReject  InventorySubmissionAction = "reject"
)

// InventorySubmission represents a pending inventory operation
type InventorySubmission struct {
	Base
	InventoryID      uint                              `json:"inventory_id"`
	Inventory        *Inventory                        `json:"inventory,omitempty" gorm:"foreignKey:InventoryID"`
	SubmissionType   InventorySubmissionType           `json:"submission_type" gorm:"not null"`
	ProcessingStatus InventorySubmissionStatus         `json:"processing_status" gorm:"default:pending"`
	ApprovalStatus   InventorySubmissionApprovalStatus `json:"approval_status" gorm:"default:pending"`
	Payload          json.RawMessage                   `json:"payload" gorm:"serializer:json"`
	Reason           string                            `json:"reason,omitempty"`
	Error            json.RawMessage                   `json:"error,omitempty" gorm:"serializer:json"`
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
