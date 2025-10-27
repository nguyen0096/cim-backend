package dto

import (
	"cim-backend/internal/models"
	"encoding/json"
)

// SubmissionResponse represents a simplified pending submission
type SubmissionResponse struct {
	ID                     uint                              `json:"id"`
	InventoryID            uint                              `json:"inventory_id"`
	DestinationInventoryID uint                              `json:"destination_inventory_id,omitempty"`
	Inventory              *models.Inventory                 `json:"inventory,omitempty"`
	SubmissionType         models.SubmissionType             `json:"submission_type"`
	Status                 models.SubmissionProcessingStatus `json:"processing_status"`
	ApprovalStatus         models.SubmissionApprovalStatus   `json:"approval_status"`
	Items                  []QuantityItem                    `json:"items"`
	Reason                 string                            `json:"reason,omitempty"`
	Errors                 json.RawMessage                   `json:"error,omitempty"`
	Warnings               []string                          `json:"warnings,omitempty"`
	CreatedBy              string                            `json:"created_by"`
	CreatedAt              string                            `json:"created_at"`
	UpdatedBy              string                            `json:"updated_by"`
	UpdatedAt              string                            `json:"updated_at"`
}

// SubmissionApprovalRequest represents a request to approve or reject a submission
type SubmissionApprovalRequest struct {
	SubmissionID uint   `json:"submission_id" validate:"required" param:"id"`
	Action       string `json:"action" validate:"required,oneof=approve reject"`
	Reason       string `json:"reason,omitempty"`
}

// UpdateSubmissionRequest represents a request to update submission items
type UpdateSubmissionRequest struct {
	SubmissionID uint           `json:"submission_id" validate:"required" param:"id"`
	Items        []QuantityItem `json:"items" validate:"required,min=1,dive"`
}
