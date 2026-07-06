package apptest

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/shopspring/decimal"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil/fixture"
)

// Found (zero-cost) stock is real and transferable: a transfer-out consuming a
// reconcile_stock_up layer must carry Price 0 to the destination transfer-in,
// and total stock across both inventories must be conserved.
var _ = Describe("Transfer of found (zero-cost) reconcile stock", func() {
	approvePerms := func(email string) context.Context {
		ctx := pkg.WithUserEmail(context.Background(), email)
		perms := map[pkg.UserPermission]struct{}{
			{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionApprove}: {},
		}
		return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
	}

	It("carries zero cost to the destination and conserves stock", func() {
		svc := buildReconInventoryService(repository.NewBaseRepository(tenv.DB))
		approveCtx := approvePerms("transfer-approver@cim.local")
		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]

		src := fixture.WithInventory(db, models.Inventory{Name: fmt.Sprintf("src-%s", suffix), Status: models.InventoryStatusActive})
		dst := fixture.WithInventory(db, models.Inventory{Name: fmt.Sprintf("dst-%s", suffix), Status: models.InventoryStatusActive})
		unit := fixture.WithUnit(db, fixture.ValidBaseUnit())
		product := fixture.WithProduct(db, fixture.ValidProduct(unit.ID))

		srcItem := &models.InventoryItem{InventoryID: src.ID, ProductID: product.ID, UnitID: unit.ID, Quantity: decimal.NewFromInt(20), Status: models.InventoryItemStatusActive}
		Expect(db.Create(srcItem).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", srcItem.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(srcItem)
		})

		// The item's entire on-hand is found (zero-cost) stock.
		stockUp := &models.InventoryTransaction{
			InventoryItemID: srcItem.ID,
			TransactionType: models.InventoryTransactionTypeReconcileStockUp,
			Price:           0,
			Quantity:        decimal.NewFromInt(20),
		}
		Expect(db.Create(stockUp).Error).NotTo(HaveOccurred())

		transferQty := decimal.NewFromInt(8)
		payload, err := json.Marshal(dto.TransferInventoryRequest{
			SourceInventoryID:      src.ID,
			DestinationInventoryID: dst.ID,
			Items:                  []dto.QuantityItem{{InventoryItemID: srcItem.ID, Quantity: &transferQty}},
		})
		Expect(err).NotTo(HaveOccurred())

		sub := &models.InventorySubmission{
			InventoryID:      src.ID,
			SubmissionType:   models.InventorySubmissionTypeTransfer,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
			Payload:          json.RawMessage(payload),
		}
		Expect(db.Create(sub).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(sub) })

		_, err = svc.ProcessSubmission(approveCtx, dto.SubmissionApprovalRequest{
			SubmissionID: sub.ID,
			Action:       string(models.InventorySubmissionActionApprove),
		})
		Expect(err).NotTo(HaveOccurred())

		// Destination transfer-in inherits the zero-cost layer's price.
		var transferIns []models.InventoryTransaction
		Expect(db.Joins("JOIN inventory_items ii ON ii.id = inventory_transactions.inventory_item_id").
			Where("ii.inventory_id = ? AND inventory_transactions.transaction_type = ?", dst.ID, models.InventoryTransactionTypeTransferIn).
			Find(&transferIns).Error).NotTo(HaveOccurred())
		Expect(transferIns).To(HaveLen(1))
		Expect(transferIns[0].Price).To(Equal(0.0), "transfer-in of found stock must carry zero cost")
		Expect(transferIns[0].Quantity.Equal(transferQty)).To(BeTrue())
		Expect(transferIns[0].IsAdjustment).To(BeTrue(), "transfer-in of found stock must carry the adjustment provenance flag")

		// Stock conserved: source 20-8=12, destination 0+8=8, total unchanged at 20.
		var reloadedSrc models.InventoryItem
		Expect(db.First(&reloadedSrc, srcItem.ID).Error).NotTo(HaveOccurred())
		Expect(reloadedSrc.Quantity.Equal(decimal.NewFromInt(12))).To(BeTrue(), "source on-hand must drop by transferred qty, got %s", reloadedSrc.Quantity)

		var dstItem models.InventoryItem
		Expect(db.Where("inventory_id = ? AND product_id = ?", dst.ID, product.ID).First(&dstItem).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", dstItem.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(&dstItem)
		})
		Expect(dstItem.Quantity.Equal(transferQty)).To(BeTrue(), "destination on-hand must equal transferred qty, got %s", dstItem.Quantity)
		Expect(reloadedSrc.Quantity.Add(dstItem.Quantity).Equal(decimal.NewFromInt(20))).To(BeTrue(), "total stock must be conserved")
	})

	It("splits a mixed FIFO transfer-out (found + normal layers) into separate transfer-in txns with correct provenance", func() {
		svc := buildReconInventoryService(repository.NewBaseRepository(tenv.DB))
		approveCtx := approvePerms("mix-approver@cim.local")
		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]

		src := fixture.WithInventory(db, models.Inventory{Name: fmt.Sprintf("mixsrc-%s", suffix), Status: models.InventoryStatusActive})
		dst := fixture.WithInventory(db, models.Inventory{Name: fmt.Sprintf("mixdst-%s", suffix), Status: models.InventoryStatusActive})
		supplier := fixture.WithSupplier(db, models.Supplier{Name: fmt.Sprintf("mixsup-%s", suffix)})
		unit := fixture.WithUnit(db, fixture.ValidBaseUnit())
		product := fixture.WithProduct(db, fixture.ValidProduct(unit.ID))

		srcItem := &models.InventoryItem{InventoryID: src.ID, ProductID: product.ID, UnitID: unit.ID, Quantity: decimal.NewFromInt(15), Status: models.InventoryItemStatusActive}
		Expect(db.Create(srcItem).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", srcItem.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(srcItem)
		})

		// FIFO layer 1 (earlier created_at → consumed first): found, zero-cost.
		Expect(db.Create(&models.InventoryTransaction{
			Base:            models.Base{CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
			InventoryItemID: srcItem.ID, TransactionType: models.InventoryTransactionTypeReconcileStockUp,
			Price: 0, Quantity: decimal.NewFromInt(5), IsAdjustment: true,
		}).Error).NotTo(HaveOccurred())
		// FIFO layer 2: normal purchase.
		Expect(db.Create(&models.InventoryTransaction{
			Base:            models.Base{CreatedAt: time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)},
			InventoryItemID: srcItem.ID, SupplierID: &supplier.ID, TransactionType: models.InventoryTransactionTypePurchase,
			Price: 8, Quantity: decimal.NewFromInt(10),
		}).Error).NotTo(HaveOccurred())

		// Transfer 8 → FIFO draws 5 from the found layer + 3 from the normal layer.
		transferQty := decimal.NewFromInt(8)
		payload, err := json.Marshal(dto.TransferInventoryRequest{
			SourceInventoryID: src.ID, DestinationInventoryID: dst.ID,
			Items: []dto.QuantityItem{{InventoryItemID: srcItem.ID, Quantity: &transferQty}},
		})
		Expect(err).NotTo(HaveOccurred())
		sub := &models.InventorySubmission{
			InventoryID: src.ID, SubmissionType: models.InventorySubmissionTypeTransfer,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
			Payload:          json.RawMessage(payload),
		}
		Expect(db.Create(sub).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(sub) })
		_, err = svc.ProcessSubmission(approveCtx, dto.SubmissionApprovalRequest{
			SubmissionID: sub.ID, Action: string(models.InventorySubmissionActionApprove),
		})
		Expect(err).NotTo(HaveOccurred())

		var dstItem models.InventoryItem
		Expect(db.Where("inventory_id = ? AND product_id = ?", dst.ID, product.ID).First(&dstItem).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", dstItem.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(&dstItem)
		})

		// Two SEPARATE transfer-in txns: found (5, price 0, flagged) + normal (3, price 8, unflagged).
		var transferIns []models.InventoryTransaction
		Expect(db.Where("inventory_item_id = ? AND transaction_type = ?", dstItem.ID, models.InventoryTransactionTypeTransferIn).
			Find(&transferIns).Error).NotTo(HaveOccurred())
		Expect(transferIns).To(HaveLen(2), "a mixed FIFO consume must yield one transfer-in per source layer")

		var found, normal *models.InventoryTransaction
		for i := range transferIns {
			if transferIns[i].IsAdjustment {
				found = &transferIns[i]
			} else {
				normal = &transferIns[i]
			}
		}
		Expect(found).NotTo(BeNil(), "found portion must be present")
		Expect(normal).NotTo(BeNil(), "normal portion must be present")
		Expect(found.Quantity.Equal(decimal.NewFromInt(5))).To(BeTrue(), "found portion qty, got %s", found.Quantity)
		Expect(found.Price).To(Equal(0.0), "found portion carries zero cost")
		Expect(normal.Quantity.Equal(decimal.NewFromInt(3))).To(BeTrue(), "normal portion qty, got %s", normal.Quantity)
		Expect(normal.Price).To(Equal(8.0), "normal portion carries the PO cost")

		// Stock conserved: source 15-8=7, destination 8, total 15. Nothing hidden.
		var reloadedSrc models.InventoryItem
		Expect(db.First(&reloadedSrc, srcItem.ID).Error).NotTo(HaveOccurred())
		Expect(reloadedSrc.Quantity.Equal(decimal.NewFromInt(7))).To(BeTrue(), "source on-hand, got %s", reloadedSrc.Quantity)
		Expect(dstItem.Quantity.Equal(decimal.NewFromInt(8))).To(BeTrue(), "destination on-hand, got %s", dstItem.Quantity)
	})

	It("propagates the found-stock flag across a multi-hop transfer (A→B→C) and conserves stock", func() {
		svc := buildReconInventoryService(repository.NewBaseRepository(tenv.DB))
		approveCtx := approvePerms("multihop-approver@cim.local")
		db := tenv.ContextfulDB()
		suffix := uuid.NewString()[:8]

		invA := fixture.WithInventory(db, models.Inventory{Name: fmt.Sprintf("A-%s", suffix), Status: models.InventoryStatusActive})
		invB := fixture.WithInventory(db, models.Inventory{Name: fmt.Sprintf("B-%s", suffix), Status: models.InventoryStatusActive})
		invC := fixture.WithInventory(db, models.Inventory{Name: fmt.Sprintf("C-%s", suffix), Status: models.InventoryStatusActive})
		unit := fixture.WithUnit(db, fixture.ValidBaseUnit())
		product := fixture.WithProduct(db, fixture.ValidProduct(unit.ID))

		itemA := &models.InventoryItem{InventoryID: invA.ID, ProductID: product.ID, UnitID: unit.ID, Quantity: decimal.NewFromInt(20), Status: models.InventoryItemStatusActive}
		Expect(db.Create(itemA).Error).NotTo(HaveOccurred())
		// Found (zero-cost) layer flagged as a real reconcile stock-up would be.
		Expect(db.Create(&models.InventoryTransaction{
			InventoryItemID: itemA.ID, TransactionType: models.InventoryTransactionTypeReconcileStockUp,
			Price: 0, Quantity: decimal.NewFromInt(20), IsAdjustment: true,
		}).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", itemA.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(itemA)
		})

		runTransfer := func(fromInv, toInv uint, fromItemID uint, qty int64) {
			q := decimal.NewFromInt(qty)
			payload, err := json.Marshal(dto.TransferInventoryRequest{
				SourceInventoryID: fromInv, DestinationInventoryID: toInv,
				Items: []dto.QuantityItem{{InventoryItemID: fromItemID, Quantity: &q}},
			})
			Expect(err).NotTo(HaveOccurred())
			sub := &models.InventorySubmission{
				InventoryID: fromInv, SubmissionType: models.InventorySubmissionTypeTransfer,
				ProcessingStatus: models.InventorySubmissionStatusPending,
				ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
				Payload:          json.RawMessage(payload),
			}
			Expect(db.Create(sub).Error).NotTo(HaveOccurred())
			DeferCleanup(func() { db.Unscoped().Delete(sub) })
			_, err = svc.ProcessSubmission(approveCtx, dto.SubmissionApprovalRequest{
				SubmissionID: sub.ID, Action: string(models.InventorySubmissionActionApprove),
			})
			Expect(err).NotTo(HaveOccurred())
		}

		itemIn := func(invID uint) *models.InventoryItem {
			var it models.InventoryItem
			Expect(db.Where("inventory_id = ? AND product_id = ?", invID, product.ID).First(&it).Error).NotTo(HaveOccurred())
			DeferCleanup(func() {
				db.Unscoped().Where("inventory_item_id = ?", it.ID).Delete(&models.InventoryTransaction{})
				db.Unscoped().Delete(&it)
			})
			return &it
		}
		transferInFlag := func(invID uint) bool {
			var ti models.InventoryTransaction
			Expect(db.Joins("JOIN inventory_items ii ON ii.id = inventory_transactions.inventory_item_id").
				Where("ii.inventory_id = ? AND inventory_transactions.transaction_type = ?", invID, models.InventoryTransactionTypeTransferIn).
				First(&ti).Error).NotTo(HaveOccurred())
			return ti.IsAdjustment
		}

		// Hop 1: A→B (12). Hop 2: B→C (9).
		runTransfer(invA.ID, invB.ID, itemA.ID, 12)
		itemB := itemIn(invB.ID)
		Expect(transferInFlag(invB.ID)).To(BeTrue(), "1st-hop transfer-in must be flagged found")

		runTransfer(invB.ID, invC.ID, itemB.ID, 9)
		itemC := itemIn(invC.ID)
		Expect(transferInFlag(invC.ID)).To(BeTrue(), "2nd-hop transfer-in must inherit found provenance across hops")

		// Stock conserved across all three: A(20-12=8) + B(12-9=3) + C(9) = 20.
		// Reload A and B (B's on-hand changed after hop 2; itemC is already fresh).
		var reloadedA, reloadedB models.InventoryItem
		Expect(db.First(&reloadedA, itemA.ID).Error).NotTo(HaveOccurred())
		Expect(db.First(&reloadedB, itemB.ID).Error).NotTo(HaveOccurred())
		Expect(reloadedA.Quantity.Equal(decimal.NewFromInt(8))).To(BeTrue(), "A on-hand, got %s", reloadedA.Quantity)
		Expect(reloadedB.Quantity.Equal(decimal.NewFromInt(3))).To(BeTrue(), "B on-hand, got %s", reloadedB.Quantity)
		Expect(itemC.Quantity.Equal(decimal.NewFromInt(9))).To(BeTrue(), "C on-hand, got %s", itemC.Quantity)
		total := reloadedA.Quantity.Add(reloadedB.Quantity).Add(itemC.Quantity)
		Expect(total.Equal(decimal.NewFromInt(20))).To(BeTrue(), "total stock conserved across hops, got %s", total)
	})
})
