package main

import (
	"context"
	"encoding/json"
	"fmt"
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

// These DB-backed (in-memory sqlite) tests exercise the one-off `reconcile dispose`
// subcommand: approve a single standalone pending dispose submission with disposal
// txns BACKDATED to the submission's created_at, replicating the app's FIFO
// dispose-approval (per-source COGS, source consumed_quantity bumps, item quantity
// decrement).

// disposeTestEnv seeds a minimal inventory (one product, one active item) plus a
// pending dispose submission, and lets each test attach purchase txns.
type disposeTestEnv struct {
	db           *gorm.DB
	ctx          context.Context
	inventoryID  uint
	itemID       uint
	submissionID uint
	submissionAt time.Time
}

func newDisposeTestEnv(t *testing.T, liveQty string, disposeQty string, disposeAt time.Time) *disposeTestEnv {
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

	const inventoryID uint = 1
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
		Quantity:    dec(liveQty),
	}
	if err := tx.Create(&item).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}

	subID := newDisposeSubAt(t, tx, inventoryID, item.ID, disposeAt, disposeQty)

	return &disposeTestEnv{
		db:           db,
		ctx:          ctx,
		inventoryID:  inventoryID,
		itemID:       item.ID,
		submissionID: subID,
		submissionAt: disposeAt,
	}
}

// newDisposeSubAt seeds a pending dispose submission for the item at the given time.
func newDisposeSubAt(t *testing.T, tx *gorm.DB, inventoryID, itemID uint, at time.Time, qty string) uint {
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

// addDispItemPurchase inserts a purchase txn for the item at a given time/price.
func (e *disposeTestEnv) addPurchase(t *testing.T, at time.Time, qty, consumed string, price float64) uint {
	t.Helper()
	txn := models.InventoryTransaction{
		Base:             models.Base{CreatedAt: at},
		InventoryItemID:  e.itemID,
		TransactionType:  models.InventoryTransactionTypePurchase,
		Price:            price,
		Quantity:         dec(qty),
		ConsumedQuantity: dec(consumed),
	}
	if err := e.db.WithContext(e.ctx).Create(&txn).Error; err != nil {
		t.Fatalf("create purchase txn: %v", err)
	}
	return txn.ID
}

// TestDispose_SpansTwoPurchases_BackdatedCOGS: a dispose of 150 spanning two
// purchases (100 @ price 10, then 200 @ price 12) must produce TWO disposal txns:
// 100 @ 10 from the older purchase, 50 @ 12 from the newer. Each backdated to the
// submission's created_at; each CounterTransactionID links the right source.
func TestDispose_SpansTwoPurchases_BackdatedCOGS(t *testing.T) {
	disposeAt := day("2024-03-15")
	env := newDisposeTestEnv(t, "300", "150", disposeAt)
	src1 := env.addPurchase(t, day("2024-01-01"), "100", "0", 10) // oldest
	src2 := env.addPurchase(t, day("2024-02-01"), "200", "0", 12) // newer

	plan, err := computeDisposePlan(env.ctx, env.db, env.inventoryID, env.submissionID)
	if err != nil {
		t.Fatalf("computeDisposePlan: %v", err)
	}

	if len(plan.Txns) != 2 {
		t.Fatalf("expected 2 disposal txns, got %d", len(plan.Txns))
	}
	// txns are sorted by item then date then source id; both same date -> by src id.
	t0, t1 := plan.Txns[0], plan.Txns[1]
	if t0.SourcePurchaseTxnID != src1 || !t0.Quantity.Equal(dec("100")) || t0.COGSPrice != 10 {
		t.Fatalf("txn0 = src %d qty %s cogs %.2f, want src %d qty 100 cogs 10", t0.SourcePurchaseTxnID, t0.Quantity, t0.COGSPrice, src1)
	}
	if t1.SourcePurchaseTxnID != src2 || !t1.Quantity.Equal(dec("50")) || t1.COGSPrice != 12 {
		t.Fatalf("txn1 = src %d qty %s cogs %.2f, want src %d qty 50 cogs 12", t1.SourcePurchaseTxnID, t1.Quantity, t1.COGSPrice, src2)
	}
	for _, txn := range plan.Txns {
		if txn.TransactionType != string(models.InventoryTransactionTypeDisposal) {
			t.Fatalf("txn type = %s, want disposal", txn.TransactionType)
		}
		if !txn.BackdatedDate.Equal(disposeAt) {
			t.Fatalf("txn backdated date = %s, want %s (submission created_at)", txn.BackdatedDate, disposeAt)
		}
	}

	// (b) Resulting item quantity is decremented by 150; consumed deltas: src1 +100, src2 +50.
	if !plan.Items[0].ResultingQty.Equal(dec("150")) {
		t.Fatalf("resulting qty = %s, want 150 (300 - 150)", plan.Items[0].ResultingQty)
	}
	u := plan.itemUpdates[0]
	if !u.originalQty.Equal(dec("300")) || !u.newQty.Equal(dec("150")) {
		t.Fatalf("item update = orig %s new %s, want orig 300 new 150", u.originalQty, u.newQty)
	}
	deltaByTxn := map[uint]decimal.Decimal{}
	for _, d := range plan.ConsumedDeltas {
		deltaByTxn[d.PurchaseTxnID] = d.Delta
	}
	if !deltaByTxn[src1].Equal(dec("100")) || !deltaByTxn[src2].Equal(dec("50")) {
		t.Fatalf("consumed deltas src1=%s src2=%s, want 100/50", deltaByTxn[src1], deltaByTxn[src2])
	}

	// Apply, then verify persisted state.
	if err := applyDisposePlan(env.ctx, env.db, plan); err != nil {
		t.Fatalf("applyDisposePlan: %v", err)
	}

	// (a) two disposal txns persisted with backdated created_at + correct counter links.
	var disposals []models.InventoryTransaction
	if err := env.db.Where("inventory_item_id = ? AND transaction_type = ?",
		env.itemID, models.InventoryTransactionTypeDisposal).
		Order("counter_transaction_id ASC").Find(&disposals).Error; err != nil {
		t.Fatalf("load disposals: %v", err)
	}
	if len(disposals) != 2 {
		t.Fatalf("persisted %d disposal txns, want 2", len(disposals))
	}
	for _, d := range disposals {
		if !d.CreatedAt.Equal(disposeAt) {
			t.Fatalf("persisted disposal created_at = %s, want backdated %s", d.CreatedAt, disposeAt)
		}
		if d.CounterTransactionID == nil {
			t.Fatalf("persisted disposal has nil CounterTransactionID")
		}
	}
	if *disposals[0].CounterTransactionID != src1 || !disposals[0].Quantity.Equal(dec("100")) {
		t.Fatalf("disposal0 src=%v qty=%s, want src1=%d qty 100", *disposals[0].CounterTransactionID, disposals[0].Quantity, src1)
	}
	if *disposals[1].CounterTransactionID != src2 || !disposals[1].Quantity.Equal(dec("50")) {
		t.Fatalf("disposal1 src=%v qty=%s, want src2=%d qty 50", *disposals[1].CounterTransactionID, disposals[1].Quantity, src2)
	}

	// (b) item quantity decremented; each source consumed_quantity bumped.
	var gotItem models.InventoryItem
	if err := env.db.First(&gotItem, env.itemID).Error; err != nil {
		t.Fatalf("reload item: %v", err)
	}
	if !gotItem.Quantity.Equal(dec("150")) {
		t.Fatalf("item quantity = %s, want 150", gotItem.Quantity)
	}
	var p1, p2 models.InventoryTransaction
	if err := env.db.First(&p1, src1).Error; err != nil {
		t.Fatalf("reload src1: %v", err)
	}
	if err := env.db.First(&p2, src2).Error; err != nil {
		t.Fatalf("reload src2: %v", err)
	}
	if !p1.ConsumedQuantity.Equal(dec("100")) {
		t.Fatalf("src1 consumed_quantity = %s, want 100", p1.ConsumedQuantity)
	}
	if !p2.ConsumedQuantity.Equal(dec("50")) {
		t.Fatalf("src2 consumed_quantity = %s, want 50", p2.ConsumedQuantity)
	}
	// purchase created_at/created_by untouched (column-scoped update).
	if !p1.CreatedAt.Equal(day("2024-01-01")) {
		t.Fatalf("src1 created_at changed to %s, want unchanged 2024-01-01", p1.CreatedAt)
	}

	// (c) submission approved + completed.
	var gotSub models.InventorySubmission
	if err := env.db.First(&gotSub, env.submissionID).Error; err != nil {
		t.Fatalf("reload submission: %v", err)
	}
	if gotSub.ApprovalStatus != models.InventorySubmissionApprovalStatusApproved {
		t.Fatalf("submission approval_status = %s, want approved", gotSub.ApprovalStatus)
	}
	if gotSub.ProcessingStatus != models.InventorySubmissionStatusCompleted {
		t.Fatalf("submission processing_status = %s, want completed", gotSub.ProcessingStatus)
	}
}

// TestDispose_PostDatedFIFOSource_ErrorsAndWritesNothing: the temporal-FIFO guard.
// On-or-before-submission stock is INSUFFICIENT for the requested qty, so FIFO must
// reach a later purchase whose created_at is AFTER the submission's created_at.
// Backdating that disposal to the submission date would place it before its source
// stock existed, so computeDisposePlan must abort (naming the post-dated source) and
// write nothing.
func TestDispose_PostDatedFIFOSource_ErrorsAndWritesNothing(t *testing.T) {
	// Submission at an explicit instant so the boundary is unambiguous.
	disposeAt := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	// Live qty 150 (requested) but only 100 from a pre-submission purchase; the
	// remaining 50 sits in a purchase created AFTER the submission.
	env := newDisposeTestEnv(t, "150", "150", disposeAt)
	env.addPurchase(t, day("2024-01-01"), "100", "0", 10)                     // pre-submission (older)
	postSrc := env.addPurchase(t, disposeAt.Add(time.Second), "200", "0", 12) // AFTER submission

	_, err := computeDisposePlan(env.ctx, env.db, env.inventoryID, env.submissionID)
	if err == nil {
		t.Fatalf("expected temporal-FIFO error, got nil")
	}
	// Error must name the post-dated source stock layer txn id.
	if want := fmt.Sprintf("stock layer txn %d", postSrc); !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not mention post-dated source %q", err.Error(), want)
	}

	// Nothing written: no disposal txns, item quantity unchanged, submission still pending.
	var disposalCount int64
	if err := env.db.Model(&models.InventoryTransaction{}).
		Where("transaction_type = ?", models.InventoryTransactionTypeDisposal).
		Count(&disposalCount).Error; err != nil {
		t.Fatalf("count disposals: %v", err)
	}
	if disposalCount != 0 {
		t.Fatalf("expected 0 disposal txns after temporal-FIFO abort, got %d", disposalCount)
	}
	var gotItem models.InventoryItem
	if err := env.db.First(&gotItem, env.itemID).Error; err != nil {
		t.Fatalf("reload item: %v", err)
	}
	if !gotItem.Quantity.Equal(dec("150")) {
		t.Fatalf("item quantity = %s, want unchanged 150", gotItem.Quantity)
	}
	var gotSub models.InventorySubmission
	if err := env.db.First(&gotSub, env.submissionID).Error; err != nil {
		t.Fatalf("reload submission: %v", err)
	}
	if gotSub.ApprovalStatus != models.InventorySubmissionApprovalStatusPending {
		t.Fatalf("submission approval_status = %s, want still pending", gotSub.ApprovalStatus)
	}
}

// TestDispose_SourceExactlyOnSubmission_Allowed: a source purchase created EXACTLY at
// the submission's created_at is NOT "after" it, so the temporal-FIFO guard must not
// fire and the plan is produced normally (boundary case). A later purchase that FIFO
// never reaches must also not trip the guard.
func TestDispose_SourceExactlyOnSubmission_Allowed(t *testing.T) {
	disposeAt := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	env := newDisposeTestEnv(t, "300", "100", disposeAt)
	onSrc := env.addPurchase(t, disposeAt, "100", "0", 10)       // exactly ON submission -> allowed
	env.addPurchase(t, disposeAt.Add(time.Hour), "200", "0", 12) // later, never reached by FIFO

	plan, err := computeDisposePlan(env.ctx, env.db, env.inventoryID, env.submissionID)
	if err != nil {
		t.Fatalf("computeDisposePlan: %v (source exactly on submission must be allowed)", err)
	}
	if len(plan.Txns) != 1 {
		t.Fatalf("expected 1 disposal txn, got %d", len(plan.Txns))
	}
	if plan.Txns[0].SourcePurchaseTxnID != onSrc || !plan.Txns[0].Quantity.Equal(dec("100")) {
		t.Fatalf("txn = src %d qty %s, want src %d qty 100", plan.Txns[0].SourcePurchaseTxnID, plan.Txns[0].Quantity, onSrc)
	}
}

// TestDispose_OverDraw_ErrorsAndWritesNothing: a dispose qty greater than available
// stock must return an error from computeDisposePlan and persist nothing.
func TestDispose_OverDraw_ErrorsAndWritesNothing(t *testing.T) {
	env := newDisposeTestEnv(t, "100", "150", day("2024-03-15"))
	env.addPurchase(t, day("2024-01-01"), "100", "0", 10)

	_, err := computeDisposePlan(env.ctx, env.db, env.inventoryID, env.submissionID)
	if err == nil {
		t.Fatalf("expected over-draw error, got nil")
	}

	// Nothing written: no disposal txns, item quantity unchanged, submission still pending.
	var disposalCount int64
	if err := env.db.Model(&models.InventoryTransaction{}).
		Where("transaction_type = ?", models.InventoryTransactionTypeDisposal).
		Count(&disposalCount).Error; err != nil {
		t.Fatalf("count disposals: %v", err)
	}
	if disposalCount != 0 {
		t.Fatalf("expected 0 disposal txns after over-draw, got %d", disposalCount)
	}
	var gotItem models.InventoryItem
	if err := env.db.First(&gotItem, env.itemID).Error; err != nil {
		t.Fatalf("reload item: %v", err)
	}
	if !gotItem.Quantity.Equal(dec("100")) {
		t.Fatalf("item quantity = %s, want unchanged 100", gotItem.Quantity)
	}
	var gotSub models.InventorySubmission
	if err := env.db.First(&gotSub, env.submissionID).Error; err != nil {
		t.Fatalf("reload submission: %v", err)
	}
	if gotSub.ApprovalStatus != models.InventorySubmissionApprovalStatusPending {
		t.Fatalf("submission approval_status = %s, want still pending", gotSub.ApprovalStatus)
	}
}
