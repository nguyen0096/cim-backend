package apptest

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/pkg/testutil/fixture"
)

// These specs cover the retry-then-success stale-error fix (epic #38, Part 6 —
// Codex P2). When an atomic apply fails, the failure-audit write records
// processing_status='failed' + the error JSON while approval rolls back to
// 'pending'. Because gating is only on approval status, the same submission can
// later be fixed and processed successfully. A subsequent SUCCESS completion must
// CLEAR the stale error so list/detail no longer show a completed/approved
// submission carrying the previous failure. The failure path itself must still
// record the error (only a later success clears it).
var _ = Describe("Inventory submission clear-error-on-success", func() {
	var (
		inventory *models.Inventory
		repo      repository.InventorySubmissionRepository
	)

	BeforeEach(func() {
		db := tenv.ContextfulDB()
		repo = repository.NewInventorySubmissionRepository(repository.NewBaseRepository(db))
		suffix := uuid.NewString()[:8]
		inventory = fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("clr-inv-%s", suffix),
			Location: fmt.Sprintf("loc-%s", suffix),
		})
	})

	// failedSubmission persists a submission left in the post-rollback failed state:
	// processing_status=failed + error JSON recorded, approval_status back to pending.
	failedSubmission := func(t models.SubmissionType) *models.InventorySubmission {
		ctx := tenv.DefaultContext
		db := tenv.ContextfulDB()
		sub := &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   t,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
		}
		Expect(db.Create(sub).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(sub) })

		// Record a failure audit (the deferred failure-audit write on apply failure).
		Expect(repo.FailSubmissionProcessingWithErrors(ctx, sub.ID, []error{
			errors.New("apply failed: insufficient stock"),
		})).NotTo(HaveOccurred())

		// Confirm the failure path DID record the error (precondition for the fix).
		stored, err := repo.GetByID(ctx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(stored.ProcessingStatus).To(Equal(models.InventorySubmissionStatusFailed))
		Expect(stored.Error).NotTo(BeEmpty(), "failure path must record the error JSON")
		return sub
	}

	It("clears the stale error when a failed submission later completes via UpdateProcessingStatus", func() {
		ctx := tenv.DefaultContext
		sub := failedSubmission(models.InventorySubmissionTypeDispose)

		// The retry succeeds: the general processing path marks it completed.
		Expect(repo.UpdateProcessingStatus(ctx, sub.ID, models.InventorySubmissionStatusCompleted)).
			NotTo(HaveOccurred())

		stored, err := repo.GetByID(ctx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(stored.ProcessingStatus).To(Equal(models.InventorySubmissionStatusCompleted))
		// The stale failure error must be gone — list/detail no longer mislead admins.
		Expect(stored.Error).To(BeEmpty(),
			"stale error should be cleared on successful completion, got %s", string(stored.Error))
	})

	It("clears the stale error when a failed reconcile later completes via MarkProcessed", func() {
		ctx := tenv.DefaultContext
		sub := failedSubmission(models.InventorySubmissionTypeReconcile)

		_, err := repo.MarkProcessed(ctx, sub.ID, models.ReconcileLifecycleStatusProcessed)
		Expect(err).NotTo(HaveOccurred())

		stored, err := repo.GetByID(ctx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(stored.ProcessingStatus).To(Equal(models.InventorySubmissionStatusCompleted))
		Expect(stored.ApprovalStatus).To(Equal(models.InventorySubmissionApprovalStatusApproved))
		Expect(stored.Error).To(BeEmpty(),
			"stale error should be cleared on MarkProcessed, got %s", string(stored.Error))
	})

	It("does NOT clear the error on a non-success processing transition (cancel)", func() {
		ctx := tenv.DefaultContext
		sub := failedSubmission(models.InventorySubmissionTypeDispose)

		// A cancel (reject path) is not a success: it must not scrub the audit trail.
		Expect(repo.UpdateProcessingStatus(ctx, sub.ID, models.InventorySubmissionStatusCanceled)).
			NotTo(HaveOccurred())

		stored, err := repo.GetByID(ctx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(stored.ProcessingStatus).To(Equal(models.InventorySubmissionStatusCanceled))
		Expect(stored.Error).NotTo(BeEmpty(),
			"error should only be cleared by a SUCCESS completion, not by cancel")
	})
})
