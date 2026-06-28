package dto

import (
	"cim-backend/internal/models"
	"encoding/json"

	"github.com/shopspring/decimal"
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
	// Breakdown carries the per-(item, label) contributions that were summed into
	// Request (issue #73). The apply math uses only the summed Request totals; this
	// is review/audit-only so an admin can see each labeled count behind a total
	// (e.g. milk "shelf" 30 + "dock" 25 -> total 55) rather than just the sum.
	Breakdown []ReconcileItemBreakdown
}

// ReconcileItemBreakdown is the review-only, per-(inventory_item, label) view of
// the counts that were summed into one synthesized line (issue #73). One entry per
// distinct label for an item (blank label allowed for the single/first count);
// Quantity is the summed counted quantity contributed under that label across the
// live staff child rows. Presentation-only — never feeds the apply math.
type ReconcileItemBreakdown struct {
	InventoryItemID uint            `json:"inventory_item_id"`
	Label           string          `json:"label"`
	Quantity        decimal.Decimal `json:"quantity"`
	// ProductName is the resolved product name for InventoryItemID, populated the
	// same way QuantityItem.ProductName is (inventory_item -> product join), so the
	// review screen can label each breakdown row without a second lookup (FE #42).
	// Presentation-only; omitted when the product can't be resolved.
	ProductName string `json:"product_name,omitempty"`
}

// ReconciliationItemLine is one count line inside a reconciliation row response
// (issue #73 / FE contract cim-ui #42): the lean {inventory_item_id, quantity,
// label} shape the FE consumes, flattened out of the row's JSONB payload. Quantity
// is a decimal serialized as a string (shopspring default).
type ReconciliationItemLine struct {
	InventoryItemID uint            `json:"inventory_item_id"`
	Quantity        decimal.Decimal `json:"quantity"`
	Label           string          `json:"label"`
}

// ReconciliationItemResponse is the row (count-session) response shape returned by
// Create / Update / List of reconciliation items (issue #73 / FE contract cim-ui
// #42). It surfaces the ROW-level Label and flattens the JSONB payload into Items,
// rather than leaking the raw model (payload bytes + the submission association).
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
	// ReviewLabel is populated only for ACTIVE reconcile submissions (initiated via
	// the new flow, not yet applied); it is empty for every other submission. It is
	// derived from the live child rows and is presentation-only (not persisted).
	ReviewLabel ReconcileReviewLabel `json:"review_label,omitempty"`
	// CountBreakdown is populated only for ACTIVE reconcile submissions: the
	// per-(inventory_item, label) contributions behind each summed item line (issue
	// #73), so the review screen can show each labeled count, not just the total. It
	// is presentation-only and derived from the live child rows.
	CountBreakdown []ReconcileItemBreakdown `json:"count_breakdown,omitempty"`
	// ReconcileStatus is the submission-level reconciliation lifecycle status
	// (open/closed/processing/processed). It is set only for initiated reconciles
	// and empty for every other submission type/flow; the FE uses it to detect an
	// OPEN reconciliation and drive the role x status editability matrix (FE #42).
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
