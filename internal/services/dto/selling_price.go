package dto

import "github.com/shopspring/decimal"

type CreateSellingPriceRequest struct {
	ProductID uint `json:"product_id" validate:"required"`
	// InventoryID must be null/omitted: only global prices are supported; a
	// non-null value is rejected with a 400.
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
	// Pointer to distinguish an explicit 0 (valid override) from an omitted/null
	// field (rejected).
	SellingPrice *decimal.Decimal `json:"selling_price" validate:"required"`
}

// BackfillSellingPriceRequest is the body for POST /selling-prices/:id/backfill.
// :id is the start price. EndEffectiveFrom is the exclusive end date the client
// previewed (null/omitted = open-ended) and acts as an optimistic-concurrency
// assertion: a mismatch with the server-resolved boundary is rejected with a 409.
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

// SellingPriceMassiveApplyingEntry describes one window a CRUD operation affects;
// informational (a preview/count).
type SellingPriceMassiveApplyingEntry struct {
	StartSellingPrice   SellingPriceRef  `json:"start_selling_price"`
	EndSellingPrice     *SellingPriceRef `json:"end_selling_price"` // nil if open-ended
	AffectedPOItemCount int64            `json:"affected_po_item_count"`
}

// SellingPriceMassiveApplying is the list of windows affected by a CRUD operation.
type SellingPriceMassiveApplying = []SellingPriceMassiveApplyingEntry
