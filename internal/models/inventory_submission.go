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
}
