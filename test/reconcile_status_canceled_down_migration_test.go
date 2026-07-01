package apptest

import (
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"cim-backend/internal/models"
	"cim-backend/pkg/testutil/fixture"
)

// Pins the down migration
// 20260701000000_inventory_submissions_reconcile_status_add_canceled.down.sql
// against a REAL Postgres: a canceled reconcile must not wedge the rollback when
// the restored (pre-canceled) CHECK is re-added. We replay the EXACT down
// statements on a live canceled row, then restore the up constraint so the shared
// suite schema keeps accepting 'canceled'.
var _ = Describe("Reconcile-status add-canceled DOWN migration", func() {
	// downMigrationSQL is byte-for-byte the down migration. Keep in sync.
	const downMigrationSQL = `
UPDATE inventory_submissions
SET reconcile_status = 'closed',
    approval_status = 'rejected'
WHERE reconcile_status = 'canceled';

ALTER TABLE inventory_submissions
    DROP CONSTRAINT IF EXISTS chk_inventory_submissions_reconcile_status;

ALTER TABLE inventory_submissions
    ADD CONSTRAINT chk_inventory_submissions_reconcile_status
        CHECK (reconcile_status IS NULL
            OR reconcile_status = ''
            OR reconcile_status IN ('open', 'closed', 'processing', 'processed'));`

	// restoreCanceledConstraintSQL re-adds the up-migration constraint (admits
	// 'canceled') so the shared suite schema is left as the down test found it.
	const restoreCanceledConstraintSQL = `
ALTER TABLE inventory_submissions
    DROP CONSTRAINT IF EXISTS chk_inventory_submissions_reconcile_status;

ALTER TABLE inventory_submissions
    ADD CONSTRAINT chk_inventory_submissions_reconcile_status
        CHECK (reconcile_status IS NULL
            OR reconcile_status = ''
            OR reconcile_status IN ('open', 'closed', 'processing', 'processed', 'canceled'));`

	reload := func(id uint) *models.InventorySubmission {
		var out models.InventorySubmission
		Expect(tenv.ContextfulDB().First(&out, id).Error).NotTo(HaveOccurred())
		return &out
	}

	It("rolls back cleanly with a canceled row present and leaves it non-active", func() {
		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]
		inv := fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("down-mig-inv-%s", suffix),
			Location: fmt.Sprintf("down-mig-loc-%s", suffix),
		})

		// A canceled reconcile: the exact terminal state CancelReconciliation writes.
		canceled := &models.InventorySubmission{
			InventoryID:      inv.ID,
			SubmissionType:   models.InventorySubmissionTypeReconcile,
			ProcessingStatus: models.InventorySubmissionStatusCanceled,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
			ReconcileStatus:  models.ReconcileLifecycleStatusCanceled,
		}
		Expect(db.Create(canceled).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(canceled) })
		// Always leave the suite schema back on the canceled-admitting constraint.
		DeferCleanup(func() {
			Expect(tenv.ContextfulDB().Exec(restoreCanceledConstraintSQL).Error).NotTo(HaveOccurred())
		})

		// The rollback (remap + restore the pre-canceled CHECK) must not error.
		Expect(db.Exec(downMigrationSQL).Error).NotTo(HaveOccurred(),
			"down migration must not fail when a canceled row exists")

		got := reload(canceled.ID)
		Expect(got.ReconcileStatus).To(Equal(models.ReconcileLifecycleStatusClosed),
			"canceled reconcile_status must remap to 'closed'")
		// Mirrors the pre-cancel reject terminal: approval=rejected + processing=canceled.
		Expect(got.ApprovalStatus).To(Equal(models.InventorySubmissionApprovalStatusRejected),
			"approval_status must terminalize to 'rejected' so the row is non-startable after rollback")
		Expect(got.ProcessingStatus).To(Equal(models.InventorySubmissionStatusCanceled),
			"processing_status is untouched by the down migration")
		Expect(got.IsActiveReconcile()).To(BeFalse(),
			"the remapped row must not resurrect as an active reconcile")
	})
})
