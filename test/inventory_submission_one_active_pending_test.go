package apptest

import (
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"cim-backend/internal/models"
	"cim-backend/pkg/testutil/fixture"
)

// These specs exercise the partial unique index
// uq_inventory_submissions_one_active_pending (epic #38, Part 3 — S5), the
// race-safe backstop for "at most one active/pending RECONCILE submission per
// inventory" (reconcile-only per the human's decision). They hit a real Postgres
// (the test suite runs all migrations), so they confirm the index — not just the
// service pre-check — enforces the rule and that its predicate (deleted_at IS NULL
// AND processing_status='pending' AND submission_type='reconcile') admits the
// intended exceptions, including that dispose/transfer are never gated.
var _ = Describe("Inventory submission one-active-pending index", func() {
	var inventory *models.Inventory

	BeforeEach(func() {
		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]
		inventory = fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("oap-inv-%s", suffix),
			Location: fmt.Sprintf("loc-%s", suffix),
		})
	})

	pending := func(t models.SubmissionType) *models.InventorySubmission {
		return &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   t,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
		}
	}

	It("rejects a second concurrent pending RECONCILE for the same inventory", func() {
		db := tenv.ContextfulDB()

		first := pending(models.InventorySubmissionTypeReconcile)
		Expect(db.Create(first).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(first) })

		// A second live pending RECONCILE for the same inventory must violate the
		// partial unique index. This is the concurrent-duplicate the service
		// check-then-insert cannot catch under a race; the index is the backstop.
		second := pending(models.InventorySubmissionTypeReconcile)
		err := db.Create(second).Error
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("uq_inventory_submissions_one_active_pending"))
	})

	It("allows a pending dispose alongside a pending reconcile for the same inventory", func() {
		db := tenv.ContextfulDB()

		// The guard is reconcile-only: a pending dispose is outside the index
		// predicate (submission_type='reconcile'), so it must coexist with a pending
		// reconcile for the same inventory.
		reconcile := pending(models.InventorySubmissionTypeReconcile)
		Expect(db.Create(reconcile).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(reconcile) })

		dispose := pending(models.InventorySubmissionTypeDispose)
		Expect(db.Create(dispose).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(dispose) })
	})

	It("allows a pending transfer alongside a pending reconcile for the same inventory", func() {
		db := tenv.ContextfulDB()

		reconcile := pending(models.InventorySubmissionTypeReconcile)
		Expect(db.Create(reconcile).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(reconcile) })

		transfer := pending(models.InventorySubmissionTypeTransfer)
		Expect(db.Create(transfer).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(transfer) })
	})

	It("allows multiple pending disposes for the same inventory (dispose is ungated)", func() {
		db := tenv.ContextfulDB()

		first := pending(models.InventorySubmissionTypeDispose)
		Expect(db.Create(first).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(first) })

		second := pending(models.InventorySubmissionTypeDispose)
		Expect(db.Create(second).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(second) })
	})

	It("allows a new pending submission once the prior one is terminal (completed/canceled)", func() {
		db := tenv.ContextfulDB()

		first := pending(models.InventorySubmissionTypeReconcile)
		Expect(db.Create(first).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(first) })

		// Resolve the first submission (mirrors approve->completed or reject->canceled):
		// it leaves the partial-index predicate, freeing the inventory.
		Expect(db.Model(first).
			Update("processing_status", models.InventorySubmissionStatusCompleted).Error).
			NotTo(HaveOccurred())

		second := pending(models.InventorySubmissionTypeReconcile)
		Expect(db.Create(second).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(second) })
	})

	It("allows a new pending reconcile once the prior one is soft-deleted", func() {
		db := tenv.ContextfulDB()

		first := pending(models.InventorySubmissionTypeReconcile)
		Expect(db.Create(first).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(first) })

		// Soft delete sets deleted_at, leaving the partial-index predicate.
		Expect(db.Delete(first).Error).NotTo(HaveOccurred())

		second := pending(models.InventorySubmissionTypeReconcile)
		Expect(db.Create(second).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(second) })
	})

	It("allows concurrent pending submissions for different inventories", func() {
		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]
		other := fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("oap-inv2-%s", suffix),
			Location: fmt.Sprintf("loc2-%s", suffix),
		})

		first := pending(models.InventorySubmissionTypeReconcile)
		Expect(db.Create(first).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(first) })

		second := &models.InventorySubmission{
			InventoryID:      other.ID,
			SubmissionType:   models.InventorySubmissionTypeReconcile,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
		}
		Expect(db.Create(second).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(second) })
	})
})
