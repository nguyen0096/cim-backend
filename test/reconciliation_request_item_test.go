package apptest

import (
	"context"
	"fmt"

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

// These specs exercise the staff reconciliation child-item lifecycle (epic #38,
// Part 4) against a REAL Postgres (the suite runs all migrations), driving the
// real inventoryService over the real repositories. They confirm the state
// machine, ownership/status guards, the counted>snapshot validation, the
// approved->in_progress escape hatch, and soft-delete round-trip against the
// actual schema (incl. the status CHECK constraint and soft-delete index).
var _ = Describe("Reconciliation request item lifecycle", func() {
	const staffEmail = "p4-staff@cim.local"
	const otherEmail = "p4-other@cim.local"

	var (
		svc          services.InventoryService
		staffCtx     context.Context
		otherCtx     context.Context
		inventory    *models.Inventory
		inventoryItm *models.InventoryItem
		submission   *models.InventorySubmission
		baseline     = decimal.NewFromInt(100)
	)

	// buildService wires the real inventoryService from the suite DB. It mirrors
	// internal/server/server.go's construction so the spec drives production code.
	buildService := func() services.InventoryService {
		base := repository.NewBaseRepository(tenv.DB)
		return services.NewInventoryService(
			repository.NewInventoryRepository(base),
			repository.NewInventoryItemRepository(base),
			repository.NewInventorySubmissionRepository(base),
			repository.NewReconciliationSnapshotRepository(base),
			repository.NewReconciliationRequestItemRepository(base),
			repository.NewProductRepository(base),
			nil, // fileStorageService: unused by the child-item paths
			base,
			tenv.DB,
		)
	}

	staffPerms := func(email string) context.Context {
		ctx := pkg.WithUserEmail(context.Background(), email)
		perms := map[pkg.UserPermission]struct{}{
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemCreate}: {},
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemUpdate}: {},
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemReady}:  {},
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemDelete}: {},
		}
		return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
	}

	BeforeEach(func() {
		svc = buildService()
		staffCtx = staffPerms(staffEmail)
		otherCtx = staffPerms(otherEmail)

		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]

		inventory = fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("p4-inv-%s", suffix),
			Location: fmt.Sprintf("p4-loc-%s", suffix),
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

		// Parent placeholder reconcile submission (as initiate would create).
		submission = &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   models.InventorySubmissionTypeReconcile,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
		}
		Expect(db.Create(submission).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(submission) })

		// Snapshot baseline = 100 for the item (the sole prev_quantity source).
		snap := &models.ReconciliationSnapshot{
			SubmissionID:    submission.ID,
			InventoryItemID: inventoryItm.ID,
			PrevQuantity:    baseline,
		}
		Expect(db.Create(snap).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(snap) })
	})

	countItems := func(q int64) []dto.ReconciliationCountItem {
		qty := decimal.NewFromInt(q)
		return []dto.ReconciliationCountItem{{InventoryItemID: inventoryItm.ID, Quantity: &qty}}
	}

	It("creates an in_progress child item owned by the staff member", func() {
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID,
			Items:        countItems(80),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(item.ID).NotTo(BeZero())
		Expect(item.Status).To(Equal(models.ReconciliationRequestItemStatusInProgress))
		Expect(item.CreatedBy).To(Equal(staffEmail))
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })
	})

	It("rejects a counted quantity greater than the snapshot baseline", func() {
		_, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID,
			Items:        countItems(101), // > snapshot 100
		})
		Expect(err).To(HaveOccurred())
		Expect(pkg.IsErrorCode(err, pkg.ErrorCodeValidation)).To(BeTrue())
	})

	It("rejects a second row when the aggregate across live rows exceeds the baseline", func() {
		// First row counts 80 of the item (baseline 100) — allowed.
		first, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(80),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(first) })

		// Second row of 80 passes per-row (80 <= 100) but 80 + 80 = 160 > 100:
		// the aggregate guard (sum across live sibling rows under the parent lock)
		// must reject it. A different staff member to model real fragmented counts.
		_, err = svc.CreateReconciliationItem(otherCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(80),
		})
		Expect(err).To(HaveOccurred())
		Expect(pkg.IsErrorCode(err, pkg.ErrorCodeValidation)).To(BeTrue())
	})

	It("allows fragmented counts that sum to exactly the baseline (60 + 40 == 100)", func() {
		first, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(60),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(first) })

		second, err := svc.CreateReconciliationItem(otherCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(40),
		})
		Expect(err).NotTo(HaveOccurred(), "60 (sibling) + 40 (new) == baseline 100 must be allowed")
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(second) })
	})

	It("excludes a soft-deleted sibling from the aggregate so its count is freed", func() {
		// A first row counts 80, then is soft-deleted: its 80 must no longer count
		// toward the aggregate, so a fresh 80 against baseline 100 is allowed again.
		first, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(80),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(first) })

		Expect(svc.DeleteReconciliationItem(staffCtx, dto.DeleteReconciliationItemRequest{
			SubmissionID: submission.ID, ItemID: first.ID,
		})).To(Succeed())

		second, err := svc.CreateReconciliationItem(otherCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(80),
		})
		Expect(err).NotTo(HaveOccurred(), "a soft-deleted sibling must not count toward the aggregate")
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(second) })
	})

	It("on update, excludes the row's own old value from the aggregate", func() {
		// Sibling counts 40; the row under update currently counts 80. Updating it to
		// 60 must pass (40 + 60 == 100); if its own old 80 were counted it would fail.
		sibling, err := svc.CreateReconciliationItem(otherCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(40),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(sibling) })

		row, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(50), // 40 + 50 == 90 <= 100 OK
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(row) })

		updated, err := svc.UpdateReconciliationItem(staffCtx, dto.UpdateReconciliationItemRequest{
			SubmissionID: submission.ID, ItemID: row.ID, Items: countItems(60),
		})
		Expect(err).NotTo(HaveOccurred(), "40 (sibling) + 60 (new) == 100; own old 50 excluded")
		Expect(updated.Status).To(Equal(models.ReconciliationRequestItemStatusInProgress))

		// Pushing it to 80 now (40 + 80 = 120 > 100) must be rejected.
		_, err = svc.UpdateReconciliationItem(staffCtx, dto.UpdateReconciliationItemRequest{
			SubmissionID: submission.ID, ItemID: row.ID, Items: countItems(80),
		})
		Expect(err).To(HaveOccurred())
		Expect(pkg.IsErrorCode(err, pkg.ErrorCodeValidation)).To(BeTrue())
	})

	It("marks ready then back to in_progress (valid transitions)", func() {
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(80),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })

		ready, err := svc.SetReconciliationItemReady(staffCtx, dto.SetReconciliationItemReadyRequest{
			SubmissionID: submission.ID, ItemID: item.ID, Ready: true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(ready.Status).To(Equal(models.ReconciliationRequestItemStatusReady))

		back, err := svc.SetReconciliationItemReady(staffCtx, dto.SetReconciliationItemReadyRequest{
			SubmissionID: submission.ID, ItemID: item.ID, Ready: false,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(back.Status).To(Equal(models.ReconciliationRequestItemStatusInProgress))
	})

	It("resets an approved row to in_progress when its payload is edited (escape hatch)", func() {
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(80),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })

		// Simulate an admin approval of the row (Part 6 path not built; set directly).
		Expect(tenv.ContextfulDB().Model(item).
			Update("status", models.ReconciliationRequestItemStatusApproved).Error).NotTo(HaveOccurred())

		updated, err := svc.UpdateReconciliationItem(staffCtx, dto.UpdateReconciliationItemRequest{
			SubmissionID: submission.ID, ItemID: item.ID, Items: countItems(70),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(updated.Status).To(Equal(models.ReconciliationRequestItemStatusInProgress),
			"editing an approved row must reset it to in_progress")
	})

	It("forbids a staff member from editing another staff member's row", func() {
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(80),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })

		_, err = svc.UpdateReconciliationItem(otherCtx, dto.UpdateReconciliationItemRequest{
			SubmissionID: submission.ID, ItemID: item.ID, Items: countItems(50),
		})
		Expect(err).To(HaveOccurred())
		Expect(pkg.IsErrorCode(err, pkg.ErrorCodeForbidden)).To(BeTrue())
	})

	It("soft-deletes an in_progress row and refuses to delete an approved row", func() {
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(80),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })

		// approved row cannot be deleted
		Expect(tenv.ContextfulDB().Model(item).
			Update("status", models.ReconciliationRequestItemStatusApproved).Error).NotTo(HaveOccurred())
		err = svc.DeleteReconciliationItem(staffCtx, dto.DeleteReconciliationItemRequest{
			SubmissionID: submission.ID, ItemID: item.ID,
		})
		Expect(err).To(HaveOccurred())
		Expect(pkg.IsErrorCode(err, pkg.ErrorCodeConflict)).To(BeTrue())

		// move back to in_progress, then delete succeeds (soft delete)
		Expect(tenv.ContextfulDB().Model(item).
			Update("status", models.ReconciliationRequestItemStatusInProgress).Error).NotTo(HaveOccurred())
		Expect(svc.DeleteReconciliationItem(staffCtx, dto.DeleteReconciliationItemRequest{
			SubmissionID: submission.ID, ItemID: item.ID,
		})).To(Succeed())

		// row is gone from the default (non-soft-deleted) scope but present unscoped.
		var live int64
		Expect(tenv.ContextfulDB().Model(&models.ReconciliationRequestItem{}).
			Where("id = ?", item.ID).Count(&live).Error).NotTo(HaveOccurred())
		Expect(live).To(BeZero())
		var any int64
		Expect(tenv.ContextfulDB().Unscoped().Model(&models.ReconciliationRequestItem{}).
			Where("id = ?", item.ID).Count(&any).Error).NotTo(HaveOccurred())
		Expect(any).To(Equal(int64(1)))
	})
})
