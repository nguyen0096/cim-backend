package apptest

import (
	"context"
	"fmt"
	"time"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/pkg/testutil/fixture"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// P1 regression: a backdated reconcile_stock_up (early created_at, high id) that
// FIFO consumes first advances consuming_transaction_id to a high id. A genuinely
// earlier, lower-id, still-unconsumed receipt must NOT be skipped when reloading
// consumable transactions — otherwise on-hand ≠ Σ(unconsumed consumable).
var _ = Describe("Backdated stock-up FIFO cursor", func() {
	var (
		repo repository.InventoryItemRepository
		ctx  context.Context
		db   *gorm.DB
	)

	BeforeEach(func() {
		ctx = tenv.DefaultContext
		db = tenv.ContextfulDB()
		repo = repository.NewInventoryItemRepository(repository.NewBaseRepository(db))
	})

	It("does not skip a lower-id unconsumed receipt behind a backdated stock-up cursor", func() {
		unit := fixture.WithUnit(db.WithContext(ctx), models.Unit{Name: fmt.Sprintf("U-%d", time.Now().UnixNano()), Symbol: "U", UnitType: "general"})
		product := fixture.WithProduct(db.WithContext(ctx), models.Product{Name: fmt.Sprintf("P-%d", time.Now().UnixNano()), Status: "active", UnitID: unit.ID})
		inventory := fixture.WithInventory(db.WithContext(ctx), models.Inventory{Name: fmt.Sprintf("Inv-%d", time.Now().UnixNano()), Status: models.InventoryStatusActive})

		item := models.InventoryItem{InventoryID: inventory.ID, ProductID: product.ID, Quantity: decimal.NewFromInt(100), Status: models.InventoryItemStatusActive, UnitID: unit.ID}
		Expect(db.WithContext(ctx).Create(&item).Error).To(BeNil())
		DeferCleanup(func() { db.Unscoped().Delete(&item) })

		// Receipt A: created LATER (Jan 10) but inserted first → LOWER id, unconsumed.
		receiptA := models.InventoryTransaction{
			Base:            models.Base{CreatedAt: time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)},
			InventoryItemID: item.ID, TransactionType: models.InventoryTransactionTypePurchase,
			Quantity: decimal.NewFromInt(100), ConsumedQuantity: decimal.Zero, Price: 5,
		}
		Expect(db.WithContext(ctx).Create(&receiptA).Error).To(BeNil())
		DeferCleanup(func() { db.Unscoped().Delete(&receiptA) })

		// Stock-up S: backdated EARLIER (Jan 1) but inserted later → HIGHER id, fully consumed.
		stockUp := models.InventoryTransaction{
			Base:            models.Base{CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
			InventoryItemID: item.ID, TransactionType: models.InventoryTransactionTypeReconcileStockUp,
			Quantity: decimal.NewFromInt(20), ConsumedQuantity: decimal.NewFromInt(20), Price: 0,
		}
		Expect(db.WithContext(ctx).Create(&stockUp).Error).To(BeNil())
		DeferCleanup(func() { db.Unscoped().Delete(&stockUp) })
		Expect(stockUp.ID).To(BeNumerically(">", receiptA.ID), "stock-up must have the higher id")

		// FIFO consumed the backdated stock-up first → cursor at the high id.
		Expect(db.WithContext(ctx).Model(&item).Update("consuming_transaction_id", stockUp.ID).Error).To(BeNil())

		items, err := repo.GetActiveInventoryItemsByProductIDs(ctx, inventory.ID, []uint{product.ID})
		Expect(err).To(BeNil())
		Expect(items).To(HaveLen(1))

		var ids []uint
		for _, txn := range items[0].ConsumableTransactions {
			ids = append(ids, txn.ID)
		}
		Expect(ids).To(ContainElement(receiptA.ID), "lower-id unconsumed receipt must still load")
		Expect(ids).NotTo(ContainElement(stockUp.ID), "fully-consumed stock-up must be excluded")

		// Invariant: on-hand (100) == Σ(unconsumed consumable) == receipt A's 100.
		remaining := decimal.Zero
		for _, txn := range items[0].ConsumableTransactions {
			remaining = remaining.Add(txn.Quantity.Sub(txn.ConsumedQuantity))
		}
		Expect(remaining.Equal(decimal.NewFromInt(100))).To(BeTrue(), "on-hand must equal Σ unconsumed, got %s", remaining)
	})
})
