package dto

import (
	"cim-backend/internal/models"

	"github.com/shopspring/decimal"
)

var (
	_ models.IDGetter = (*QuantityItem)(nil)
)

// QuantityItem represents a single inventory item quantity
type QuantityItem struct {
	InventoryItemID uint             `json:"inventory_item_id" validate:"required"`
	Quantity        *decimal.Decimal `json:"quantity" validate:"required"`

	// PrevQuantity is previus stock state.
	// Only available for stateful submission like Reconcile.
	PrevQuantity decimal.Decimal `json:"prev_quantity,omitempty" validate:"required"`

	// Response fields

	InventoryItem   *models.InventoryItem `json:"inventory_item,omitempty" validate:"-"`
	ProductName     string                `json:"product_name,omitempty" validate:"-"`
	CurrentQuantity decimal.Decimal       `json:"current_quantity,omitempty" validate:"-"`
}

func (d QuantityItem) GetID() uint {
	return d.InventoryItemID
}

// ReconcileInventoryRequest represents the request for confirming inventory
type ReconcileInventoryRequest struct {
	InventoryID uint           `json:"inventory_id" validate:"required" param:"id"`
	Items       []QuantityItem `json:"items" validate:"required,min=1,dive"`
}

func (d ReconcileInventoryRequest) GetItemIDs() []uint {
	itemIDs := make([]uint, len(d.Items))
	for i, item := range d.Items {
		itemIDs[i] = item.InventoryItemID
	}
	return itemIDs
}

// InitiateReconcileRequest starts a reconciliation for an inventory (epic #38,
// Part 2). It creates a placeholder reconcile inventory_submissions row and
// captures one reconciliation_snapshots row per active inventory item, recording
// prev_quantity = that item's live quantity at initiate time. No counts are
// supplied here; staff enter counted quantities later via child items.
// InventoryID is intentionally NOT JSON-bindable (`json:"-"`): this endpoint is
// path-scoped (/inventories/:id/reconcile/initiate) and must take its scope
// solely from the path. The handler sets it from the `:id` path param after
// binding, so a request body cannot change which inventory is reconciled.
type InitiateReconcileRequest struct {
	InventoryID uint `json:"-" validate:"required"`
}

// DisposeInventoryRequest represents the request for disposing inventory items
type DisposeInventoryRequest struct {
	InventoryID uint           `json:"inventory_id" validate:"required" param:"id"`
	Items       []QuantityItem `json:"items" validate:"required,min=1,dive"`
}

func (d DisposeInventoryRequest) GetItemIDs() []uint {
	itemIDs := make([]uint, len(d.Items))
	for i, item := range d.Items {
		itemIDs[i] = item.InventoryItemID
	}
	return itemIDs
}

// TransferInventoryRequest represents the request for transferring inventory items between inventories
type TransferInventoryRequest struct {
	SourceInventoryID      uint           `json:"source_inventory_id" validate:"required"`
	DestinationInventoryID uint           `json:"destination_inventory_id" validate:"required"`
	Items                  []QuantityItem `json:"items" validate:"required,min=1,dive"`
}

func (d TransferInventoryRequest) GetItemIDs() []uint {
	itemIDs := make([]uint, len(d.Items))
	for i, item := range d.Items {
		itemIDs[i] = item.InventoryItemID
	}
	return itemIDs
}

// ReconciliationCountItem is one counted line in a staff reconciliation child
// item (epic #38, Part 4). It carries COUNTED quantities only — the baseline
// (prev_quantity) is the parent snapshot captured at initiate, NOT supplied by
// the client. Quantity must be non-negative and must not exceed the snapshot
// baseline for the item (the S2 "counted > snapshot is rejected" rule).
type ReconciliationCountItem struct {
	InventoryItemID uint `json:"inventory_item_id" validate:"required"`
	// Quantity is a pointer so an omitted `quantity` (nil) is distinguishable from
	// an explicit zero count. It deliberately carries NO validate:"required" tag:
	// the binding-layer validator would otherwise short-circuit a missing/null
	// quantity into a generic validation error before the service runs, hiding the
	// localized recon_item_missing_quantity domain error. Letting nil through to
	// the service's nil-check (validateCountsAgainstSnapshot) yields that localized
	// error instead. An explicit 0 still binds as a non-nil pointer and is a valid
	// count; the service validates the dereferenced value (non-negative, <=
	// snapshot baseline).
	Quantity *decimal.Decimal `json:"quantity"`
	// Label is an OPTIONAL free-text identifier for this count (e.g. "shelf",
	// "loading dock") so multiple counts of the SAME inventory_item_id across staff
	// child rows can be told apart in review/audit (issue #73). It is
	// representation-only: synthesis still sums by inventory_item_id and ignores the
	// label for the apply math. Max length 255 RUNES (app-validated;
	// utf8.RuneCountInString — Vietnamese is multibyte so a byte cap would reject
	// valid labels). The distinct-labels-at-most-one-blank rule is enforced at write
	// time across the live sibling rows of the same parent plus this payload — see
	// validateCountsAgainstSnapshot.
	Label string `json:"label,omitempty"`
}

// CreateReconciliationItemRequest creates a new staff child item under a parent
// initiated reconcile submission. SubmissionID is path-scoped (set from the
// `:id` path param after binding) and intentionally not JSON-bindable so a body
// cannot retarget which submission the item is filed under.
type CreateReconciliationItemRequest struct {
	SubmissionID uint `json:"-" validate:"required"`
	// Label is the optional ROW-level count-session identifier (issue #73). It
	// carries no validate:"required" tag: the row-label rule (required once the user
	// already has a 2nd live row; ≤255 runes; distinct per (submission,user)) is
	// enforced in the service so the localized domain errors surface instead of a
	// generic binding error.
	Label string                    `json:"label"`
	Items []ReconciliationCountItem `json:"items" validate:"required,min=1,dive"`
}

// UpdateReconciliationItemRequest replaces the counted payload of an existing
// child item. SubmissionID and ItemID are path-scoped (not JSON-bindable). Update
// is a full replace: the row Label and the entire Items payload are overwritten.
type UpdateReconciliationItemRequest struct {
	SubmissionID uint `json:"-" validate:"required"`
	ItemID       uint `json:"-" validate:"required"`
	// Label fully replaces the row's existing label on update (issue #73); see the
	// CreateReconciliationItemRequest.Label note on why it has no required tag.
	Label string                    `json:"label"`
	Items []ReconciliationCountItem `json:"items" validate:"required,min=1,dive"`
}

// DeleteReconciliationItemRequest soft-deletes a child item. Both ids are
// path-scoped.
type DeleteReconciliationItemRequest struct {
	SubmissionID uint `json:"-" validate:"required"`
	ItemID       uint `json:"-" validate:"required"`
}

// StartProcessingResult is the outcome of POST .../start-processing (epic #38,
// Part 6 redesign). On a clean apply, Submission is the finalized (processed)
// submission. On drift, DriftDetected is true and Warnings carries the
// warning-shaped payload (one line per consuming submission that processed during
// the reconcile window — locked decision Q8); the apply was rolled back and no
// inventory was mutated, so the admin can edit/reopen and restart processing.
type StartProcessingResult struct {
	Submission    *models.InventorySubmission `json:"submission,omitempty"`
	DriftDetected bool                        `json:"drift_detected"`
	Warnings      []string                    `json:"warnings,omitempty"`
}
