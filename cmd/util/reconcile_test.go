package main

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func dec(v string) decimal.Decimal {
	out, err := decimal.NewFromString(v)
	if err != nil {
		panic(err)
	}
	return out
}

func day(s string) time.Time {
	out, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return out
}

// purchasesBefore builds a single purchase txn dated before the first reconcile,
// representing start stock.
func startPurchase(id uint, qty string, at time.Time) purchaseTxn {
	return purchaseTxn{txnID: id, createdAt: at, quantity: dec(qty), consumedQuantity: decimal.Zero, price: 10}
}

// S1: purchases 0 between subs, start stock 1000; sub1 300 corrected -> 650,
// sell 350 @ sub1, 0 @ sub2.
func TestResolveItem_S1(t *testing.T) {
	in := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			startPurchase(100, "1000", day("2024-01-01")), // start stock 1000, no in-range purchases
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("1000"), qty: dec("300")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("1000"), qty: dec("650")},
		},
	}
	start, steps, sells, disposals, _, _, err := resolveItem(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !start.Equal(dec("1000")) {
		t.Fatalf("start stock = %s, want 1000", start)
	}
	if !steps[0].newQty.Equal(dec("650")) {
		t.Fatalf("sub1 corrected = %s, want 650", steps[0].newQty)
	}
	if !steps[0].corrected {
		t.Fatalf("sub1 should be corrected")
	}
	if !steps[0].drop.Equal(dec("350")) {
		t.Fatalf("sub1 drop = %s, want 350", steps[0].drop)
	}
	if !steps[0].prevQty.Equal(dec("1000")) {
		t.Fatalf("sub1 prev = %s, want 1000 (= start stock)", steps[0].prevQty)
	}
	if !steps[1].drop.Equal(dec("0")) {
		t.Fatalf("sub2 drop = %s, want 0", steps[1].drop)
	}
	if steps[1].corrected {
		t.Fatalf("sub2 should not be corrected")
	}
	if len(disposals) != 0 {
		t.Fatalf("want no disposals, got %d", len(disposals))
	}
	total := decimal.Zero
	for _, s := range sells {
		total = total.Add(s.Quantity)
		if !s.BackdatedDate.Equal(day("2024-02-01")) {
			t.Fatalf("sell dated %s, want sub1 date", s.BackdatedDate)
		}
	}
	if !total.Equal(dec("350")) {
		t.Fatalf("total sells = %s, want 350", total)
	}
	// final == last reconcile count
	if !steps[len(steps)-1].newQty.Equal(dec("650")) {
		t.Fatalf("final = %s, want 650", steps[len(steps)-1].newQty)
	}
}

// S2: purchases 100, sub1 300 corrected -> 550, sell 450 @ sub1.
func TestResolveItem_S2(t *testing.T) {
	in := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			startPurchase(100, "1000", day("2024-01-01")),                               // start stock 1000
			{txnID: 101, createdAt: day("2024-02-15"), quantity: dec("100"), price: 10}, // in range (sub1, sub2]
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("1000"), qty: dec("300")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("1100"), qty: dec("650")},
		},
	}
	start, steps, sells, _, _, _, err := resolveItem(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !start.Equal(dec("1000")) {
		t.Fatalf("start = %s, want 1000", start)
	}
	// rule: 650 <= 300 + 100 - 0 = 400? no -> correct sub1 = 650 - 100 + 0 = 550
	if !steps[0].newQty.Equal(dec("550")) {
		t.Fatalf("sub1 corrected = %s, want 550", steps[0].newQty)
	}
	// drop @ sub1 = start(1000) - 550 = 450
	if !steps[0].drop.Equal(dec("450")) {
		t.Fatalf("sub1 drop = %s, want 450", steps[0].drop)
	}
	// sub2 prev = corrected(550) + purchases(100) - disposes(0) = 650
	if !steps[1].prevQty.Equal(dec("650")) {
		t.Fatalf("sub2 prev = %s, want 650", steps[1].prevQty)
	}
	if !steps[1].drop.Equal(dec("0")) {
		t.Fatalf("sub2 drop = %s, want 0", steps[1].drop)
	}
	total := decimal.Zero
	for _, s := range sells {
		total = total.Add(s.Quantity)
		if !s.BackdatedDate.Equal(day("2024-02-01")) {
			t.Fatalf("sell dated %s, want sub1", s.BackdatedDate)
		}
	}
	if !total.Equal(dec("450")) {
		t.Fatalf("total sells = %s, want 450", total)
	}
}

// A consistent pair: no correction.
func TestResolveItem_Consistent(t *testing.T) {
	in := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			startPurchase(100, "1000", day("2024-01-01")),
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("1000"), qty: dec("800")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("800"), qty: dec("650")},
		},
	}
	_, steps, sells, _, _, _, err := resolveItem(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 650 <= 800 + 0 - 0 -> consistent, no correction
	if steps[0].corrected || steps[1].corrected {
		t.Fatalf("no correction expected; got %v/%v", steps[0].corrected, steps[1].corrected)
	}
	if !steps[0].newQty.Equal(dec("800")) || !steps[1].newQty.Equal(dec("650")) {
		t.Fatalf("counts changed unexpectedly: %s/%s", steps[0].newQty, steps[1].newQty)
	}
	// per-period drops: 200 @ sub1, 150 @ sub2
	if !steps[0].drop.Equal(dec("200")) {
		t.Fatalf("sub1 drop = %s, want 200", steps[0].drop)
	}
	if !steps[1].drop.Equal(dec("150")) {
		t.Fatalf("sub2 drop = %s, want 150", steps[1].drop)
	}
	total := decimal.Zero
	for _, s := range sells {
		total = total.Add(s.Quantity)
	}
	if !total.Equal(dec("350")) {
		t.Fatalf("total sells = %s, want 350", total)
	}
}

// 3-reconcile backward propagation chain.
func TestResolveItem_BackwardPropagation(t *testing.T) {
	// counts 300, 650, 500; no purchases. start stock 650.
	// k=2: 500 <= 650 + 0 -> consistent
	// k=1: 650 <= 300 + 0 -> no -> correct sub1 := 650 -> drop @ sub1
	in := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			startPurchase(100, "650", day("2024-01-01")),
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("650"), qty: dec("300")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("650"), qty: dec("650")},
			{subID: 3, createdAt: day("2024-04-01"), prev: dec("650"), qty: dec("500")},
		},
	}
	start, steps, _, _, _, _, err := resolveItem(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !start.Equal(dec("650")) {
		t.Fatalf("start = %s, want 650", start)
	}
	if !steps[0].newQty.Equal(dec("650")) || !steps[0].corrected {
		t.Fatalf("sub1 = %s corrected=%v, want 650/true", steps[0].newQty, steps[0].corrected)
	}
	if !steps[0].drop.Equal(dec("0")) {
		t.Fatalf("sub1 drop = %s, want 0 (650 -> 650)", steps[0].drop)
	}
	if !steps[1].drop.Equal(dec("0")) {
		t.Fatalf("sub2 drop = %s, want 0", steps[1].drop)
	}
	// sub3 prev = 650, count 500 -> drop 150
	if !steps[2].drop.Equal(dec("150")) {
		t.Fatalf("sub3 drop = %s, want 150", steps[2].drop)
	}
	if !steps[len(steps)-1].newQty.Equal(dec("500")) {
		t.Fatalf("final = %s, want 500", steps[len(steps)-1].newQty)
	}
}

// Range with both a purchase and a dispose.
func TestResolveItem_PurchaseAndDispose(t *testing.T) {
	// start 1000; in range (sub1,sub2]: purchase 100, dispose 50.
	// sub1 count 300, sub2 count 650.
	// rule: 650 <= 300 + 100 - 50 = 350? no -> correct sub1 := 650 - 100 + 50 = 600
	in := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			startPurchase(100, "1000", day("2024-01-01")),
			{txnID: 101, createdAt: day("2024-02-10"), quantity: dec("100"), price: 10},
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("1000"), qty: dec("300")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("1050"), qty: dec("650")},
		},
		disposeSubs: []disposeStep{
			{subID: 3, createdAt: day("2024-02-20"), qty: dec("50")},
		},
	}
	start, steps, sells, disposals, consumedAfter, _, err := resolveItem(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !start.Equal(dec("1000")) {
		t.Fatalf("start = %s, want 1000", start)
	}
	if !steps[0].newQty.Equal(dec("600")) {
		t.Fatalf("sub1 corrected = %s, want 600", steps[0].newQty)
	}
	// sub1 drop = start(1000) - 600 = 400
	if !steps[0].drop.Equal(dec("400")) {
		t.Fatalf("sub1 drop = %s, want 400", steps[0].drop)
	}
	// sub2 prev = 600 + 100 - 50 = 650 -> drop 0
	if !steps[1].prevQty.Equal(dec("650")) {
		t.Fatalf("sub2 prev = %s, want 650", steps[1].prevQty)
	}
	if !steps[1].drop.Equal(dec("0")) {
		t.Fatalf("sub2 drop = %s, want 0", steps[1].drop)
	}
	// dispose applied as a real disposal of 50
	dispTotal := decimal.Zero
	for _, d := range disposals {
		dispTotal = dispTotal.Add(d.Quantity)
		if !d.BackdatedDate.Equal(day("2024-02-20")) {
			t.Fatalf("dispose dated %s, want its own date", d.BackdatedDate)
		}
	}
	if !dispTotal.Equal(dec("50")) {
		t.Fatalf("dispose total = %s, want 50", dispTotal)
	}
	// sells = 400 @ sub1
	sellTotal := decimal.Zero
	for _, s := range sells {
		sellTotal = sellTotal.Add(s.Quantity)
	}
	if !sellTotal.Equal(dec("400")) {
		t.Fatalf("sell total = %s, want 400", sellTotal)
	}
	// total consumed = 400 sell + 50 dispose = 450; never exceeds purchased 1100
	totalConsumed := decimal.Zero
	for _, c := range consumedAfter {
		totalConsumed = totalConsumed.Add(c)
	}
	if !totalConsumed.Equal(dec("450")) {
		t.Fatalf("total consumed = %s, want 450", totalConsumed)
	}
	// final stock == last reconcile count
	if !steps[len(steps)-1].newQty.Equal(dec("650")) {
		t.Fatalf("final = %s, want 650", steps[len(steps)-1].newQty)
	}
}

// Boundary: a purchase whose created_at exactly equals sub2's timestamp belongs
// to range (sub1, sub2] (not the next range). A dispose exactly on sub1 timestamp
// is rejected (must be strictly after the first reconcile / map to a single range).
func TestResolveItem_BoundaryExactlyOnSubTimestamp(t *testing.T) {
	sub2at := day("2024-03-01")
	in := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			startPurchase(100, "1000", day("2024-01-01")),
			{txnID: 101, createdAt: sub2at, quantity: dec("100"), price: 10}, // exactly on sub2 -> range (sub1,sub2]
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("1000"), qty: dec("300")},
			{subID: 2, createdAt: sub2at, prev: dec("1100"), qty: dec("650")},
		},
	}
	_, steps, _, _, _, _, err := resolveItem(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// purchase counted in range -> rule: 650 <= 300 + 100 -> correct sub1 = 550
	if !steps[0].newQty.Equal(dec("550")) {
		t.Fatalf("boundary purchase not mapped into (sub1,sub2]: sub1 = %s, want 550", steps[0].newQty)
	}
}

// Fatal: corrected first-sub stock > start stock.
func TestResolveItem_FatalFirstSubExceedsStart(t *testing.T) {
	// start stock only 400; counts force sub1 correction above 400.
	in := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			startPurchase(100, "400", day("2024-01-01")),
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("400"), qty: dec("300")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("400"), qty: dec("650")},
		},
	}
	_, _, _, _, _, _, err := resolveItem(in)
	if err == nil {
		t.Fatalf("expected fatal error (corrected sub1 650 > start 400)")
	}
}

// Stock never negative: a dispose that overdraws available stock errors out.
func TestResolveItem_OverdrawErrors(t *testing.T) {
	in := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			startPurchase(100, "100", day("2024-01-01")),
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("100"), qty: dec("100")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("100"), qty: dec("100")},
		},
		disposeSubs: []disposeStep{
			{subID: 3, createdAt: day("2024-02-15"), qty: dec("500")}, // overdraw
		},
	}
	_, _, _, _, _, _, err := resolveItem(in)
	if err == nil {
		t.Fatalf("expected overdraw error")
	}
}

// Negative dispose quantity: a selected dispose submission with qty < 0 must
// hard-fail BEFORE any range folding/correction, naming the sub + inventory item,
// with nothing applied. Reachable from existing pending data because
// CreateDisposeSubmission only rejects qty greater than available stock, not
// negatives. A negative would otherwise distort the "- range_disposes" term while
// building no disposal event (events require qty > 0).
func TestResolveItem_NegativeDisposeQtyErrors(t *testing.T) {
	in := itemInput{
		itemID: 7,
		purchases: []purchaseTxn{
			startPurchase(100, "100", day("2024-01-01")),
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("100"), qty: dec("100")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("100"), qty: dec("100")},
		},
		disposeSubs: []disposeStep{
			{subID: 42, createdAt: day("2024-02-15"), qty: dec("-5")}, // negative
		},
	}
	start, steps, sells, disposals, consumedAfter, _, err := resolveItem(in)
	if err == nil {
		t.Fatalf("expected hard error for negative dispose quantity")
	}
	// Error names the offending submission id, inventory_item_id, and qty.
	msg := err.Error()
	if !contains(msg, "dispose submission 42") {
		t.Fatalf("error should name dispose submission 42: %q", msg)
	}
	if !contains(msg, "inventory_item_id 7") {
		t.Fatalf("error should name inventory_item_id 7: %q", msg)
	}
	if !contains(msg, "-5") {
		t.Fatalf("error should name the offending qty -5: %q", msg)
	}
	// Nothing folded/applied: all outputs are zero/nil on abort.
	if !start.IsZero() || steps != nil || sells != nil || disposals != nil || consumedAfter != nil {
		t.Fatalf("expected no outputs on abort, got start=%s steps=%v sells=%v disposals=%v consumedAfter=%v",
			start.String(), steps, sells, disposals, consumedAfter)
	}
}

// A ZERO dispose quantity remains a benign no-op (prior decision): it must not
// hard-fail, and produces no disposal event.
func TestResolveItem_ZeroDisposeQtyIsNoOp(t *testing.T) {
	in := itemInput{
		itemID: 8,
		purchases: []purchaseTxn{
			startPurchase(100, "100", day("2024-01-01")),
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("100"), qty: dec("100")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("100"), qty: dec("100")},
		},
		disposeSubs: []disposeStep{
			{subID: 9, createdAt: day("2024-02-15"), qty: dec("0")},
		},
	}
	_, _, _, disposals, _, _, err := resolveItem(in)
	if err != nil {
		t.Fatalf("zero dispose qty should be a benign no-op, got error: %v", err)
	}
	if len(disposals) != 0 {
		t.Fatalf("zero dispose qty should build no disposal event, got %d", len(disposals))
	}
}

// Clone payload: corrected payload updates BOTH quantity and prev_quantity for the
// corrected item, leaving others untouched.
func TestCorrectedPayload(t *testing.T) {
	orig := []byte(`{"inventory_id":1,"items":[{"inventory_item_id":1,"quantity":"300","prev_quantity":"650"},{"inventory_item_id":2,"quantity":"50","prev_quantity":"50"}]}`)
	corrs := []SubmissionCorrection{
		{SubmissionID: 1, InventoryItemID: 1, NewQuantity: dec("650"), NewPrev: dec("650")},
	}
	out, err := correctedPayload(orig, corrs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	// item 1 quantity now 650, prev 650; item 2 unchanged.
	if !contains(s, `"quantity":"650"`) {
		t.Fatalf("corrected quantity not in payload: %s", s)
	}
	// ensure item 2 stayed at 50/50
	if !contains(s, `"quantity":"50"`) || !contains(s, `"prev_quantity":"50"`) {
		t.Fatalf("untouched item changed: %s", s)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// #52: the temporal-FIFO guard is REMOVED. A dispose dated before the only purchase
// that could cover it now FIFO-consumes from that purchase regardless of chronology
// (this is a one-off historical correction over an unconsumed ledger). Mirrors the
// prod scenario: sub1 stock 0, dispose 100 on Feb 10, purchase 100 on Feb 20, sub2
// stock 0. There IS enough total stock (the one purchase covers it), so it resolves
// cleanly instead of hard-failing — the disposal is sourced from the Feb-20 purchase
// and dated at the dispose's own date.
func TestResolveItem_FuturePurchaseConsumedFIFO(t *testing.T) {
	in := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			// purchase dated AFTER the dispose (Feb 20). No purchase exists at/before
			// the first reconcile, so start stock is 0 — but the dispose still draws
			// from it FIFO now that the temporal guard is gone.
			{txnID: 200, createdAt: day("2024-02-20"), quantity: dec("100"), price: 10},
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("0"), qty: dec("0")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("0"), qty: dec("0")},
		},
		disposeSubs: []disposeStep{
			{subID: 3, createdAt: day("2024-02-10"), qty: dec("100")}, // before the Feb-20 purchase
		},
	}
	_, _, _, disposals, _, _, err := resolveItem(in)
	if err != nil {
		t.Fatalf("unexpected error (temporal guard removed; total stock suffices): %v", err)
	}
	total := decimal.Zero
	for _, d := range disposals {
		total = total.Add(d.Quantity)
		if d.SourcePurchaseTxnID != 200 {
			t.Fatalf("disposal sourced from txn %d, want 200 (the only purchase)", d.SourcePurchaseTxnID)
		}
		if !d.BackdatedDate.Equal(day("2024-02-10")) {
			t.Fatalf("disposal dated %s, want its own date 2024-02-10", d.BackdatedDate)
		}
	}
	if !total.Equal(dec("100")) {
		t.Fatalf("disposal total = %s, want 100", total)
	}
}

// FIFO valid case: a purchase covers the dispose, so it resolves cleanly and the
// disposal is dated at the dispose's own date sourced from that purchase.
func TestResolveItem_EarlierPurchaseCovers(t *testing.T) {
	in := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			// purchase before the first reconcile -> start stock 100.
			startPurchase(100, "100", day("2024-01-15")),
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("100"), qty: dec("100")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("0"), qty: dec("0")},
		},
		disposeSubs: []disposeStep{
			{subID: 3, createdAt: day("2024-02-10"), qty: dec("100")},
		},
	}
	_, _, _, disposals, _, _, err := resolveItem(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	total := decimal.Zero
	for _, d := range disposals {
		total = total.Add(d.Quantity)
		if d.SourcePurchaseTxnID != 100 {
			t.Fatalf("disposal sourced from txn %d, want 100 (the earlier purchase)", d.SourcePurchaseTxnID)
		}
		if !d.BackdatedDate.Equal(day("2024-02-10")) {
			t.Fatalf("disposal dated %s, want its own date 2024-02-10", d.BackdatedDate)
		}
	}
	if !total.Equal(dec("100")) {
		t.Fatalf("disposal total = %s, want 100", total)
	}
}

// #52: a DISPOSE-ONLY item (no reconcile submissions) must NOT abort. It has no
// counts to correct and no windows — just disposal events that FIFO-consume from the
// full purchase ledger, dated at each dispose's own date. start_stock/steps are zero/
// empty (no reconciles); the disposals are synthesized like any other item.
func TestResolveItem_DisposeOnlyNoReconcile(t *testing.T) {
	in := itemInput{
		itemID: 9,
		purchases: []purchaseTxn{
			startPurchase(100, "60", day("2024-01-01")),
			{txnID: 101, createdAt: day("2024-02-01"), quantity: dec("40"), price: 10},
		},
		// no reconcileSubs
		disposeSubs: []disposeStep{
			{subID: 5, createdAt: day("2024-03-01"), qty: dec("70")}, // draws 60 from txn100, 10 from txn101
		},
	}
	start, steps, sells, disposals, consumedAfter, _, err := resolveItem(in)
	if err != nil {
		t.Fatalf("dispose-only item must not abort: %v", err)
	}
	if !start.IsZero() {
		t.Fatalf("dispose-only start stock = %s, want 0 (no reconciles)", start)
	}
	if len(steps) != 0 {
		t.Fatalf("dispose-only steps = %d, want 0 (no reconciles)", len(steps))
	}
	if len(sells) != 0 {
		t.Fatalf("dispose-only sells = %d, want 0 (no reconcile shrinkage)", len(sells))
	}
	total := decimal.Zero
	for _, d := range disposals {
		total = total.Add(d.Quantity)
		if !d.BackdatedDate.Equal(day("2024-03-01")) {
			t.Fatalf("disposal dated %s, want its own date 2024-03-01", d.BackdatedDate)
		}
	}
	if !total.Equal(dec("70")) {
		t.Fatalf("disposal total = %s, want 70", total)
	}
	// FIFO: 60 consumed from txn100 (fully), 10 from txn101.
	if !consumedAfter[100].Equal(dec("60")) {
		t.Fatalf("txn100 consumed = %s, want 60", consumedAfter[100])
	}
	if !consumedAfter[101].Equal(dec("10")) {
		t.Fatalf("txn101 consumed = %s, want 10", consumedAfter[101])
	}
}

// #52: a dispose dated strictly AFTER the item's last reconcile is now IGNORED
// (no abort, no list), mirroring the trailing-purchase treatment. The result must be
// identical to the same item without that trailing dispose.
func TestResolveItem_TrailingDisposeAfterLastReconcileIgnored(t *testing.T) {
	base := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			startPurchase(100, "1000", day("2024-01-01")),
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("1000"), qty: dec("1000")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("1000"), qty: dec("1000")},
		},
	}
	withTrailing := base
	withTrailing.disposeSubs = []disposeStep{
		// dispose AFTER sub2's timestamp -> out of scope, ignored (#52).
		{subID: 9, createdAt: day("2024-06-01"), qty: dec("100")},
	}

	_, _, _, baseDisp, _, baseIgnored, baseErr := resolveItem(base)
	_, _, _, disp, _, ignored, err := resolveItem(withTrailing)
	if baseErr != nil || err != nil {
		t.Fatalf("unexpected error: base=%v with=%v", baseErr, err)
	}
	if len(disp) != len(baseDisp) {
		t.Fatalf("trailing dispose should be ignored: got %d disposals, want %d", len(disp), len(baseDisp))
	}
	// The baseline (no trailing dispose) reports nothing ignored.
	if len(baseIgnored) != 0 {
		t.Fatalf("baseline should report no ignored trailing disposes, got %v", baseIgnored)
	}
	// resolveItem must REPORT the trailing dispose sub it ignored, so the caller can
	// keep it out of the preview steps and AppliedAsIs (left pending).
	if len(ignored) != 1 || ignored[0] != 9 {
		t.Fatalf("ignored trailing dispose set = %v, want [9]", ignored)
	}
}

// First-boundary purchase: a purchase whose created_at EXACTLY equals the first
// reconcile's timestamp counts in start_stock and is NOT also mapped into a range
// (no double-count, no "did not map to any range" abort).
func TestResolveItem_FirstBoundaryPurchaseIsStartStock(t *testing.T) {
	firstAt := day("2024-02-01")
	in := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			startPurchase(100, "600", day("2024-01-01")),                      // clearly before first
			{txnID: 101, createdAt: firstAt, quantity: dec("400"), price: 10}, // exactly on first reconcile
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: firstAt, prev: dec("1000"), qty: dec("1000")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("1000"), qty: dec("1000")},
		},
	}
	start, steps, _, _, _, _, err := resolveItem(in)
	if err != nil {
		t.Fatalf("unexpected error (boundary purchase should map to start, not abort): %v", err)
	}
	// start = 600 + 400 = 1000 (counted exactly once).
	if !start.Equal(dec("1000")) {
		t.Fatalf("start stock = %s, want 1000 (boundary purchase counted in start)", start)
	}
	// consistent counts -> no correction, no drop.
	if steps[0].corrected || steps[1].corrected {
		t.Fatalf("no correction expected; got %v/%v", steps[0].corrected, steps[1].corrected)
	}
	if !steps[0].drop.Equal(dec("0")) || !steps[1].drop.Equal(dec("0")) {
		t.Fatalf("drops = %s/%s, want 0/0", steps[0].drop, steps[1].drop)
	}
}

// #50: a purchase dated strictly AFTER the item's last in-scope reconcile is
// OUTSIDE every reconcile window and must be IGNORED (not folded into a range,
// not added to start-stock, not an error). The correction/drops/sells must be
// identical to the same item WITHOUT that trailing purchase (i.e. computed from
// the windows only). This is the S2 scenario plus one trailing purchase.
func TestResolveItem_TrailingPurchaseAfterLastReconcileIgnored(t *testing.T) {
	// Windows-only baseline (== TestResolveItem_S2): start 1000, in-range +100,
	// sub1 300 -> corrected 550, sell 450 @ sub1, final 650.
	base := itemInput{
		itemID: 1,
		purchases: []purchaseTxn{
			startPurchase(100, "1000", day("2024-01-01")),                               // start stock 1000
			{txnID: 101, createdAt: day("2024-02-15"), quantity: dec("100"), price: 10}, // in range (sub1, sub2]
		},
		reconcileSubs: []reconcileStep{
			{subID: 1, createdAt: day("2024-02-01"), prev: dec("1000"), qty: dec("300")},
			{subID: 2, createdAt: day("2024-03-01"), prev: dec("1100"), qty: dec("650")},
		},
	}

	// Same item PLUS a purchase dated after the last reconcile (sub2 = 2024-03-01).
	withTrailing := base
	withTrailing.purchases = append([]purchaseTxn{}, base.purchases...)
	withTrailing.purchases = append(withTrailing.purchases,
		// trailing purchase: AFTER sub2's timestamp -> out of scope, ignored.
		purchaseTxn{txnID: 999, createdAt: day("2024-06-01"), quantity: dec("777"), price: 10})

	baseStart, baseSteps, baseSells, _, baseConsumed, _, baseErr := resolveItem(base)
	if baseErr != nil {
		t.Fatalf("baseline (windows-only) unexpected error: %v", baseErr)
	}

	start, steps, sells, _, consumed, _, err := resolveItem(withTrailing)
	if err != nil {
		t.Fatalf("trailing purchase must NOT error (it is out of scope), got: %v", err)
	}

	// Trailing purchase is dropped, not added to start-stock.
	if !start.Equal(dec("1000")) {
		t.Fatalf("start stock = %s, want 1000 (trailing purchase must be ignored, not added to start)", start)
	}
	if !start.Equal(baseStart) {
		t.Fatalf("start = %s, want == windows-only baseline %s", start, baseStart)
	}

	// Correction/drops identical to windows-only.
	if len(steps) != len(baseSteps) {
		t.Fatalf("got %d steps, want %d (same as baseline)", len(steps), len(baseSteps))
	}
	for i := range steps {
		if !steps[i].newQty.Equal(baseSteps[i].newQty) ||
			steps[i].corrected != baseSteps[i].corrected ||
			!steps[i].drop.Equal(baseSteps[i].drop) ||
			!steps[i].prevQty.Equal(baseSteps[i].prevQty) {
			t.Fatalf("step %d differs from windows-only baseline: got newQty=%s corrected=%v drop=%s prev=%s; want newQty=%s corrected=%v drop=%s prev=%s",
				i, steps[i].newQty, steps[i].corrected, steps[i].drop, steps[i].prevQty,
				baseSteps[i].newQty, baseSteps[i].corrected, baseSteps[i].drop, baseSteps[i].prevQty)
		}
	}

	// Sells identical (total + dates), and none sourced from the trailing purchase.
	sellTotal := decimal.Zero
	for _, s := range sells {
		sellTotal = sellTotal.Add(s.Quantity)
		if s.SourcePurchaseTxnID == 999 {
			t.Fatalf("a sell was sourced from the trailing (ignored) purchase 999")
		}
	}
	baseSellTotal := decimal.Zero
	for _, s := range baseSells {
		baseSellTotal = baseSellTotal.Add(s.Quantity)
	}
	if !sellTotal.Equal(dec("450")) || !sellTotal.Equal(baseSellTotal) {
		t.Fatalf("sell total = %s, want 450 (== baseline %s)", sellTotal, baseSellTotal)
	}

	// The trailing purchase contributes nothing to consumption (it isn't in the
	// ledger at all), so total consumed matches the windows-only baseline.
	totalConsumed := decimal.Zero
	for _, c := range consumed {
		totalConsumed = totalConsumed.Add(c)
	}
	baseTotalConsumed := decimal.Zero
	for _, c := range baseConsumed {
		baseTotalConsumed = baseTotalConsumed.Add(c)
	}
	if _, ok := consumed[999]; ok {
		t.Fatalf("trailing purchase 999 must not appear in the consumed ledger")
	}
	if !totalConsumed.Equal(baseTotalConsumed) {
		t.Fatalf("total consumed = %s, want == windows-only baseline %s", totalConsumed, baseTotalConsumed)
	}
}

// isLocalHost: an empty/missing host is treated as NON-local so a hostless DSN
// requires --prod-confirm before --apply.
func TestIsLocalHost_EmptyIsNonLocal(t *testing.T) {
	if isLocalHost("") {
		t.Fatalf("empty host must be treated as non-local (require --prod-confirm)")
	}
	for _, h := range []string{"localhost", "127.0.0.1", "::1", "host.docker.internal"} {
		if !isLocalHost(h) {
			t.Fatalf("%q should be local", h)
		}
	}
	if isLocalHost("prod-db.internal") {
		t.Fatalf("remote host should be non-local")
	}
}

// openReconcileDB: a hostless --db-url (postgres:///db) with --apply but without
// --prod-confirm must be refused (the apply guard fires; no DB connection attempted).
func TestOpenReconcileDB_HostlessDSNRequiresProdConfirm(t *testing.T) {
	origURL, origApply, origConfirm := reconcileDBURL, reconcileApply, reconcileProdConfirm
	defer func() {
		reconcileDBURL, reconcileApply, reconcileProdConfirm = origURL, origApply, origConfirm
	}()

	reconcileDBURL = "postgres:///somedb"
	reconcileApply = true
	reconcileProdConfirm = false

	_, err := openReconcileDB()
	if err == nil {
		t.Fatalf("expected refusal: hostless --db-url + --apply without --prod-confirm")
	}
	if !contains(err.Error(), "prod-confirm") {
		t.Fatalf("error should mention prod-confirm; got: %v", err)
	}
}

// rangeIndexFor: a timestamp exactly on a sub boundary maps to exactly one range.
func TestRangeIndexFor_HalfOpen(t *testing.T) {
	recs := []reconcileStep{
		{subID: 1, createdAt: day("2024-02-01")},
		{subID: 2, createdAt: day("2024-03-01")},
		{subID: 3, createdAt: day("2024-04-01")},
	}
	// exactly on sub2 -> range 1 (sub1, sub2]
	if k := rangeIndexFor(recs, day("2024-03-01")); k != 1 {
		t.Fatalf("on-sub2 maps to range %d, want 1", k)
	}
	// exactly on sub1 -> not in any inter-reconcile range (-1; belongs to start stock)
	if k := rangeIndexFor(recs, day("2024-02-01")); k != -1 {
		t.Fatalf("on-sub1 maps to range %d, want -1", k)
	}
	// between sub2 and sub3 -> range 2
	if k := rangeIndexFor(recs, day("2024-03-15")); k != 2 {
		t.Fatalf("mid maps to range %d, want 2", k)
	}
	// exactly on sub3 -> range 2 (sub2, sub3]
	if k := rangeIndexFor(recs, day("2024-04-01")); k != 2 {
		t.Fatalf("on-sub3 maps to range %d, want 2", k)
	}
}
