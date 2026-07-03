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

// These specs pin the terminal-on-cancel contract (issue #109) against a REAL
// Postgres: a canceled reconcile must drop out of the approval_status-keyed
// pending list (#5) yet still surface its retained child count rows in the
// detail/read path for audit (#3), while the active queue keeps excluding it.
var _ = Describe("Reconciliation cancel terminal state", func() {
	const staffEmail = "cancel-staff@cim.local"
	const adminEmail = "cancel-admin@cim.local"

	var (
		svc       services.InventoryService
		staffCtx  context.Context
		adminCtx  context.Context
		inventory *models.Inventory
		itm       *models.InventoryItem
		sub       *models.InventorySubmission
	)

	buildService := func() services.InventoryService {
		return buildReconInventoryService(repository.NewBaseRepository(tenv.DB))
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
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconManage}:   {},
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemView}: {},
		}
		return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
	}

	listParams := func() models.ListParams {
		p := models.ListParams{Page: 1, Limit: 100, Sort: "id", Order: "asc"}
		p.ValidateAndSetDefaults()
		return p
	}

	findSub := func(rows []dto.SubmissionResponse, id uint) *dto.SubmissionResponse {
		for i := range rows {
			if rows[i].ID == id {
				return &rows[i]
			}
		}
		return nil
	}

	BeforeEach(func() {
		svc = buildService()
		staffCtx = staffPerms(staffEmail)
		adminCtx = adminPerms(adminEmail)

		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]

		inventory = fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("cancel-inv-%s", suffix),
			Location: fmt.Sprintf("cancel-loc-%s", suffix),
		})
		unit := fixture.WithUnit(db, fixture.ValidBaseUnit())
		product := fixture.WithProduct(db, fixture.ValidProduct(unit.ID))

		itm = &models.InventoryItem{
			InventoryID: inventory.ID,
			ProductID:   product.ID,
			UnitID:      unit.ID,
			Quantity:    decimal.NewFromInt(100),
			Status:      models.InventoryItemStatusActive,
		}
		Expect(db.Create(itm).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(itm) })

		// Initiated reconcile: snapshot baseline = 100, lifecycle open, approval pending.
		sub = &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   models.InventorySubmissionTypeReconcile,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
			ReconcileStatus:  models.ReconcileLifecycleStatusOpen,
		}
		Expect(db.Create(sub).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("submission_id = ?", sub.ID).Delete(&models.ReconciliationRequestItem{})
			db.Unscoped().Where("submission_id = ?", sub.ID).Delete(&models.ReconciliationSnapshot{})
			db.Unscoped().Delete(sub)
		})

		snap := &models.ReconciliationSnapshot{
			SubmissionID:    sub.ID,
			InventoryItemID: itm.ID,
			PrevQuantity:    decimal.NewFromInt(100),
		}
		Expect(db.Create(snap).Error).NotTo(HaveOccurred())
	})

	// seedCountThenCancel files one staff count row (counted 60) then cancels the
	// reconcile, returning it to the terminal canceled state with the child row kept.
	seedCountThenCancel := func() {
		qty := decimal.NewFromInt(60)
		_, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: sub.ID,
			Items:        []dto.ReconciliationCountItem{{InventoryItemID: itm.ID, Quantity: &qty}},
		})
		Expect(err).NotTo(HaveOccurred())

		canceled, err := svc.CancelReconciliation(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(canceled.ReconcileStatus).To(Equal(models.ReconcileLifecycleStatusCanceled))
		Expect(canceled.ProcessingStatus).To(Equal(models.InventorySubmissionStatusCanceled))
	}

	It("terminalizes approval_status so a canceled reconcile drops out of the pending list (#5)", func() {
		seedCountThenCancel()

		pending, _, err := svc.ListSubmissions(adminCtx, listParams(),
			[]string{string(models.InventorySubmissionApprovalStatusPending)},
			inventory.ID,
			[]string{string(models.InventorySubmissionTypeReconcile)})
		Expect(err).NotTo(HaveOccurred())
		Expect(findSub(pending, sub.ID)).To(BeNil(),
			"a canceled reconcile must not appear in the approval_status=pending list")

		// The row itself carries a terminal approval_status.
		var reloaded models.InventorySubmission
		Expect(tenv.ContextfulDB().First(&reloaded, sub.ID).Error).NotTo(HaveOccurred())
		Expect(reloaded.ApprovalStatus).NotTo(Equal(models.InventorySubmissionApprovalStatusPending),
			"cancel must terminalize approval_status")
	})

	It("surfaces the retained child count rows in the canceled reconcile's detail/read (#3)", func() {
		seedCountThenCancel()

		all, _, err := svc.ListSubmissions(adminCtx, listParams(),
			nil, inventory.ID,
			[]string{string(models.InventorySubmissionTypeReconcile)})
		Expect(err).NotTo(HaveOccurred())

		got := findSub(all, sub.ID)
		Expect(got).NotTo(BeNil(), "the canceled reconcile must still be readable")
		Expect(got.ReconcileStatus).To(Equal(models.ReconcileLifecycleStatusCanceled))
		Expect(got.Items).To(HaveLen(1), "the kept child count rows must be surfaced for audit")
		Expect(got.Items[0].InventoryItemID).To(Equal(itm.ID))
		Expect(got.Items[0].Quantity).NotTo(BeNil())
		Expect(got.Items[0].Quantity.Equal(decimal.NewFromInt(60))).To(BeTrue(),
			"the surfaced count must be the summed child rows, got %s", got.Items[0].Quantity)
	})

	It("keeps the active reconcile queue excluding a canceled reconcile", func() {
		seedCountThenCancel()

		queue, _, err := svc.ListActiveReconciliations(adminCtx, listParams(), nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(findSub(queue, sub.ID)).To(BeNil(),
			"a canceled reconcile must never appear in the active queue")
	})

	It("rejects a re-cancel of an already-canceled reconcile with a 409 (#108)", func() {
		seedCountThenCancel()

		_, err := svc.CancelReconciliation(adminCtx, sub.ID)
		Expect(err).To(HaveOccurred())
		Expect(pkg.IsErrorCode(err, pkg.ErrorCodeConflict)).To(BeTrue(),
			"re-cancel must be a 409/conflict, got %v", err)
	})

	It("rejects a child-item edit on a canceled reconcile (loadActiveReconcileParent, #108)", func() {
		seedCountThenCancel()

		qty := decimal.NewFromInt(10)
		_, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: sub.ID,
			Items:        []dto.ReconciliationCountItem{{InventoryItemID: itm.ID, Quantity: &qty}},
		})
		Expect(err).To(HaveOccurred(),
			"a canceled reconcile is no longer in flight; child edits must be rejected")
		Expect(pkg.IsErrorCode(err, pkg.ErrorCodeConflict)).To(BeTrue(),
			"child edit on a canceled reconcile must be a 409/conflict, got %v", err)
	})
})
