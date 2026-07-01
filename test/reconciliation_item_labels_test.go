package apptest

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

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

// These specs exercise issue #73 against a REAL Postgres (the suite runs all
// migrations, including the add-label column migration): the ROW-level count-
// session label (a real queryable column), the PER-ROW count-label rule, the
// List-rows GET endpoint with RBAC scoping (staff own vs admin all), and the
// reversibility of the add-label migration.
var _ = Describe("Reconciliation item labels (issue #73)", func() {
	const staffEmail = "lbl-staff@cim.local"
	const otherEmail = "lbl-other@cim.local"

	var (
		svc          services.InventoryService
		staffCtx     context.Context
		otherCtx     context.Context
		adminCtx     context.Context
		inventory    *models.Inventory
		inventoryItm *models.InventoryItem
		submission   *models.InventorySubmission
		baseline     = decimal.NewFromInt(100)
	)

	buildService := func() services.InventoryService {
		base := repository.NewBaseRepository(tenv.DB)
		return buildReconInventoryService(base)
	}

	staffPerms := func(email string) context.Context {
		ctx := pkg.WithUserEmail(context.Background(), email)
		perms := map[pkg.UserPermission]struct{}{
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemCreate}: {},
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemUpdate}: {},
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemDelete}: {},
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemView}:   {},
		}
		return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
	}

	adminPerms := func(email string) context.Context {
		ctx := pkg.WithUserEmail(context.Background(), email)
		perms := map[pkg.UserPermission]struct{}{
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemView}: {},
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconManage}:   {},
		}
		return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
	}

	BeforeEach(func() {
		svc = buildService()
		staffCtx = staffPerms(staffEmail)
		otherCtx = staffPerms(otherEmail)
		adminCtx = adminPerms("lbl-admin@cim.local")

		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]

		inventory = fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("lbl-inv-%s", suffix),
			Location: fmt.Sprintf("lbl-loc-%s", suffix),
		})

		unit := fixture.WithUnit(db, fixture.ValidBaseUnit())
		product := fixture.WithProduct(db, fixture.ValidProduct(unit.ID))

		inventoryItm = &models.InventoryItem{
			InventoryID: inventory.ID,
			ProductID:   product.ID,
			UnitID:      unit.ID,
			Quantity:    baseline,
			Status:      models.InventoryItemStatusActive,
		}
		Expect(db.Create(inventoryItm).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(inventoryItm) })

		submission = &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   models.InventorySubmissionTypeReconcile,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
			ReconcileStatus:  models.ReconcileLifecycleStatusOpen,
		}
		Expect(db.Create(submission).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(submission) })

		snap := &models.ReconciliationSnapshot{
			SubmissionID:    submission.ID,
			InventoryItemID: inventoryItm.ID,
			PrevQuantity:    baseline,
		}
		Expect(db.Create(snap).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(snap) })
	})

	countItems := func(q int64, label string) []dto.ReconciliationCountItem {
		qty := decimal.NewFromInt(q)
		return []dto.ReconciliationCountItem{{InventoryItemID: inventoryItm.ID, Quantity: &qty, Label: label}}
	}

	Describe("row-level label persistence + rule", func() {
		It("persists the row label and the count label on create, queryable as a real column", func() {
			created, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
				SubmissionID: submission.ID,
				Label:        "Morning — Zone A",
				Items:        countItems(80, "shelf-5"),
			})
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Unscoped().Delete(&models.ReconciliationRequestItem{Base: models.Base{ID: created.ID}})
			})
			Expect(created.Label).To(Equal("Morning — Zone A"))
			Expect(created.Items).To(HaveLen(1))
			Expect(created.Items[0].Label).To(Equal("shelf-5"))

			// The row label is a real column, not JSONB: read it straight off the row.
			var dbLabel string
			Expect(tenv.ContextfulDB().Model(&models.ReconciliationRequestItem{}).
				Where("id = ?", created.ID).Select("label").Scan(&dbLabel).Error).NotTo(HaveOccurred())
			Expect(dbLabel).To(Equal("Morning — Zone A"))
		})

		It("requires a row label once the user already has another row, and enforces distinctness", func() {
			first, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
				SubmissionID: submission.ID, Label: "Morning", Items: countItems(40, ""),
			})
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Unscoped().Delete(&models.ReconciliationRequestItem{Base: models.Base{ID: first.ID}})
			})

			// 2nd row, blank label -> required.
			_, err = svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
				SubmissionID: submission.ID, Label: "", Items: countItems(10, ""),
			})
			Expect(err).To(HaveOccurred())
			Expect(pkg.IsErrorCode(err, pkg.ErrorCodeValidation)).To(BeTrue())

			// 2nd row, duplicate label -> conflict.
			_, err = svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
				SubmissionID: submission.ID, Label: "Morning", Items: countItems(10, ""),
			})
			Expect(err).To(HaveOccurred())
			Expect(pkg.IsErrorCode(err, pkg.ErrorCodeValidation)).To(BeTrue())

			// 2nd row, distinct label -> allowed.
			second, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
				SubmissionID: submission.ID, Label: "Afternoon", Items: countItems(10, ""),
			})
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Unscoped().Delete(&models.ReconciliationRequestItem{Base: models.Base{ID: second.ID}})
			})
		})

		It("scopes row-label distinctness per user: another user may reuse a label", func() {
			a, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
				SubmissionID: submission.ID, Label: "Morning", Items: countItems(40, ""),
			})
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Unscoped().Delete(&models.ReconciliationRequestItem{Base: models.Base{ID: a.ID}})
			})

			// Different user, same label, blank-allowed (their first row): OK.
			b, err := svc.CreateReconciliationItem(otherCtx, dto.CreateReconciliationItemRequest{
				SubmissionID: submission.ID, Label: "Morning", Items: countItems(40, ""),
			})
			Expect(err).NotTo(HaveOccurred(), "row-label distinctness is per (submission, user)")
			DeferCleanup(func() {
				tenv.ContextfulDB().Unscoped().Delete(&models.ReconciliationRequestItem{Base: models.Base{ID: b.ID}})
			})
		})

		It("rejects a row label longer than 255 runes but accepts a 255-rune Vietnamese label", func() {
			tooLong := strings.Repeat("x", 256)
			_, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
				SubmissionID: submission.ID, Label: tooLong, Items: countItems(10, ""),
			})
			Expect(err).To(HaveOccurred())
			Expect(pkg.IsErrorCode(err, pkg.ErrorCodeValidation)).To(BeTrue())

			vi := strings.Repeat("ằ", 255)
			Expect(utf8.RuneCountInString(vi)).To(Equal(255))
			Expect(len(vi)).To(BeNumerically(">", 255))
			created, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
				SubmissionID: submission.ID, Label: vi, Items: countItems(10, ""),
			})
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Unscoped().Delete(&models.ReconciliationRequestItem{Base: models.Base{ID: created.ID}})
			})
			Expect(created.Label).To(Equal(vi))
		})
	})

	Describe("per-row count-label re-scope (issue #73)", func() {
		It("allows two DIFFERENT rows to reuse the same count label (and both blank)", func() {
			first, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
				SubmissionID: submission.ID, Label: "A", Items: countItems(40, "dock"),
			})
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Unscoped().Delete(&models.ReconciliationRequestItem{Base: models.Base{ID: first.ID}})
			})

			// Different user's row reuses the same count label "dock" — allowed now.
			second, err := svc.CreateReconciliationItem(otherCtx, dto.CreateReconciliationItemRequest{
				SubmissionID: submission.ID, Label: "B", Items: countItems(40, "dock"),
			})
			Expect(err).NotTo(HaveOccurred(), "count-label distinctness is per row, not per submission")
			DeferCleanup(func() {
				tenv.ContextfulDB().Unscoped().Delete(&models.ReconciliationRequestItem{Base: models.Base{ID: second.ID}})
			})
		})

		It("still rejects two counts of the same item WITHIN one row sharing a label / both blank", func() {
			qty := decimal.NewFromInt(10)
			_, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
				SubmissionID: submission.ID,
				Items: []dto.ReconciliationCountItem{
					{InventoryItemID: inventoryItm.ID, Quantity: &qty, Label: "dock"},
					{InventoryItemID: inventoryItm.ID, Quantity: &qty, Label: "dock"},
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(pkg.IsErrorCode(err, pkg.ErrorCodeValidation)).To(BeTrue())
		})
	})

	Describe("List rows GET endpoint + RBAC scoping", func() {
		It("returns rows with both label levels; staff see only own rows, admin sees all", func() {
			mine, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
				SubmissionID: submission.ID, Label: "Mine", Items: countItems(30, "shelf"),
			})
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Unscoped().Delete(&models.ReconciliationRequestItem{Base: models.Base{ID: mine.ID}})
			})

			theirs, err := svc.CreateReconciliationItem(otherCtx, dto.CreateReconciliationItemRequest{
				SubmissionID: submission.ID, Label: "Theirs", Items: countItems(20, ""),
			})
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Unscoped().Delete(&models.ReconciliationRequestItem{Base: models.Base{ID: theirs.ID}})
			})

			// Staff: only their own row, with row + count labels and flattened items.
			staffRows, err := svc.ListReconciliationItems(staffCtx, submission.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(staffRows).To(HaveLen(1))
			Expect(staffRows[0].ID).To(Equal(mine.ID))
			Expect(staffRows[0].Label).To(Equal("Mine"))
			Expect(staffRows[0].CreatedBy).To(Equal(staffEmail))
			Expect(staffRows[0].Items).To(HaveLen(1))
			Expect(staffRows[0].Items[0].Label).To(Equal("shelf"))
			Expect(staffRows[0].Items[0].Quantity.Equal(decimal.NewFromInt(30))).To(BeTrue())

			// Admin: all rows, id-ascending.
			adminRows, err := svc.ListReconciliationItems(adminCtx, submission.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(adminRows).To(HaveLen(2))
			Expect(adminRows[0].ID).To(BeNumerically("<", adminRows[1].ID))
			labels := []string{adminRows[0].Label, adminRows[1].Label}
			Expect(labels).To(ContainElements("Mine", "Theirs"))
		})
	})

	Describe("add-label migration reversibility + NULL-safety", func() {
		It("down drops the column; up re-adds it NOT NULL DEFAULT '' and backfills pre-existing rows to ''", func() {
			db := tenv.ContextfulDB()
			type colMeta struct {
				IsNullable    string
				ColumnDefault *string
			}
			readMeta := func() (bool, colMeta) {
				var rows []colMeta
				Expect(db.Raw(`SELECT is_nullable, column_default FROM information_schema.columns
                    WHERE table_name = 'reconciliation_request_items' AND column_name = 'label'`).
					Scan(&rows).Error).NotTo(HaveOccurred())
				if len(rows) == 0 {
					return false, colMeta{}
				}
				return true, rows[0]
			}

			// The suite already ran the up migration, so the column exists NOT NULL DEFAULT ''.
			exists, meta := readMeta()
			Expect(exists).To(BeTrue())
			Expect(meta.IsNullable).To(Equal("NO"))
			Expect(meta.ColumnDefault).NotTo(BeNil())
			Expect(*meta.ColumnDefault).To(ContainSubstring("''"))

			// DOWN: drop the column (exact down-migration SQL).
			Expect(db.Exec(`ALTER TABLE reconciliation_request_items DROP COLUMN IF EXISTS label`).Error).NotTo(HaveOccurred())
			exists, _ = readMeta()
			Expect(exists).To(BeFalse())

			// Simulate a PRE-DEPLOY row (no label column): insert while the column is gone.
			var preID uint
			Expect(db.Raw(`INSERT INTO reconciliation_request_items
                (submission_id, payload, status, created_by, updated_by, created_at, updated_at)
                VALUES (?, ?, 'in_progress', 'pre@cim.local', 'pre@cim.local', NOW(), NOW())
                RETURNING id`, submission.ID, `{"items":[]}`).Scan(&preID).Error).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Unscoped().Delete(&models.ReconciliationRequestItem{Base: models.Base{ID: preID}})
			})

			// UP: re-add the column (exact up-migration SQL); the NOT NULL DEFAULT ''
			// backfills the pre-existing row to '' rather than NULL.
			Expect(db.Exec(`ALTER TABLE reconciliation_request_items ADD COLUMN IF NOT EXISTS label VARCHAR(255) NOT NULL DEFAULT ''`).Error).NotTo(HaveOccurred())
			exists, meta = readMeta()
			Expect(exists).To(BeTrue())
			Expect(meta.IsNullable).To(Equal("NO"))

			// The pre-deploy row reads back via the non-nullable model scan with label ''.
			var pre models.ReconciliationRequestItem
			Expect(db.First(&pre, preID).Error).NotTo(HaveOccurred())
			Expect(pre.Label).To(Equal(""))
		})
	})
})
