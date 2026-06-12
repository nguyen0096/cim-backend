package dto

import "github.com/shopspring/decimal"

type CreateSellingPriceRequest struct {
	ProductID uint `json:"product_id" validate:"required"`
	// InventoryID must be null/omitted: inventory-specific selling prices are not
	// supported yet (would require an end_date + FE timeline). Only global prices
	// (inventory_id NULL) are allowed; a non-null value is rejected with a 400.
	InventoryID   *uint           `json:"inventory_id"`
	Price         decimal.Decimal `json:"price" validate:"required"`
	EffectiveFrom string          `json:"effective_from" validate:"required"` // "2026-04-11" format
	Notes         string          `json:"notes"`
}

type UpdateSellingPriceRequest struct {
	Price         decimal.Decimal `json:"price" validate:"required"`
	EffectiveFrom string          `json:"effective_from" validate:"required"`
	Notes         string          `json:"notes"`
}

type UpdatePOItemSellingPriceRequest struct {
	// Pointer so we can distinguish an explicit 0 (valid: clears the implied
	// price to a real zero override) from an omitted/null field (rejected). The
	// handler does NOT call validate and the custom decimal validator treats "0"
	// as present, so nil-checking the pointer is the only reliable gate.
	SellingPrice *decimal.Decimal `json:"selling_price" validate:"required"`
}

// BackfillSellingPriceRequest is the body for POST /selling-prices/:id/backfill.
// :id is the START price (the link to apply); the server resolves the effective
// range from the current ledger. EndEffectiveFrom is the exclusive end DATE the
// client PREVIEWED (end_selling_price.effective_from from the massive_applying
// entry; null/omitted = previewed open-ended) and acts as an
// optimistic-concurrency assertion: if it no longer matches the server-resolved
// boundary date, the apply is rejected with a 409 and the client must re-fetch
// the preview. The DATE is asserted (not a boundary price id) because the
// applied set depends only on the exclusive end date: an id would pass even
// when the boundary price's date was edited after the preview (id and resolved
// end move together), silently applying a window the user never saw.
type BackfillSellingPriceRequest struct {
	EndEffectiveFrom *string `json:"end_effective_from"` // "YYYY-MM-DD"
}

// SellingPriceRef is a lightweight reference to a selling price in the ledger,
// used in the massive-apply preview payload.
type SellingPriceRef struct {
	ID            uint            `json:"id"`
	Price         decimal.Decimal `json:"price"`
	EffectiveFrom string          `json:"effective_from"` // "YYYY-MM-DD"
}

// SellingPriceMassiveApplyingEntry describes one window that a CRUD operation
// affects. The actual apply happens via the backfill endpoint keyed by
// StartSellingPrice.ID; this payload is INFORMATIONAL (a preview/count).
type SellingPriceMassiveApplyingEntry struct {
	StartSellingPrice   SellingPriceRef  `json:"start_selling_price"`
	EndSellingPrice     *SellingPriceRef `json:"end_selling_price"` // nil if open-ended
	AffectedPOItemCount int64            `json:"affected_po_item_count"`
}

// SellingPriceMassiveApplying is the list of windows affected by a CRUD operation.
type SellingPriceMassiveApplying = []SellingPriceMassiveApplyingEntry
