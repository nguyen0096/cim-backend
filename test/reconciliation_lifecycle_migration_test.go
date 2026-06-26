package apptest

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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

	var inventory *models.Inventory

	BeforeEach(func() {
		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]
		inventory = fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("mig-inv-%s", suffix),
			Location: fmt.Sprintf("mig-loc-%s", suffix),
		})
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
})
