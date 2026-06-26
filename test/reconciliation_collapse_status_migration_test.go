package apptest

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/shopspring/decimal"

	"cim-backend/internal/models"
	"cim-backend/pkg/testutil/fixture"
)

// These specs pin the data-correctness behavior of the status-collapse migration
// 20260624000000_reconciliation_request_items_collapse_status.up.sql against a
// REAL Postgres. The Part-6 redesign removed the per-row ready/approved/applied
// states; the migration relabels any leftover non-in_progress row to in_progress
// (a pure relabel — counts unchanged) BEFORE narrowing the CHECK to in_progress
// only. The migration already ran at suite start, so the relabel UPDATE cannot be
// exercised on rows inserted afterward (the narrowed CHECK rejects the old
// statuses). We therefore reproduce the pre-migration state exactly: drop to the
// permissive Part-1 CHECK, insert rows under the OLD statuses, re-run the EXACT
// relabel UPDATE, then re-apply the narrowed CHECK — and assert the relabel and
// the post-narrowing rejection.
var _ = Describe("Reconcile request-item status-collapse migration", func() {
	// relabelSQL is byte-for-byte the relabel UPDATE from the up migration. Keep in
	// sync with the migration file.
	const relabelSQL = `
UPDATE reconciliation_request_items
    SET status = 'in_progress'
    WHERE status <> 'in_progress'`

	const dropCheckSQL = `ALTER TABLE reconciliation_request_items DROP CONSTRAINT IF EXISTS chk_reconciliation_request_items_status`

	var (
		inventory  *models.Inventory
		submission *models.InventorySubmission
	)

	// setPermissiveCheck restores the original permissive Part-1 CHECK so the OLD
	// statuses can be inserted (statements run separately: the pgx extended protocol
	// rejects multiple statements in one Exec).
	setPermissiveCheck := func() {
		db := tenv.ContextfulDB()
		Expect(db.Exec(dropCheckSQL).Error).NotTo(HaveOccurred())
		Expect(db.Exec(`ALTER TABLE reconciliation_request_items
    ADD CONSTRAINT chk_reconciliation_request_items_status
        CHECK (status IN ('in_progress', 'ready', 'approved', 'applied'))`).Error).NotTo(HaveOccurred())
	}

	// setNarrowedCheck re-applies the narrowed CHECK exactly as the up migration does.
	setNarrowedCheck := func() {
		db := tenv.ContextfulDB()
		Expect(db.Exec(dropCheckSQL).Error).NotTo(HaveOccurred())
		Expect(db.Exec(`ALTER TABLE reconciliation_request_items
    ADD CONSTRAINT chk_reconciliation_request_items_status
        CHECK (status IN ('in_progress'))`).Error).NotTo(HaveOccurred())
	}

	// countedPayload is the legacy reconcile payload shape carried by a child row;
	// the relabel is a pure status change and must NOT touch it.
	countedPayload := func(itemID uint, qty int64) json.RawMessage {
		b, err := json.Marshal(map[string]interface{}{
			"items": []map[string]interface{}{
				{"inventory_item_id": itemID, "quantity": qty},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		return b
	}

	// insertItemWithStatus inserts a child row carrying an OLD (pre-collapse)
	// status via a raw INSERT (the model only knows in_progress now). Requires the
	// permissive CHECK to be in place.
	insertItemWithStatus := func(itemID uint, status string, qty int64) uint {
		db := tenv.ContextfulDB()
		var id uint
		Expect(db.Raw(`
			INSERT INTO reconciliation_request_items
				(submission_id, payload, status, created_at, updated_at)
			VALUES (?, ?, ?, now(), now())
			RETURNING id`,
			submission.ID, countedPayload(itemID, qty), status,
		).Scan(&id).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(&models.ReconciliationRequestItem{}, id) })
		return id
	}

	reload := func(id uint) *models.ReconciliationRequestItem {
		var out models.ReconciliationRequestItem
		Expect(tenv.ContextfulDB().First(&out, id).Error).NotTo(HaveOccurred())
		return &out
	}

	BeforeEach(func() {
		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]
		inventory = fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("collapse-inv-%s", suffix),
			Location: fmt.Sprintf("collapse-loc-%s", suffix),
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

		// Drop to the permissive Part-1 CHECK so the old statuses can be inserted,
		// reproducing the pre-collapse table state. Restore the narrowed CHECK after
		// each spec so the suite's other specs see the migrated constraint.
		setPermissiveCheck()
		DeferCleanup(setNarrowedCheck)
	})

	It("relabels leftover ready/approved/applied rows to in_progress with counts unchanged", func() {
		// Three rows in OLD statuses + one already in_progress. Distinct counts so we
		// can assert the relabel did not touch the payload.
		ready := insertItemWithStatus(101, "ready", 11)
		approved := insertItemWithStatus(102, "approved", 22)
		applied := insertItemWithStatus(103, "applied", 33)
		already := insertItemWithStatus(104, "in_progress", 44)

		Expect(tenv.ContextfulDB().Exec(relabelSQL).Error).NotTo(HaveOccurred())

		for id, qty := range map[uint]int64{ready: 11, approved: 22, applied: 33, already: 44} {
			r := reload(id)
			Expect(r.Status).To(Equal(models.ReconciliationRequestItemStatusInProgress),
				"every row must normalize to in_progress")
			// Counts are unchanged: the relabel is a pure status change.
			var payload struct {
				Items []struct {
					Quantity decimal.Decimal `json:"quantity"`
				} `json:"items"`
			}
			Expect(json.Unmarshal(r.Payload, &payload)).NotTo(HaveOccurred())
			Expect(payload.Items).To(HaveLen(1))
			Expect(payload.Items[0].Quantity.Equal(decimal.NewFromInt(qty))).To(BeTrue(),
				"relabel must not mutate the counted payload (id %d)", id)
		}
	})

	It("the narrowed CHECK then rejects a non-in_progress insert", func() {
		// Relabel, then re-apply the narrowed CHECK exactly as the migration does.
		Expect(tenv.ContextfulDB().Exec(relabelSQL).Error).NotTo(HaveOccurred())
		setNarrowedCheck()

		// An attempt to insert any old status now violates the narrowed CHECK.
		err := tenv.ContextfulDB().Exec(`
			INSERT INTO reconciliation_request_items
				(submission_id, payload, status, created_at, updated_at)
			VALUES (?, ?, 'ready', now(), now())`,
			submission.ID, countedPayload(105, 55),
		).Error
		Expect(err).To(HaveOccurred(), "the narrowed CHECK must reject a non-in_progress status")
		Expect(err.Error()).To(ContainSubstring("chk_reconciliation_request_items_status"))
	})
})
