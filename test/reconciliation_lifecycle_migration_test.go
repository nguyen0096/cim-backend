package apptest

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/shopspring/decimal"

	"cim-backend/internal/models"
	"cim-backend/pkg/testutil/fixture"
)

// These specs pin the data-correctness behavior of the up migration
// 20260624000001_inventory_submissions_reconcile_lifecycle.up.sql backfills
// against a REAL Postgres. The migration itself has already run at suite start, so
// we re-execute the EXACT backfill UPDATE statement on freshly inserted rows that
// reproduce the pre-deploy state (processed_at forced NULL) and assert the result.
var _ = Describe("Reconcile lifecycle migration backfills", func() {
	// processedAtBackfillSQL is byte-for-byte the processed_at backfill from the up
	// migration. Keep in sync with the migration file.
	const processedAtBackfillSQL = `
UPDATE inventory_submissions
SET processed_at = COALESCE(updated_at, created_at)
WHERE submission_type IN ('dispose', 'transfer', 'reconcile')
    AND processing_status = 'completed'
    AND approval_status = 'approved'
    AND processed_at IS NULL
    AND deleted_at IS NULL`

	var (
		inventory *models.Inventory
		invItem   *models.InventoryItem
		// pendingReconcileItem maps a pending-reconcile submission id -> the backing
		// inventory_item id, so withSnapshot can satisfy the snapshot FK. Reset per spec.
		pendingReconcileItem map[uint]uint
	)

	BeforeEach(func() {
		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]
		inventory = fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("mig-inv-%s", suffix),
			Location: fmt.Sprintf("mig-loc-%s", suffix),
		})
		// A real inventory item so reconciliation_snapshots.inventory_item_id (FK ->
		// inventory_items) is satisfiable when the open-backfill specs attach snapshots.
		unit := fixture.WithUnit(db, fixture.ValidBaseUnit())
		product := fixture.WithProduct(db, fixture.ValidProduct(unit.ID))
		invItem = &models.InventoryItem{
			InventoryID: inventory.ID,
			ProductID:   product.ID,
			UnitID:      unit.ID,
			Quantity:    decimal.NewFromInt(7),
			Status:      models.InventoryItemStatusActive,
		}
		Expect(db.Create(invItem).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(invItem) })

		pendingReconcileItem = map[uint]uint{}
	})

	// newCompletedConsuming inserts a completed+approved consuming submission and
	// then forces processed_at back to NULL (MarkProcessed/the model stamps it now),
	// reproducing a pre-deploy row that the column add left NULL.
	newCompletedConsuming := func(t models.SubmissionType) *models.InventorySubmission {
		db := tenv.ContextfulDB()
		sub := &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   t,
			ProcessingStatus: models.InventorySubmissionStatusCompleted,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusApproved,
		}
		Expect(db.Create(sub).Error).NotTo(HaveOccurred())
		Expect(db.Model(&models.InventorySubmission{}).Where("id = ?", sub.ID).
			Update("processed_at", nil).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(sub) })
		return sub
	}

	reload := func(id uint) *models.InventorySubmission {
		var out models.InventorySubmission
		Expect(tenv.ContextfulDB().First(&out, id).Error).NotTo(HaveOccurred())
		return &out
	}

	It("stamps processed_at = COALESCE(updated_at, created_at) for completed consuming rows", func() {
		dispose := newCompletedConsuming(models.InventorySubmissionTypeDispose)
		transfer := newCompletedConsuming(models.InventorySubmissionTypeTransfer)
		// reconcile is also a consuming type the drift re-check inspects.
		recon := newCompletedConsuming(models.InventorySubmissionTypeReconcile)

		Expect(reload(dispose.ID).ProcessedAt).To(BeNil(), "precondition: NULL before backfill")

		Expect(tenv.ContextfulDB().Exec(processedAtBackfillSQL).Error).NotTo(HaveOccurred())

		for _, s := range []*models.InventorySubmission{dispose, transfer, recon} {
			r := reload(s.ID)
			Expect(r.ProcessedAt).NotTo(BeNil(), "backfill must stamp a non-null processed_at")
			// proxy == COALESCE(updated_at, created_at); updated_at is set on insert.
			Expect(r.ProcessedAt.Sub(r.UpdatedAt).Abs()).To(BeNumerically("<", time.Second),
				"processed_at must equal the updated_at completion-time proxy")
		}
	})

	It("does NOT stamp non-completed, non-approved, deleted, or already-stamped rows", func() {
		db := tenv.ContextfulDB()

		// pending (not completed) — must stay NULL.
		pending := newCompletedConsuming(models.InventorySubmissionTypeDispose)
		Expect(db.Model(&models.InventorySubmission{}).Where("id = ?", pending.ID).
			Update("processing_status", models.InventorySubmissionStatusPending).Error).NotTo(HaveOccurred())

		// completed but already stamped — must keep its existing value, not be rewritten.
		existing := time.Now().Add(-72 * time.Hour).Truncate(time.Second)
		stamped := newCompletedConsuming(models.InventorySubmissionTypeTransfer)
		Expect(db.Model(&models.InventorySubmission{}).Where("id = ?", stamped.ID).
			Update("processed_at", existing).Error).NotTo(HaveOccurred())

		Expect(db.Exec(processedAtBackfillSQL).Error).NotTo(HaveOccurred())

		Expect(reload(pending.ID).ProcessedAt).To(BeNil(), "a pending row must not be backfilled")
		got := reload(stamped.ID)
		Expect(got.ProcessedAt).NotTo(BeNil())
		Expect(got.ProcessedAt.Sub(existing).Abs()).To(BeNumerically("<", time.Second),
			"an already-stamped processed_at must be preserved (idempotent)")
	})

	// reconcileOpenBackfillSQL is byte-for-byte the SECOND backfill from the up
	// migration (the in-flight-initiated -> `open` relabel). Keep in sync with the
	// migration file.
	const reconcileOpenBackfillSQL = `
UPDATE inventory_submissions s
SET reconcile_status = 'open'
WHERE s.submission_type = 'reconcile'
    AND s.processing_status = 'pending'
    AND s.approval_status = 'pending'
    AND (s.reconcile_status IS NULL OR s.reconcile_status = '')
    AND s.deleted_at IS NULL
    AND EXISTS (
        SELECT 1
        FROM reconciliation_snapshots rs
        WHERE rs.submission_id = s.id
            AND rs.deleted_at IS NULL
    )`

	// newPendingReconcile inserts a pending+pending reconcile submission with
	// reconcile_status forced to NULL (the state the column-add left a pre-deploy
	// in-flight reconcile in, before this backfill ran). Each gets its OWN inventory
	// + item so the uq_inventory_submissions_one_active_pending index (one live
	// pending reconcile per inventory) is not tripped, and so withSnapshot has a
	// valid inventory_item FK to point at.
	newPendingReconcile := func() *models.InventorySubmission {
		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]
		inv := fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("mig-inv2-%s", suffix),
			Location: fmt.Sprintf("mig-loc2-%s", suffix),
		})
		item := &models.InventoryItem{
			InventoryID: inv.ID,
			ProductID:   invItem.ProductID,
			UnitID:      invItem.UnitID,
			Quantity:    decimal.NewFromInt(7),
			Status:      models.InventoryItemStatusActive,
		}
		Expect(db.Create(item).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(item) })

		sub := &models.InventorySubmission{
			InventoryID:      inv.ID,
			SubmissionType:   models.InventorySubmissionTypeReconcile,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
		}
		Expect(db.Create(sub).Error).NotTo(HaveOccurred())
		Expect(db.Model(&models.InventorySubmission{}).Where("id = ?", sub.ID).
			Update("reconcile_status", nil).Error).NotTo(HaveOccurred())
		// Record the backing item on the snapshot via the submission's transient field
		// so withSnapshot can resolve a valid FK without a second lookup.
		sub.InventoryID = inv.ID
		pendingReconcileItem[sub.ID] = item.ID
		DeferCleanup(func() { db.Unscoped().Delete(sub) })
		return sub
	}

	// newPendingNonReconcile inserts a NON-reconcile (dispose) submission that
	// matches EVERY OTHER open-backfill predicate clause — pending processing,
	// pending approval, reconcile_status NULL, not soft-deleted — on its OWN
	// inventory + item (so withSnapshot has a valid FK and the one-active-pending
	// reconcile index is irrelevant anyway, being reconcile-only). Pairing it with
	// withSnapshot leaves submission_type <> 'reconcile' as the ONLY clause that can
	// exclude it, so the spec fails if the type predicate is ever dropped.
	newPendingNonReconcile := func() *models.InventorySubmission {
		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]
		inv := fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("mig-inv3-%s", suffix),
			Location: fmt.Sprintf("mig-loc3-%s", suffix),
		})
		item := &models.InventoryItem{
			InventoryID: inv.ID,
			ProductID:   invItem.ProductID,
			UnitID:      invItem.UnitID,
			Quantity:    decimal.NewFromInt(7),
			Status:      models.InventoryItemStatusActive,
		}
		Expect(db.Create(item).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(item) })

		sub := &models.InventorySubmission{
			InventoryID:      inv.ID,
			SubmissionType:   models.InventorySubmissionTypeDispose,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
		}
		Expect(db.Create(sub).Error).NotTo(HaveOccurred())
		Expect(db.Model(&models.InventorySubmission{}).Where("id = ?", sub.ID).
			Update("reconcile_status", nil).Error).NotTo(HaveOccurred())
		sub.InventoryID = inv.ID
		pendingReconcileItem[sub.ID] = item.ID
		DeferCleanup(func() { db.Unscoped().Delete(sub) })
		return sub
	}

	// withSnapshot attaches a live snapshot row (the INITIATED marker) to a
	// submission so the backfill's EXISTS predicate sees it.
	withSnapshot := func(submissionID uint) {
		db := tenv.ContextfulDB()
		snap := &models.ReconciliationSnapshot{
			SubmissionID:    submissionID,
			InventoryItemID: pendingReconcileItem[submissionID],
			PrevQuantity:    decimal.NewFromInt(7),
		}
		Expect(db.Create(snap).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(snap) })
	}

	It("relabels only in-flight initiated reconciles (pending+pending+has-snapshot) to reconcile_status='open'", func() {
		db := tenv.ContextfulDB()

		// IN-FLIGHT INITIATED: pending+pending reconcile WITH a live snapshot -> open.
		inFlight := newPendingReconcile()
		withSnapshot(inFlight.ID)

		// LEGACY reconcile: pending+pending but NO snapshot (created via the old
		// create-submission path) -> must stay NULL.
		legacy := newPendingReconcile()

		// TERMINAL reconcile: completed/approved WITH a snapshot -> processed, not
		// in flight; the predicate's processing/approval=pending excludes it.
		terminal := newPendingReconcile()
		Expect(db.Model(&models.InventorySubmission{}).Where("id = ?", terminal.ID).
			Updates(map[string]interface{}{
				"processing_status": models.InventorySubmissionStatusCompleted,
				"approval_status":   models.InventorySubmissionApprovalStatusApproved,
			}).Error).NotTo(HaveOccurred())
		withSnapshot(terminal.ID)

		// SOFT-DELETED in-flight reconcile WITH a snapshot -> deleted_at IS NULL
		// predicate excludes it.
		deleted := newPendingReconcile()
		withSnapshot(deleted.ID)
		Expect(db.Delete(&models.InventorySubmission{}, deleted.ID).Error).NotTo(HaveOccurred())

		// DISPOSE (non-reconcile): pending+pending, reconcile_status NULL, not deleted,
		// AND with a live snapshot — i.e. it matches every other predicate clause, so
		// submission_type <> 'reconcile' is the ONLY reason it is excluded. This makes
		// the spec genuinely guard the type predicate: drop it and this row gets
		// wrongly relabeled to 'open', failing the assertion below.
		dispose := newPendingNonReconcile()
		withSnapshot(dispose.ID)

		Expect(reload(inFlight.ID).ReconcileStatus).To(BeEmpty(), "precondition: NULL before the backfill")

		Expect(db.Exec(reconcileOpenBackfillSQL).Error).NotTo(HaveOccurred())

		// Only the in-flight initiated reconcile flips to open.
		Expect(reload(inFlight.ID).ReconcileStatus).To(Equal(models.ReconcileLifecycleStatusOpen),
			"an in-flight initiated reconcile with a snapshot must become open")

		// Everything else is left untouched (still NULL / unchanged status).
		Expect(reload(legacy.ID).ReconcileStatus).To(BeEmpty(),
			"a legacy reconcile with no snapshot must not be relabeled")
		Expect(reload(terminal.ID).ReconcileStatus).To(BeEmpty(),
			"a terminal (completed/approved) reconcile must not be relabeled")
		Expect(reload(dispose.ID).ReconcileStatus).To(BeEmpty(),
			"a non-reconcile submission must not be relabeled even when it matches every "+
				"other predicate clause (pending+pending+snapshot) — only the type predicate excludes it")

		var deletedRow models.InventorySubmission
		Expect(db.Unscoped().First(&deletedRow, deleted.ID).Error).NotTo(HaveOccurred())
		Expect(deletedRow.ReconcileStatus).To(BeEmpty(),
			"a soft-deleted in-flight reconcile must not be relabeled")
	})

	It("is idempotent: a reconcile already stamped 'closed' is not reset to 'open'", func() {
		db := tenv.ContextfulDB()

		// An in-flight reconcile the NEW flow already advanced to closed; the backfill
		// only touches NULL/'' reconcile_status, so it must leave closed alone.
		closed := newPendingReconcile()
		withSnapshot(closed.ID)
		Expect(db.Model(&models.InventorySubmission{}).Where("id = ?", closed.ID).
			Update("reconcile_status", models.ReconcileLifecycleStatusClosed).Error).NotTo(HaveOccurred())

		Expect(db.Exec(reconcileOpenBackfillSQL).Error).NotTo(HaveOccurred())

		Expect(reload(closed.ID).ReconcileStatus).To(Equal(models.ReconcileLifecycleStatusClosed),
			"an already-advanced reconcile_status must be preserved (idempotent / re-run safe)")
	})
})
