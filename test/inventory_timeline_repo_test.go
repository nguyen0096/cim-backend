package apptest

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/shopspring/decimal"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil/fixture"
)

// inventoryTimelineRepoFixtures bundles a self-contained set of records used by
// the integration tests below. Each test builds its own scoped fixtures with
// uuid-suffixed names so tests can run in any order or in parallel without
// colliding on unique constraints.
type inventoryTimelineRepoFixtures struct {
	Inventory *models.Inventory
	Supplier  *models.Supplier
	Unit      *models.Unit
	Product   *models.Product
}

// setupBasicFixtures creates a fresh inventory/supplier/unit/product set with
// uuid-tagged names. Cleanup is registered via DeferCleanup inside the existing
// WithX helpers, so callers don't need to manage teardown themselves.
func setupBasicFixtures() inventoryTimelineRepoFixtures {
	suffix := uuid.NewString()[:8]
	db := tenv.ContextfulDB()

	supplier := fixture.WithSupplier(db, models.Supplier{
		Name:         fmt.Sprintf("supplier-%s", suffix),
		ContactEmail: fmt.Sprintf("s-%s@example.com", suffix),
	})
	unit := fixture.WithUnit(db, models.Unit{
		Name:             fmt.Sprintf("unit-%s", suffix),
		Symbol:           fmt.Sprintf("u%s", suffix[:4]),
		UnitType:         "general",
		ConversionFactor: 1,
	})
	product := fixture.WithProduct(db, models.Product{
		Name:   fmt.Sprintf("product-%s", suffix),
		UnitID: unit.ID,
		Status: "active",
	})
	inventory := fixture.WithInventory(db, models.Inventory{
		Name: fmt.Sprintf("inventory-%s", suffix),
	})
	return inventoryTimelineRepoFixtures{
		Inventory: inventory,
		Supplier:  supplier,
		Unit:      unit,
		Product:   product,
	}
}

// createPOWithItem creates a PO with a single POI and returns both. Cleanup is
// scoped via DeferCleanup; pisp rows referencing this POI are also deleted.
func createPOWithItem(f inventoryTimelineRepoFixtures, qtyOrdered, qtyReceived float64) (*models.PurchaseOrder, *models.PurchaseOrderItem) {
	db := tenv.ContextfulDB()
	suffix := uuid.NewString()[:8]
	po := fixture.WithPurchaseOrder(db, models.PurchaseOrder{
		OrderNumber: fmt.Sprintf("PO-%s", suffix),
		InventoryID: pkg.Ptr(f.Inventory.ID),
		Status:      models.PurchaseOrderStatusFullyDelivered,
	})
	poi := &models.PurchaseOrderItem{
		PurchaseOrderID:  pkg.Ptr(po.ID),
		ProductID:        pkg.Ptr(f.Product.ID),
		SupplierID:       pkg.Ptr(f.Supplier.ID),
		UnitID:           pkg.Ptr(f.Unit.ID),
		Quantity:         decimal.NewFromFloat(qtyOrdered),
		ReceivedQuantity: decimal.NewFromFloat(qtyReceived),
		UnitPrice:        5.0,
		Status:           models.PurchaseOrderItemStatusDelivered,
	}
	Expect(db.Create(poi).Error).To(Succeed())
	DeferCleanup(func() {
		db.Exec("DELETE FROM purchase_order_item_selling_prices WHERE purchase_order_item_id = ?", poi.ID)
		db.Exec("DELETE FROM purchase_order_items WHERE id = ?", poi.ID)
	})
	return po, poi
}

// createPISPRow inserts a POItemSellingPrice with the given fields. spID may be nil.
func createPISPRow(poiID uint, override *decimal.Decimal, spID *uint) {
	db := tenv.ContextfulDB()
	row := &models.POItemSellingPrice{
		PurchaseOrderItemID: poiID,
		SellingPrice:        override,
		SellingPriceID:      spID,
	}
	Expect(db.Create(row).Error).To(Succeed())
	DeferCleanup(func() {
		db.Exec("DELETE FROM purchase_order_item_selling_prices WHERE id = ?", row.ID)
	})
}

// createSellingPrice inserts a selling_prices row and returns its ID.
func createSellingPrice(productID uint, price float64) uint {
	db := tenv.ContextfulDB()
	sp := &models.SellingPrice{
		ProductID:     productID,
		Price:         decimal.NewFromFloat(price),
		EffectiveFrom: time.Now().AddDate(0, 0, -7),
	}
	Expect(db.Create(sp).Error).To(Succeed())
	DeferCleanup(func() {
		db.Exec("DELETE FROM selling_prices WHERE id = ?", sp.ID)
	})
	return sp.ID
}

// createInventoryItem inserts an InventoryItem and returns it.
func createInventoryItem(f inventoryTimelineRepoFixtures, qty float64) *models.InventoryItem {
	db := tenv.ContextfulDB()
	item := &models.InventoryItem{
		InventoryID: f.Inventory.ID,
		ProductID:   f.Product.ID,
		UnitID:      f.Unit.ID,
		Quantity:    decimal.NewFromFloat(qty),
		Status:      models.InventoryItemStatusActive,
	}
	Expect(db.Create(item).Error).To(Succeed())
	DeferCleanup(func() {
		db.Exec("DELETE FROM inventory_transactions WHERE inventory_item_id = ?", item.ID)
		db.Exec("DELETE FROM inventory_items WHERE id = ?", item.ID)
	})
	return item
}

func createPurchaseTxn(itemID uint, poiID *uint, qty float64, at time.Time) *models.InventoryTransaction {
	db := tenv.ContextfulDB()
	txn := &models.InventoryTransaction{
		InventoryItemID:     itemID,
		TransactionType:     models.InventoryTransactionTypePurchase,
		Quantity:            decimal.NewFromFloat(qty),
		Price:               5.0,
		PurchaseOrderItemID: poiID,
		Base:                models.Base{CreatedAt: at},
	}
	Expect(db.Create(txn).Error).To(Succeed())
	DeferCleanup(func() {
		db.Exec("DELETE FROM inventory_transactions WHERE id = ?", txn.ID)
	})
	return txn
}

func createConsumeTxn(itemID, counterID uint, txType models.InventoryTransactionType, qty float64, at time.Time) *models.InventoryTransaction {
	db := tenv.ContextfulDB()
	txn := &models.InventoryTransaction{
		InventoryItemID:      itemID,
		TransactionType:      txType,
		Quantity:             decimal.NewFromFloat(qty),
		Price:                5.0,
		CounterTransactionID: pkg.Ptr(counterID),
		Base:                 models.Base{CreatedAt: at},
	}
	Expect(db.Create(txn).Error).To(Succeed())
	DeferCleanup(func() {
		db.Exec("DELETE FROM inventory_transactions WHERE id = ?", txn.ID)
	})
	return txn
}

var _ = Describe("SellingPriceRepository.GetPOItemsWithPriceByIDs", func() {
	var repo repository.SellingPriceRepository

	BeforeEach(func() {
		repo = repository.NewSellingPriceRepository(repository.NewBaseRepository(tenv.DB))
	})

	It("returns the override price when pisp.selling_price is set", func(ctx SpecContext) {
		f := setupBasicFixtures()
		_, poi := createPOWithItem(f, 10, 10)
		override := decimal.NewFromFloat(99.50)
		createPISPRow(poi.ID, &override, nil)

		result, err := repo.GetPOItemsWithPriceByIDs(ctx, []uint{poi.ID}, f.Inventory.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(HaveLen(1))
		info := result[poi.ID]
		Expect(info).NotTo(BeNil())
		Expect(info.EffectivePrice).NotTo(BeNil())
		Expect(info.EffectivePrice.Equal(override)).To(BeTrue())
		Expect(info.ProductID).To(Equal(f.Product.ID))
	})

	It("falls back to the referenced selling_prices.price when override is nil", func(ctx SpecContext) {
		f := setupBasicFixtures()
		_, poi := createPOWithItem(f, 10, 10)
		spID := createSellingPrice(f.Product.ID, 42.0)
		createPISPRow(poi.ID, nil, &spID)

		result, err := repo.GetPOItemsWithPriceByIDs(ctx, []uint{poi.ID}, f.Inventory.ID)
		Expect(err).NotTo(HaveOccurred())
		info := result[poi.ID]
		Expect(info).NotTo(BeNil())
		Expect(info.EffectivePrice).NotTo(BeNil())
		Expect(info.EffectivePrice.Equal(decimal.NewFromFloat(42.0))).To(BeTrue())
	})

	It("returns nil EffectivePrice when both override and reference are nil", func(ctx SpecContext) {
		f := setupBasicFixtures()
		_, poi := createPOWithItem(f, 10, 10)
		createPISPRow(poi.ID, nil, nil)

		result, err := repo.GetPOItemsWithPriceByIDs(ctx, []uint{poi.ID}, f.Inventory.ID)
		Expect(err).NotTo(HaveOccurred())
		info := result[poi.ID]
		Expect(info).NotTo(BeNil())
		Expect(info.EffectivePrice).To(BeNil())
		// PO/POI metadata should still be present.
		Expect(info.POID).NotTo(BeZero())
		Expect(info.POID).NotTo(Equal(uint(0)))
	})

	It("returns nil EffectivePrice when no pisp row exists", func(ctx SpecContext) {
		f := setupBasicFixtures()
		_, poi := createPOWithItem(f, 10, 10)
		// No pisp row created.

		result, err := repo.GetPOItemsWithPriceByIDs(ctx, []uint{poi.ID}, f.Inventory.ID)
		Expect(err).NotTo(HaveOccurred())
		info := result[poi.ID]
		Expect(info).NotTo(BeNil())
		Expect(info.EffectivePrice).To(BeNil())
	})

	It("filters out POIs whose PO belongs to a different inventory", func(ctx SpecContext) {
		f := setupBasicFixtures()
		_, poi := createPOWithItem(f, 10, 10)
		override := decimal.NewFromFloat(50.0)
		createPISPRow(poi.ID, &override, nil)

		// Query with a different (random) inventory ID — POI must be excluded.
		otherInventory := fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
			Name: fmt.Sprintf("other-%s", uuid.NewString()[:8]),
		})

		result, err := repo.GetPOItemsWithPriceByIDs(ctx, []uint{poi.ID}, otherInventory.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeEmpty())
	})

	It("excludes soft-deleted POIs and POs", func(ctx SpecContext) {
		f := setupBasicFixtures()
		po, poi := createPOWithItem(f, 10, 10)
		db := tenv.ContextfulDB()

		// Soft-delete the POI.
		Expect(db.Delete(&models.PurchaseOrderItem{}, poi.ID).Error).To(Succeed())

		result, err := repo.GetPOItemsWithPriceByIDs(ctx, []uint{poi.ID}, f.Inventory.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeEmpty())

		// Restore POI; soft-delete the PO instead.
		Expect(db.Unscoped().Model(&models.PurchaseOrderItem{}).Where("id = ?", poi.ID).Update("deleted_at", nil).Error).To(Succeed())
		Expect(db.Delete(&models.PurchaseOrder{}, po.ID).Error).To(Succeed())

		result, err = repo.GetPOItemsWithPriceByIDs(ctx, []uint{poi.ID}, f.Inventory.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeEmpty())
	})

	It("treats a soft-deleted pisp as absent (POI returns with nil EffectivePrice)", func(ctx SpecContext) {
		f := setupBasicFixtures()
		_, poi := createPOWithItem(f, 10, 10)
		override := decimal.NewFromFloat(99.0)
		createPISPRow(poi.ID, &override, nil)
		db := tenv.ContextfulDB()

		// Soft-delete the pisp row. With the LEFT JOIN ON-clause filter, the POI
		// must still be returned but with a nil EffectivePrice (the override is
		// invisible because pisp is treated as absent).
		Expect(db.Where("purchase_order_item_id = ?", poi.ID).
			Delete(&models.POItemSellingPrice{}).Error).To(Succeed())

		result, err := repo.GetPOItemsWithPriceByIDs(ctx, []uint{poi.ID}, f.Inventory.ID)
		Expect(err).NotTo(HaveOccurred())
		info := result[poi.ID]
		Expect(info).NotTo(BeNil())
		Expect(info.EffectivePrice).To(BeNil())
	})

	It("treats a soft-deleted referenced selling_prices row as absent", func(ctx SpecContext) {
		f := setupBasicFixtures()
		_, poi := createPOWithItem(f, 10, 10)
		spID := createSellingPrice(f.Product.ID, 42.0)
		// pisp references sp via selling_price_id; no override.
		createPISPRow(poi.ID, nil, &spID)
		db := tenv.ContextfulDB()

		// Soft-delete the referenced selling_prices row. With the LEFT JOIN
		// ON-clause filter, the POI is still returned but EffectivePrice is nil
		// (sp is invisible, COALESCE collapses to nil).
		Expect(db.Delete(&models.SellingPrice{}, spID).Error).To(Succeed())

		result, err := repo.GetPOItemsWithPriceByIDs(ctx, []uint{poi.ID}, f.Inventory.ID)
		Expect(err).NotTo(HaveOccurred())
		info := result[poi.ID]
		Expect(info).NotTo(BeNil())
		Expect(info.EffectivePrice).To(BeNil())
	})

	It("returns an empty map for an empty input slice", func(ctx SpecContext) {
		result, err := repo.GetPOItemsWithPriceByIDs(ctx, nil, 1)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeEmpty())
	})
})

var _ = Describe("InventoryRepository.GetTransactionsByInventoryIDsWithCounter", func() {
	var repo repository.InventoryRepository

	BeforeEach(func() {
		repo = repository.NewInventoryRepository(repository.NewBaseRepository(tenv.DB))
	})

	It("returns purchase txns with their own purchase_order_item_id and nil CounterPOIID", func(ctx SpecContext) {
		f := setupBasicFixtures()
		_, poi := createPOWithItem(f, 10, 10)
		item := createInventoryItem(f, 10)
		now := time.Now()
		createPurchaseTxn(item.ID, pkg.Ptr(poi.ID), 10, now)

		from := now.Add(-1 * time.Hour)
		to := now.Add(1 * time.Hour)
		rows, err := repo.GetTransactionsByInventoryIDsWithCounter(ctx, f.Inventory.ID, &from, &to)
		Expect(err).NotTo(HaveOccurred())
		Expect(rows).To(HaveLen(1))
		Expect(rows[0].TransactionType).To(Equal(models.InventoryTransactionTypePurchase))
		Expect(rows[0].PurchaseOrderItemID).NotTo(BeNil())
		Expect(*rows[0].PurchaseOrderItemID).To(Equal(poi.ID))
		Expect(rows[0].CounterPOIID).To(BeNil())
	})

	It("exposes counter purchase's POI on sell/disposal/transfer txns via the self-join", func(ctx SpecContext) {
		f := setupBasicFixtures()
		_, poi := createPOWithItem(f, 10, 10)
		item := createInventoryItem(f, 0)
		now := time.Now()

		// Counter purchase txn is OUTSIDE the timeline window (created earlier).
		counter := createPurchaseTxn(item.ID, pkg.Ptr(poi.ID), 10, now.Add(-2*time.Hour))
		// Sell txn IS inside the window.
		sell := createConsumeTxn(item.ID, counter.ID, models.InventoryTransactionTypeSell, 3, now)

		from := now.Add(-30 * time.Minute)
		to := now.Add(30 * time.Minute)
		rows, err := repo.GetTransactionsByInventoryIDsWithCounter(ctx, f.Inventory.ID, &from, &to)
		Expect(err).NotTo(HaveOccurred())

		// Only the sell is in the window.
		var found *repository.InventoryTransactionWithCounter
		for _, r := range rows {
			if r.ID == sell.ID {
				found = r
				break
			}
		}
		Expect(found).NotTo(BeNil(), "sell txn must be returned")
		Expect(found.CounterPOIID).NotTo(BeNil(), "counter POI must be resolved via self-join")
		Expect(*found.CounterPOIID).To(Equal(poi.ID))
	})

	It("returns CounterPOIID = nil when the counter purchase has no purchase_order_item_id", func(ctx SpecContext) {
		f := setupBasicFixtures()
		item := createInventoryItem(f, 0)
		now := time.Now()

		// Counter purchase has no POI — manual stock-in.
		counter := createPurchaseTxn(item.ID, nil, 10, now.Add(-1*time.Hour))
		sell := createConsumeTxn(item.ID, counter.ID, models.InventoryTransactionTypeSell, 1, now)

		from := now.Add(-30 * time.Minute)
		to := now.Add(30 * time.Minute)
		rows, err := repo.GetTransactionsByInventoryIDsWithCounter(ctx, f.Inventory.ID, &from, &to)
		Expect(err).NotTo(HaveOccurred())

		var found *repository.InventoryTransactionWithCounter
		for _, r := range rows {
			if r.ID == sell.ID {
				found = r
				break
			}
		}
		Expect(found).NotTo(BeNil())
		Expect(found.CounterPOIID).To(BeNil(), "manual-stock-in counter has no POI")
	})

	It("scopes to the given inventoryID — txns from other inventories must not appear", func(ctx SpecContext) {
		f := setupBasicFixtures()
		other := setupBasicFixtures()

		myItem := createInventoryItem(f, 5)
		otherItem := createInventoryItem(other, 5)
		now := time.Now()
		createPurchaseTxn(myItem.ID, nil, 5, now)
		createPurchaseTxn(otherItem.ID, nil, 5, now)

		from := now.Add(-1 * time.Hour)
		to := now.Add(1 * time.Hour)
		rows, err := repo.GetTransactionsByInventoryIDsWithCounter(ctx, f.Inventory.ID, &from, &to)
		Expect(err).NotTo(HaveOccurred())
		for _, r := range rows {
			Expect(r.InventoryItemID).To(Equal(myItem.ID), "no foreign-inventory txns should leak in")
		}
	})

	It("respects the from/to window", func(ctx SpecContext) {
		f := setupBasicFixtures()
		item := createInventoryItem(f, 0)
		now := time.Now()

		createPurchaseTxn(item.ID, nil, 1, now.Add(-2*time.Hour)) // before window
		mid := createPurchaseTxn(item.ID, nil, 2, now)            // in window
		createPurchaseTxn(item.ID, nil, 3, now.Add(2*time.Hour))  // after window

		from := now.Add(-1 * time.Hour)
		to := now.Add(1 * time.Hour)
		rows, err := repo.GetTransactionsByInventoryIDsWithCounter(ctx, f.Inventory.ID, &from, &to)
		Expect(err).NotTo(HaveOccurred())

		var idsInWindow []uint
		for _, r := range rows {
			idsInWindow = append(idsInWindow, r.ID)
		}
		Expect(idsInWindow).To(ConsistOf(mid.ID))
	})

	It("excludes soft-deleted transactions", func(ctx SpecContext) {
		f := setupBasicFixtures()
		item := createInventoryItem(f, 0)
		now := time.Now()
		live := createPurchaseTxn(item.ID, nil, 5, now)
		soft := createPurchaseTxn(item.ID, nil, 7, now)
		db := tenv.ContextfulDB()

		// Soft-delete one of them.
		Expect(db.Delete(&models.InventoryTransaction{}, soft.ID).Error).To(Succeed())

		from := now.Add(-1 * time.Hour)
		to := now.Add(1 * time.Hour)
		rows, err := repo.GetTransactionsByInventoryIDsWithCounter(ctx, f.Inventory.ID, &from, &to)
		Expect(err).NotTo(HaveOccurred())

		var ids []uint
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		Expect(ids).To(ContainElement(live.ID))
		Expect(ids).NotTo(ContainElement(soft.ID), "soft-deleted txn must be excluded")
	})

	It("excludes txns whose inventory_item is soft-deleted", func(ctx SpecContext) {
		f := setupBasicFixtures()
		item := createInventoryItem(f, 0)
		now := time.Now()
		txn := createPurchaseTxn(item.ID, nil, 5, now)
		db := tenv.ContextfulDB()

		// Soft-delete the inventory_item. The INNER JOIN ON-clause filter must
		// exclude txns whose item is soft-deleted.
		Expect(db.Delete(&models.InventoryItem{}, item.ID).Error).To(Succeed())

		from := now.Add(-1 * time.Hour)
		to := now.Add(1 * time.Hour)
		rows, err := repo.GetTransactionsByInventoryIDsWithCounter(ctx, f.Inventory.ID, &from, &to)
		Expect(err).NotTo(HaveOccurred())

		for _, r := range rows {
			Expect(r.ID).NotTo(Equal(txn.ID), "txn whose inventory_item is soft-deleted must be excluded")
		}
	})

	It("returns the main txn with nil CounterPOIID when the counter txn is soft-deleted", func(ctx SpecContext) {
		f := setupBasicFixtures()
		_, poi := createPOWithItem(f, 10, 10)
		item := createInventoryItem(f, 0)
		now := time.Now()
		counter := createPurchaseTxn(item.ID, pkg.Ptr(poi.ID), 10, now.Add(-2*time.Hour))
		sell := createConsumeTxn(item.ID, counter.ID, models.InventoryTransactionTypeSell, 3, now)
		db := tenv.ContextfulDB()

		// Soft-delete the counter purchase txn. The LEFT JOIN ON-clause filter
		// must drop the counter side so the sell still surfaces with nil
		// CounterPOIID rather than disappearing entirely.
		Expect(db.Delete(&models.InventoryTransaction{}, counter.ID).Error).To(Succeed())

		from := now.Add(-30 * time.Minute)
		to := now.Add(30 * time.Minute)
		rows, err := repo.GetTransactionsByInventoryIDsWithCounter(ctx, f.Inventory.ID, &from, &to)
		Expect(err).NotTo(HaveOccurred())

		var found *repository.InventoryTransactionWithCounter
		for _, r := range rows {
			if r.ID == sell.ID {
				found = r
				break
			}
		}
		Expect(found).NotTo(BeNil(), "sell must still be returned even when its counter is soft-deleted")
		Expect(found.CounterPOIID).To(BeNil(), "soft-deleted counter is treated as absent")
	})
})

// Compile-time guarantee that BeforeSuite tenv is available.
var _ = context.Background
