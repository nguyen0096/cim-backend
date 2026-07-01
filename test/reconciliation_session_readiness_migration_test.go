package apptest

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"cim-backend/internal/models"
	"cim-backend/pkg/testutil/fixture"
)

// These specs exercise the widened reconciliation_request_items status CHECK
// against a real Postgres: a ready_for_review insert is accepted, and an unknown
// status is still rejected by the named constraint.
var _ = Describe("Reconcile request-item session-readiness migration", func() {
	var (
		inventory  *models.Inventory
		submission *models.InventorySubmission
	)

	countedPayload := func(itemID uint, qty int64) json.RawMessage {
		b, err := json.Marshal(map[string]interface{}{
			"items": []map[string]interface{}{
				{"inventory_item_id": itemID, "quantity": qty},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		return b
	}

	BeforeEach(func() {
		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]
		inventory = fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("readiness-inv-%s", suffix),
			Location: fmt.Sprintf("readiness-loc-%s", suffix),
		})
		submission = &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   models.InventorySubmissionTypeReconcile,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
			ReconcileStatus:  models.ReconcileLifecycleStatusOpen,
		}
		Expect(db.Create(submission).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(submission) })
	})

	It("accepts a ready_for_review row under the widened CHECK", func() {
		db := tenv.ContextfulDB()
		var id uint
		Expect(db.Raw(`
			INSERT INTO reconciliation_request_items
				(submission_id, payload, status, created_at, updated_at)
			VALUES (?, ?, 'ready_for_review', now(), now())
			RETURNING id`,
			submission.ID, countedPayload(201, 10),
		).Scan(&id).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(&models.ReconciliationRequestItem{}, id) })

		var out models.ReconciliationRequestItem
		Expect(db.First(&out, id).Error).NotTo(HaveOccurred())
		Expect(out.Status).To(Equal(models.ReconciliationRequestItemStatusReadyForReview))
	})

	It("still rejects an unknown status (the named CHECK was only widened, not dropped)", func() {
		err := tenv.ContextfulDB().Exec(`
			INSERT INTO reconciliation_request_items
				(submission_id, payload, status, created_at, updated_at)
			VALUES (?, ?, 'approved', now(), now())`,
			submission.ID, countedPayload(202, 20),
		).Error
		Expect(err).To(HaveOccurred(), "the widened CHECK must still reject a status outside the enum")
		Expect(err.Error()).To(ContainSubstring("chk_reconciliation_request_items_status"))
	})
})
