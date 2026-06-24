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
}

// CreateReconciliationItemRequest creates a new staff child item under a parent
// initiated reconcile submission. SubmissionID is path-scoped (set from the
// `:id` path param after binding) and intentionally not JSON-bindable so a body
// cannot retarget which submission the item is filed under.
type CreateReconciliationItemRequest struct {
	SubmissionID uint                      `json:"-" validate:"required"`
	Items        []ReconciliationCountItem `json:"items" validate:"required,min=1,dive"`
}

// UpdateReconciliationItemRequest replaces the counted payload of an existing
// child item. SubmissionID and ItemID are path-scoped (not JSON-bindable). A
// staff edit of an `approved` row resets it back to `in_progress` (escape hatch).
type UpdateReconciliationItemRequest struct {
	SubmissionID uint                      `json:"-" validate:"required"`
	ItemID       uint                      `json:"-" validate:"required"`
	Items        []ReconciliationCountItem `json:"items" validate:"required,min=1,dive"`
}

// SetReconciliationItemReadyRequest marks a child item ready or not_ready
// (status-only transition). Both ids are path-scoped.
type SetReconciliationItemReadyRequest struct {
	SubmissionID uint `json:"-" validate:"required"`
	ItemID       uint `json:"-" validate:"required"`
	// Ready true => in_progress -> ready; false => ready -> in_progress.
	Ready bool `json:"-"`
}

// DeleteReconciliationItemRequest soft-deletes a staff-owned child item. Both
// ids are path-scoped.
type DeleteReconciliationItemRequest struct {
	SubmissionID uint `json:"-" validate:"required"`
	ItemID       uint `json:"-" validate:"required"`
}
