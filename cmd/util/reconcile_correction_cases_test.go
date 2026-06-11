package main

import (
	"testing"

	"github.com/shopspring/decimal"
)

// sumQty sums the Quantity of a slice of synthetic txns.
func sumQty(txns []SyntheticTxn) decimal.Decimal {
	total := decimal.Zero
	for _, t := range txns {
		total = total.Add(t.Quantity)
	}
	return total
}

// Case A — 4 reconciles, all counts consistent (NO corrections), with purchases
// and disposes in different ranges.
func TestResolveItem_FourSub_AllCorrect_WithPurchaseAndDispose(t *testing.T) {
	in := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			startPurchase(100, "1000", day("2024-01-01")),
			{txnID: 101, createdAt: day("2024-02-10"), quantity: dec("200"), price: 10},
			{txnID: 102, createdAt: day("2024-03-10"), quantity: dec("100"), price: 10},
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("1000"), qty: dec("900")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("900"), qty: dec("1000")},
			{subID: 3, createdAt: day("2024-04-01"), prev: dec("1000"), qty: dec("1050")},
			{subID: 4, createdAt: day("2024-05-01"), prev: dec("1050"), qty: dec("1000")},
		},
		disposeSubs: []disposeStep{
			{subID: 30, createdAt: day("2024-02-20"), qty: dec("50")},
			{subID: 31, createdAt: day("2024-04-20"), qty: dec("30")},
		},
	}
	start, steps, sells, disposals, _, _, err := resolveItem(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !start.Equal(dec("1000")) {
		t.Fatalf("start = %s, want 1000", start)
	}

	wantNewQty := []string{"900", "1000", "1050", "1000"}
	wantDrop := []string{"100", "50", "50", "20"}
	for i := range steps {
		if steps[i].corrected {
			t.Fatalf("step %d should NOT be corrected", i)
		}
		if !steps[i].newQty.Equal(dec(wantNewQty[i])) {
			t.Fatalf("step %d newQty = %s, want %s", i, steps[i].newQty, wantNewQty[i])
		}
		if !steps[i].drop.Equal(dec(wantDrop[i])) {
			t.Fatalf("step %d drop = %s, want %s", i, steps[i].drop, wantDrop[i])
		}
	}

	// total sells = 220, dated at the four reconcile dates respectively.
	if !sumQty(sells).Equal(dec("220")) {
		t.Fatalf("total sells = %s, want 220", sumQty(sells))
	}
	wantSellDates := map[string]bool{
		"2024-02-01": false, "2024-03-01": false, "2024-04-01": false, "2024-05-01": false,
	}
	for _, s := range sells {
		ds := s.BackdatedDate.Format("2006-01-02")
		if _, ok := wantSellDates[ds]; !ok {
			t.Fatalf("sell dated %s, not one of the reconcile dates", ds)
		}
		wantSellDates[ds] = true
	}
	for ds, seen := range wantSellDates {
		if !seen {
			t.Fatalf("expected a sell dated %s, none found", ds)
		}
	}

	// total disposals = 80 (50 @ 2024-02-20, 30 @ 2024-04-20).
	if !sumQty(disposals).Equal(dec("80")) {
		t.Fatalf("total disposals = %s, want 80", sumQty(disposals))
	}
	dispByDate := map[string]decimal.Decimal{}
	for _, d := range disposals {
		ds := d.BackdatedDate.Format("2006-01-02")
		cur := dispByDate[ds]
		dispByDate[ds] = cur.Add(d.Quantity)
	}
	if !dispByDate["2024-02-20"].Equal(dec("50")) {
		t.Fatalf("dispose @2024-02-20 = %s, want 50", dispByDate["2024-02-20"])
	}
	if !dispByDate["2024-04-20"].Equal(dec("30")) {
		t.Fatalf("dispose @2024-04-20 = %s, want 30", dispByDate["2024-04-20"])
	}

	if !steps[len(steps)-1].newQty.Equal(dec("1000")) {
		t.Fatalf("final = %s, want 1000", steps[len(steps)-1].newQty)
	}
}

// Case B — violation only at the (sub1,sub2) boundary -> correct sub1,
// keep sub2/3/4 as-is. No purchases/disposes between.
func TestResolveItem_FourSub_ViolationAtSub2_CorrectsSub1Only(t *testing.T) {
	in := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			startPurchase(100, "1000", day("2024-01-01")),
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("1000"), qty: dec("300")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("300"), qty: dec("950")},
			{subID: 3, createdAt: day("2024-04-01"), prev: dec("950"), qty: dec("800")},
			{subID: 4, createdAt: day("2024-05-01"), prev: dec("800"), qty: dec("700")},
		},
	}
	start, steps, sells, disposals, _, _, err := resolveItem(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !start.Equal(dec("1000")) {
		t.Fatalf("start = %s, want 1000", start)
	}

	wantCorrected := []bool{true, false, false, false}
	wantNewQty := []string{"950", "950", "800", "700"}
	wantDrop := []string{"50", "0", "150", "100"}
	for i := range steps {
		if steps[i].corrected != wantCorrected[i] {
			t.Fatalf("step %d corrected = %v, want %v", i, steps[i].corrected, wantCorrected[i])
		}
		if !steps[i].newQty.Equal(dec(wantNewQty[i])) {
			t.Fatalf("step %d newQty = %s, want %s", i, steps[i].newQty, wantNewQty[i])
		}
		if !steps[i].drop.Equal(dec(wantDrop[i])) {
			t.Fatalf("step %d drop = %s, want %s", i, steps[i].drop, wantDrop[i])
		}
	}

	if !sumQty(sells).Equal(dec("300")) {
		t.Fatalf("total sells = %s, want 300", sumQty(sells))
	}
	if len(disposals) != 0 {
		t.Fatalf("want no disposals, got %d", len(disposals))
	}
	if !steps[len(steps)-1].newQty.Equal(dec("700")) {
		t.Fatalf("final = %s, want 700", steps[len(steps)-1].newQty)
	}
}

// Case C — a late violation propagates back through all three boundaries
// (sub4 anchor; sub3, sub2, sub1 all corrected). Strictly increasing recorded
// counts, no purchases/disposes.
func TestResolveItem_FourSub_FullCascade_FixesThreeEarlier(t *testing.T) {
	in := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			startPurchase(100, "500", day("2024-01-01")),
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("500"), qty: dec("100")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("100"), qty: dec("200")},
			{subID: 3, createdAt: day("2024-04-01"), prev: dec("200"), qty: dec("300")},
			{subID: 4, createdAt: day("2024-05-01"), prev: dec("300"), qty: dec("400")},
		},
	}
	start, steps, sells, disposals, _, _, err := resolveItem(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !start.Equal(dec("500")) {
		t.Fatalf("start = %s, want 500", start)
	}

	wantCorrected := []bool{true, true, true, false}
	wantNewQty := []string{"400", "400", "400", "400"}
	wantDrop := []string{"100", "0", "0", "0"}
	for i := range steps {
		if steps[i].corrected != wantCorrected[i] {
			t.Fatalf("step %d corrected = %v, want %v", i, steps[i].corrected, wantCorrected[i])
		}
		if !steps[i].newQty.Equal(dec(wantNewQty[i])) {
			t.Fatalf("step %d newQty = %s, want %s", i, steps[i].newQty, wantNewQty[i])
		}
		if !steps[i].drop.Equal(dec(wantDrop[i])) {
			t.Fatalf("step %d drop = %s, want %s", i, steps[i].drop, wantDrop[i])
		}
	}

	if !sumQty(sells).Equal(dec("100")) {
		t.Fatalf("total sells = %s, want 100", sumQty(sells))
	}
	for _, s := range sells {
		if !s.BackdatedDate.Equal(day("2024-02-01")) {
			t.Fatalf("sell dated %s, want sub1 2024-02-01", s.BackdatedDate)
		}
	}
	if len(disposals) != 0 {
		t.Fatalf("want no disposals, got %d", len(disposals))
	}
	if !steps[len(steps)-1].newQty.Equal(dec("400")) {
		t.Fatalf("final = %s, want 400", steps[len(steps)-1].newQty)
	}
}

// Case D — violation only at (sub3,sub4) -> correct sub3; after that,
// (sub2,sub3) and (sub1,sub2) are consistent so they're kept. No purchases/disposes.
func TestResolveItem_FourSub_PartialCascade_FixesSub3Only(t *testing.T) {
	in := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			startPurchase(100, "1000", day("2024-01-01")),
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("1000"), qty: dec("900")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("900"), qty: dec("850")},
			{subID: 3, createdAt: day("2024-04-01"), prev: dec("850"), qty: dec("750")},
			{subID: 4, createdAt: day("2024-05-01"), prev: dec("750"), qty: dec("800")},
		},
	}
	start, steps, sells, disposals, _, _, err := resolveItem(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !start.Equal(dec("1000")) {
		t.Fatalf("start = %s, want 1000", start)
	}

	wantCorrected := []bool{false, false, true, false}
	wantNewQty := []string{"900", "850", "800", "800"}
	wantDrop := []string{"100", "50", "50", "0"}
	for i := range steps {
		if steps[i].corrected != wantCorrected[i] {
			t.Fatalf("step %d corrected = %v, want %v", i, steps[i].corrected, wantCorrected[i])
		}
		if !steps[i].newQty.Equal(dec(wantNewQty[i])) {
			t.Fatalf("step %d newQty = %s, want %s", i, steps[i].newQty, wantNewQty[i])
		}
		if !steps[i].drop.Equal(dec(wantDrop[i])) {
			t.Fatalf("step %d drop = %s, want %s", i, steps[i].drop, wantDrop[i])
		}
	}

	if !sumQty(sells).Equal(dec("200")) {
		t.Fatalf("total sells = %s, want 200", sumQty(sells))
	}
	if len(disposals) != 0 {
		t.Fatalf("want no disposals, got %d", len(disposals))
	}
	if !steps[len(steps)-1].newQty.Equal(dec("800")) {
		t.Fatalf("final = %s, want 800", steps[len(steps)-1].newQty)
	}
}
