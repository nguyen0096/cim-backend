package apptest

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

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

// These specs cover the approve branch of ProcessSubmission, which must be
// idempotent under concurrency. Two simultaneous
// approves of the same pending submission must apply the inventory op EXACTLY
// once — the loser is rejected as no-longer-pending and stock is consumed a
// single time.
var _ = Describe("Inventory submission approve idempotency under concurrency", func() {
	const approverEmail = "idem-approver@cim.local"

	var (
		svc       services.InventoryService
		approveCtx context.Context
		inventory *models.Inventory
		itm       *models.InventoryItem
		sub       *models.InventorySubmission
		supplier  *models.Supplier
	)

	approvePerms := func(email string) context.Context {
		ctx := pkg.WithUserEmail(context.Background(), email)
		perms := map[pkg.UserPermission]struct{}{
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionApprove}: {},
		}
		return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
	}

	itemQty := func() decimal.Decimal {
		var reloaded models.InventoryItem
		Expect(tenv.ContextfulDB().First(&reloaded, itm.ID).Error).NotTo(HaveOccurred())
		return reloaded.Quantity
	}

	BeforeEach(func() {
		svc = buildReconInventoryService(repository.NewBaseRepository(tenv.DB))
		approveCtx = approvePerms(approverEmail)

		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]

		inventory = fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("idem-inv-%s", suffix),
			Location: fmt.Sprintf("idem-loc-%s", suffix),
		})
		supplier = fixture.WithSupplier(db, models.Supplier{Name: fmt.Sprintf("idem-sup-%s", suffix)})
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
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", itm.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(itm)
		})

		purchase := &models.InventoryTransaction{
			InventoryItemID: itm.ID,
			SupplierID:      &supplier.ID,
			TransactionType: models.InventoryTransactionTypePurchase,
			Price:           10.0,
			Quantity:        decimal.NewFromInt(100),
		}
		Expect(db.Create(purchase).Error).NotTo(HaveOccurred())

		disposeQty := decimal.NewFromInt(10)
		payload, err := json.Marshal(dto.DisposeInventoryRequest{
			InventoryID: inventory.ID,
			Items:       []dto.QuantityItem{{InventoryItemID: itm.ID, Quantity: &disposeQty}},
		})
		Expect(err).NotTo(HaveOccurred())

		sub = &models.InventorySubmission{
			InventoryID:      inventory.ID,
			SubmissionType:   models.InventorySubmissionTypeDispose,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
			Payload:          json.RawMessage(payload),
		}
		Expect(db.Create(sub).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(sub) })
	})

	It("applies the dispose exactly once when approves race", func() {
		const approvers = 8

		// Barrier so both goroutines pass the outside-tx pending read before either
		// commits, forcing the sub-millisecond race.
		release := make(chan struct{})
		var wg sync.WaitGroup
		errs := make([]error, approvers)

		for i := 0; i < approvers; i++ {
			wg.Add(1)
			go func(idx int) {
				defer GinkgoRecover()
				defer wg.Done()
				<-release
				_, errs[idx] = svc.ProcessSubmission(approveCtx, dto.SubmissionApprovalRequest{
					SubmissionID: sub.ID,
					Action:       string(models.InventorySubmissionActionApprove),
				})
			}(i)
		}
		close(release)
		wg.Wait()

		succeeded := 0
		for _, err := range errs {
			if err == nil {
				succeeded++
			}
		}
		Expect(succeeded).To(Equal(1), "exactly one approve must succeed; got errs=%v", errs)

		// The op applied once: 100 - 10 = 90 (a double-apply would leave 80).
		Expect(itemQty().Equal(decimal.NewFromInt(90))).To(BeTrue(),
			"stock must be consumed exactly once, got %s", itemQty())

		// The submission is approved and completed exactly once.
		stored := &models.InventorySubmission{}
		Expect(tenv.ContextfulDB().First(stored, sub.ID).Error).NotTo(HaveOccurred())
		Expect(stored.ApprovalStatus).To(Equal(models.InventorySubmissionApprovalStatusApproved))
		Expect(stored.ProcessingStatus).To(Equal(models.InventorySubmissionStatusCompleted))
	})
})
