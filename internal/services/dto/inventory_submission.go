package dto

import (
	"cim-backend/internal/models"
	"encoding/json"
)

// ReconcileReviewLabel is the admin-facing progress label for an ACTIVE
// (initiated, not-yet-applied) reconciliation, derived from the child rows
// (epic #38, Part 5 / S4). It is presentation-only and computed on read; it is
// not persisted.
type ReconcileReviewLabel string

const (
	// ReconcileReviewLabelInProgress means at least one live child row is not yet
	// past review (some row is still in_progress / ready), so staff entry is still
	// settling.
	ReconcileReviewLabelInProgress ReconcileReviewLabel = "in_progress"
	// ReconcileReviewLabelReadyForReview means every live child row has reached
	// `ready` or beyond (ready/approved/applied) — the whole reconcile is ready for
	// the admin to review. An active reconcile with NO live child rows is treated
	// as in_progress (nothing to review yet), never ready.
	ReconcileReviewLabelReadyForReview ReconcileReviewLabel = "ready_for_review"
)

// SynthesizedReconcile is the read-only result of folding the live staff child
// rows of an initiated reconcile into the legacy ReconcileInventoryRequest shape
// the apply path consumes (epic #38, Part 5). Request carries one line per
// inventory_item_id with the summed counted quantity (Quantity) and the snapshot
// baseline (PrevQuantity); Label is the derived review progress; Anomalies lists
// any data-correctness oddity surfaced rather than silently corrupted (e.g. a
// stored aggregate that somehow exceeds the snapshot baseline — the Part-4
// write-time guard should make this impossible, but synthesis surfaces it instead
// of trusting the invariant blindly).
type SynthesizedReconcile struct {
	Request   ReconcileInventoryRequest
	Label     ReconcileReviewLabel
	Anomalies []string
}

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
	// ReviewLabel is populated only for ACTIVE reconcile submissions (initiated via
	// the new flow, not yet applied); it is empty for every other submission. It is
	// derived from the live child rows and is presentation-only (not persisted).
	ReviewLabel ReconcileReviewLabel `json:"review_label,omitempty"`
	CreatedBy   string               `json:"created_by"`
	CreatedAt   string               `json:"created_at"`
	UpdatedBy   string               `json:"updated_by"`
	UpdatedAt   string               `json:"updated_at"`
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

type SubmissionSortField string

const (
	SubmissionSortFieldUpdatedAt SubmissionSortField = "updated_at"
)

func GetSubmissionSortFields() []string {
	return []string{
		string(SubmissionSortFieldUpdatedAt),
	}
}
