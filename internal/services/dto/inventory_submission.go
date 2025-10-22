package dto

import (
	"cim-backend/internal/models"
)

// PendingSubmissionResponse represents a simplified pending submission
type PendingSubmissionResponse struct {
	ID             uint                                     `json:"id"`
	InventoryID    uint                                     `json:"inventory_id"`
	Inventory      *models.Inventory                        `json:"inventory,omitempty"`
	SubmissionType models.InventorySubmissionType           `json:"submission_type"`
	Status         models.InventorySubmissionStatus         `json:"processing_status"`
	ApprovalStatus models.InventorySubmissionApprovalStatus `json:"approval_status"`
	Items          []PendingSubmissionItemSummary           `json:"items"`
	Reason         string                                   `json:"reason,omitempty"`
	CreatedBy      string                                   `json:"created_by"`
	CreatedAt      string                                   `json:"created_at"`
	UpdatedBy      string                                   `json:"updated_by"`
	UpdatedAt      string                                   `json:"updated_at"`
}

// PendingSubmissionItemSummary represents a simplified item in pending submission
type PendingSubmissionItemSummary struct {
	ProductName string `json:"product_name"`
	Quantity    int    `json:"quantity"`
}

// ProcessSubmissionRequest represents a request to approve or reject a submission
type ProcessSubmissionRequest struct {
	SubmissionID uint   `json:"submission_id" validate:"required" param:"id"`
	Action       string `json:"action" validate:"required,oneof=approve reject"`
	Reason       string `json:"reason,omitempty"`
}
