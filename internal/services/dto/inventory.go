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

// InitiateReconcileRequest starts a reconciliation for an inventory. InventoryID
// is path-scoped (set from the :id path param), not JSON-bindable, so a body
// cannot change which inventory is reconciled.
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
// item. It carries counted quantities only; the baseline is the parent snapshot
// captured at initiate.
type ReconciliationCountItem struct {
	InventoryItemID uint `json:"inventory_item_id" validate:"required"`
	// Quantity is a pointer so an omitted quantity (nil) is distinguishable from an
	// explicit zero. It has no validate:"required" tag so nil reaches the service
	// and surfaces the localized domain error instead of a generic binding error.
	Quantity *decimal.Decimal `json:"quantity"`
	// Label is an optional free-text identifier so multiple counts of the same item
	// can be told apart. Max 255 runes; presentation-only (synthesis sums by
	// inventory_item_id).
	Label string `json:"label,omitempty"`
}

// CreateReconciliationItemRequest creates a staff child item under a parent
// reconcile submission. SubmissionID is path-scoped (not JSON-bindable).
type CreateReconciliationItemRequest struct {
	SubmissionID uint `json:"-" validate:"required"`
	// Label is the optional row-level session identifier (no required tag; validated
	// in the service so localized errors surface). A blank label is allowed as the
	// owner's single unlabelled session; non-empty labels must be distinct among the
	// owner's sessions in this reconciliation (max 255 runes).
	Label string                    `json:"label"`
	Items []ReconciliationCountItem `json:"items" validate:"required,min=1,dive"`
}

// UpdateReconciliationItemRequest replaces the counted payload of an existing
// child item. SubmissionID and ItemID are path-scoped; update is a full replace.
type UpdateReconciliationItemRequest struct {
	SubmissionID uint `json:"-" validate:"required"`
	ItemID       uint `json:"-" validate:"required"`
	// Label fully replaces the row's existing label on update. Blank is allowed as the
	// owner's single unlabelled session; non-empty labels must be distinct among the
	// owner's other sessions in this reconciliation (max 255 runes).
	Label string                    `json:"label"`
	Items []ReconciliationCountItem `json:"items" validate:"required,min=1,dive"`
}

// DeleteReconciliationItemRequest soft-deletes a child item. Both ids are
// path-scoped.
type DeleteReconciliationItemRequest struct {
	SubmissionID uint `json:"-" validate:"required"`
	ItemID       uint `json:"-" validate:"required"`
}

// SetReconciliationItemReadinessRequest toggles a staff count session's
// readiness. SubmissionID and ItemID are path-scoped (not JSON-bindable).
type SetReconciliationItemReadinessRequest struct {
	SubmissionID uint   `json:"-" validate:"required"`
	ItemID       uint   `json:"-" validate:"required"`
	Status       string `json:"status" validate:"required,oneof=in_progress ready_for_review"`
}

// StartProcessingResult is the outcome of start-processing. On a clean apply,
// Submission is the finalized submission. On drift, DriftDetected is true and
// Warnings lists the consuming submissions that processed during the window; the
// apply was rolled back and no inventory was mutated.
type StartProcessingResult struct {
	Submission    *models.InventorySubmission `json:"submission,omitempty"`
	DriftDetected bool                        `json:"drift_detected"`
	Warnings      []string                    `json:"warnings,omitempty"`
}

// CloseReconciliationResult is the outcome of close. Warnings is a non-blocking
// list of sessions still in_progress at close time.
type CloseReconciliationResult struct {
	Submission *models.InventorySubmission `json:"submission,omitempty"`
	Warnings   []string                    `json:"warnings,omitempty"`
}
