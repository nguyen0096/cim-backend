package services

import (
	"context"
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

// labeledLine builds a count line carrying an issue-#73 label.
func labeledLine(itemID uint, qty, label string) reconItemPayloadLine {
	return reconItemPayloadLine{InventoryItemID: itemID, Quantity: decimal.RequireFromString(qty), Label: label}
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

	syn, err := synthesizeReconcile(7, rows, baselines, nil)
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

func TestSynthesizeReconcile_LabelBreakdownPerItemLabel(t *testing.T) {
	// Issue #73: synthesis sums by inventory_item_id (label is representation-only),
	// AND surfaces a per-(item, label) breakdown so review can show each labeled
	// count behind a total. Item 1 is counted under "shelf" (30) + "dock" (25) across
	// two rows -> total 55, two breakdown lines. Item 2 has a single blank-label count.
	rows := []models.ReconciliationRequestItem{
		childRow(1, models.ReconciliationRequestItemStatusInProgress, labeledLine(1, "30", "shelf"), labeledLine(2, "40", "")),
		childRow(2, models.ReconciliationRequestItemStatusInProgress, labeledLine(1, "25", "dock")),
	}
	baselines := baselineMap(map[uint]string{1: "100", 2: "100"})

	syn, err := synthesizeReconcile(7, rows, baselines, nil)
	require.NoError(t, err)
	assert.Empty(t, syn.Anomalies)

	// Totals are summed by item regardless of label (apply math ignores the label).
	it1, _ := findItem(syn.Request.Items, 1)
	require.NotNil(t, it1.Quantity)
	assert.True(t, it1.Quantity.Equal(decimal.NewFromInt(55)), "item 1 total = shelf 30 + dock 25")

	// Breakdown: item 1 -> shelf 30, dock 25 (first-seen label order); item 2 -> blank 40.
	require.Len(t, syn.Breakdown, 3)
	assert.Equal(t, dto.ReconcileItemBreakdown{InventoryItemID: 1, Label: "shelf", Quantity: decimal.NewFromInt(30)}, syn.Breakdown[0])
	assert.Equal(t, dto.ReconcileItemBreakdown{InventoryItemID: 1, Label: "dock", Quantity: decimal.NewFromInt(25)}, syn.Breakdown[1])
	assert.Equal(t, uint(2), syn.Breakdown[2].InventoryItemID)
	assert.Equal(t, "", syn.Breakdown[2].Label)
	assert.True(t, syn.Breakdown[2].Quantity.Equal(decimal.NewFromInt(40)))
}

func TestSynthesizeReconcile_SameLabelAcrossRowsSummedInBreakdown(t *testing.T) {
	// Two rows contribute to item 1 under the SAME label "shelf" (e.g. an admin
	// review edit + a staff row): the breakdown collapses them into one line summing
	// the quantity (issue #73 breakdown is keyed per (item, label)).
	rows := []models.ReconciliationRequestItem{
		childRow(1, models.ReconciliationRequestItemStatusInProgress, labeledLine(1, "30", "shelf")),
		childRow(2, models.ReconciliationRequestItemStatusInProgress, labeledLine(1, "20", "shelf")),
	}
	baselines := baselineMap(map[uint]string{1: "100"})

	syn, err := synthesizeReconcile(7, rows, baselines, nil)
	require.NoError(t, err)
	require.Len(t, syn.Breakdown, 1)
	assert.Equal(t, dto.ReconcileItemBreakdown{InventoryItemID: 1, Label: "shelf", Quantity: decimal.NewFromInt(50)}, syn.Breakdown[0])
}

func TestSynthesizeReconcile_DecimalMath(t *testing.T) {
	rows := []models.ReconciliationRequestItem{
		childRow(1, models.ReconciliationRequestItemStatusInProgress, line(1, "10.25")),
		childRow(2, models.ReconciliationRequestItemStatusInProgress, line(1, "5.50")),
	}
	baselines := baselineMap(map[uint]string{1: "100"})

	syn, err := synthesizeReconcile(1, rows, baselines, nil)
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
	syn, err := synthesizeReconcile(1, []models.ReconciliationRequestItem{empty}, map[uint]decimal.Decimal{}, nil)
	require.NoError(t, err)
	assert.Empty(t, syn.Request.Items)

	// No rows at all -> empty items, in_progress label.
	syn, err = synthesizeReconcile(1, nil, map[uint]decimal.Decimal{}, nil)
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

	syn, err := synthesizeReconcile(1, rows, baselines, nil)
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
	syn, err := synthesizeReconcile(1, rows, map[uint]decimal.Decimal{}, nil) // no baseline for item 9
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

func TestSynthesizeReconcile_LabelAggregatesSessionReadiness(t *testing.T) {
	baselines := baselineMap(map[uint]string{1: "100"})

	t.Run("all live sessions ready => ready_for_review", func(t *testing.T) {
		rows := []models.ReconciliationRequestItem{
			childRow(1, models.ReconciliationRequestItemStatusReadyForReview, line(1, "10")),
			childRow(2, models.ReconciliationRequestItemStatusReadyForReview, line(1, "20")),
		}
		syn, err := synthesizeReconcile(1, rows, baselines, nil)
		require.NoError(t, err)
		assert.Equal(t, dto.ReconcileReviewLabelReadyForReview, syn.Label)
	})

	t.Run("one session still in_progress => in_progress", func(t *testing.T) {
		rows := []models.ReconciliationRequestItem{
			childRow(1, models.ReconciliationRequestItemStatusReadyForReview, line(1, "10")),
			childRow(2, models.ReconciliationRequestItemStatusInProgress, line(1, "20")),
		}
		syn, err := synthesizeReconcile(1, rows, baselines, nil)
		require.NoError(t, err)
		assert.Equal(t, dto.ReconcileReviewLabelInProgress, syn.Label)
	})

	t.Run("zero live rows => in_progress (nothing to review)", func(t *testing.T) {
		syn, err := synthesizeReconcile(1, nil, map[uint]decimal.Decimal{}, nil)
		require.NoError(t, err)
		assert.Equal(t, dto.ReconcileReviewLabelInProgress, syn.Label)
	})

	t.Run("a ready session with an empty payload still counts toward readiness", func(t *testing.T) {
		// A live ready_for_review row contributes no counts but must still be honored
		// by the aggregate (it is a live session marked done).
		empty := models.ReconciliationRequestItem{Status: models.ReconciliationRequestItemStatusReadyForReview}
		empty.ID = 1
		syn, err := synthesizeReconcile(1, []models.ReconciliationRequestItem{empty}, map[uint]decimal.Decimal{}, nil)
		require.NoError(t, err)
		assert.Empty(t, syn.Request.Items)
		assert.Equal(t, dto.ReconcileReviewLabelReadyForReview, syn.Label)
	})
}

func TestAggregateReviewLabel_ExcludesManagerOwnedSessions(t *testing.T) {
	t.Run("manager in_progress session does not hold the submission in_progress", func(t *testing.T) {
		rows := []models.ReconciliationRequestItem{
			childRow(1, models.ReconciliationRequestItemStatusInProgress, line(1, "10")),    // manager
			childRow(2, models.ReconciliationRequestItemStatusReadyForReview, line(1, "20")), // staff, ready
		}
		assert.Equal(t, dto.ReconcileReviewLabelReadyForReview, aggregateReviewLabel(rows, map[uint]bool{1: true}))
	})

	t.Run("a staff session still in_progress drives in_progress", func(t *testing.T) {
		rows := []models.ReconciliationRequestItem{
			childRow(1, models.ReconciliationRequestItemStatusReadyForReview, line(1, "10")), // manager
			childRow(2, models.ReconciliationRequestItemStatusInProgress, line(1, "20")),     // staff, not ready
		}
		assert.Equal(t, dto.ReconcileReviewLabelInProgress, aggregateReviewLabel(rows, map[uint]bool{1: true}))
	})

	t.Run("only a manager session => in_progress (no staff session to review)", func(t *testing.T) {
		rows := []models.ReconciliationRequestItem{
			childRow(1, models.ReconciliationRequestItemStatusReadyForReview, line(1, "10")),
		}
		assert.Equal(t, dto.ReconcileReviewLabelInProgress, aggregateReviewLabel(rows, map[uint]bool{1: true}))
	})
}

func TestBuildNotReadySessionWarnings_ExcludesManagerOwnedSessions(t *testing.T) {
	rows := []models.ReconciliationRequestItem{
		childRow(1, models.ReconciliationRequestItemStatusInProgress, line(1, "10")),    // manager, not ready
		childRow(2, models.ReconciliationRequestItemStatusInProgress, line(1, "20")),    // staff, not ready
		childRow(3, models.ReconciliationRequestItemStatusReadyForReview, line(1, "5")), // staff, ready
	}
	warnings := buildNotReadySessionWarnings(context.Background(), rows, map[uint]bool{1: true})
	require.Len(t, warnings, 1, "only the staff in_progress session warns; manager session excluded")

	onlyManager := []models.ReconciliationRequestItem{
		childRow(1, models.ReconciliationRequestItemStatusInProgress, line(1, "10")),
	}
	assert.Nil(t, buildNotReadySessionWarnings(context.Background(), onlyManager, map[uint]bool{1: true}))
}
