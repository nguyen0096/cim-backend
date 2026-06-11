package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
)

// These DB-backed (in-memory sqlite) tests close the computeResolution seam for
// the trailing-dispose BLOCKER (#53): a dispose dated strictly after an item's
// last in-scope reconcile must be IGNORED and the submission LEFT PENDING — not
// applied, not completed, not shown as a disposal step, and with no synthetic
// disposal txn. resolveItem reports the ignored sub ids; computeResolution must
// honor them in the preview Steps AND in AppliedAsIs.

// disposeStepFor reports whether the item plan has a dispose step for subID.
func disposeStepFor(ip ItemPlan, subID uint) bool {
	for _, s := range ip.Steps {
		if s.SubmissionType == string(models.InventorySubmissionTypeDispose) && s.SubmissionID == subID {
			return true
		}
	}
	return false
}

func containsUint(xs []uint, want uint) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// A dispose dated after the item's last reconcile must be excluded everywhere in
// the plan: not in AppliedAsIs, not a dispose Step, and no disposal txn. Reuses the
// S2 reconcile env (sub1 @ 2024-02-01, sub2 @ 2024-03-01) and adds a dispose at
// 2024-06-01 (after sub2).
func TestComputeResolution_TrailingDisposeLeftPending(t *testing.T) {
	// live qty = in-scope purchases (1100); the trailing dispose synthesizes nothing.
	env := newApplyQtyEnv(t, dec("1100"))
	env.addPurchase(t, day("2024-01-01"), "1000", "0") // start stock
	env.addPurchase(t, day("2024-02-15"), "100", "0")  // in window
	dispID := newDisposeSub(t, env.db.WithContext(env.ctx), env.inventoryID, env.itemID, day("2024-06-01"), "100")

	plan, err := computeResolution(env.ctx, env.db, env.inventoryID, env.recIDs, []uint{dispID})
	if err != nil {
		t.Fatalf("computeResolution (trailing dispose must not abort): %v", err)
	}

	// 1. Not in AppliedAsIs -> apply leaves it pending.
	if containsUint(plan.AppliedAsIs, dispID) {
		t.Fatalf("trailing dispose %d must NOT be in AppliedAsIs (must stay pending), got %v", dispID, plan.AppliedAsIs)
	}

	// 2. Not present as a dispose step in the item's plan.
	if len(plan.Items) != 1 {
		t.Fatalf("got %d item plans, want 1", len(plan.Items))
	}
	if disposeStepFor(plan.Items[0], dispID) {
		t.Fatalf("trailing dispose %d must NOT appear as a dispose Step", dispID)
	}

	// 3. No disposal txn synthesized.
	for _, txn := range plan.Txns {
		if txn.TransactionType == string(models.InventoryTransactionTypeDisposal) {
			t.Fatalf("trailing dispose must synthesize no disposal txn, got %+v", txn)
		}
	}

	// AppliedQuantity is unaffected by the trailing dispose: current (1100) minus
	// only the in-window shrinkage sells (450) = 650.
	if !plan.Items[0].AppliedQuantity.Equal(dec("650")) {
		t.Fatalf("AppliedQuantity = %s, want 650 (trailing dispose decrements nothing)", plan.Items[0].AppliedQuantity)
	}

	// 4. (if feasible) After applyResolution the dispose submission is still pending.
	if err := applyResolution(env.ctx, env.db, plan); err != nil {
		t.Fatalf("applyResolution: %v", err)
	}
	var after models.InventorySubmission
	if err := env.db.WithContext(env.ctx).First(&after, dispID).Error; err != nil {
		t.Fatalf("reload dispose submission after apply: %v", err)
	}
	if after.ApprovalStatus != models.InventorySubmissionApprovalStatusPending {
		t.Fatalf("trailing dispose approval_status = %s after apply, want pending", after.ApprovalStatus)
	}
}

// MIXED case (narrow integrity guard): a single dispose submission that is trailing
// for one item but in-scope for another is genuinely contradictory and must produce
// a FATAL error naming the submission and the conflicting items.
//
// Two items share one dispose submission dated 2024-06-01: item A's last reconcile
// (2024-03-01) is BEFORE the dispose -> trailing/ignored for A; item B's window
// (2024-05-01 .. 2024-07-01) CONTAINS the dispose -> in-scope/applied for B.
func TestComputeResolution_MixedDisposeIsFatal(t *testing.T) {
	env := newMixedDisposeEnv(t)

	_, err := computeResolution(env.ctx, env.db, env.inventoryID, env.recIDs, []uint{env.dispID})
	if err == nil {
		t.Fatalf("expected fatal item-level conflict for a mixed (trailing+in-scope) dispose submission")
	}
	msg := err.Error()
	if !contains(msg, "item-level conflict") || !contains(msg, "trailing") {
		t.Fatalf("error should describe the item-level conflict, got: %v", err)
	}
	// Names the offending dispose submission id.
	if !contains(msg, "dispose submission") {
		t.Fatalf("error should name the dispose submission, got: %v", err)
	}
}

// mixedDisposeEnv seeds one product, TWO items, a dispose submission listing BOTH
// items, and per-item reconcile pairs straddling the dispose date so the SAME
// dispose is trailing for item A and in-scope for item B.
type mixedDisposeEnv struct {
	*scopeTestEnv
	dispID uint
}

func newMixedDisposeEnv(t *testing.T) *mixedDisposeEnv {
	t.Helper()
	base := newScopeTestEnv(t) // gives us db/ctx/inventoryID + item A with recs @ Feb-01/Mar-01
	tx := base.db.WithContext(base.ctx)

	// Item A already exists (base.itemID) with reconciles at 2024-02-01 and 2024-03-01.
	// The dispose at 2024-06-01 is AFTER A's last reconcile -> trailing for A.
	// Add purchases for A so the in-scope (windows) resolve cleanly.
	base.addPurchase(t, day("2024-01-01"), "1000", "0")
	base.addPurchase(t, day("2024-02-15"), "100", "0")

	// Item B: reconciles at 2024-05-01 and 2024-07-01 — the dispose at 2024-06-01
	// falls in B's window (in-scope/applied for B). Needs its OWN product (the
	// inventory_items table is unique on inventory_id+product_id).
	const productBID uint = 12
	productB := models.Product{Base: models.Base{ID: productBID}, Name: "Gadget", UnitID: 1}
	if err := tx.Create(&productB).Error; err != nil {
		t.Fatalf("create product B: %v", err)
	}
	// B's counts are CONSISTENT with the in-window dispose so B resolves cleanly and
	// the run reaches the AppliedAsIs membership guard (not an unrelated fatal):
	// start 1000, rec1 1000, dispose 50 in window, rec2 950 -> 950 <= 1000 - 50. ✓
	itemB := models.InventoryItem{
		InventoryID: base.inventoryID,
		ProductID:   productBID,
		UnitID:      1,
		Status:      models.InventoryItemStatusActive,
		Quantity:    decimal.NewFromInt(950),
	}
	if err := tx.Create(&itemB).Error; err != nil {
		t.Fatalf("create item B: %v", err)
	}
	recB1 := newReconcileSub(t, tx, base.inventoryID, itemB.ID, day("2024-05-01"), "1000", "1000")
	recB2 := newReconcileSub(t, tx, base.inventoryID, itemB.ID, day("2024-07-01"), "1000", "950")
	// Start stock for B (before its first reconcile) covering the dispose.
	addPurchaseFor(t, tx, itemB.ID, day("2024-04-01"), "1000")

	base.recIDs = append(base.recIDs, recB1, recB2)

	// One dispose submission listing BOTH items, dated 2024-06-01.
	dispID := newDisposeSubMulti(t, tx, base.inventoryID, day("2024-06-01"),
		map[uint]string{base.itemID: "50", itemB.ID: "50"})

	return &mixedDisposeEnv{scopeTestEnv: base, dispID: dispID}
}

// addPurchaseFor inserts a purchase txn for an arbitrary item id.
func addPurchaseFor(t *testing.T, tx *gorm.DB, itemID uint, at time.Time, qty string) {
	t.Helper()
	txn := models.InventoryTransaction{
		Base:            models.Base{CreatedAt: at},
		InventoryItemID: itemID,
		TransactionType: models.InventoryTransactionTypePurchase,
		Price:           10,
		Quantity:        dec(qty),
	}
	if err := tx.Create(&txn).Error; err != nil {
		t.Fatalf("create purchase txn for item %d: %v", itemID, err)
	}
}

// newDisposeSubMulti seeds a single pending dispose submission listing several
// items (item id -> qty). Used to construct the MIXED case: one submission whose
// items straddle the trailing/in-scope boundary.
func newDisposeSubMulti(t *testing.T, tx *gorm.DB, inventoryID uint, at time.Time, qtyByItem map[uint]string) uint {
	t.Helper()
	items := make([]dto.QuantityItem, 0, len(qtyByItem))
	for itemID, qty := range qtyByItem {
		q := dec(qty)
		items = append(items, dto.QuantityItem{InventoryItemID: itemID, Quantity: &q})
	}
	req := dto.DisposeInventoryRequest{InventoryID: inventoryID, Items: items}
	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal multi-item dispose payload: %v", err)
	}
	sub := models.InventorySubmission{
		Base:           models.Base{CreatedAt: at},
		InventoryID:    inventoryID,
		SubmissionType: models.InventorySubmissionTypeDispose,
		ApprovalStatus: models.InventorySubmissionApprovalStatusPending,
		Payload:        payload,
	}
	if err := tx.Create(&sub).Error; err != nil {
		t.Fatalf("create multi-item dispose sub: %v", err)
	}
	return sub.ID
}
