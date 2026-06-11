package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
)

// These DB-backed (in-memory sqlite) tests exercise the #51 fix: the
// unconsumed-ledger assumption check in computeResolution must be SCOPED to the
// purchases the resolver actually replays (start-stock + within-window). A
// TRAILING purchase (dated after the item's last in-scope reconcile, ignored by
// resolveItem per #50) is exempt, even when it has consumed_quantity > 0. An
// IN-SCOPE purchase with consumed_quantity > 0 must still hard-fail.

// scopeTestEnv seeds a minimal inventory (one product/item, two reconcile subs)
// against an in-memory sqlite DB and lets each test attach purchase txns.
type scopeTestEnv struct {
	db          *gorm.DB
	ctx         context.Context
	inventoryID uint
	itemID      uint
	recIDs      []uint
}

func newScopeTestEnv(t *testing.T) *scopeTestEnv {
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
		Quantity:    decimal.NewFromInt(650),
	}
	if err := tx.Create(&item).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}

	// Two reconcile submissions forming a single window:
	//   sub1 @ 2024-02-01: prev 1000, counted 300
	//   sub2 @ 2024-03-01: prev 1100, counted 650
	// (mirrors the resolver S2 baseline used elsewhere).
	rec1 := newReconcileSub(t, tx, inventoryID, item.ID, day("2024-02-01"), "1000", "300")
	rec2 := newReconcileSub(t, tx, inventoryID, item.ID, day("2024-03-01"), "1100", "650")

	return &scopeTestEnv{
		db:          db,
		ctx:         ctx,
		inventoryID: inventoryID,
		itemID:      item.ID,
		recIDs:      []uint{rec1, rec2},
	}
}

func newReconcileSub(t *testing.T, tx *gorm.DB, inventoryID, itemID uint, at time.Time, prev, qty string) uint {
	t.Helper()
	q := dec(qty)
	req := dto.ReconcileInventoryRequest{
		InventoryID: inventoryID,
		Items: []dto.QuantityItem{{
			InventoryItemID: itemID,
			Quantity:        &q,
			PrevQuantity:    dec(prev),
		}},
	}
	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	sub := models.InventorySubmission{
		Base:           models.Base{CreatedAt: at},
		InventoryID:    inventoryID,
		SubmissionType: models.InventorySubmissionTypeReconcile,
		ApprovalStatus: models.InventorySubmissionApprovalStatusPending,
		Payload:        payload,
	}
	if err := tx.Create(&sub).Error; err != nil {
		t.Fatalf("create reconcile sub: %v", err)
	}
	return sub.ID
}

// addPurchase inserts a purchase txn for the item with the given created_at and
// consumed_quantity.
func (e *scopeTestEnv) addPurchase(t *testing.T, at time.Time, qty, consumed string) uint {
	t.Helper()
	txn := models.InventoryTransaction{
		Base:             models.Base{CreatedAt: at},
		InventoryItemID:  e.itemID,
		TransactionType:  models.InventoryTransactionTypePurchase,
		Price:            10,
		Quantity:         dec(qty),
		ConsumedQuantity: dec(consumed),
	}
	if err := e.db.WithContext(e.ctx).Create(&txn).Error; err != nil {
		t.Fatalf("create purchase txn: %v", err)
	}
	return txn.ID
}

// TestComputeResolution_TrailingConsumedPurchaseDoesNotAbort: a trailing purchase
// (after the last reconcile) with consumed_quantity > 0 must NOT trip the
// unconsumed-ledger assertion. It is ignored by the resolver, so the correction
// must be identical to the windows-only baseline (start 1000, sub1 -> 550,
// sub2 -> 650).
func TestComputeResolution_TrailingConsumedPurchaseDoesNotAbort(t *testing.T) {
	// Baseline: only in-scope purchases (start-stock + in-window), no consumption.
	base := newScopeTestEnv(t)
	base.addPurchase(t, day("2024-01-01"), "1000", "0") // start stock (<= first reconcile)
	base.addPurchase(t, day("2024-02-15"), "100", "0")  // in window (sub1, sub2]

	basePlan, err := computeResolution(base.ctx, base.db, base.inventoryID, base.recIDs, nil)
	if err != nil {
		t.Fatalf("baseline computeResolution unexpected error: %v", err)
	}

	// With trailing: same in-scope purchases PLUS a trailing purchase (after sub2)
	// that has been consumed by normal post-reconcile activity.
	env := newScopeTestEnv(t)
	env.addPurchase(t, day("2024-01-01"), "1000", "0")
	env.addPurchase(t, day("2024-02-15"), "100", "0")
	trailingID := env.addPurchase(t, day("2024-06-01"), "777", "500") // trailing, partly consumed

	plan, err := computeResolution(env.ctx, env.db, env.inventoryID, env.recIDs, nil)
	if err != nil {
		t.Fatalf("trailing consumed purchase must NOT abort (it is out of scope), got: %v", err)
	}

	if len(plan.Items) != 1 {
		t.Fatalf("got %d item plans, want 1", len(plan.Items))
	}
	ip := plan.Items[0]
	bip := basePlan.Items[0]

	if !ip.StartStock.Equal(dec("1000")) || !ip.StartStock.Equal(bip.StartStock) {
		t.Fatalf("start stock = %s, want 1000 (== baseline %s); trailing purchase must be ignored",
			ip.StartStock, bip.StartStock)
	}
	if !ip.FinalStock.Equal(bip.FinalStock) {
		t.Fatalf("final stock = %s, want == baseline %s", ip.FinalStock, bip.FinalStock)
	}
	if len(ip.Steps) != len(bip.Steps) {
		t.Fatalf("got %d steps, want %d (== baseline)", len(ip.Steps), len(bip.Steps))
	}
	for i := range ip.Steps {
		if !ip.Steps[i].CorrectedQuantity.Equal(bip.Steps[i].CorrectedQuantity) ||
			ip.Steps[i].Corrected != bip.Steps[i].Corrected ||
			!ip.Steps[i].Drop.Equal(bip.Steps[i].Drop) {
			t.Fatalf("step %d differs from windows-only baseline: got corrected=%s/%v drop=%s; want corrected=%s/%v drop=%s",
				i, ip.Steps[i].CorrectedQuantity, ip.Steps[i].Corrected, ip.Steps[i].Drop,
				bip.Steps[i].CorrectedQuantity, bip.Steps[i].Corrected, bip.Steps[i].Drop)
		}
	}

	// The trailing purchase must contribute nothing to the consumed-delta plan.
	for _, d := range plan.ConsumedDeltas {
		if d.PurchaseTxnID == trailingID {
			t.Fatalf("trailing purchase %d must not appear in consumed deltas", trailingID)
		}
	}
}

// TestComputeResolution_InScopeConsumedPurchaseStillFails: an IN-SCOPE purchase
// (start-stock or within a window) with consumed_quantity > 0 violates the
// FIFO-replay assumption and MUST still hard-fail the whole run.
func TestComputeResolution_InScopeConsumedPurchaseStillFails(t *testing.T) {
	// Case A: start-stock purchase consumed.
	t.Run("start_stock_consumed", func(t *testing.T) {
		env := newScopeTestEnv(t)
		startID := env.addPurchase(t, day("2024-01-01"), "1000", "5") // <= first reconcile, consumed
		env.addPurchase(t, day("2024-02-15"), "100", "0")

		_, err := computeResolution(env.ctx, env.db, env.inventoryID, env.recIDs, nil)
		if err == nil {
			t.Fatalf("in-scope (start-stock) consumed purchase must hard-fail, got nil error")
		}
		assertAssumptionFailure(t, err, startID)
	})

	// Case B: within-window purchase consumed.
	t.Run("within_window_consumed", func(t *testing.T) {
		env := newScopeTestEnv(t)
		env.addPurchase(t, day("2024-01-01"), "1000", "0")
		windowID := env.addPurchase(t, day("2024-02-15"), "100", "7") // in (sub1, sub2], consumed

		_, err := computeResolution(env.ctx, env.db, env.inventoryID, env.recIDs, nil)
		if err == nil {
			t.Fatalf("in-scope (within-window) consumed purchase must hard-fail, got nil error")
		}
		assertAssumptionFailure(t, err, windowID)
	})
}

func assertAssumptionFailure(t *testing.T, err error, txnID uint) {
	t.Helper()
	msg := err.Error()
	if !strings.Contains(msg, "assumption failed") || !strings.Contains(msg, "consumed_quantity") {
		t.Fatalf("expected unconsumed-ledger assumption failure, got: %v", err)
	}
	// Ensure the failure names the offending in-scope txn, not a different one.
	if !strings.Contains(msg, "txn ") {
		t.Fatalf("expected error to reference the offending purchase txn, got: %v", err)
	}
	_ = txnID
}
