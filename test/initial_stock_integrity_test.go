package apptest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"

	"cim-backend/internal/auth"
	"cim-backend/internal/mocks/servicemocks"
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services"
	"cim-backend/internal/services/dto"
	"cim-backend/internal/services/excel"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil"
	"cim-backend/pkg/testutil/fixture"
)

// Integrity specs for the opening-stock loader: the phantom-stock guard, FIFO
// ordering, the reporting-invisibility contract, and transfer provenance.
var _ = Describe("Initial stock import integrity", func() {
	var (
		client    *testutil.Client
		inventory *models.Inventory
		unit      *models.Unit
		suffix    string
	)

	BeforeEach(func() {
		suffix = uuid.NewString()[:8]
		db := tenv.ContextfulDB()
		client = testutil.NewClient(tenv, models.RoleDeveloper)
		inventory = fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("is-inv-%s", suffix),
			Location: fmt.Sprintf("is-loc-%s", suffix),
			Status:   models.InventoryStatusActive,
		})
		unit = fixture.WithUnit(db, models.Unit{
			Name: fmt.Sprintf("ISUNIT%s", suffix), Symbol: fmt.Sprintf("ISUNIT%s", suffix),
			UnitType: "general", ConversionFactor: 1, Level: 1, DecimalPlaces: 2,
		})
	})

	// workbook renders a single-sheet workbook whose rows carry the given unit.
	workbook := func(sheet string, rows []fixture.InitialStockRowSpec) []byte {
		return fixture.CreateInitialStockWorkbook([]fixture.InitialStockSheetSpec{
			{Name: sheet, Rows: rows},
		})
	}

	apply := func(data []byte, sheet string, opts ...testutil.RequestOptions) *http.Response {
		options := append([]testutil.RequestOptions{
			testutil.WithAuth(),
			testutil.WithMultipartFormData(initialStockForm(data, inventory.ID, sheet, "false")),
		}, opts...)
		resp, err := client.MakeRequest(http.MethodPost, "/api/v1/tools/initial-stock/import", nil, options...)
		Expect(err).NotTo(HaveOccurred())
		return resp
	}

	itemFor := func(productName string) *models.InventoryItem {
		var item models.InventoryItem
		Expect(tenv.ContextfulDB().
			Joins("JOIN products p ON p.id = inventory_items.product_id").
			Where("inventory_items.inventory_id = ? AND UPPER(TRIM(p.name)) = UPPER(TRIM(?))",
				inventory.ID, productName).
			First(&item).Error).NotTo(HaveOccurred())
		return &item
	}

	// This is the load-bearing spec. A reconcile snapshot captures on-hand at
	// initiate and applies its surplus against CURRENT on-hand, while drift
	// detection only scans submission rows. An import writes no submission row, so
	// an import inside an open reconcile window is invisible to both: baseline B,
	// import adds X, staff count the shelf and honestly report B+X, the snapshot
	// delta reads that as a surplus of X and books a reconcile_stock_up of X on top
	// of the already-raised on-hand, landing at B+2X and audit-trailing the phantom
	// as "found during a count".
	It("refuses to load while a reconcile is open, so the B+2X phantom cannot be created", func() {
		db := tenv.ContextfulDB()
		const baseline int64 = 40
		const loaded int64 = 25

		product := fixture.WithProduct(db, models.Product{
			Name: fmt.Sprintf("PHANTOM %s", suffix), UnitID: unit.ID, Status: "active",
		})
		item := &models.InventoryItem{
			InventoryID: inventory.ID, ProductID: product.ID, UnitID: unit.ID,
			Quantity: decimal.NewFromInt(baseline), Status: models.InventoryItemStatusActive,
		}
		Expect(db.Create(item).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", item.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(item)
		})
		Expect(db.Create(&models.InventoryTransaction{
			InventoryItemID: item.ID, TransactionType: models.InventoryTransactionTypePurchase,
			Price: 10, Quantity: decimal.NewFromInt(baseline),
		}).Error).NotTo(HaveOccurred())

		// An open reconcile whose snapshot baseline is the pre-import on-hand.
		sub := &models.InventorySubmission{
			InventoryID: inventory.ID, SubmissionType: models.InventorySubmissionTypeReconcile,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
			ReconcileStatus:  models.ReconcileLifecycleStatusOpen,
		}
		Expect(db.Create(sub).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(sub) })
		snap := &models.ReconciliationSnapshot{
			SubmissionID: sub.ID, InventoryItemID: item.ID, PrevQuantity: decimal.NewFromInt(baseline),
		}
		Expect(db.Create(snap).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(snap) })

		data := workbook("TON", []fixture.InitialStockRowSpec{
			{Name: product.Name, Unit: unit.Name, Quantity: fmt.Sprintf("%d", loaded), Category: "NƯỚC"},
		})

		// The guard: refused with a machine-detectable conflict, nothing written.
		resp := apply(data, "TON", testutil.WithHeader("Idempotency-Key", uuid.NewString()))
		Expect(resp.StatusCode).To(Equal(http.StatusConflict))
		body := testutil.ParseResponse(resp)
		Expect(body["key"]).To(Equal(pkg.ErrKeyInitialStockReconcileOpen))
		Expect(body["code"]).To(Equal("conflict"))

		var reloaded models.InventoryItem
		Expect(db.First(&reloaded, item.ID).Error).NotTo(HaveOccurred())
		Expect(reloaded.Quantity.Equal(decimal.NewFromInt(baseline))).To(BeTrue(),
			"a refused load must not move on-hand, got %s", reloaded.Quantity)

		var initialCount int64
		Expect(db.Model(&models.InventoryTransaction{}).
			Where("inventory_item_id = ? AND transaction_type = ?", item.ID, models.InventoryTransactionTypeInitial).
			Count(&initialCount).Error).NotTo(HaveOccurred())
		Expect(initialCount).To(BeZero(), "a refused load must write no initial transaction")

		// Now prove the guard is what stands between us and B+2X: simulate exactly
		// what the load would have done had it been allowed through (add X to
		// on-hand plus a consumable layer), then let the reconcile apply an honest
		// shelf count of B+X. The snapshot-delta path books a surplus of X, so
		// on-hand lands at B+2X. Without the guard, that is the loader's outcome.
		Expect(db.Model(item).Update("quantity", decimal.NewFromInt(baseline+loaded)).Error).NotTo(HaveOccurred())
		Expect(db.Create(&models.InventoryTransaction{
			InventoryItemID: item.ID, TransactionType: models.InventoryTransactionTypeInitial,
			Price: 0, Quantity: decimal.NewFromInt(loaded),
		}).Error).NotTo(HaveOccurred())

		svc := buildReconInventoryService(repository.NewBaseRepository(tenv.DB))
		staffCtx := reconPermsCtx("is-staff@cim.local", pkg.RBACActionReconItemCreate)
		adminCtx := reconPermsCtx("is-admin@cim.local", pkg.RBACActionReconManage)

		counted := decimal.NewFromInt(baseline + loaded)
		created, err := svc.CreateReconciliationItem(staffCtx, dto.CreateReconciliationItemRequest{
			SubmissionID: sub.ID,
			Items:        []dto.ReconciliationCountItem{{InventoryItemID: item.ID, Quantity: &counted}},
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.ContextfulDB().Unscoped().Delete(created) })

		_, err = svc.CloseReconciliation(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())
		_, err = svc.StartProcessing(adminCtx, sub.ID)
		Expect(err).NotTo(HaveOccurred())

		var phantom models.InventoryItem
		Expect(db.First(&phantom, item.ID).Error).NotTo(HaveOccurred())
		Expect(phantom.Quantity.Equal(decimal.NewFromInt(baseline+2*loaded))).To(BeTrue(),
			"the unguarded sequence must reproduce B+2X (%d), got %s", baseline+2*loaded, phantom.Quantity)

		var stockUps []models.InventoryTransaction
		Expect(db.Where("inventory_item_id = ? AND transaction_type = ?",
			item.ID, models.InventoryTransactionTypeReconcileStockUp).Find(&stockUps).Error).NotTo(HaveOccurred())
		Expect(stockUps).To(HaveLen(1))
		Expect(stockUps[0].Quantity.Equal(decimal.NewFromInt(loaded))).To(BeTrue(),
			"the phantom is booked as a counted surplus of %d, got %s", loaded, stockUps[0].Quantity)
	})

	It("sorts an initial layer after pre-existing layers under FIFO", func() {
		db := tenv.ContextfulDB()
		product := fixture.WithProduct(db, models.Product{
			Name: fmt.Sprintf("FIFO %s", suffix), UnitID: unit.ID, Status: "active",
		})
		item := &models.InventoryItem{
			InventoryID: inventory.ID, ProductID: product.ID, UnitID: unit.ID,
			Quantity: decimal.NewFromInt(6), Status: models.InventoryItemStatusActive,
		}
		Expect(db.Create(item).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", item.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(item)
		})
		purchase := &models.InventoryTransaction{
			InventoryItemID: item.ID, TransactionType: models.InventoryTransactionTypePurchase,
			Price: 7, Quantity: decimal.NewFromInt(6),
		}
		Expect(db.Create(purchase).Error).NotTo(HaveOccurred())

		data := workbook("TON", []fixture.InitialStockRowSpec{
			{Name: product.Name, Unit: unit.Name, Quantity: "10", Category: "NƯỚC"},
		})
		resp := apply(data, "TON", testutil.WithHeader("Idempotency-Key", uuid.NewString()))
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		var layers []models.InventoryTransaction
		Expect(db.Where("inventory_item_id = ? AND transaction_type IN ?", item.ID,
			models.GetConsumableTransactionTypes()).
			Order("created_at ASC, id ASC").Find(&layers).Error).NotTo(HaveOccurred())
		Expect(layers).To(HaveLen(2))
		Expect(layers[0].TransactionType).To(Equal(models.InventoryTransactionTypePurchase),
			"the pre-existing purchase layer must drain first")
		Expect(layers[1].TransactionType).To(Equal(models.InventoryTransactionTypeInitial))
		Expect(layers[1].Price).To(Equal(0.0), "an opening-stock layer carries no cost")
		Expect(layers[1].IsAdjustment).To(BeTrue(),
			"an opening-stock layer keys its own report row family, like a stock-up")
		Expect(layers[1].CreatedAt).To(BeTemporally(">=", purchase.CreatedAt))
		Expect(layers[1].CreatedAt).To(BeTemporally("~", time.Now(), time.Minute),
			"the layer is stamped at run time, never backdated")
	})

	// Timeline visibility is deliberately NOT part of Option 2: the load is visible in
	// the in/out export and the monthly report under Tồn đầu kỳ, and nowhere else. A
	// timeline row would be indistinguishable from a count correction (that view has
	// no per-category label) and would fold opening stock into Tổng điều chỉnh.
	It("stays out of the timeline rows and TotalAdjustment, while its sale still shows", func() {
		db := tenv.ContextfulDB()
		now := time.Now().UTC()
		product := fixture.WithProduct(db, models.Product{
			Name: fmt.Sprintf("HIDDEN %s", suffix), UnitID: unit.ID, Status: "active",
		})

		data := workbook("TON", []fixture.InitialStockRowSpec{
			{Name: product.Name, Unit: unit.Name, Quantity: "12", Category: "NƯỚC"},
		})
		Expect(apply(data, "TON", testutil.WithHeader("Idempotency-Key", uuid.NewString())).StatusCode).
			To(Equal(http.StatusOK))

		item := itemFor(fmt.Sprintf("HIDDEN %s", suffix))
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", item.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(item)
		})
		Expect(item.Quantity.Equal(decimal.NewFromInt(12))).To(BeTrue())

		// Sell 5 out of the loaded layer. The timeline appends movements
		// unconditionally, so this must remain visible even though its source is not.
		var layer models.InventoryTransaction
		Expect(db.Where("inventory_item_id = ? AND transaction_type = ?",
			item.ID, models.InventoryTransactionTypeInitial).First(&layer).Error).NotTo(HaveOccurred())
		Expect(db.Model(&layer).Update("consumed_quantity", decimal.NewFromInt(5)).Error).NotTo(HaveOccurred())
		Expect(db.Create(&models.InventoryTransaction{
			InventoryItemID: item.ID, TransactionType: models.InventoryTransactionTypeSell,
			Price: 0, Quantity: decimal.NewFromInt(5), CounterTransactionID: &layer.ID,
		}).Error).NotTo(HaveOccurred())
		Expect(db.Model(item).Update("quantity", decimal.NewFromInt(7)).Error).NotTo(HaveOccurred())

		base := repository.NewBaseRepository(tenv.DB)
		timeline, err := services.NewInventoryTimelineService(
			repository.NewInventoryRepository(base),
			repository.NewInventoryItemRepository(base),
			repository.NewSellingPriceRepository(base),
		).GetInventoryTimeline(tenv.DefaultContext, dto.InventoryTimelineRequest{
			InventoryID: inventory.ID,
			StartDate:   now.AddDate(0, 0, -1).Format("2006-01-02"),
			EndDate:     now.AddDate(0, 0, 1).Format("2006-01-02"),
			Limit:       50,
		})
		Expect(err).NotTo(HaveOccurred())

		var seen *dto.ProductTimeline
		for i := range timeline.Products {
			if timeline.Products[i].ProductID == product.ID {
				seen = &timeline.Products[i]
			}
		}
		Expect(seen).NotTo(BeNil(), "the product itself is still listed")

		// The load contributes no row and no adjustment metric.
		for _, txn := range seen.Transactions {
			Expect(txn.TransactionType).NotTo(Equal(string(models.InventoryTransactionTypeInitial)),
				"an opening-stock load must not appear as a timeline row")
			Expect(txn.TransactionID).NotTo(Equal(layer.ID),
				"opening-stock layer %d leaked into the timeline", layer.ID)
		}
		Expect(seen.Metrics.TotalAdjustment).To(BeZero(),
			"an opening-stock load must not be folded into Tổng điều chỉnh with count corrections")
		Expect(seen.Metrics.TotalPurchased).To(BeZero(), "it is not a purchase either")

		// Its sale is a real movement and stays visible, at zero cost.
		Expect(seen.Transactions).To(HaveLen(1), "exactly the sell, not the load")
		sell := seen.Transactions[0]
		Expect(sell.TransactionType).To(Equal("sell"))
		Expect(sell.Quantity).To(BeNumerically("==", 5))
		Expect(sell.CostPrice).NotTo(BeNil())
		Expect(*sell.CostPrice).To(BeZero(), "opening stock carries no cost into its consume")
		Expect(seen.Metrics.TotalSold).To(BeNumerically("==", 5))

		// Ending stock still foots on the hidden inflow: 12 loaded - 5 sold.
		Expect(seen.EndingStock).To(BeNumerically("==", 7))

		// Meanwhile the monthly report DOES show it, as its own labelled source row.
		rep, err := buildReportInventoryService().GetMonthlyTransactionReport(
			tenv.DefaultContext, inventory.ID, int(now.Month()), now.Year())
		Expect(err).NotTo(HaveOccurred())
		var labelled bool
		for _, it := range rep.Items {
			if it.SourceTransaction != nil &&
				it.SourceTransaction.TransactionType == models.InventoryTransactionTypeInitial {
				labelled = true
			}
		}
		Expect(labelled).To(BeTrue(),
			"export and monthly-report visibility is the scope of Option 2 and must survive")
	})

	It("marks a transfer of opening stock as an adjustment at the destination so its export row foots", func() {
		db := tenv.ContextfulDB()
		dst := fixture.WithInventory(db, models.Inventory{
			Name: fmt.Sprintf("is-dst-%s", suffix), Location: "dst", Status: models.InventoryStatusActive,
		})
		product := fixture.WithProduct(db, models.Product{
			Name: fmt.Sprintf("MOVED %s", suffix), UnitID: unit.ID, Status: "active",
		})

		data := workbook("TON", []fixture.InitialStockRowSpec{
			{Name: product.Name, Unit: unit.Name, Quantity: "20", Category: "NƯỚC"},
		})
		Expect(apply(data, "TON", testutil.WithHeader("Idempotency-Key", uuid.NewString())).StatusCode).
			To(Equal(http.StatusOK))

		srcItem := itemFor(fmt.Sprintf("MOVED %s", suffix))
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", srcItem.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(srcItem)
		})

		svc := buildReconInventoryService(repository.NewBaseRepository(tenv.DB))
		qty := decimal.NewFromInt(8)
		payload := mustMarshal(dto.TransferInventoryRequest{
			SourceInventoryID:      inventory.ID,
			DestinationInventoryID: dst.ID,
			Items:                  []dto.QuantityItem{{InventoryItemID: srcItem.ID, Quantity: &qty}},
		})
		sub := &models.InventorySubmission{
			InventoryID: inventory.ID, SubmissionType: models.InventorySubmissionTypeTransfer,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
			Payload:          payload,
		}
		Expect(db.Create(sub).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(sub) })

		_, err := svc.ProcessSubmission(
			reconPermsCtx("is-transfer@cim.local", pkg.RBACActionApprove),
			dto.SubmissionApprovalRequest{SubmissionID: sub.ID, Action: string(models.InventorySubmissionActionApprove)},
		)
		Expect(err).NotTo(HaveOccurred())

		var transferIn models.InventoryTransaction
		Expect(db.Joins("JOIN inventory_items ii ON ii.id = inventory_transactions.inventory_item_id").
			Where("ii.inventory_id = ? AND inventory_transactions.transaction_type = ?",
				dst.ID, models.InventoryTransactionTypeTransferIn).
			First(&transferIn).Error).NotTo(HaveOccurred())
		Expect(transferIn.IsAdjustment).To(BeTrue(),
			"transferred opening stock must carry found provenance, not pass as a genuine zero-cost layer")
		Expect(transferIn.Price).To(Equal(0.0))

		var dstItem models.InventoryItem
		Expect(db.Where("inventory_id = ? AND product_id = ?", dst.ID, product.ID).
			First(&dstItem).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", dstItem.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(&dstItem)
		})

		// Stock conserved across both inventories: 20 - 8 = 12 and 8.
		var reloadedSrc models.InventoryItem
		Expect(db.First(&reloadedSrc, srcItem.ID).Error).NotTo(HaveOccurred())
		Expect(reloadedSrc.Quantity.Equal(decimal.NewFromInt(12))).To(BeTrue(), "source on-hand %s", reloadedSrc.Quantity)
		Expect(dstItem.Quantity.Equal(decimal.NewFromInt(8))).To(BeTrue(), "destination on-hand %s", dstItem.Quantity)
	})

	// The monthly report drives buildSourceView, which emits one item per source
	// layer and hangs each consume off its own source. An opening-stock layer is a
	// source in its own right, so its drawdown is attributed to it and the report
	// foots against physical on-hand.
	It("reports opening stock as its own source row and foots against on-hand", func() {
		db := tenv.ContextfulDB()
		now := time.Now().UTC()
		// 10 days before this month's first day: AddDate(0,-1,0) normalizes back into
		// the current month on a 31st, which would put these layers in the period.
		priorMonth := time.Date(now.Year(), now.Month(), 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, -10)

		product := fixture.WithProduct(db, models.Product{
			Name: fmt.Sprintf("FOOTING %s", suffix), UnitID: unit.ID, Status: "active",
		})
		// On-hand 5 = purchase 6 + initial 10 - disposed 2 (prior month) - disposed 9.
		item := &models.InventoryItem{
			InventoryID: inventory.ID, ProductID: product.ID, UnitID: unit.ID,
			Quantity: decimal.NewFromInt(5), Status: models.InventoryItemStatusActive,
		}
		Expect(db.Create(item).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", item.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(item)
		})

		// Both layers loaded in the prior month, fully/partly consumed this month.
		purchase := &models.InventoryTransaction{
			Base: models.Base{CreatedAt: priorMonth}, InventoryItemID: item.ID,
			TransactionType: models.InventoryTransactionTypePurchase,
			Price:           10, Quantity: decimal.NewFromInt(6), ConsumedQuantity: decimal.NewFromInt(6),
		}
		Expect(db.Create(purchase).Error).NotTo(HaveOccurred())
		opening := &models.InventoryTransaction{
			Base: models.Base{CreatedAt: priorMonth}, InventoryItemID: item.ID,
			TransactionType: models.InventoryTransactionTypeInitial,
			Price:           0, Quantity: decimal.NewFromInt(10), ConsumedQuantity: decimal.NewFromInt(5),
		}
		Expect(db.Create(opening).Error).NotTo(HaveOccurred())

		// A prior-period outflow drawn from opening stock. Its inflow is excluded from
		// the opening balance, so it has to be excluded there too or the balance is
		// understated by exactly this quantity.
		Expect(db.Create(&models.InventoryTransaction{
			Base: models.Base{CreatedAt: priorMonth}, InventoryItemID: item.ID,
			TransactionType: models.InventoryTransactionTypeDisposal,
			Price:           0, Quantity: decimal.NewFromInt(2), CounterTransactionID: &opening.ID,
		}).Error).NotTo(HaveOccurred())

		// FIFO split the dispose of 9 across both layers.
		Expect(db.Create(&models.InventoryTransaction{
			Base: models.Base{CreatedAt: now}, InventoryItemID: item.ID,
			TransactionType: models.InventoryTransactionTypeDisposal,
			Price:           10, Quantity: decimal.NewFromInt(6), CounterTransactionID: &purchase.ID,
		}).Error).NotTo(HaveOccurred())
		Expect(db.Create(&models.InventoryTransaction{
			Base: models.Base{CreatedAt: now}, InventoryItemID: item.ID,
			TransactionType: models.InventoryTransactionTypeDisposal,
			Price:           0, Quantity: decimal.NewFromInt(3), CounterTransactionID: &opening.ID,
		}).Error).NotTo(HaveOccurred())

		report, err := buildReportInventoryService().GetMonthlyTransactionReport(
			tenv.DefaultContext, inventory.ID, int(now.Month()), now.Year())
		Expect(err).NotTo(HaveOccurred())

		byLayer := map[uint]*models.TxnReportInventoryItem{}
		for _, it := range report.Items {
			if it.InventoryItem != nil && it.InventoryItem.ID == item.ID && it.SourceTransaction != nil {
				byLayer[it.SourceTransaction.ID] = it
			}
		}
		Expect(byLayer).To(HaveKey(purchase.ID), "the purchase layer must have a row")
		Expect(byLayer).To(HaveKey(opening.ID), "the opening-stock layer must have its own row")

		openingRow := byLayer[opening.ID]
		Expect(openingRow.SourceTransaction.TransactionType).
			To(Equal(models.InventoryTransactionTypeInitial))

		// Balances and outflows now agree with the ledger. Opening balance carries the
		// 10 loaded units net of the prior-month dispose of 2; the period shows the
		// full dispose of 9 across both layers; on-hand is 5.
		// StartQuantity is the item's opening balance, repeated on each of its source
		// rows, so read it once and sum the outflows across rows.
		start := openingRow.StartQuantity
		Expect(byLayer[purchase.ID].StartQuantity.Equal(start)).To(BeTrue(),
			"every source row of an item carries the same opening balance")
		out := decimal.Zero
		for _, it := range byLayer {
			for _, consume := range it.ConsumeDetails {
				out = out.Add(consume.Quantity)
			}
		}
		Expect(start.Equal(decimal.NewFromInt(14))).To(BeTrue(),
			"opening balance must include the loaded units net of prior draws, got %s", start)
		Expect(out.Equal(decimal.NewFromInt(9))).To(BeTrue(),
			"the full dispose must be visible, not just its PO-sourced portion, got %s", out)

		var live models.InventoryItem
		Expect(db.First(&live, item.ID).Error).NotTo(HaveOccurred())
		Expect(start.Sub(out).Equal(live.Quantity)).To(BeTrue(),
			"report must foot against on-hand: start %s - out %s != on-hand %s",
			start, out, live.Quantity)
		Expect(live.Quantity.Equal(decimal.NewFromInt(5))).To(BeTrue(), "on-hand %s", live.Quantity)

		// The opening-stock drawdown is attributed to the opening-stock layer, never
		// to the purchase layer.
		openingOut := decimal.Zero
		for _, consume := range openingRow.ConsumeDetails {
			Expect(*consume.CounterTransactionID).To(Equal(opening.ID))
			openingOut = openingOut.Add(consume.Quantity)
		}
		Expect(openingOut.Equal(decimal.NewFromInt(3))).To(BeTrue(),
			"3 of the 9 disposed came out of opening stock, got %s", openingOut)
	})

	// The load and its drawdown in the SAME month: the layer reaches the report as a
	// period source rather than through a consume's counter link, so it exercises the
	// period source allow-list rather than the historical path.
	It("reports a same-month load and its drawdown, and foots", func() {
		db := tenv.ContextfulDB()
		now := time.Now().UTC()

		product := fixture.WithProduct(db, models.Product{
			Name: fmt.Sprintf("SAMEMONTH %s", suffix), UnitID: unit.ID, Status: "active",
		})
		// On-hand 7 = purchase 6 + initial 10 - disposed 9.
		item := &models.InventoryItem{
			InventoryID: inventory.ID, ProductID: product.ID, UnitID: unit.ID,
			Quantity: decimal.NewFromInt(7), Status: models.InventoryItemStatusActive,
		}
		Expect(db.Create(item).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", item.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(item)
		})

		purchase := &models.InventoryTransaction{
			InventoryItemID: item.ID, TransactionType: models.InventoryTransactionTypePurchase,
			Price: 10, Quantity: decimal.NewFromInt(6), ConsumedQuantity: decimal.NewFromInt(6),
		}
		Expect(db.Create(purchase).Error).NotTo(HaveOccurred())
		opening := &models.InventoryTransaction{
			InventoryItemID: item.ID, TransactionType: models.InventoryTransactionTypeInitial,
			Price: 0, Quantity: decimal.NewFromInt(10), ConsumedQuantity: decimal.NewFromInt(3),
			IsAdjustment: true,
		}
		Expect(db.Create(opening).Error).NotTo(HaveOccurred())
		Expect(db.Create(&models.InventoryTransaction{
			InventoryItemID: item.ID, TransactionType: models.InventoryTransactionTypeDisposal,
			Price: 10, Quantity: decimal.NewFromInt(6), CounterTransactionID: &purchase.ID,
		}).Error).NotTo(HaveOccurred())
		Expect(db.Create(&models.InventoryTransaction{
			InventoryItemID: item.ID, TransactionType: models.InventoryTransactionTypeDisposal,
			Price: 0, Quantity: decimal.NewFromInt(3), CounterTransactionID: &opening.ID,
		}).Error).NotTo(HaveOccurred())

		report, err := buildReportInventoryService().GetMonthlyTransactionReport(
			tenv.DefaultContext, inventory.ID, int(now.Month()), now.Year())
		Expect(err).NotTo(HaveOccurred())

		byLayer := map[uint]*models.TxnReportInventoryItem{}
		for _, it := range report.Items {
			if it.InventoryItem != nil && it.InventoryItem.ID == item.ID && it.SourceTransaction != nil {
				byLayer[it.SourceTransaction.ID] = it
			}
		}
		Expect(byLayer).To(HaveKey(purchase.ID))
		Expect(byLayer).To(HaveKey(opening.ID),
			"a same-month load must reach the report as a period source in its own right")

		sources, out := decimal.Zero, decimal.Zero
		for _, it := range byLayer {
			sources = sources.Add(it.SourceTransaction.Quantity)
			for _, consume := range it.ConsumeDetails {
				out = out.Add(consume.Quantity)
			}
		}
		Expect(sources.Equal(decimal.NewFromInt(16))).To(BeTrue(), "sources %s", sources)
		Expect(out.Equal(decimal.NewFromInt(9))).To(BeTrue(),
			"the full dispose must be visible, got %s", out)

		start := byLayer[opening.ID].StartQuantity
		Expect(start.Add(sources).Sub(out).Equal(decimal.NewFromInt(7))).To(BeTrue(),
			"report must foot against on-hand 7: start %s + in %s - out %s", start, sources, out)
	})

	// Transferred opening stock is labelled generically at the destination, exactly as
	// a transferred reconcile_stock_up has always been: IsAdjustment carries
	// adjustment-ness across the hop but records no origin, and preserving origin was
	// deliberately not built. The SOURCE inventory's own row keeps Tồn đầu kỳ, which is
	// what this pins.
	It("keeps Tồn đầu kỳ on the source row and shows the generic adjustment label at the destination", func() {
		db := tenv.ContextfulDB()
		base := repository.NewBaseRepository(tenv.DB)
		dst := fixture.WithInventory(db, models.Inventory{
			Name: fmt.Sprintf("hopB-%s", suffix), Location: "B", Status: models.InventoryStatusActive,
		})
		product := fixture.WithProduct(db, models.Product{
			Name: fmt.Sprintf("HOPS %s", suffix), UnitID: unit.ID, Status: "active",
		})

		data := workbook("TON", []fixture.InitialStockRowSpec{
			{Name: product.Name, Unit: unit.Name, Quantity: "20", Category: "NƯỚC"},
		})
		Expect(apply(data, "TON", testutil.WithHeader("Idempotency-Key", uuid.NewString())).StatusCode).
			To(Equal(http.StatusOK))
		srcItem := itemFor(fmt.Sprintf("HOPS %s", suffix))
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", srcItem.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(srcItem)
		})

		svc := buildReconInventoryService(base)
		q := decimal.NewFromInt(12)
		sub := &models.InventorySubmission{
			InventoryID: inventory.ID, SubmissionType: models.InventorySubmissionTypeTransfer,
			ProcessingStatus: models.InventorySubmissionStatusPending,
			ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
			Payload: mustMarshal(dto.TransferInventoryRequest{
				SourceInventoryID: inventory.ID, DestinationInventoryID: dst.ID,
				Items: []dto.QuantityItem{{InventoryItemID: srcItem.ID, Quantity: &q}},
			}),
		}
		Expect(db.Create(sub).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { db.Unscoped().Delete(sub) })
		_, err := svc.ProcessSubmission(
			reconPermsCtx("hop@cim.local", pkg.RBACActionApprove),
			dto.SubmissionApprovalRequest{SubmissionID: sub.ID, Action: string(models.InventorySubmissionActionApprove)},
		)
		Expect(err).NotTo(HaveOccurred())

		var dstItem models.InventoryItem
		Expect(db.Where("inventory_id = ? AND product_id = ?", dst.ID, product.ID).
			First(&dstItem).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", dstItem.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(&dstItem)
		})

		labelFor := func(invID, productID uint) string {
			txns, err := repository.NewInventoryRepository(base).
				GetTransactionsByInventoryIDsWithCounter(tenv.DefaultContext, invID, nil, nil)
			Expect(err).NotTo(HaveOccurred())
			var items []*models.InventoryItem
			Expect(db.Where("inventory_id = ?", invID).Find(&items).Error).NotTo(HaveOccurred())
			infos := make([]*excel.ItemInfo, 0, len(items))
			for _, it := range items {
				infos = append(infos, &excel.ItemInfo{
					ItemID: it.ID, ProductID: it.ProductID, ProductName: "P", UnitName: "U",
				})
			}
			rows := excel.BuildExportRows(excel.ShaperInput{
				StartDate:   time.Now().UTC().AddDate(0, 0, -1),
				EndDate:     time.Now().UTC().AddDate(0, 0, 1),
				InventoryID: invID, Items: infos, PeriodTxns: txns,
				POInfo: map[uint]*repository.POItemSellingPriceInfo{},
			})
			Expect(rows).NotTo(BeNil())
			for _, row := range rows.Rows {
				if row.ProductID == productID && row.AdjustmentSourceTxnID != 0 {
					return row.PONumber
				}
			}
			return ""
		}

		Expect(labelFor(inventory.ID, product.ID)).To(Equal(models.OpeningStockCategoryLabel),
			"the source inventory's own row must keep Tồn đầu kỳ")
		Expect(labelFor(dst.ID, product.ID)).To(Equal(models.AdjustmentCategoryLabel),
			"transferred opening stock shows the generic adjustment label, as transferred "+
				"reconcile_stock_up always has; preserving origin across transfers was not built")
	})

	It("keeps dispose and transfer working after a load (on-hand still equals its layers)", func() {
		db := tenv.ContextfulDB()
		product := fixture.WithProduct(db, models.Product{
			Name: fmt.Sprintf("STATE %s", suffix), UnitID: unit.ID, Status: "active",
		})
		item := &models.InventoryItem{
			InventoryID: inventory.ID, ProductID: product.ID, UnitID: unit.ID,
			Quantity: decimal.NewFromInt(5), Status: models.InventoryItemStatusActive,
		}
		Expect(db.Create(item).Error).NotTo(HaveOccurred())
		DeferCleanup(func() {
			db.Unscoped().Where("inventory_item_id = ?", item.ID).Delete(&models.InventoryTransaction{})
			db.Unscoped().Delete(item)
		})
		Expect(db.Create(&models.InventoryTransaction{
			InventoryItemID: item.ID, TransactionType: models.InventoryTransactionTypePurchase,
			Price: 3, Quantity: decimal.NewFromInt(5),
		}).Error).NotTo(HaveOccurred())

		data := workbook("TON", []fixture.InitialStockRowSpec{
			{Name: product.Name, Unit: unit.Name, Quantity: "9", Category: "NƯỚC"},
		})
		Expect(apply(data, "TON", testutil.WithHeader("Idempotency-Key", uuid.NewString())).StatusCode).
			To(Equal(http.StatusOK))

		// ValidateActivePurchaseTransactions asserts on-hand equals the sum of
		// unconsumed consumable layers; this is why `initial` had to join that set.
		itemRepo := repository.NewInventoryItemRepository(repository.NewBaseRepository(tenv.DB))
		loaded, err := itemRepo.GetActiveInventoryItems(tenv.DefaultContext, inventory.ID, []uint{item.ID})
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded).To(HaveLen(1))
		Expect(loaded[0].Quantity.Equal(decimal.NewFromInt(14))).To(BeTrue(), "on-hand %s", loaded[0].Quantity)
		Expect(loaded[0].ValidateActivePurchaseTransactions()).NotTo(HaveOccurred(),
			"an opening-stock layer must be consumable, else every dispose/transfer/reconcile in the batch fails")
	})
})

// buildReportInventoryService wires the real inventory service with a stubbed file
// storage, which the monthly report needs in order to reach its own output; the
// shared reconciliation helper passes nil there.
func buildReportInventoryService() services.InventoryService {
	base := repository.NewBaseRepository(tenv.DB)
	casbinService, err := auth.NewCasbinService(tenv.DB, tenv.Config.Casbin)
	Expect(err).NotTo(HaveOccurred())

	storage := &servicemocks.FileStorageService{}
	storage.On("PopulateExportURL", mock.Anything, mock.Anything).Return(nil)

	return services.NewInventoryService(
		repository.NewInventoryRepository(base),
		repository.NewInventoryItemRepository(base),
		repository.NewInventorySubmissionRepository(base),
		repository.NewReconciliationSnapshotRepository(base),
		repository.NewReconciliationRequestItemRepository(base),
		repository.NewProductRepository(base),
		repository.NewUserRepository(base, tenv.Config.Environment),
		casbinService,
		storage,
		base,
		tenv.DB,
	)
}

// reconPermsCtx builds a service-layer context carrying one reconciliation permission.
func reconPermsCtx(email, action string) context.Context {
	ctx := pkg.WithUserEmail(context.Background(), email)
	perms := map[pkg.UserPermission]struct{}{
		{Resource: pkg.RBACResourceInventorySubmissions, Action: action}: {},
	}
	return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
}
