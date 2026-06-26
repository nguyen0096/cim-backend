package services

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
)

// These are pure unit tests of the synthesis core (epic #38, Part 5): summing
// counted quantities by inventory_item_id across child rows, attaching the
// snapshot baseline, computing the review label, and the anomaly handling for a
// stored aggregate that exceeds its baseline. The DB-backed end-to-end path
// (ListSubmissions over real child rows + soft-delete exclusion) is covered by the
// integration spec in test/reconciliation_synthesize_test.go.

// childRow builds a live child row carrying the legacy counts-only payload.
func childRow(id uint, status models.ReconciliationRequestItemStatus, lines ...reconItemPayloadLine) models.ReconciliationRequestItem {
	payload, _ := json.Marshal(reconItemPayload{Items: lines})
	row := models.ReconciliationRequestItem{Status: status, Payload: payload}
	row.ID = id
	return row
}

func line(itemID uint, qty string) reconItemPayloadLine {
	return reconItemPayloadLine{InventoryItemID: itemID, Quantity: decimal.RequireFromString(qty)}
}

func baselineMap(pairs map[uint]string) map[uint]decimal.Decimal {
	out := make(map[uint]decimal.Decimal, len(pairs))
	for id, v := range pairs {
		out[id] = decimal.RequireFromString(v)
	}
	return out
}

// findItem returns the synthesized line for an item id (items are ordered by id).
func findItem(items []dto.QuantityItem, id uint) (dto.QuantityItem, bool) {
	for _, it := range items {
		if it.InventoryItemID == id {
			return it, true
		}
	}
	return dto.QuantityItem{}, false
}

func TestSynthesizeReconcile_SumsByItemAcrossRowsAndItems(t *testing.T) {
	rows := []models.ReconciliationRequestItem{
		// item 1: 30 + 25 across two rows; item 2: 40 in row 1.
		childRow(1, models.ReconciliationRequestItemStatusInProgress, line(1, "30"), line(2, "40")),
		childRow(2, models.ReconciliationRequestItemStatusInProgress, line(1, "25")),
		// item 3 only in row 3.
		childRow(3, models.ReconciliationRequestItemStatusInProgress, line(3, "10")),
	}
	baselines := baselineMap(map[uint]string{1: "100", 2: "100", 3: "100"})

	syn, err := synthesizeReconcile(7, models.ReconcileLifecycleStatusOpen, rows, baselines)
	require.NoError(t, err)
	assert.Equal(t, uint(7), syn.Request.InventoryID)
	assert.Empty(t, syn.Anomalies)

	// Deterministic ascending order by item id.
	require.Len(t, syn.Request.Items, 3)
	assert.Equal(t, uint(1), syn.Request.Items[0].InventoryItemID)
	assert.Equal(t, uint(2), syn.Request.Items[1].InventoryItemID)
	assert.Equal(t, uint(3), syn.Request.Items[2].InventoryItemID)

	it1, _ := findItem(syn.Request.Items, 1)
	require.NotNil(t, it1.Quantity)
	assert.True(t, it1.Quantity.Equal(decimal.NewFromInt(55)), "item 1 = 30+25")
	assert.True(t, it1.PrevQuantity.Equal(decimal.NewFromInt(100)), "baseline attached as PrevQuantity")

	it2, _ := findItem(syn.Request.Items, 2)
	assert.True(t, it2.Quantity.Equal(decimal.NewFromInt(40)))
}

func TestSynthesizeReconcile_DecimalMath(t *testing.T) {
	rows := []models.ReconciliationRequestItem{
		childRow(1, models.ReconciliationRequestItemStatusInProgress, line(1, "10.25")),
		childRow(2, models.ReconciliationRequestItemStatusInProgress, line(1, "5.50")),
	}
	baselines := baselineMap(map[uint]string{1: "100"})

	syn, err := synthesizeReconcile(1, models.ReconcileLifecycleStatusOpen, rows, baselines)
	require.NoError(t, err)
	require.Len(t, syn.Request.Items, 1)
	assert.True(t, syn.Request.Items[0].Quantity.Equal(decimal.RequireFromString("15.75")),
		"decimal sum must be exact, got %s", syn.Request.Items[0].Quantity)
	assert.Empty(t, syn.Anomalies)
}

func TestSynthesizeReconcile_EmptyPayloadRowsAndNoRows(t *testing.T) {
	// A row with an empty payload contributes nothing.
	empty := models.ReconciliationRequestItem{Status: models.ReconciliationRequestItemStatusInProgress}
	empty.ID = 1
	syn, err := synthesizeReconcile(1, models.ReconcileLifecycleStatusOpen, []models.ReconciliationRequestItem{empty}, map[uint]decimal.Decimal{})
	require.NoError(t, err)
	assert.Empty(t, syn.Request.Items)

	// No rows at all -> empty items, in_progress label.
	syn, err = synthesizeReconcile(1, models.ReconcileLifecycleStatusOpen, nil, map[uint]decimal.Decimal{})
	require.NoError(t, err)
	assert.Empty(t, syn.Request.Items)
	assert.Equal(t, dto.ReconcileReviewLabelInProgress, syn.Label)
}

func TestSynthesizeReconcile_AggregateExceedsBaselineIsSurfacedAndCapped(t *testing.T) {
	// Two rows of 80 against baseline 100 sum to 160 (> baseline). The Part-4 write
	// guard should make this impossible; synthesis must still surface it as an
	// anomaly and cap the emitted counted at the baseline so a downstream consume
	// can never go negative.
	rows := []models.ReconciliationRequestItem{
		childRow(1, models.ReconciliationRequestItemStatusInProgress, line(1, "80")),
		childRow(2, models.ReconciliationRequestItemStatusInProgress, line(1, "80")),
	}
	baselines := baselineMap(map[uint]string{1: "100"})

	syn, err := synthesizeReconcile(1, models.ReconcileLifecycleStatusOpen, rows, baselines)
	require.NoError(t, err)
	require.Len(t, syn.Request.Items, 1)
	require.Len(t, syn.Anomalies, 1)
	assert.True(t, syn.Request.Items[0].Quantity.Equal(decimal.NewFromInt(100)),
		"emitted counted must be capped at baseline 100, got %s", syn.Request.Items[0].Quantity)
}

func TestSynthesizeReconcile_MissingBaselineIsSurfaced(t *testing.T) {
	rows := []models.ReconciliationRequestItem{
		childRow(1, models.ReconciliationRequestItemStatusInProgress, line(9, "5")),
	}
	syn, err := synthesizeReconcile(1, models.ReconcileLifecycleStatusOpen, rows, map[uint]decimal.Decimal{}) // no baseline for item 9
	require.NoError(t, err)
	require.Len(t, syn.Request.Items, 1)
	require.Len(t, syn.Anomalies, 1)
	assert.True(t, syn.Request.Items[0].PrevQuantity.Equal(decimal.Zero), "missing baseline falls back to zero")
	// The emitted counted must ALSO be clamped to the (zero) baseline: emitting a
	// positive count against a zero baseline would make a downstream
	// snapshot - counted go negative — the same FIFO-corruption the cap prevents.
	assert.True(t, syn.Request.Items[0].Quantity.Equal(decimal.Zero),
		"missing-baseline counted must be clamped to zero, got %s", syn.Request.Items[0].Quantity)
}

func TestComputeReviewLabel(t *testing.T) {
	// The label now mirrors the SUBMISSION lifecycle (Q1 collapse): open =>
	// in-progress; closed/processing/processed => ready-for-review.
	cases := []struct {
		name   string
		status models.ReconcileLifecycleStatus
		want   dto.ReconcileReviewLabel
	}{
		{"empty (legacy)", "", dto.ReconcileReviewLabelInProgress},
		{"open", models.ReconcileLifecycleStatusOpen, dto.ReconcileReviewLabelInProgress},
		{"closed", models.ReconcileLifecycleStatusClosed, dto.ReconcileReviewLabelReadyForReview},
		{"processing", models.ReconcileLifecycleStatusProcessing, dto.ReconcileReviewLabelReadyForReview},
		{"processed", models.ReconcileLifecycleStatusProcessed, dto.ReconcileReviewLabelReadyForReview},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, computeReviewLabel(tc.status))
		})
	}
}
