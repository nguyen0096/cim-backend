package apptest

import (
	"context"
	"encoding/json"
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

// These specs exercise SynthesizeSubmissionPayload and the ListSubmissions
// synthesis wiring (epic #38, Part 5) against a REAL Postgres. They confirm that
// an ACTIVE reconcile (initiated, parent payload empty) renders its items by
// summing the live staff child rows by inventory_item_id, attaches the snapshot
// baseline, derives the In-progress/Ready-for-review label, surfaces the
// snapshot-vs-live drift warning, and excludes soft-deleted child rows — while a
// legacy single-payload reconcile keeps reading its persisted payload.
var _ = Describe("Reconciliation synthesize + list/detail/label/warnings", func() {
	const staffEmail = "p5-staff@cim.local"
	const otherEmail = "p5-other@cim.local"

	var (
		svc        services.InventoryService
		staffCtx   context.Context
		otherCtx   context.Context
		listCtx    context.Context
		inventory  *models.Inventory
		itm        *models.InventoryItem
		submission *models.InventorySubmission
		baseline   = decimal.NewFromInt(100)
	)

	buildService := func() services.InventoryService {
		base := repository.NewBaseRepository(tenv.DB)
		return services.NewInventoryService(
			repository.NewInventoryRepository(base),
			repository.NewInventoryItemRepository(base),
			repository.NewInventorySubmissionRepository(base),
			repository.NewReconciliationSnapshotRepository(base),
			repository.NewReconciliationRequestItemRepository(base),
			repository.NewProductRepository(base),
			nil,
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

	list := func() []dto.SubmissionResponse {
		// Mirror the handler: set a valid sort field/order (the handler does this via
		// ValidateAndSetSortParams; ValidateAndSetDefaults alone leaves Sort empty,
		// which would produce an invalid `ORDER BY DESC`).
		params := models.ListParams{
			Sort:  string(dto.SubmissionSortFieldUpdatedAt),
			Order: models.DefaultSortOrder,
		}
		params.ValidateAndSetDefaults()
		resp, _, err := svc.ListSubmissions(listCtx, params, []string{"pending"}, inventory.ID, []string{"reconcile"})
		Expect(err).NotTo(HaveOccurred())
		return resp
	}

	findResp := func(resps []dto.SubmissionResponse, id uint) *dto.SubmissionResponse {
		for i := range resps {
			if resps[i].ID == id {
				return &resps[i]
			}
		}
		return nil
	}

	countItems := func(q int64) []dto.ReconciliationCountItem {
		qty := decimal.NewFromInt(q)
		return []dto.ReconciliationCountItem{{InventoryItemID: itm.ID, Quantity: &qty}}
	}

	BeforeEach(func() {
		svc = buildService()
		staffCtx = staffPerms(staffEmail)
		otherCtx = staffPerms(otherEmail)
		listCtx = pkg.WithUserEmail(context.Background(), "p5-admin@cim.local")

		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]

		inventory = fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("p5-inv-%s", suffix),
			Location: fmt.Sprintf("p5-loc-%s", suffix),
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

	It("synthesizes the summed payload + snapshot baseline for an active reconcile", func() {
		first, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(30),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(first) })

		second, err := svc.CreateReconciliationItem(otherCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(25),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(second) })

		syn, err := svc.SynthesizeSubmissionPayload(listCtx, submission.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(syn.Request.InventoryID).To(Equal(inventory.ID))
		Expect(syn.Request.Items).To(HaveLen(1))
		Expect(syn.Request.Items[0].InventoryItemID).To(Equal(itm.ID))
		Expect(syn.Request.Items[0].Quantity.Equal(decimal.NewFromInt(55))).To(BeTrue(), "30 + 25")
		Expect(syn.Request.Items[0].PrevQuantity.Equal(baseline)).To(BeTrue())
		Expect(syn.Anomalies).To(BeEmpty())
	})

	It("renders synthesized items via ListSubmissions for the active reconcile", func() {
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(60),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })

		resp := findResp(list(), submission.ID)
		Expect(resp).NotTo(BeNil())
		Expect(resp.Items).To(HaveLen(1))
		Expect(resp.Items[0].InventoryItemID).To(Equal(itm.ID))
		Expect(resp.Items[0].Quantity.Equal(decimal.NewFromInt(60))).To(BeTrue())
		// Live (100) == snapshot (100) here, so no drift warning yet.
		Expect(resp.Warnings).To(BeEmpty())
	})

	It("labels In-progress while any live row is below ready, Ready-for-review when all are ready", func() {
		a, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(40),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(a) })

		b, err := svc.CreateReconciliationItem(otherCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(20),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(b) })

		// Mark only one ready -> mixed -> In-progress.
		_, err = svc.SetReconciliationItemReady(staffCtx, dto.SetReconciliationItemReadyRequest{
			SubmissionID: submission.ID, ItemID: a.ID, Ready: true,
		})
		Expect(err).NotTo(HaveOccurred())

		resp := findResp(list(), submission.ID)
		Expect(resp).NotTo(BeNil())
		Expect(resp.ReviewLabel).To(Equal(dto.ReconcileReviewLabelInProgress))

		// Mark the other ready too -> all ready -> Ready-for-review.
		_, err = svc.SetReconciliationItemReady(otherCtx, dto.SetReconciliationItemReadyRequest{
			SubmissionID: submission.ID, ItemID: b.ID, Ready: true,
		})
		Expect(err).NotTo(HaveOccurred())

		resp = findResp(list(), submission.ID)
		Expect(resp).NotTo(BeNil())
		Expect(resp.ReviewLabel).To(Equal(dto.ReconcileReviewLabelReadyForReview))
	})

	It("surfaces the snapshot-vs-live drift warning when live stock changes", func() {
		item, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(50),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(item) })

		// Simulate a PO received during the reconcile: live rises to 110, snapshot
		// stays 100. The B2 drift view must surface this difference.
		Expect(tenv.ContextfulDB().Model(&models.InventoryItem{}).
			Where("id = ?", itm.ID).
			Update("quantity", decimal.NewFromInt(110)).Error).NotTo(HaveOccurred())

		resp := findResp(list(), submission.ID)
		Expect(resp).NotTo(BeNil())
		Expect(resp.Warnings).NotTo(BeEmpty(), "snapshot 100 vs live 110 must raise a drift warning")
		Expect(resp.Items[0].PrevQuantity.Equal(decimal.NewFromInt(100))).To(BeTrue())
		Expect(resp.Items[0].CurrentQuantity.Equal(decimal.NewFromInt(110))).To(BeTrue())
	})

	It("excludes a soft-deleted child row from the synthesized total", func() {
		first, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(70),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(first) })

		second, err := svc.CreateReconciliationItem(otherCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: submission.ID, Items: countItems(20),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(second) })

		Expect(svc.DeleteReconciliationItem(staffCtx, dto.DeleteReconciliationItemRequest{
			SubmissionID: submission.ID, ItemID: first.ID,
		})).To(Succeed())

		syn, err := svc.SynthesizeSubmissionPayload(listCtx, submission.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(syn.Request.Items).To(HaveLen(1))
		Expect(syn.Request.Items[0].Quantity.Equal(decimal.NewFromInt(20))).To(BeTrue(),
			"soft-deleted 70 must be excluded, leaving only 20")
	})

	It("keeps reading the persisted payload for a legacy single-payload reconcile", func() {
		// A legacy reconcile carries a non-empty parent payload and NO snapshot/child
		// rows; ListSubmissions must read that payload, not synthesize, and must not
		// attach a review label. Use a SEPARATE inventory so it does not collide with
		// the BeforeEach pending reconcile under the one-active-pending unique index.
		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]
		legacyInv := fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("p5-legacy-inv-%s", suffix),
			Location: fmt.Sprintf("p5-legacy-loc-%s", suffix),
		})
		unit := fixture.WithUnit(db, fixture.ValidBaseUnit())
		product := fixture.WithProduct(db, fixture.ValidProduct(unit.ID))
		legacyItm := &models.InventoryItem{
			InventoryID: legacyInv.ID,
			ProductID:   product.ID,
			UnitID:      unit.ID,
			Quantity:    baseline,
			Status:      models.InventoryItemStatusActive,
		}
		Expect(db.Create(legacyItm).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(legacyItm) })

		qty := decimal.NewFromInt(42)
		legacyReq := dto.ReconcileInventoryRequest{
			InventoryID: legacyInv.ID,
			Items: []dto.QuantityItem{{
				InventoryItemID: legacyItm.ID,
				Quantity:        &qty,
				PrevQuantity:    baseline,
			}},
		}
		payloadBytes, mErr := json.Marshal(legacyReq)
		Expect(mErr).NotTo(HaveOccurred())
		legacy := &models.InventorySubmission{
			InventoryID:      legacyInv.ID,
			SubmissionType:   models.InventorySubmissionTypeReconcile,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
			Payload:          json.RawMessage(payloadBytes),
		}
		Expect(db.Create(legacy).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(legacy) })

		params := models.ListParams{
			Sort:  string(dto.SubmissionSortFieldUpdatedAt),
			Order: models.DefaultSortOrder,
		}
		params.ValidateAndSetDefaults()
		resps, _, err := svc.ListSubmissions(listCtx, params, []string{"pending"}, legacyInv.ID, []string{"reconcile"})
		Expect(err).NotTo(HaveOccurred())

		resp := findResp(resps, legacy.ID)
		Expect(resp).NotTo(BeNil())
		Expect(resp.Items).To(HaveLen(1))
		Expect(resp.Items[0].Quantity.Equal(decimal.NewFromInt(42))).To(BeTrue())
		Expect(resp.ReviewLabel).To(BeEmpty(), "legacy reconcile must carry no synthesized review label")
	})
})
