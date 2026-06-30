package scenario

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestCountedQuantityStaysWithinBaseline(t *testing.T) {
	cases := []struct {
		baseline int64
		want     int64
	}{
		{0, 0},
		{1, 0},  // 0.6 floored
		{10, 6}, // 0.6 * 10
		{100, 60},
		{7, 4}, // 4.2 floored
	}
	for _, c := range cases {
		// Use an explicit 0.6 here (independent of the countFraction const) to
		// pin the floor/clamp math regardless of the configured fraction.
		got := countedQuantity(decimal.NewFromInt(c.baseline), 0.6)
		if !got.Equal(decimal.NewFromInt(c.want)) {
			t.Errorf("countedQuantity(%d) = %s, want %d", c.baseline, got, c.want)
		}
		// The S2 invariant: a count must never exceed its snapshot baseline.
		if got.GreaterThan(decimal.NewFromInt(c.baseline)) {
			t.Errorf("countedQuantity(%d) = %s exceeds baseline", c.baseline, got)
		}
		if got.IsNegative() {
			t.Errorf("countedQuantity(%d) = %s is negative", c.baseline, got)
		}
	}
}

func TestCountedQuantityNegativeBaseline(t *testing.T) {
	if got := countedQuantity(decimal.NewFromInt(-5), 0.6); !got.IsZero() {
		t.Errorf("countedQuantity(-5) = %s, want 0", got)
	}
}

// Both the initial-count fraction and the adjust fraction must keep each line at
// or below its snapshot baseline for a range of baselines.
func TestCountFractionsWithinBaseline(t *testing.T) {
	for _, fraction := range []float64{countFraction, adjustFraction} {
		for _, b := range []int64{0, 1, 5, 33, 100, 1000} {
			baseline := decimal.NewFromInt(b)
			got := countedQuantity(baseline, fraction)
			if got.GreaterThan(baseline) {
				t.Errorf("fraction %.2f, baseline %d: counted %s exceeds baseline", fraction, b, got)
			}
			if got.IsNegative() {
				t.Errorf("fraction %.2f, baseline %d: counted %s is negative", fraction, b, got)
			}
		}
	}
}

// The reopen->adjust path replaces the existing row in full (it does NOT add a
// second competing row). So the per-item TOTAL across live rows equals just the
// updated row's counts — which must stay <= the snapshot baseline (the service's
// aggregate guard). This guards against a regression to the additive second-row
// approach that summed to >100% of baseline.
func TestAdjustReplacesRowAggregateWithinBaseline(t *testing.T) {
	items := []inventoryItem{
		{ID: 1, Quantity: decimal.NewFromInt(100)},
		{ID: 2, Quantity: decimal.NewFromInt(40)},
	}

	initial := countItems(items, "shelf-1", countFraction)
	adjusted := countItems(items, "dock-2", adjustFraction)

	if len(initial) != len(items) || len(adjusted) != len(items) {
		t.Fatalf("expected one count line per item")
	}

	for i, it := range items {
		// On the update path the row is REPLACED, so the live aggregate for the
		// item is the adjusted row's count alone — assert it is within baseline.
		if adjusted[i].Quantity == nil {
			t.Fatalf("item %d: adjusted quantity is nil", it.ID)
		}
		if adjusted[i].Quantity.GreaterThan(it.Quantity) {
			t.Errorf("item %d: adjusted aggregate %s exceeds baseline %s", it.ID, adjusted[i].Quantity, it.Quantity)
		}
		// Sanity: had we instead ADDED the adjust row on top of the initial one
		// (the rejected approach), the aggregate would breach the baseline — proving
		// why a full-replace update is required.
		sumIfAdditive := initial[i].Quantity.Add(*adjusted[i].Quantity)
		if !sumIfAdditive.GreaterThan(it.Quantity) {
			t.Errorf("item %d: expected additive sum %s to exceed baseline %s (fixture invariant)", it.ID, sumIfAdditive, it.Quantity)
		}
	}
}
