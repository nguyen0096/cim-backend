package dto

import (
	"cim-backend/internal/models"
	"encoding/json"

	"github.com/shopspring/decimal"
)

// ReconcileReviewLabel is the admin-facing progress label for an active
// reconciliation, aggregated on read from per-session readiness.
type ReconcileReviewLabel string

const (
	ReconcileReviewLabelInProgress     ReconcileReviewLabel = "in_progress"
	ReconcileReviewLabelReadyForReview ReconcileReviewLabel = "ready_for_review"
)

// SynthesizedReconcile folds the live staff child rows of a reconcile into the
// legacy ReconcileInventoryRequest shape the apply path consumes. Request has one
// line per inventory_item_id with the summed counted quantity and snapshot
// baseline; Anomalies lists any data-correctness oddity surfaced.
type SynthesizedReconcile struct {
	Request   ReconcileInventoryRequest
	Label     ReconcileReviewLabel
	Anomalies []string
	// Breakdown carries the session-grained contributions behind Request: one
	// entry per (count session, inventory_item, count-label). Review/audit-only,
	// not used by the apply math.
	Breakdown []ReconcileItemBreakdown
}

// ReconcileItemBreakdown is the review-only, session-grained view of one count
// contribution behind a synthesized line: a single (inventory_item, creator,
// session-label, count-label) quantity plus the session's provenance.
// (creator + session-label) uniquely identifies a session (per-owner session
// labels are distinct). Presentation-only.
type ReconcileItemBreakdown struct {
	InventoryItemID uint            `json:"inventory_item_id"`
	Label           string          `json:"label"`
	Quantity        decimal.Decimal `json:"quantity"`
	// ProductName is the resolved product name for InventoryItemID;
	// presentation-only, omitted when unresolved.
	ProductName string `json:"product_name,omitempty"`
	// SessionLabel is the row-level count-session label.
	SessionLabel string `json:"session_label,omitempty"`
	// CreatedBy / CreatedAt are the session creator and creation timestamp.
	CreatedBy string `json:"created_by,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// ReconciliationItemLine is one count line inside a reconciliation row response
// ({inventory_item_id, quantity, label}), flattened out of the row's JSONB payload.
type ReconciliationItemLine struct {
	InventoryItemID uint            `json:"inventory_item_id"`
	Quantity        decimal.Decimal `json:"quantity"`
	Label           string          `json:"label"`
}

// ReconciliationItemResponse is the row (count-session) response returned by
// Create/Update/List of reconciliation items. It surfaces the row-level Label and
// flattens the JSONB payload into Items.
type ReconciliationItemResponse struct {
	ID           uint                     `json:"id"`
	SubmissionID uint                     `json:"submission_id"`
	Label        string                   `json:"label"`
	Status       string                   `json:"status"`
	Items        []ReconciliationItemLine `json:"items"`
	CreatedBy    string                   `json:"created_by"`
	CreatedAt    string                   `json:"created_at"`
	UpdatedBy    string                   `json:"updated_by"`
	UpdatedAt    string                   `json:"updated_at"`
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
	Errors                 json.RawMessage                   `json:"error,omitempty" swaggertype:"object"`
	Warnings               []string                          `json:"warnings,omitempty"`
	// ReviewLabel is set only for active reconcile submissions; aggregated on read
	// from per-session readiness. Empty for every other submission.
	ReviewLabel ReconcileReviewLabel `json:"review_label,omitempty"`
	// CountBreakdown is populated only for active reconcile submissions: the
	// per-(inventory_item, label) contributions behind each summed item line.
	// Presentation-only, derived from the live child rows.
	CountBreakdown []ReconcileItemBreakdown `json:"count_breakdown,omitempty"`
	// ReconcileStatus is the submission-level reconciliation lifecycle status; set
	// only for initiated reconciles, empty otherwise.
	ReconcileStatus models.ReconcileLifecycleStatus `json:"reconcile_status,omitempty"`
	CreatedBy       string                          `json:"created_by"`
	CreatedAt       string                          `json:"created_at"`
	UpdatedBy       string                          `json:"updated_by"`
	UpdatedAt       string                          `json:"updated_at"`
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
