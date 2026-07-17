package apptest

import (
	"context"
	"encoding/json"
	"errors"
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

// These specs exercise GetSubmissionByID (issue #141): it must build one
// submission via the SAME construction path as ListSubmissions (synthesized
// items, count_breakdown, review_label, warnings), scoped strictly by id, with a
// 404 for a missing id. Per-session provenance in count_breakdown is scoped like
// the sibling ListReconciliationItems: a manager (recon_manage) sees every
// session, a staff caller only their own.
var _ = Describe("GetSubmissionByID (scoped single submission)", func() {
	const staffEmail = "be141-staff@cim.local"
	const otherStaffEmail = "be141-other@cim.local"
	const managerEmail = "be141-manager@cim.local"

	var (
		svc        services.InventoryService
		staffCtx   context.Context
		otherCtx   context.Context
		managerCtx context.Context
		inventory  *models.Inventory
		itm        *models.InventoryItem
		submission *models.InventorySubmission
		baseline   = decimal.NewFromInt(100)
	)

	staffPerms := func(email string) context.Context {
		ctx := pkg.WithUserEmail(context.Background(), email)
		perms := map[pkg.UserPermission]struct{}{
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemCreate}: {},
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemView}:   {},
		}
		return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
	}

	// managerPerms holds recon_manage: the by-id read then folds every session,
	// matching the ListSubmissions element.
	managerPerms := func(email string) context.Context {
		ctx := pkg.WithUserEmail(context.Background(), email)
		perms := map[pkg.UserPermission]struct{}{
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemView}: {},
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconManage}:   {},
		}
		return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
	}

	// listResp returns the ListSubmissions element for the given id (the manager-wide
	// baseline this endpoint must match for a recon_manage caller).
	listResp := func(inventoryID, id uint) *dto.SubmissionResponse {
		params := models.ListParams{Sort: string(dto.SubmissionSortFieldUpdatedAt), Order: models.DefaultSortOrder}
		params.ValidateAndSetDefaults()
		resps, _, err := svc.ListSubmissions(managerCtx, params, []string{"pending"}, inventoryID, []string{"reconcile"})
		Expect(err).NotTo(HaveOccurred())
		for i := range resps {
			if resps[i].ID == id {
				return &resps[i]
			}
		}
		return nil
	}

	// expectMatchesList asserts the by-id response equals the ListSubmissions
	// element field-for-field on the synthesized/derived fields.
	expectMatchesList := func(byID, fromList *dto.SubmissionResponse) {
		Expect(fromList).NotTo(BeNil())
		Expect(byID.ID).To(Equal(fromList.ID))
		Expect(byID.InventoryID).To(Equal(fromList.InventoryID))
		Expect(byID.SubmissionType).To(Equal(fromList.SubmissionType))
		Expect(byID.Status).To(Equal(fromList.Status))
		Expect(byID.ApprovalStatus).To(Equal(fromList.ApprovalStatus))
		Expect(byID.ReconcileStatus).To(Equal(fromList.ReconcileStatus))
		Expect(byID.ReviewLabel).To(Equal(fromList.ReviewLabel))
		Expect(byID.Warnings).To(Equal(fromList.Warnings))
		Expect(byID.ItemWarnings).To(Equal(fromList.ItemWarnings))
		Expect(byID.Items).To(Equal(fromList.Items))
		Expect(byID.CountBreakdown).To(Equal(fromList.CountBreakdown))
		if fromList.Inventory != nil {
			Expect(byID.Inventory).NotTo(BeNil())
			Expect(byID.Inventory.ID).To(Equal(fromList.Inventory.ID))
		}
	}

	countItems := func(q int64, label string) []dto.ReconciliationCountItem {
		qty := decimal.NewFromInt(q)
		return []dto.ReconciliationCountItem{{InventoryItemID: itm.ID, Quantity: &qty, Label: label}}
	}

	BeforeEach(func() {
		base := repository.NewBaseRepository(tenv.DB)
		svc = buildReconInventoryService(base)
		staffCtx = staffPerms(staffEmail)
		otherCtx = staffPerms(otherStaffEmail)
		managerCtx = managerPerms(managerEmail)

		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]

		inventory = fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("be141-inv-%s", suffix),
			Location: fmt.Sprintf("be141-loc-%s", suffix),
		})

		unit := fixture.WithUnit(db, fixture.ValidBaseUnit())
		product := fixture.WithProduct(db, fixture.ValidProduct(unit.ID))

		itm = &models.InventoryItem{
			InventoryID: inventory.ID,
			ProductID:   product.ID,
			UnitID:      unit.ID,
			Quantity:    baseline,
			Status:      models.InventoryItemStatusActive,
		}
		Expect(db.Create(itm).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(itm) })

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
			InventoryItemID: itm.ID,
			PrevQuantity:    baseline,
		}
		Expect(db.Create(snap).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(snap) })
	})

	It("returns the synthesized submission matching the ListSubmissions element for a manager", func() {
		row, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(60, "dock"),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(row) })

		byID, err := svc.GetSubmissionByID(managerCtx, submission.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(byID.Items).To(HaveLen(1))
		Expect(byID.Items[0].InventoryItemID).To(Equal(itm.ID))
		Expect(byID.Items[0].Quantity.Equal(decimal.NewFromInt(60))).To(BeTrue())
		Expect(byID.ReconcileStatus).To(Equal(models.ReconcileLifecycleStatusOpen))
		Expect(byID.ReviewLabel).To(Equal(dto.ReconcileReviewLabelInProgress))
		Expect(byID.CountBreakdown).To(HaveLen(1))
		Expect(byID.CountBreakdown[0].Label).To(Equal("dock"))

		expectMatchesList(byID, listResp(inventory.ID, submission.ID))
	})

	It("surfaces the snapshot-vs-live drift warning identically to the list path", func() {
		row, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(50, ""),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(row) })

		// Live rises to 110 while the snapshot stays 100 (a PO received mid-reconcile).
		Expect(tenv.ContextfulDB().Model(&models.InventoryItem{}).
			Where("id = ?", itm.ID).Update("quantity", decimal.NewFromInt(110)).Error).NotTo(HaveOccurred())

		byID, err := svc.GetSubmissionByID(managerCtx, submission.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(byID.Warnings).NotTo(BeEmpty())
		Expect(byID.Items[0].PrevQuantity.Equal(decimal.NewFromInt(100))).To(BeTrue())
		Expect(byID.Items[0].CurrentQuantity.Equal(decimal.NewFromInt(110))).To(BeTrue())

		expectMatchesList(byID, listResp(inventory.ID, submission.ID))
	})

	It("scopes count_breakdown to the caller's own sessions for staff, full for a manager", func() {
		// Two DISTINCT staff each file their own count session.
		rowA, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Label: "A-session", Items: countItems(30, "aisle-A"),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(rowA) })

		rowB, err := svc.CreateReconciliationItem(otherCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Label: "B-session", Items: countItems(25, "aisle-B"),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(rowB) })

		// Staff A: count_breakdown carries ONLY A's own session — never B's.
		staffView, err := svc.GetSubmissionByID(staffCtx, submission.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(staffView.CountBreakdown).To(HaveLen(1))
		Expect(staffView.CountBreakdown[0].CreatedBy).To(Equal(staffEmail))
		Expect(staffView.CountBreakdown[0].SessionLabel).To(Equal("A-session"))
		for _, b := range staffView.CountBreakdown {
			Expect(b.CreatedBy).NotTo(Equal(otherStaffEmail), "staff must not see another staff member's session")
		}
		// The staff view folds only their own contribution.
		Expect(staffView.Items).To(HaveLen(1))
		Expect(staffView.Items[0].Quantity.Equal(decimal.NewFromInt(30))).To(BeTrue())

		// Manager: sees BOTH sessions and the full aggregate — equal to the list.
		managerView, err := svc.GetSubmissionByID(managerCtx, submission.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(managerView.CountBreakdown).To(HaveLen(2))
		creators := map[string]struct{}{}
		for _, b := range managerView.CountBreakdown {
			creators[b.CreatedBy] = struct{}{}
		}
		Expect(creators).To(HaveKey(staffEmail))
		Expect(creators).To(HaveKey(otherStaffEmail))
		Expect(managerView.Items[0].Quantity.Equal(decimal.NewFromInt(55))).To(BeTrue(), "30 + 25")
		expectMatchesList(managerView, listResp(inventory.ID, submission.ID))
	})

	It("scopes overage item_warnings to the caller's own counts", func() {
		// A's 60 + B's 50 = 110 exceeds baseline 100 (aggregate overage), but A's own
		// 60 does not. The overage item_warning must show for a manager (folds both)
		// and NOT for staff A (folds only their own) — no cross-staff leak via warnings.
		rowA, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(60, ""),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(rowA) })

		rowB, err := svc.CreateReconciliationItem(otherCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(50, ""),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(rowB) })

		hasOverage := func(ws []dto.SubmissionItemWarning) bool {
			for _, w := range ws {
				if w.Code == dto.SubmissionItemWarningOverage {
					return true
				}
			}
			return false
		}

		managerView, err := svc.GetSubmissionByID(managerCtx, submission.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(hasOverage(managerView.ItemWarnings)).To(BeTrue(), "aggregate 110 > baseline 100")
		expectMatchesList(managerView, listResp(inventory.ID, submission.ID))

		staffView, err := svc.GetSubmissionByID(staffCtx, submission.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(hasOverage(staffView.ItemWarnings)).To(BeFalse(), "own 60 <= baseline 100: no overage leaked from B")
	})

	It("scopes a PROCESSED reconcile to the caller's own sessions for staff; manager gets the persisted payload", func() {
		// Two staff file sessions while the reconcile is open.
		rowA, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Label: "A-session", Items: countItems(30, "aisle-A"),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(rowA) })

		rowB, err := svc.CreateReconciliationItem(otherCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Label: "B-session", Items: countItems(25, "aisle-B"),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(rowB) })

		// Simulate StartProcessing: persist the manager-wide synthesized payload and
		// mark the submission processed (child rows + snapshot are retained).
		syn, err := svc.SynthesizeSubmissionPayload(managerCtx, submission.ID)
		Expect(err).NotTo(HaveOccurred())
		payloadBytes, mErr := json.Marshal(syn.Request)
		Expect(mErr).NotTo(HaveOccurred())
		Expect(tenv.ContextfulDB().Model(&models.InventorySubmission{}).
			Where("id = ?", submission.ID).
			Updates(map[string]interface{}{
				"payload":           payloadBytes,
				"processing_status": models.InventorySubmissionStatusCompleted,
				"reconcile_status":  models.ReconcileLifecycleStatusProcessed,
			}).Error).NotTo(HaveOccurred())

		// Manager: the persisted manager-wide payload (55 = 30 + 25), read as-is.
		managerView, err := svc.GetSubmissionByID(managerCtx, submission.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(managerView.Items).To(HaveLen(1))
		Expect(managerView.Items[0].Quantity.Equal(decimal.NewFromInt(55))).To(BeTrue(), "manager sees the applied total")

		// Staff A: re-synthesized from OWN rows only — never staff B's session.
		staffView, err := svc.GetSubmissionByID(staffCtx, submission.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(staffView.Items).To(HaveLen(1))
		Expect(staffView.Items[0].Quantity.Equal(decimal.NewFromInt(30))).To(BeTrue(), "staff sees only their own count, not the applied total")
		for _, b := range staffView.CountBreakdown {
			Expect(b.CreatedBy).NotTo(Equal(otherStaffEmail), "processed by-id must not disclose another staff member's session")
		}
		if len(staffView.CountBreakdown) > 0 {
			Expect(staffView.CountBreakdown[0].CreatedBy).To(Equal(staffEmail))
			Expect(staffView.CountBreakdown[0].SessionLabel).To(Equal("A-session"))
		}
	})

	It("404s for a missing id", func() {
		var maxID uint
		Expect(tenv.ContextfulDB().Model(&models.InventorySubmission{}).
			Select("COALESCE(MAX(id),0)").Scan(&maxID).Error).NotTo(HaveOccurred())

		_, err := svc.GetSubmissionByID(managerCtx, maxID+1)
		Expect(err).To(HaveOccurred())
		var appErr *pkg.AppError
		Expect(errors.As(err, &appErr)).To(BeTrue())
		Expect(appErr.HTTPStatus()).To(Equal(404))
	})

	It("resolves strictly by submission id, independent of the caller's inventory context", func() {
		// A second reconcile under a DIFFERENT inventory. The by-id read has no
		// inventory in the request (the FE holds only the submission id), so — like
		// the sibling reconciliation-items endpoint — it scopes purely by id.
		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]
		otherInv := fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("be141-other-inv-%s", suffix),
			Location: fmt.Sprintf("be141-other-loc-%s", suffix),
		})
		otherSub := &models.InventorySubmission{
			InventoryID:      otherInv.ID,
			SubmissionType:   models.InventorySubmissionTypeReconcile,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
			ReconcileStatus:  models.ReconcileLifecycleStatusOpen,
		}
		Expect(db.Create(otherSub).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(otherSub) })

		byID, err := svc.GetSubmissionByID(managerCtx, otherSub.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(byID.ID).To(Equal(otherSub.ID))
		Expect(byID.InventoryID).To(Equal(otherInv.ID))

		expectMatchesList(byID, listResp(otherInv.ID, otherSub.ID))
	})
})
