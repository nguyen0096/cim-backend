package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// SubmissionDrop is the per-submission contribution to an item's total drop.
// It covers BOTH reconcile shrinkage and explicit disposals; SubmissionType
// distinguishes them (the dispose path skips the prev/effective/raw clamp math
// and uses ClampedDrop == the requested remove-N directly).
type SubmissionDrop struct {
	SubmissionID uint `json:"submission_id"`
	// SubmissionType is "reconcile" or "dispose".
	SubmissionType string          `json:"submission_type"`
	CreatedAt      time.Time       `json:"created_at"`
	PrevQuantity   decimal.Decimal `json:"prev_quantity"`
	ActualCount    decimal.Decimal `json:"actual_count"`
	// EffectivePrev is prev_quantity minus the quantity already consumed by
	// earlier submissions in the chain (effective_prev = prev - consumed_so_far).
	EffectivePrev decimal.Decimal `json:"effective_prev"`
	// RawDelta is effective_prev - actual_count (may be negative when the count
	// implies a stock increase).
	RawDelta decimal.Decimal `json:"raw_delta"`
	// ClampedDrop is max(0, RawDelta) — the quantity actually consumed.
	ClampedDrop decimal.Decimal `json:"clamped_drop"`
	// Clamped is true when RawDelta was <= 0 (an increase or no-op), so no drop
	// was synthesized for this submission/item.
	Clamped bool `json:"clamped"`
}

// ItemResolution summarizes the full chained resolution for one inventory item.
// TotalDrop / FinalStock account for BOTH reconcile shrinkage and disposals.
type ItemResolution struct {
	InventoryItemID uint             `json:"inventory_item_id"`
	ProductName     string           `json:"product_name"`
	StartStock      decimal.Decimal  `json:"start_stock"`
	Drops           []SubmissionDrop `json:"drops"`
	// TotalDrop is the sum of all reconcile drops AND dispose removals.
	TotalDrop decimal.Decimal `json:"total_drop"`
	// TotalDisposed is the portion of TotalDrop attributable to dispose
	// submissions (reconcile-only callers leave this zero).
	TotalDisposed decimal.Decimal `json:"total_disposed"`
	FinalStock    decimal.Decimal `json:"final_stock"`
}

// SyntheticSell describes one backdated sell transaction that the plan will (or
// did, on --apply) create against a specific source purchase batch.
type SyntheticSell struct {
	InventoryItemID     uint            `json:"inventory_item_id"`
	Quantity            decimal.Decimal `json:"quantity"`
	SourcePurchaseTxnID uint            `json:"source_purchase_txn_id"`
	COGSPrice           float64         `json:"cogs_price"`
	BackdatedDate       time.Time       `json:"backdated_date"`
}

// SyntheticDisposal describes one backdated disposal transaction that the plan
// will (or did, on --apply) create against a specific source purchase batch.
// Same shape as SyntheticSell but its TransactionType is Disposal, not Sell.
type SyntheticDisposal struct {
	InventoryItemID     uint            `json:"inventory_item_id"`
	Quantity            decimal.Decimal `json:"quantity"`
	SourcePurchaseTxnID uint            `json:"source_purchase_txn_id"`
	COGSPrice           float64         `json:"cogs_price"`
	BackdatedDate       time.Time       `json:"backdated_date"`
}

// ConsumedQuantityDelta records the increase in consumed_quantity applied to a
// source purchase transaction.
type ConsumedQuantityDelta struct {
	PurchaseTxnID uint            `json:"purchase_txn_id"`
	Delta         decimal.Decimal `json:"delta"`
}

// ResolutionPlan is the full, faithful preview of what --apply will persist.
// Preview (default) builds this and persists nothing; --apply builds the
// identical plan via the same code path and persists it in one transaction.
type ResolutionPlan struct {
	InventoryID         uint                    `json:"inventory_id"`
	SubmissionIDs       []uint                  `json:"submission_ids"`
	Applied             bool                    `json:"applied"`
	Items               []ItemResolution        `json:"items"`
	Sells               []SyntheticSell         `json:"sells"`
	Disposals           []SyntheticDisposal     `json:"disposals"`
	ConsumedDeltas      []ConsumedQuantityDelta `json:"consumed_quantity_deltas"`
	TotalSells          int                     `json:"total_sells"`
	TotalDisposals      int                     `json:"total_disposals"`
	ClampedRowCount     int                     `json:"clamped_row_count"`
	TotalClampedRowList []ClampedRowRef         `json:"clamped_rows"`
}

// ClampedRowRef points at a (submission, item) pair whose raw delta was clamped
// to zero (the count implied an increase / no-op).
type ClampedRowRef struct {
	SubmissionID    uint            `json:"submission_id"`
	InventoryItemID uint            `json:"inventory_item_id"`
	RawDelta        decimal.Decimal `json:"raw_delta"`
}
