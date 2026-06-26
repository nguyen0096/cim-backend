package apptest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/shopspring/decimal"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil/fixture"
)

// These specs exercise the reconciliation Start-Processing apply (epic #38, Part
// 6 redesign) against a REAL Postgres: the atomic, advisory-locked transaction
// that performs the event-based drift re-check and the snapshot-aware consume
// (snapshot − counted, Reading B). They confirm the data-correctness bar: the
// right Sell is booked, processed_at is stamped, statuses finalize, a consuming
// sibling processed in the window rolls the apply back with a warning payload,
// and a received-PO-during-reconcile (additive, never a submission) does NOT trip
// the gate and SURVIVES on top of the counted figure.
var _ = Describe("Reconciliation start-processing apply", func() {
	const staffEmail = "p6-staff@cim.local"
	const adminEmail = "p6-admin@cim.local"

	var (
		svc       services.InventoryService
		staffCtx  context.Context
		adminCtx  context.Context
		inventory *models.Inventory
		itm       *models.InventoryItem
		sub       *models.InventorySubmission
		supplier  *models.Supplier
	)

	buildService := func() services.InventoryService {
		base := repository.NewBaseRepository(tenv.DB)
		return services.NewInventoryService(
			repository.NewInventoryRepository(base),
			repository.NewInventoryItemRepository(base),
			repository.NewInventorySubmissionRepository(base),
			repository.NewReconciliationSnapshotRepository(base),
			repository.NewReconciliationRequestItemRepository(base),
			repository.NewProductRepository(base),
			nil,
			base,
			tenv.DB,
		)
	}

	staffPerms := func(email string) context.Context {
		ctx := pkg.WithUserEmail(context.Background(), email)
		perms := map[pkg.UserPermission]struct{}{
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemCreate}: {},
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemUpdate}: {},
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemDelete}: {},
		}
		return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
	}
	adminPerms := func(email string) context.Context {
		ctx := pkg.WithUserEmail(context.Background(), email)
		perms := map[pkg.UserPermission]struct{}{
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconManage}: {},
		}
		return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
	}

	// setStock sets the live quantity on the inventory item directly (models a PO
	// receipt that raised stock outside the submissions table during the count).
	setStock := func(q int64) {
		Expect(tenv.ContextfulDB().Model(itm).Update("quantity", decimal.NewFromInt(q)).Error).NotTo(HaveOccurred())
	}

	countItems := func(q int64) []dto.ReconciliationCountItem {
		qty := decimal.NewFromInt(q)
		return []dto.ReconciliationCountItem{{InventoryItemID: itm.ID, Quantity: &qty}}
	}

	itemQty := func() decimal.Decimal {
		var reloaded models.InventoryItem
		Expect(tenv.ContextfulDB().First(&reloaded, itm.ID).Error).NotTo(HaveOccurred())
		return reloaded.Quantity
	}

	BeforeEach(func() {
		svc = buildService()
		staffCtx = staffPerms(staffEmail)
		adminCtx = adminPerms(adminEmail)

		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]

		inventory = fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("p6-inv-%s", suffix),
			Location: fmt.Sprintf("p6-loc-%s", suffix),
		})
		supplier = fixture.WithSupplier(db, models.Supplier{Name: fmt.Sprintf("p6-sup-%s", suffix)})
		unit := fixture.WithUnit(db, fixture.ValidBaseUnit())
		product := fixture.WithProduct(db, fixture.ValidProduct(unit.ID))

		// Live stock starts at 100, backed by a single purchase transaction so FIFO
		// has something to consume.
		itm = &models.InventoryItem{
			InventoryID: inventory.ID,
			ProductID:   product.ID,
			UnitID:      unit.ID,
			Quantity:    decimal.NewFromInt(100),
			Status:      models.InventoryItemStatusActive,
		}
		Expect(db.Create(itm).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", itm.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(itm)
		})

		purchase := &models.InventoryTransaction{
			InventoryItemID: itm.ID,
			SupplierID:      &supplier.ID,
			TransactionType: models.InventoryTransactionTypePurchase,
			Price:           10.0,
			Quantity:        decimal.NewFromInt(100),
		}
		Expect(db.Create(purchase).Error).NotTo(HaveOccurred())

		// Initiated reconcile: snapshot baseline = 100, lifecycle open.
		sub = &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   models.InventorySubmissionTypeReconcile,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
			ReconcileStatus:  models.ReconcileLifecycleStatusOpen,
		}
		Expect(db.Create(sub).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(sub) })

		snap := &models.ReconciliationSnapshot{
			SubmissionID:    sub.ID,
			InventoryItemID: itm.ID,
			PrevQuantity:    decimal.NewFromInt(100),
		}
		Expect(db.Create(snap).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(snap) })
	})

	It("applies snapshot − counted and finalizes the submission (happy path)", func() {
		// Counted 60 against snapshot 100 -> consume 40 -> live 100 - 40 = 60.
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: sub.ID, Items: countItems(60),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })

		_, err = svc.CloseReconciliation(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())

		res, err := svc.StartProcessing(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.DriftDetected).To(BeFalse())
		Expect(res.Submission).NotTo(BeNil())
		Expect(res.Submission.ReconcileStatus).To(Equal(models.ReconcileLifecycleStatusProcessed))
		Expect(res.Submission.ProcessingStatus).To(Equal(models.InventorySubmissionStatusCompleted))
		Expect(res.Submission.ApprovalStatus).To(Equal(models.InventorySubmissionApprovalStatusApproved))
		Expect(res.Submission.ProcessedAt).NotTo(BeNil())

		Expect(itemQty().Equal(decimal.NewFromInt(60))).To(BeTrue(), "live should be 60 (100 - consume 40), got %s", itemQty())

		// A Sell of 40 was booked.
		var sells []models.InventoryTransaction
		Expect(tenv.ContextfulDB().Where("inventory_item_id = ? AND transaction_type = ?", itm.ID, models.InventoryTransactionTypeSell).Find(&sells).Error).NotTo(HaveOccurred())
		var sold decimal.Decimal
		for _, s := range sells {
			sold = sold.Add(s.Quantity)
		}
		Expect(sold.Equal(decimal.NewFromInt(40))).To(BeTrue(), "Sell total should be snapshot-counted = 40, got %s", sold)
	})

	// AUDIT GAP 3 (PARTIAL -> explicit): double-apply must be blocked. After a
	// successful StartProcessing the submission is `processed` (approval_status flips
	// to approved by MarkProcessed), so a SECOND StartProcessing is rejected at the
	// loadActiveReconcileParent in-flight guard (approval_status != pending ->
	// ErrReconParentNotInFlight, a 409/conflict) BEFORE the drift re-check or apply.
	// It must book NO additional Sell and leave stock unchanged. This directly
	// exercises the processed->processing edge the lifecycle guard protects, which
	// the suite previously covered only transitively via "rejects start-processing
	// from open".
	It("rejects a second StartProcessing after a successful apply (no double consume)", func() {
		// First apply: counted 60 against snapshot 100 -> consume 40 -> live 60.
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: sub.ID, Items: countItems(60),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })

		_, err = svc.CloseReconciliation(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())

		res, err := svc.StartProcessing(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Submission).NotTo(BeNil())
		Expect(res.Submission.ReconcileStatus).To(Equal(models.ReconcileLifecycleStatusProcessed))
		Expect(itemQty().Equal(decimal.NewFromInt(60))).To(BeTrue(), "first apply consumes 40 -> live 60")

		// Count the consuming Sell transactions booked by the first (and only valid) apply.
		sellCount := func() int64 {
			var n int64
			Expect(tenv.ContextfulDB().Model(&models.InventoryTransaction{}).
				Where("inventory_item_id = ? AND transaction_type = ?", itm.ID, models.InventoryTransactionTypeSell).
				Count(&n).Error).NotTo(HaveOccurred())
			return n
		}
		firstSells := sellCount()
		Expect(firstSells).To(BeNumerically(">", 0), "the first apply booked at least one Sell")

		// SECOND StartProcessing on the now-processed submission must be rejected.
		res2, err := svc.StartProcessing(adminCtx, sub.ID)
		Expect(err).To(HaveOccurred(), "a second StartProcessing must be rejected (not re-applied)")
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("%d", sub.ID)))
		Expect(res2).To(BeNil())

		// No double consume: stock unchanged and no additional Sell booked.
		Expect(itemQty().Equal(decimal.NewFromInt(60))).To(BeTrue(),
			"the rejected second apply must not consume again; live stays 60")
		Expect(sellCount()).To(Equal(firstSells), "no additional Sell transactions on the rejected second apply")

		// The submission is unchanged: still processed/completed/approved.
		var reloaded models.InventorySubmission
		Expect(tenv.ContextfulDB().First(&reloaded, sub.ID).Error).NotTo(HaveOccurred())
		Expect(reloaded.ReconcileStatus).To(Equal(models.ReconcileLifecycleStatusProcessed))
		Expect(reloaded.ProcessingStatus).To(Equal(models.InventorySubmissionStatusCompleted))
		Expect(reloaded.ApprovalStatus).To(Equal(models.InventorySubmissionApprovalStatusApproved))
	})

	It("keeps a PO received during the count (snapshot-aware, Reading B): consume=snapshot-counted, PO survives", func() {
		// Snapshot 100, counted 50. A PO of +30 arrives during the count -> live 130.
		// Reading B: consume = snapshot(100) - counted(50) = 50; remaining = live(130)
		// - 50 = 80 (the +30 PO survives on top of the counted 50). NOT counted=50.
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: sub.ID, Items: countItems(50),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })

		// PO receipt during the count: raise live to 130 and add a backing purchase
		// txn so FIFO has 130 available.
		setStock(130)
		extraPO := &models.InventoryTransaction{
			InventoryItemID: itm.ID,
			SupplierID:      &supplier.ID,
			TransactionType: models.InventoryTransactionTypePurchase,
			Price:           10.0,
			Quantity:        decimal.NewFromInt(30),
		}
		Expect(tenv.ContextfulDB().Create(extraPO).Error).NotTo(HaveOccurred())

		_, err = svc.CloseReconciliation(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())

		res, err := svc.StartProcessing(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.DriftDetected).To(BeFalse())
		Expect(itemQty().Equal(decimal.NewFromInt(80))).To(BeTrue(),
			"live should be 130 - (100-50)=80; the +30 PO survives, got %s", itemQty())
	})

	It("rolls back with a warning payload when a consuming submission processed in the window (drift)", func() {
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: sub.ID, Items: countItems(60),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })

		// A sibling DISPOSE for the same inventory, approved + processed (completed)
		// AFTER the reconcile snapshot capture (processed_at = now).
		drift := &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   models.InventorySubmissionTypeDispose,
			ProcessingStatus: models.InventorySubmissionStatusCompleted,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusApproved,
		}
		now := time.Now()
		drift.ProcessedAt = &now
		Expect(tenv.ContextfulDB().Create(drift).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(drift) })

		_, err = svc.CloseReconciliation(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())

		res, err := svc.StartProcessing(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.DriftDetected).To(BeTrue())
		Expect(res.Warnings).NotTo(BeEmpty())
		Expect(res.Submission).To(BeNil())

		// Nothing applied: stock unchanged, submission still closed (rolled back).
		Expect(itemQty().Equal(decimal.NewFromInt(100))).To(BeTrue(), "drift must roll back the apply; stock unchanged")
		var reloaded models.InventorySubmission
		Expect(tenv.ContextfulDB().First(&reloaded, sub.ID).Error).NotTo(HaveOccurred())
		Expect(reloaded.ReconcileStatus).To(Equal(models.ReconcileLifecycleStatusClosed))
		Expect(reloaded.ProcessedAt).To(BeNil())
	})

	It("does NOT trip the gate for a sibling processed BEFORE the window", func() {
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: sub.ID, Items: countItems(60),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })

		// A consuming dispose processed 2h BEFORE the reconcile snapshot (created_at).
		pre := &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   models.InventorySubmissionTypeDispose,
			ProcessingStatus: models.InventorySubmissionStatusCompleted,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusApproved,
		}
		before := sub.CreatedAt.Add(-2 * time.Hour)
		pre.ProcessedAt = &before
		Expect(tenv.ContextfulDB().Create(pre).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(pre) })

		_, err = svc.CloseReconciliation(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())

		res, err := svc.StartProcessing(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.DriftDetected).To(BeFalse(), "a pre-window consuming submission must not trip the gate")
		Expect(itemQty().Equal(decimal.NewFromInt(60))).To(BeTrue())
	})

	// FINDING 2 (Codex P2): the drift window must open at the SNAPSHOT-CAPTURE
	// instant, not the parent submission's created_at. InitiateReconcile inserts the
	// parent row FIRST, then acquires the advisory lock, then captures snapshots — so
	// a consuming apply can stamp processed_at AFTER parent.CreatedAt yet COMMIT
	// BEFORE the snapshot read, leaving its effect already in the baseline. With the
	// old parent.CreatedAt lower bound that sibling was a FALSE drift; with the
	// snapshot-capture lower bound it is correctly excluded. We reproduce the timing
	// gap deterministically by stamping the snapshot's created_at LATER than the
	// parent's and placing the sibling's processed_at strictly between the two.
	It("does NOT trip the gate for a sibling processed AFTER parent.CreatedAt but BEFORE the snapshot capture (P2)", func() {
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: sub.ID, Items: countItems(60),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })

		// Model the initiate ordering: parent created at T0, advisory lock acquired,
		// snapshot captured at T0+1h (the real flow captures after the lock). Force the
		// snapshot's created_at to T0+1h so GetSnapshotCapturedAt returns the true
		// post-lock capture instant rather than the parent's earlier created_at.
		capturedAt := sub.CreatedAt.Add(1 * time.Hour)
		Expect(tenv.ContextfulDB().Model(&models.ReconciliationSnapshot{}).
			Where("submission_id = ?", sub.ID).
			Update("created_at", capturedAt).Error).NotTo(HaveOccurred())

		// A consuming dispose that committed BEFORE the snapshot read (its stock effect
		// is already in the baseline) but stamped processed_at AFTER parent.CreatedAt:
		// T0+30m, strictly inside (parent.CreatedAt, capturedAt). Old window => false
		// drift; new window => correctly excluded.
		inGap := sub.CreatedAt.Add(30 * time.Minute)
		sibling := &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   models.InventorySubmissionTypeDispose,
			ProcessingStatus: models.InventorySubmissionStatusCompleted,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusApproved,
		}
		sibling.ProcessedAt = &inGap
		Expect(tenv.ContextfulDB().Create(sibling).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(sibling) })

		_, err = svc.CloseReconciliation(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())

		res, err := svc.StartProcessing(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.DriftDetected).To(BeFalse(),
			"a sibling already in the baseline (committed before snapshot capture) must NOT be flagged as drift")
		Expect(itemQty().Equal(decimal.NewFromInt(60))).To(BeTrue(), "the apply proceeds; live = 100 - (100-60) = 60")
	})

	// FINDING 2 companion: a sibling that committed AFTER the snapshot capture IS
	// drift, even when its processed_at is later than the (earlier) parent.CreatedAt.
	// This pins the upper invariant: the new window-start neither over- nor
	// under-counts relative to the true capture instant.
	It("DOES trip the gate for a sibling processed AFTER the snapshot capture (P2 upper bound)", func() {
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: sub.ID, Items: countItems(60),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })

		capturedAt := sub.CreatedAt.Add(1 * time.Hour)
		Expect(tenv.ContextfulDB().Model(&models.ReconciliationSnapshot{}).
			Where("submission_id = ?", sub.ID).
			Update("created_at", capturedAt).Error).NotTo(HaveOccurred())

		// processed_at AFTER the capture instant => a real post-snapshot consume => drift.
		afterCapture := capturedAt.Add(5 * time.Minute)
		sibling := &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   models.InventorySubmissionTypeDispose,
			ProcessingStatus: models.InventorySubmissionStatusCompleted,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusApproved,
		}
		sibling.ProcessedAt = &afterCapture
		Expect(tenv.ContextfulDB().Create(sibling).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(sibling) })

		_, err = svc.CloseReconciliation(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())

		res, err := svc.StartProcessing(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.DriftDetected).To(BeTrue(),
			"a sibling that committed after the snapshot capture must be flagged as drift")
		Expect(itemQty().Equal(decimal.NewFromInt(100))).To(BeTrue(), "drift rolls the apply back; stock unchanged")
	})

	It("does NOT trip the gate for an approved-but-unprocessed consuming sibling", func() {
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: sub.ID, Items: countItems(60),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })

		// Approved but NOT processed (no consuming txn created -> no real drift):
		// processing_status pending, processed_at nil.
		unproc := &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   models.InventorySubmissionTypeDispose,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusApproved,
		}
		Expect(tenv.ContextfulDB().Create(unproc).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(unproc) })

		_, err = svc.CloseReconciliation(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())

		res, err := svc.StartProcessing(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.DriftDetected).To(BeFalse(), "an approved-but-unprocessed sibling created no consuming txn")
	})

	It("serializes Start-Processing behind a held per-inventory advisory lock", func() {
		// Deterministic proof of the advisory-lock serialization that closes the
		// TOCTOU: a separate transaction holds pg_advisory_xact_lock(inventory_id)
		// (exactly what the consuming ProcessSubmission apply takes). StartProcessing
		// must BLOCK on that lock and only proceed once the holder commits — so a
		// concurrent consuming apply can never interleave between the drift re-check
		// and the apply.
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: sub.ID, Items: countItems(60),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })
		_, err = svc.CloseReconciliation(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())

		// Hold the advisory lock for this inventory in a separate, still-open tx.
		holder := tenv.DB.Begin()
		Expect(holder.Error).NotTo(HaveOccurred())
		Expect(holder.Exec("SELECT pg_advisory_xact_lock(?)", int64(inventory.ID)).Error).NotTo(HaveOccurred())

		done := make(chan struct{})
		var spErr error
		go func() {
			defer close(done)
			_, spErr = svc.StartProcessing(adminCtx, sub.ID)
		}()

		// While the lock is held, StartProcessing must be blocked (not yet done).
		Consistently(func() bool {
			select {
			case <-done:
				return true
			default:
				return false
			}
		}, "1s", "100ms").Should(BeFalse(), "StartProcessing must block while the advisory lock is held")

		// Release the lock; StartProcessing now proceeds and completes.
		Expect(holder.Rollback().Error).NotTo(HaveOccurred())
		Eventually(done, "5s").Should(BeClosed())
		Expect(spErr).NotTo(HaveOccurred())
		Expect(itemQty().Equal(decimal.NewFromInt(60))).To(BeTrue())
	})

	// FINDING 1 (Codex P1): a consuming apply that fails AFTER the approval-status
	// write — now that SaveInventoryItemChanges enlists in the approve tx — must
	// roll the WHOLE transaction back. No partial stock mutation may commit, the
	// submission must NOT be flipped to approved, and no processed_at may be
	// stamped. (Before the fix, processSubmission swallowed the error into
	// applied==false and the closure returned nil, committing the approval flip
	// while the failed status was recorded out-of-band.)
	It("rolls the approve transaction back when the consuming apply fails (no partial commit)", func() {
		approveCtx := func() context.Context {
			ctx := pkg.WithUserEmail(context.Background(), adminEmail)
			perms := map[pkg.UserPermission]struct{}{
				{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionApprove}: {},
			}
			return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
		}

		// A DISPOSE submission whose payload consumes MORE than the live stock (100).
		// consumeFIFO rejects it at APPLY time — i.e. after UpdateApprovalStatus has
		// already written `approved` into the open tx. We build the payload directly
		// so it bypasses CreateDisposeSubmission's pre-validation and reaches the apply.
		over := decimal.NewFromInt(1000)
		payload, err := json.Marshal(dto.DisposeInventoryRequest{
			InventoryID: inventory.ID,
			Items:       []dto.QuantityItem{{InventoryItemID: itm.ID, Quantity: &over}},
		})
		Expect(err).NotTo(HaveOccurred())

		dispose := &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   models.InventorySubmissionTypeDispose,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
			Payload:          payload,
		}
		Expect(tenv.ContextfulDB().Create(dispose).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(dispose) })

		_, err = svc.ProcessSubmission(approveCtx(), dto.SubmissionApprovalRequest{
			SubmissionID: dispose.ID,
			Action:       string(models.InventorySubmissionActionApprove),
		})
		Expect(err).To(HaveOccurred(), "a failing consuming apply must surface an error, not a silent applied==false")

		// Stock unchanged: the partial mutation (if any) was rolled back.
		Expect(itemQty().Equal(decimal.NewFromInt(100))).To(BeTrue(), "apply failure must not mutate stock")

		// Approval flip rolled back: the submission is still PENDING (not approved),
		// and no processed_at was stamped. processing_status=failed is recorded by
		// ps.end on its own connection (the preserved failure-audit trail).
		var reloaded models.InventorySubmission
		Expect(tenv.ContextfulDB().First(&reloaded, dispose.ID).Error).NotTo(HaveOccurred())
		Expect(reloaded.ApprovalStatus).To(Equal(models.InventorySubmissionApprovalStatusPending),
			"approval flip must roll back with the failed apply")
		Expect(reloaded.ProcessedAt).To(BeNil(), "no processed_at on a rolled-back apply")

		// No consuming transactions were persisted for this item beyond the seed purchase.
		var consumingCount int64
		Expect(tenv.ContextfulDB().Model(&models.InventoryTransaction{}).
			Where("inventory_item_id = ?", itm.ID).
			Where("transaction_type = ?", models.InventoryTransactionTypeDisposal).
			Count(&consumingCount).Error).NotTo(HaveOccurred())
		Expect(consumingCount).To(BeZero(), "no disposal transactions may commit on a rolled-back apply")
	})

	// FINDING (Codex P2): on the approve SUCCESS path, the RESPONSE must reflect the
	// COMMITTED row, not the struct read before the tx. `submission` is loaded by
	// GetByID BEFORE the approve tx; on a RETRY after an earlier failed apply it still
	// carries processing_status=failed + the old error JSON. The committed tx clears
	// the error, marks the row completed, and DB-stamps processed_at. Before the fix
	// the success path only patched approval/reason on the stale struct, so the
	// process response echoed the prior FAILED status/error even though the DB row was
	// completed. The fix reloads the row; the response must now show completed + no
	// error + a stamped processed_at.
	It("returns the COMMITTED state (completed, no error, processed_at) when approving a retried-after-failure submission (P2)", func() {
		approveCtx := func() context.Context {
			ctx := pkg.WithUserEmail(context.Background(), adminEmail)
			perms := map[pkg.UserPermission]struct{}{
				{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionApprove}: {},
			}
			return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
		}

		// A DISPOSE that consumes 40 of the live 100 — a VALID apply that will succeed.
		consume := decimal.NewFromInt(40)
		payload, err := json.Marshal(dto.DisposeInventoryRequest{
			InventoryID: inventory.ID,
			Items:       []dto.QuantityItem{{InventoryItemID: itm.ID, Quantity: &consume}},
		})
		Expect(err).NotTo(HaveOccurred())

		dispose := &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   models.InventorySubmissionTypeDispose,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
			Payload:          payload,
		}
		Expect(tenv.ContextfulDB().Create(dispose).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			tenv.ContextfulDB().Unscoped().Where("inventory_item_id = ?", itm.ID).
				Where("transaction_type = ?", models.InventoryTransactionTypeDisposal).
				Delete(&models.InventoryTransaction{})
			tenv.ContextfulDB().Unscoped().Delete(dispose)
		})

		// Simulate the post-rollback state of a PRIOR failed apply attempt:
		// processing_status=failed + error JSON recorded, approval still pending.
		repo := repository.NewInventorySubmissionRepository(repository.NewBaseRepository(tenv.DB))
		Expect(repo.FailSubmissionProcessingWithErrors(tenv.DefaultContext, dispose.ID, []error{
			errors.New("apply failed: transient error on first attempt"),
		})).NotTo(HaveOccurred())
		stale, err := repo.GetByID(tenv.DefaultContext, dispose.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(stale.ProcessingStatus).To(Equal(models.InventorySubmissionStatusFailed))
		Expect(stale.Error).NotTo(BeEmpty(), "precondition: the prior attempt recorded a failure error")

		// Retry: approve. The apply now succeeds; the RESPONSE must reflect the
		// committed row, NOT the stale pre-tx struct.
		resp, err := svc.ProcessSubmission(approveCtx(), dto.SubmissionApprovalRequest{
			SubmissionID: dispose.ID,
			Action:       string(models.InventorySubmissionActionApprove),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())
		Expect(resp.ApprovalStatus).To(Equal(models.InventorySubmissionApprovalStatusApproved))
		Expect(resp.ProcessingStatus).To(Equal(models.InventorySubmissionStatusCompleted),
			"response must show completed, not the stale failed status")
		Expect(resp.Error).To(BeEmpty(),
			"response must NOT echo the prior failure error, got %s", string(resp.Error))
		Expect(resp.ProcessedAt).NotTo(BeNil(), "response must carry the DB-stamped processed_at")

		// And the persisted row agrees (the response mirrors it).
		reloaded, err := repo.GetByID(tenv.DefaultContext, dispose.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(reloaded.ProcessingStatus).To(Equal(models.InventorySubmissionStatusCompleted))
		Expect(reloaded.Error).To(BeEmpty())
		Expect(reloaded.ProcessedAt).NotTo(BeNil())
		Expect(itemQty().Equal(decimal.NewFromInt(60))).To(BeTrue(), "live = 100 - 40 = 60, got %s", itemQty())
	})

	// FINDING (Codex P2 sweep): the close/reopen responses must reflect the committed
	// reconcile_status, and the start-processing success response (covered by the
	// happy-path spec above) must reflect the finalized statuses + DB-stamped
	// processed_at. These siblings build their struct fresh under FOR UPDATE inside
	// the tx and mirror the single mutated field, so the response is already
	// authoritative; this pins that contract.
	It("returns the committed reconcile_status on close and reopen (P2 sweep)", func() {
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: sub.ID, Items: countItems(60),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })

		closed, err := svc.CloseReconciliation(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(closed.ReconcileStatus).To(Equal(models.ReconcileLifecycleStatusClosed),
			"close response must reflect the committed closed status")

		reopened, err := svc.ReopenReconciliation(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(reopened.ReconcileStatus).To(Equal(models.ReconcileLifecycleStatusOpen),
			"reopen response must reflect the committed open status")
	})

	// FINDING 2 (Codex P1): InitiateReconcile (snapshot capture) must take the SAME
	// per-inventory advisory lock the consuming applies take, BEFORE
	// BuildReconciliationSnapshots reads row versions — so snapshot capture
	// serializes with consuming applies and can never read a stale baseline. We
	// prove serialization deterministically: while a separate tx holds
	// pg_advisory_xact_lock(inventory_id), InitiateReconcile must BLOCK and only
	// proceed once the lock is released.
	It("serializes snapshot capture behind a held per-inventory advisory lock", func() {
		initiateCtx := func() context.Context {
			ctx := pkg.WithUserEmail(context.Background(), adminEmail)
			perms := map[pkg.UserPermission]struct{}{
				{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionInitiateReconciliation}: {},
			}
			return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
		}

		// Fresh inventory with one active item so initiate has a baseline to capture
		// and no pre-existing active reconcile trips the one-active-pending guard.
		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]
		inv2 := fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("p6-inv2-%s", suffix),
			Location: fmt.Sprintf("p6-loc2-%s", suffix),
		})
		unit := fixture.WithUnit(db, fixture.ValidBaseUnit())
		product := fixture.WithProduct(db, fixture.ValidProduct(unit.ID))
		item2 := &models.InventoryItem{
			InventoryID: inv2.ID,
			ProductID:   product.ID,
			UnitID:      unit.ID,
			Quantity:    decimal.NewFromInt(50),
			Status:      models.InventoryItemStatusActive,
		}
		Expect(db.Create(item2).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("submission_id IN (?)",
				db.Model(&models.InventorySubmission{}).Select("id").Where("inventory_id = ?", inv2.ID)).
				Delete(&models.ReconciliationSnapshot{})
			db.Unscoped().Where("inventory_id = ?", inv2.ID).Delete(&models.InventorySubmission{})
			db.Unscoped().Delete(item2)
		})

		// Hold the advisory lock for inv2 in a separate, still-open tx.
		holder := tenv.DB.Begin()
		Expect(holder.Error).NotTo(HaveOccurred())
		Expect(holder.Exec("SELECT pg_advisory_xact_lock(?)", int64(inv2.ID)).Error).NotTo(HaveOccurred())

		done := make(chan struct{})
		var initErr error
		var created *models.InventorySubmission
		go func() {
			defer close(done)
			created, initErr = svc.InitiateReconcile(initiateCtx(), dto.InitiateReconcileRequest{InventoryID: inv2.ID})
		}()

		// While the lock is held, InitiateReconcile must block on snapshot-capture
		// serialization (not yet done).
		Consistently(func() bool {
			select {
			case <-done:
				return true
			default:
				return false
			}
		}, "1s", "100ms").Should(BeFalse(), "InitiateReconcile must block while the advisory lock is held")

		// Release the lock; InitiateReconcile now proceeds and captures the snapshot.
		Expect(holder.Rollback().Error).NotTo(HaveOccurred())
		Eventually(done, "5s").Should(BeClosed())
		Expect(initErr).NotTo(HaveOccurred())
		Expect(created).NotTo(BeNil())

		var snapCount int64
		Expect(tenv.ContextfulDB().Model(&models.ReconciliationSnapshot{}).
			Where("submission_id = ?", created.ID).Count(&snapCount).Error).NotTo(HaveOccurred())
		Expect(snapCount).To(Equal(int64(1)), "baseline snapshot captured after the lock cleared")
	})

	// FINDING (Codex P2): the snapshot capture time must be the REAL post-lock
	// instant. InitiateReconcile opens its tx, THEN waits on the per-inventory
	// advisory lock, THEN runs BuildReconciliationSnapshots. PostgreSQL NOW()
	// (== transaction_timestamp()) is frozen at tx start — BEFORE the lock wait — so
	// a sibling consuming apply that COMMITS DURING the wait (its stock effect is
	// already in the post-lock baseline the snapshot reads) would still get
	// processed_at >= MIN(created_at) and be falsely flagged as drift. The fix
	// stamps created_at = clock_timestamp() (the real post-lock statement instant),
	// so the drift window-start is strictly AFTER any apply that committed during
	// the wait.
	//
	// This reproduces the timing END-TO-END (no manual created_at patching): while
	// InitiateReconcile is blocked on the advisory lock, the lock-holding tx commits
	// a consuming sibling stamped with the SERVER clock (clock_timestamp()), then
	// releases the lock. InitiateReconcile then captures the snapshot — and because
	// capture uses clock_timestamp() (not the frozen tx-start NOW()), the snapshot's
	// created_at lands AFTER the sibling's processed_at, so StartProcessing's drift
	// re-check correctly excludes the sibling already reflected in the baseline.
	It("does NOT flag a consuming apply that committed DURING the initiate lock-wait (clock_timestamp post-lock capture, P2)", func() {
		initiateCtx := func() context.Context {
			ctx := pkg.WithUserEmail(context.Background(), adminEmail)
			perms := map[pkg.UserPermission]struct{}{
				{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionInitiateReconciliation}: {},
			}
			return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
		}

		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]
		inv2 := fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("p6-inv3-%s", suffix),
			Location: fmt.Sprintf("p6-loc3-%s", suffix),
		})
		unit := fixture.WithUnit(db, fixture.ValidBaseUnit())
		product := fixture.WithProduct(db, fixture.ValidProduct(unit.ID))
		item2 := &models.InventoryItem{
			InventoryID: inv2.ID,
			ProductID:   product.ID,
			UnitID:      unit.ID,
			Quantity:    decimal.NewFromInt(100),
			Status:      models.InventoryItemStatusActive,
		}
		Expect(db.Create(item2).Error).NotTo(HaveOccurred())
		// A backing purchase so FIFO has stock for the later StartProcessing apply.
		Expect(db.Create(&models.InventoryTransaction{
			InventoryItemID: item2.ID,
			SupplierID:      &supplier.ID,
			TransactionType: models.InventoryTransactionTypePurchase,
			Price:           10.0,
			Quantity:        decimal.NewFromInt(100),
		}).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("submission_id IN (?)",
				db.Model(&models.InventorySubmission{}).Select("id").Where("inventory_id = ?", inv2.ID)).
				Delete(&models.ReconciliationSnapshot{})
			db.Unscoped().Where("inventory_id = ?", inv2.ID).Delete(&models.InventorySubmission{})
			db.Unscoped().Where("inventory_item_id = ?", item2.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(item2)
		})

		// Hold the advisory lock for inv2 in a separate, still-open tx — this models a
		// consuming apply (dispose/transfer/reconcile) already holding the lock when
		// initiate arrives, forcing initiate to wait.
		holder := tenv.DB.Begin()
		Expect(holder.Error).NotTo(HaveOccurred())
		Expect(holder.Exec("SELECT pg_advisory_xact_lock(?)", int64(inv2.ID)).Error).NotTo(HaveOccurred())

		done := make(chan struct{})
		var initErr error
		var created *models.InventorySubmission
		go func() {
			defer close(done)
			created, initErr = svc.InitiateReconcile(initiateCtx(), dto.InitiateReconcileRequest{InventoryID: inv2.ID})
		}()

		// While initiate is blocked on the lock, the holder commits the consuming
		// sibling and stamps processed_at with the SERVER clock (clock_timestamp(),
		// evaluated now — strictly DURING the wait, before the snapshot read). A
		// frozen-at-tx-start NOW() in BuildReconciliationSnapshots could be earlier
		// than this instant (initiate's tx may have begun before the holder ran),
		// which is exactly the false-drift window the fix closes.
		Consistently(func() bool {
			select {
			case <-done:
				return true
			default:
				return false
			}
		}, "1s", "100ms").Should(BeFalse(), "InitiateReconcile must block while the advisory lock is held")

		var siblingID uint
		Expect(holder.Raw(`
			INSERT INTO inventory_submissions
				(inventory_id, submission_type, processing_status, approval_status, processed_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, clock_timestamp(), clock_timestamp(), clock_timestamp())
			RETURNING id`,
			inv2.ID,
			models.InventorySubmissionTypeDispose,
			models.InventorySubmissionStatusCompleted,
			models.InventorySubmissionApprovalStatusApproved,
		).Scan(&siblingID).Error).NotTo(HaveOccurred())
		// Commit the sibling AND release the advisory lock (held by this same tx).
		Expect(holder.Commit().Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			tenv.ContextfulDB().Unscoped().Where("id = ?", siblingID).Delete(&models.InventorySubmission{})
		})

		// Initiate proceeds: BuildReconciliationSnapshots runs post-lock and stamps
		// created_at = clock_timestamp() — AFTER the sibling's processed_at.
		Eventually(done, "5s").Should(BeClosed())
		Expect(initErr).NotTo(HaveOccurred())
		Expect(created).NotTo(BeNil())

		// Prove the invariant at the data layer: the snapshot capture instant is
		// strictly after the sibling's processed_at (would FAIL with NOW()).
		var capturedAt, siblingProcessedAt time.Time
		Expect(tenv.ContextfulDB().Model(&models.ReconciliationSnapshot{}).
			Where("submission_id = ?", created.ID).
			Select("MIN(created_at)").Scan(&capturedAt).Error).NotTo(HaveOccurred())
		Expect(tenv.ContextfulDB().Model(&models.InventorySubmission{}).
			Where("id = ?", siblingID).
			Select("processed_at").Scan(&siblingProcessedAt).Error).NotTo(HaveOccurred())
		Expect(capturedAt.After(siblingProcessedAt)).To(BeTrue(),
			"post-lock snapshot capture (%s) must be after the sibling committed during the wait (%s)",
			capturedAt, siblingProcessedAt)

		// End-to-end: count + close, then StartProcessing must NOT flag the sibling
		// (it is already reflected in the post-lock baseline) and the apply proceeds.
		staffItems := []dto.ReconciliationCountItem{{
			InventoryItemID: item2.ID, Quantity: func() *decimal.Decimal { q := decimal.NewFromInt(60); return &q }(),
		}}
		_, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: created.ID, Items: staffItems,
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = svc.CloseReconciliation(adminCtx, created.ID)
		Expect(err).NotTo(HaveOccurred())

		res, err := svc.StartProcessing(adminCtx, created.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.DriftDetected).To(BeFalse(),
			"a consuming apply that committed during the initiate lock-wait is in the baseline and must NOT be drift")
		var reloaded models.InventoryItem
		Expect(tenv.ContextfulDB().First(&reloaded, item2.ID).Error).NotTo(HaveOccurred())
		Expect(reloaded.Quantity.Equal(decimal.NewFromInt(60))).To(BeTrue(),
			"apply proceeds: live = 100 - (100-60) = 60, got %s", reloaded.Quantity)
	})
})
