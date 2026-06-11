package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
)

// These DB-backed (in-memory sqlite) tests exercise the #51 apply-correctness fix:
// apply must set new_quantity = current_live - Σ(sells + disposals it creates),
// NOT overwrite with FinalStock. This preserves post-reconcile activity (e.g. a
// trailing purchase consumed after the last reconcile) and equals FinalStock
// exactly when there is no such activity. The preview (ItemPlan.AppliedQuantity)
// must show the SAME value apply writes (plan.itemUpdates[].newQty).
//
// Resolver scenario reused from S2 / consumed-scope tests:
//   start stock 1000 (purchase 2024-01-01), in-window +100 (2024-02-15),
//   sub1 @ 2024-02-01 counted 300 -> corrected 550, sub2 @ 2024-03-01 counted 650.
//   FinalStock = 650, outbound (sells) = 450.
// With an unconsumed ledger and NO trailing activity, the live quantity equals
// the sum of in-scope purchases = 1000 + 100 = 1100, so 1100 - 450 = 650 = FinalStock.

// newApplyQtyEnv seeds the S2 inventory but lets the caller set the item's live
// quantity (which models real post-reconcile activity).
func newApplyQtyEnv(t *testing.T, liveQty decimal.Decimal) *scopeTestEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Product{},
		&models.InventoryItem{},
		&models.InventorySubmission{},
		&models.InventoryTransaction{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := pkg.WithUserEmail(context.Background(), reconcileActor)
	tx := db.WithContext(ctx)

	const inventoryID uint = 7
	const productID uint = 11

	product := models.Product{Base: models.Base{ID: productID}, Name: "Widget", UnitID: 1}
	if err := tx.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	item := models.InventoryItem{
		InventoryID: inventoryID,
		ProductID:   productID,
		UnitID:      1,
		Status:      models.InventoryItemStatusActive,
		Quantity:    liveQty,
	}
	if err := tx.Create(&item).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}
	rec1 := newReconcileSub(t, tx, inventoryID, item.ID, day("2024-02-01"), "1000", "300")
	rec2 := newReconcileSub(t, tx, inventoryID, item.ID, day("2024-03-01"), "1100", "650")

	return &scopeTestEnv{db: db, ctx: ctx, inventoryID: inventoryID, itemID: item.ID, recIDs: []uint{rec1, rec2}}
}

// itemUpdateFor returns the plan's itemQtyUpdate for the given item id.
func itemUpdateFor(t *testing.T, plan *ResolutionPlan, itemID uint) itemQtyUpdate {
	t.Helper()
	for _, u := range plan.itemUpdates {
		if u.itemID == itemID {
			return u
		}
	}
	t.Fatalf("no itemUpdate for item %d", itemID)
	return itemQtyUpdate{}
}

// No trailing activity: live qty == Σ in-scope purchases (1100). Applied quantity
// must equal FinalStock (650) — the regression case (no behavior change).
func TestApplyQty_NoTrailingActivity_EqualsFinalStock(t *testing.T) {
	env := newApplyQtyEnv(t, dec("1100"))
	env.addPurchase(t, day("2024-01-01"), "1000", "0") // start stock
	env.addPurchase(t, day("2024-02-15"), "100", "0")  // in window

	plan, err := computeResolution(env.ctx, env.db, env.inventoryID, env.recIDs, nil)
	if err != nil {
		t.Fatalf("computeResolution: %v", err)
	}
	ip := plan.Items[0]
	if !ip.FinalStock.Equal(dec("650")) {
		t.Fatalf("FinalStock = %s, want 650", ip.FinalStock)
	}
	if !ip.CurrentQuantity.Equal(dec("1100")) {
		t.Fatalf("CurrentQuantity = %s, want 1100", ip.CurrentQuantity)
	}
	if !ip.AppliedQuantity.Equal(dec("650")) {
		t.Fatalf("AppliedQuantity = %s, want 650 (== FinalStock, no trailing activity)", ip.AppliedQuantity)
	}
	if !ip.AppliedQuantity.Equal(ip.FinalStock) {
		t.Fatalf("with no trailing activity AppliedQuantity (%s) must equal FinalStock (%s)", ip.AppliedQuantity, ip.FinalStock)
	}
	// preview == apply: the plan's itemUpdate writes exactly AppliedQuantity, and
	// the optimistic-lock baseline is the live quantity.
	u := itemUpdateFor(t, plan, env.itemID)
	if !u.newQty.Equal(ip.AppliedQuantity) {
		t.Fatalf("apply newQty (%s) != preview AppliedQuantity (%s)", u.newQty, ip.AppliedQuantity)
	}
	if !u.originalQty.Equal(dec("1100")) {
		t.Fatalf("optimistic-lock baseline = %s, want live qty 1100", u.originalQty)
	}
}

// Trailing net +277 (777 purchased after the last reconcile, 500 consumed) plus the
// in-window shrinkage drop. Live qty = 1100 + 277 = 1377. Applied quantity must be
// current - outbound = 1377 - 450 = 927, preserving the +277, NOT FinalStock (650).
func TestApplyQty_TrailingNet277_PreservesActivity(t *testing.T) {
	// live = in-scope purchases (1100) + trailing net (+777 - 500 = +277) = 1377.
	env := newApplyQtyEnv(t, dec("1377"))
	env.addPurchase(t, day("2024-01-01"), "1000", "0")  // start stock
	env.addPurchase(t, day("2024-02-15"), "100", "0")   // in window
	env.addPurchase(t, day("2024-06-01"), "777", "500") // trailing, partly consumed (out of scope)

	plan, err := computeResolution(env.ctx, env.db, env.inventoryID, env.recIDs, nil)
	if err != nil {
		t.Fatalf("computeResolution (trailing must not abort): %v", err)
	}
	ip := plan.Items[0]
	// FinalStock unchanged by trailing activity (resolver ignores the trailing purchase).
	if !ip.FinalStock.Equal(dec("650")) {
		t.Fatalf("FinalStock = %s, want 650", ip.FinalStock)
	}
	if !ip.CurrentQuantity.Equal(dec("1377")) {
		t.Fatalf("CurrentQuantity = %s, want 1377", ip.CurrentQuantity)
	}
	// Applied = current (1377) - outbound (450 sells) = 927 (= FinalStock 650 + 277).
	if !ip.AppliedQuantity.Equal(dec("927")) {
		t.Fatalf("AppliedQuantity = %s, want 927 (current 1377 - outbound 450, preserves +277)", ip.AppliedQuantity)
	}
	if ip.AppliedQuantity.Equal(ip.FinalStock) {
		t.Fatalf("AppliedQuantity (%s) must NOT equal FinalStock (%s) when there is trailing activity", ip.AppliedQuantity, ip.FinalStock)
	}
	// Overwriting with FinalStock would have dropped 277 of real stock.
	if !ip.AppliedQuantity.Sub(ip.FinalStock).Equal(dec("277")) {
		t.Fatalf("preserved net = %s, want 277", ip.AppliedQuantity.Sub(ip.FinalStock))
	}
	u := itemUpdateFor(t, plan, env.itemID)
	if !u.newQty.Equal(ip.AppliedQuantity) {
		t.Fatalf("apply newQty (%s) != preview AppliedQuantity (%s)", u.newQty, ip.AppliedQuantity)
	}
	if !u.originalQty.Equal(dec("1377")) {
		t.Fatalf("optimistic-lock baseline = %s, want live qty 1377", u.originalQty)
	}
}

// Sell + disposal together: the decrement must be sells + disposals total. Reuses
// the PurchaseAndDispose resolver scenario (start 1000, in-window +100, dispose 50
// in window, sub1 300 -> corrected 600, sub2 650). Sells = 400, disposals = 50,
// outbound = 450. With no trailing activity, live = 1100, applied = 650 = FinalStock.
func TestApplyQty_SellPlusDisposal_DecrementIsTotal(t *testing.T) {
	env := newApplyQtyEnv(t, dec("1100"))
	env.addPurchase(t, day("2024-01-01"), "1000", "0") // start stock
	env.addPurchase(t, day("2024-02-10"), "100", "0")  // in window
	dispID := newDisposeSub(t, env.db.WithContext(env.ctx), env.inventoryID, env.itemID, day("2024-02-20"), "50")

	plan, err := computeResolution(env.ctx, env.db, env.inventoryID, env.recIDs, []uint{dispID})
	if err != nil {
		t.Fatalf("computeResolution: %v", err)
	}
	ip := plan.Items[0]
	// Total synthesized outbound = sells (400) + disposals (50) = 450.
	sells, disposals := decimal.Zero, decimal.Zero
	for _, txn := range plan.Txns {
		switch txn.TransactionType {
		case string(models.InventoryTransactionTypeSell):
			sells = sells.Add(txn.Quantity)
		case string(models.InventoryTransactionTypeDisposal):
			disposals = disposals.Add(txn.Quantity)
		}
	}
	if !sells.Equal(dec("400")) || !disposals.Equal(dec("50")) {
		t.Fatalf("sells=%s disposals=%s, want 400/50", sells, disposals)
	}
	// Applied = current (1100) - (sells 400 + disposals 50) = 650.
	if !ip.AppliedQuantity.Equal(dec("650")) {
		t.Fatalf("AppliedQuantity = %s, want 650 (current 1100 - 450 total outbound)", ip.AppliedQuantity)
	}
	u := itemUpdateFor(t, plan, env.itemID)
	if !u.newQty.Equal(ip.CurrentQuantity.Sub(sells.Add(disposals))) {
		t.Fatalf("decrement must be sells+disposals: newQty=%s, want %s",
			u.newQty, ip.CurrentQuantity.Sub(sells.Add(disposals)))
	}
}

// newDisposeSub seeds a pending dispose submission for the item. The dispose
// payload shares QuantityItem's shape with reconcile (parseSub decodes both via
// ReconcileInventoryRequest); only the submission type differs.
func newDisposeSub(t *testing.T, tx *gorm.DB, inventoryID, itemID uint, at time.Time, qty string) uint {
	t.Helper()
	q := dec(qty)
	req := dto.DisposeInventoryRequest{
		InventoryID: inventoryID,
		Items:       []dto.QuantityItem{{InventoryItemID: itemID, Quantity: &q}},
	}
	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal dispose payload: %v", err)
	}
	sub := models.InventorySubmission{
		Base:           models.Base{CreatedAt: at},
		InventoryID:    inventoryID,
		SubmissionType: models.InventorySubmissionTypeDispose,
		ApprovalStatus: models.InventorySubmissionApprovalStatusPending,
		Payload:        payload,
	}
	if err := tx.Create(&sub).Error; err != nil {
		t.Fatalf("create dispose sub: %v", err)
	}
	return sub.ID
}
