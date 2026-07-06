package excel

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
)

func ts(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
}

func uintPtr(u uint) *uint { return &u }
func decPtr(f float64) *decimal.Decimal {
	d := decimal.NewFromFloat(f)
	return &d
}

func newPurchaseTxn(itemID, poiID uint, qty float64, at time.Time, price float64) *repository.InventoryTransactionWithCounter {
	poi := poiID
	return &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:                models.Base{ID: 0, CreatedAt: at},
			InventoryItemID:     itemID,
			TransactionType:     models.InventoryTransactionTypePurchase,
			Quantity:            decimal.NewFromFloat(qty),
			Price:               price,
			PurchaseOrderItemID: &poi,
		},
	}
}

func newConsumeTxn(itemID uint, qty float64, at time.Time, ttype models.InventoryTransactionType, counterPOI *uint) *repository.InventoryTransactionWithCounter {
	return &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:            models.Base{ID: 0, CreatedAt: at},
			InventoryItemID: itemID,
			TransactionType: ttype,
			Quantity:        decimal.NewFromFloat(qty),
		},
		CounterPOIID: counterPOI,
	}
}

func TestBuildExportRows_PurchaseAndSell_FIFOToSourcePOI(t *testing.T) {
	start := ts(2026, 4, 1)
	end := ts(2026, 4, 30)

	in := ShaperInput{
		StartDate: start,
		EndDate:   end,
		Items: []*ItemInfo{
			{ItemID: 100, ProductID: 1, ProductName: "Widget", UnitName: "kg"},
		},
		HistoricalTxns: nil,
		PeriodTxns: []*repository.InventoryTransactionWithCounter{
			newPurchaseTxn(100, 200, 10, ts(2026, 4, 5), 5),  // POI 200
			newPurchaseTxn(100, 201, 5, ts(2026, 4, 10), 6),  // POI 201
			newConsumeTxn(100, 3, ts(2026, 4, 15), models.InventoryTransactionTypeSell, uintPtr(200)),
		},
		POInfo: map[uint]*repository.POItemSellingPriceInfo{
			200: {POItemID: 200, POID: 50, PONumber: "PO-A", ProductID: 1, EffectivePrice: decPtr(8)},
			201: {POItemID: 201, POID: 51, PONumber: "PO-B", ProductID: 1, EffectivePrice: decPtr(9)},
		},
	}

	out := BuildExportRows(in)
	require.Len(t, out.Rows, 2, "two POI rows")

	// First row: PO-A — purchase 10, sell 3, ending stock 7
	r0 := out.Rows[0]
	assert.Equal(t, "PO-A", r0.PONumber)
	assert.Equal(t, "10", r0.TotalPurchasedAmount.String())
	assert.Equal(t, "3", r0.SubtotalSold.String())
	assert.Equal(t, "0", r0.BeginningStock.String())
	assert.Equal(t, "7", r0.EndingStock.String())
	// Day-of-window for 2026-04-05 = 4
	assert.Equal(t, "10", r0.DailyPurchases[4].String())

	// Second row: PO-B — purchase 5, no consumes, ending stock 5
	r1 := out.Rows[1]
	assert.Equal(t, "PO-B", r1.PONumber)
	assert.Equal(t, "5", r1.TotalPurchasedAmount.String())
	assert.Equal(t, "0", r1.SubtotalSold.String())
	assert.Equal(t, "5", r1.EndingStock.String())
	assert.Equal(t, "5", r1.DailyPurchases[9].String()) // 2026-04-10 → day 9
}

func TestBuildExportRows_HistoricalCarryOverAndDepleted(t *testing.T) {
	start := ts(2026, 4, 1)
	end := ts(2026, 4, 30)

	// POI 300: purchased 10 historically, fully consumed 10 historically -> excluded
	// POI 301: purchased 8 historically, consumed 3 historically -> begin 5, included
	// Consume historically resolves the source POI via CounterPOIID.
	in := ShaperInput{
		StartDate: start,
		EndDate:   end,
		Items: []*ItemInfo{
			{ItemID: 100, ProductID: 1, ProductName: "Widget", UnitName: "kg"},
		},
		HistoricalTxns: []*repository.InventoryTransactionWithCounter{
			newPurchaseTxn(100, 300, 10, ts(2026, 3, 1), 0),
			newConsumeTxn(100, 10, ts(2026, 3, 20), models.InventoryTransactionTypeSell, uintPtr(300)),
			newPurchaseTxn(100, 301, 8, ts(2026, 3, 5), 0),
			newConsumeTxn(100, 3, ts(2026, 3, 25), models.InventoryTransactionTypeSell, uintPtr(301)),
		},
		PeriodTxns: nil,
		POInfo: map[uint]*repository.POItemSellingPriceInfo{
			300: {POItemID: 300, POID: 60, PONumber: "PO-X", ProductID: 1, EffectivePrice: decPtr(8)},
			301: {POItemID: 301, POID: 61, PONumber: "PO-Y", ProductID: 1, EffectivePrice: decPtr(9)},
		},
	}

	out := BuildExportRows(in)
	require.Len(t, out.Rows, 1, "depleted POI 300 excluded; only POI 301")
	assert.Equal(t, "PO-Y", out.Rows[0].PONumber)
	assert.Equal(t, "5", out.Rows[0].BeginningStock.String())
	assert.Equal(t, "5", out.Rows[0].EndingStock.String())
}

func TestBuildExportRows_DisposalAndTransfer(t *testing.T) {
	start := ts(2026, 4, 1)
	end := ts(2026, 4, 30)

	in := ShaperInput{
		StartDate: start,
		EndDate:   end,
		Items: []*ItemInfo{
			{ItemID: 100, ProductID: 1, ProductName: "Widget", UnitName: "kg"},
		},
		PeriodTxns: []*repository.InventoryTransactionWithCounter{
			newPurchaseTxn(100, 400, 20, ts(2026, 4, 1), 10),
			newConsumeTxn(100, 4, ts(2026, 4, 5), models.InventoryTransactionTypeDisposal, uintPtr(400)),
			newConsumeTxn(100, 6, ts(2026, 4, 10), models.InventoryTransactionTypeTransferOut, uintPtr(400)),
		},
		POInfo: map[uint]*repository.POItemSellingPriceInfo{
			400: {POItemID: 400, POID: 70, PONumber: "PO-Z", ProductID: 1, EffectivePrice: decPtr(15)},
		},
	}
	out := BuildExportRows(in)
	require.Len(t, out.Rows, 1)
	r := out.Rows[0]
	assert.Equal(t, "20", r.TotalPurchasedAmount.String())
	assert.Equal(t, "4", r.TotalDisposedAmount.String())
	assert.Equal(t, "6", r.TotalTransferredOut.String())
	assert.Equal(t, "10", r.EndingStock.String()) // 20 - 4 - 6
}

func TestBuildExportRows_DayCountAndDayIndex(t *testing.T) {
	start := ts(2026, 4, 1)
	end := ts(2026, 4, 5) // 5-day inclusive window
	in := ShaperInput{
		StartDate: start,
		EndDate:   end,
		Items: []*ItemInfo{
			{ItemID: 100, ProductID: 1, ProductName: "Widget", UnitName: "kg"},
		},
		PeriodTxns: []*repository.InventoryTransactionWithCounter{
			newPurchaseTxn(100, 500, 1, ts(2026, 4, 1), 1),
			newPurchaseTxn(100, 501, 2, ts(2026, 4, 5), 1),
		},
		POInfo: map[uint]*repository.POItemSellingPriceInfo{
			500: {POItemID: 500, POID: 80, PONumber: "PO-1", ProductID: 1, EffectivePrice: decPtr(2)},
			501: {POItemID: 501, POID: 81, PONumber: "PO-2", ProductID: 1, EffectivePrice: decPtr(2)},
		},
	}
	out := BuildExportRows(in)
	assert.Equal(t, 5, out.DayCount)
	require.Len(t, out.Rows, 2)
	// PO-1 → day 0, PO-2 → day 4
	for _, r := range out.Rows {
		switch r.PONumber {
		case "PO-1":
			assert.Equal(t, "1", r.DailyPurchases[0].String())
		case "PO-2":
			assert.Equal(t, "2", r.DailyPurchases[4].String())
		}
	}
}

func TestBuildExportRows_PurchasePriceFromPOInfoForCarryOverRows(t *testing.T) {
	start := ts(2026, 4, 1)
	end := ts(2026, 4, 30)

	// POI 700: stock carried into the window from a historical purchase, no
	// in-window purchase. PurchasePrice on the row must come from POInfo
	// (purchase_order_items.unit_price), not be left at zero.
	in := ShaperInput{
		StartDate: start,
		EndDate:   end,
		Items: []*ItemInfo{
			{ItemID: 100, ProductID: 1, ProductName: "Widget", UnitName: "kg"},
		},
		HistoricalTxns: []*repository.InventoryTransactionWithCounter{
			newPurchaseTxn(100, 700, 10, ts(2026, 3, 15), 0),
		},
		PeriodTxns: []*repository.InventoryTransactionWithCounter{
			// In-window sell against the carry-over POI.
			newConsumeTxn(100, 2, ts(2026, 4, 5), models.InventoryTransactionTypeSell, uintPtr(700)),
		},
		POInfo: map[uint]*repository.POItemSellingPriceInfo{
			700: {
				POItemID: 700, POID: 110, PONumber: "PO-CARRY", ProductID: 1,
				PurchasePrice: decimal.NewFromFloat(42),
				EffectivePrice: decPtr(80),
			},
		},
	}

	out := BuildExportRows(in)
	require.Len(t, out.Rows, 1)
	r := out.Rows[0]
	assert.Equal(t, "PO-CARRY", r.PONumber)
	assert.Equal(t, "10", r.BeginningStock.String())
	assert.Equal(t, "2", r.SubtotalSold.String())
	assert.Equal(t, "0", r.TotalPurchasedAmount.String())
	// Key assertion: carry-over rows pick up purchase price from POInfo.
	assert.Equal(t, "42", r.PurchasePrice.String(), "carry-over rows must use POInfo.PurchasePrice, not zero")
}

func newStockUpTxn(itemID, txnID uint, qty float64, at time.Time) *repository.InventoryTransactionWithCounter {
	return &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:            models.Base{ID: txnID, CreatedAt: at},
			InventoryItemID: itemID,
			TransactionType: models.InventoryTransactionTypeReconcileStockUp,
			Quantity:        decimal.NewFromFloat(qty),
			Price:           0,
		},
	}
}

// newFoundConsumeTxn is a consume whose counter is a found layer (its counter id
// is in the found-layer set the shaper builds).
func newFoundConsumeTxn(itemID, counterLayerID uint, qty float64, at time.Time, ttype models.InventoryTransactionType) *repository.InventoryTransactionWithCounter {
	cid := counterLayerID
	return &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:                 models.Base{ID: 0, CreatedAt: at},
			InventoryItemID:      itemID,
			TransactionType:      ttype,
			Quantity:             decimal.NewFromFloat(qty),
			CounterTransactionID: &cid,
		},
	}
}

func TestBuildExportRows_AdjustmentRowFamily_FoundStockAndSale(t *testing.T) {
	start := ts(2026, 4, 1)
	end := ts(2026, 4, 30)

	in := ShaperInput{
		StartDate: start,
		EndDate:   end,
		Items: []*ItemInfo{
			{ItemID: 100, ProductID: 1, ProductName: "Widget", UnitName: "kg"},
		},
		// Stock-up 900 backdated before window (30 found), plus a normal purchase carry-over.
		HistoricalTxns: []*repository.InventoryTransactionWithCounter{
			newStockUpTxn(100, 900, 30, ts(2026, 3, 10)),
			newPurchaseTxn(100, 200, 10, ts(2026, 3, 1), 5),
		},
		PeriodTxns: []*repository.InventoryTransactionWithCounter{
			// Sale of 10 found units (counter = stock-up 900).
			newFoundConsumeTxn(100, 900, 10, ts(2026, 4, 15), models.InventoryTransactionTypeSell),
			// A fresh in-window stock-up (5 found), keyed on its own id 901.
			newStockUpTxn(100, 901, 5, ts(2026, 4, 20)),
		},
		POInfo: map[uint]*repository.POItemSellingPriceInfo{
			200: {POItemID: 200, POID: 50, PONumber: "PO-A", ProductID: 1, EffectivePrice: decPtr(8), PurchasePrice: decimal.NewFromInt(5)},
		},
	}

	out := BuildExportRows(in)

	var adj900, adj901, po *ExportRow
	for _, r := range out.Rows {
		switch {
		case r.AdjustmentSourceTxnID == 900:
			adj900 = r
		case r.AdjustmentSourceTxnID == 901:
			adj901 = r
		case r.PONumber == "PO-A":
			po = r
		}
	}
	require.NotNil(t, adj900, "found-stock adjustment row (900) must be present")
	require.NotNil(t, adj901, "in-window adjustment row (901) must be present")
	require.NotNil(t, po, "normal PO carry-over row must be present")

	// Adjustment rows carry the VI label, zero cost, and no selling price.
	assert.Equal(t, models.AdjustmentCategoryLabel, adj900.PONumber)
	assert.True(t, adj900.PurchasePrice.IsZero(), "adjustment purchase price must be 0")
	assert.Nil(t, adj900.SellingPrice, "adjustment selling price must be nil")

	// Row 900: begin 30, sold 10 → ending 20 (found stock and its sale both appear, foots).
	assert.Equal(t, "30", adj900.BeginningStock.String())
	assert.Equal(t, "10", adj900.SubtotalSold.String())
	assert.Equal(t, "20", adj900.EndingStock.String())

	// Row 901: window in 5 → ending 5.
	assert.Equal(t, "0", adj901.BeginningStock.String())
	assert.Equal(t, "5", adj901.TotalPurchasedAmount.String())
	assert.Equal(t, "5", adj901.EndingStock.String())

	// Product-level conservation: total ending = 10 (PO carry-over) + 20 + 5 = 35.
	total := decimal.Zero
	for _, r := range out.Rows {
		total = total.Add(r.EndingStock)
	}
	assert.Equal(t, "35", total.String())
}

// A found-unit transfer sets destination transfer_in.CounterTransactionID = the
// source stock-up layer, so at the destination the transfer-in and any re-sale of
// those units must both land on one adjustment row and ending on-hand must foot.
func TestBuildExportRows_FoundStockTransferredInAndResoldAtDestination(t *testing.T) {
	start := ts(2026, 4, 1)
	end := ts(2026, 4, 30)

	const srcStockUpID = uint(900) // stock-up lives in the SOURCE inventory
	const transferInID = uint(950) // found units arriving at the destination
	stockUp := srcStockUpID

	// transfer_in of 12 found units: flagged IsAdjustment at transfer time (its
	// counter is the source stock-up).
	transferIn := &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:                 models.Base{ID: transferInID, CreatedAt: ts(2026, 4, 10)},
			InventoryItemID:      100,
			TransactionType:      models.InventoryTransactionTypeTransferIn,
			Quantity:             decimal.NewFromFloat(12),
			Price:                0,
			CounterTransactionID: &stockUp,
			IsAdjustment:         true,
		},
	}
	// re-sale of 4 of those units: counter is the transfer-in (a found layer).
	resaleCounter := transferInID
	reSale := &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:                 models.Base{ID: 0, CreatedAt: ts(2026, 4, 20)},
			InventoryItemID:      100,
			TransactionType:      models.InventoryTransactionTypeSell,
			Quantity:             decimal.NewFromFloat(4),
			CounterTransactionID: &resaleCounter,
		},
	}

	in := ShaperInput{
		StartDate: start,
		EndDate:   end,
		Items: []*ItemInfo{
			{ItemID: 100, ProductID: 1, ProductName: "Widget", UnitName: "kg"},
		},
		PeriodTxns: []*repository.InventoryTransactionWithCounter{transferIn, reSale},
		POInfo:     map[uint]*repository.POItemSellingPriceInfo{},
	}

	out := BuildExportRows(in)
	require.Len(t, out.Rows, 1, "one destination adjustment row")
	r := out.Rows[0]
	assert.Equal(t, transferInID, r.AdjustmentSourceTxnID, "row keyed by the destination found layer (transfer-in)")
	assert.Equal(t, models.AdjustmentCategoryLabel, r.PONumber)
	assert.Equal(t, "12", r.TotalTransferredIn.String(), "transfer-in of found units must appear")
	assert.Equal(t, "4", r.SubtotalSold.String(), "re-sale must not vanish")
	assert.Equal(t, "8", r.EndingStock.String(), "destination ending on-hand foots: 12 in - 4 sold")
}

// Multi-hop (A→B→C): at the 2nd-hop destination C, the transfer-in's counter is
// B's transfer-in (NOT a stock-up), and B's row is not in C's per-inventory txn
// set. The persisted IsAdjustment flag (propagated at transfer time) still marks
// C's layer as found, so C's transfer-in and a re-sale there both foot.
func TestBuildExportRows_MultiHopFoundStockFootsAtSecondDestination(t *testing.T) {
	start := ts(2026, 4, 1)
	end := ts(2026, 4, 30)

	const bTransferInID = uint(960) // B's transfer-in (lives in inventory B, absent from C's set)
	const cTransferInID = uint(970) // found units arriving at C
	counterAtC := bTransferInID

	// C's transfer-in: IsAdjustment propagated; its counter (B's transfer-in) is a
	// transfer_in, and is NOT present in C's txn set.
	cTransferIn := &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:                 models.Base{ID: cTransferInID, CreatedAt: ts(2026, 4, 10)},
			InventoryItemID:      100,
			TransactionType:      models.InventoryTransactionTypeTransferIn,
			Quantity:             decimal.NewFromFloat(9),
			Price:                0,
			CounterTransactionID: &counterAtC,
			IsAdjustment:         true,
		},
	}
	reSaleAtC := newFoundConsumeTxn(100, cTransferInID, 3, ts(2026, 4, 22), models.InventoryTransactionTypeSell)

	in := ShaperInput{
		StartDate: start,
		EndDate:   end,
		Items: []*ItemInfo{
			{ItemID: 100, ProductID: 1, ProductName: "Widget", UnitName: "kg"},
		},
		PeriodTxns: []*repository.InventoryTransactionWithCounter{cTransferIn, reSaleAtC},
		POInfo:     map[uint]*repository.POItemSellingPriceInfo{},
	}

	out := BuildExportRows(in)
	require.Len(t, out.Rows, 1, "2nd-hop destination must have one adjustment row (no vanished stock)")
	r := out.Rows[0]
	assert.Equal(t, cTransferInID, r.AdjustmentSourceTxnID)
	assert.Equal(t, "9", r.TotalTransferredIn.String())
	assert.Equal(t, "3", r.SubtotalSold.String())
	assert.Equal(t, "6", r.EndingStock.String(), "2nd-hop ending on-hand foots: 9 in - 3 sold")
}

// A mixed FIFO transfer produces two destination transfer-ins for the same
// product: the found portion (IsAdjustment) and a normal PO-backed portion. The
// export must foot both — found on its adjustment row, normal on the PO row —
// with nothing hidden or vanished.
func TestBuildExportRows_MixedTransferInFootsOnBothPaths(t *testing.T) {
	start := ts(2026, 4, 1)
	end := ts(2026, 4, 30)

	const foundTransferInID = uint(980)
	const srcPOI = uint(400)
	poi := srcPOI

	foundIn := &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:            models.Base{ID: foundTransferInID, CreatedAt: ts(2026, 4, 10)},
			InventoryItemID: 100,
			TransactionType: models.InventoryTransactionTypeTransferIn,
			Quantity:        decimal.NewFromFloat(5),
			Price:           0,
			IsAdjustment:    true,
		},
	}
	// Normal transfer-in: PO-backed (CounterPOIID resolves to a PO row), not flagged.
	normalIn := &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:            models.Base{ID: 981, CreatedAt: ts(2026, 4, 10)},
			InventoryItemID: 100,
			TransactionType: models.InventoryTransactionTypeTransferIn,
			Quantity:        decimal.NewFromFloat(3),
			Price:           8,
		},
		CounterPOIID: &poi,
	}

	in := ShaperInput{
		StartDate: start,
		EndDate:   end,
		Items: []*ItemInfo{
			{ItemID: 100, ProductID: 1, ProductName: "Widget", UnitName: "kg"},
		},
		PeriodTxns: []*repository.InventoryTransactionWithCounter{foundIn, normalIn},
		POInfo: map[uint]*repository.POItemSellingPriceInfo{
			srcPOI: {POItemID: srcPOI, POID: 70, PONumber: "PO-Z", ProductID: 1, EffectivePrice: decPtr(12), PurchasePrice: decimal.NewFromInt(8)},
		},
	}

	out := BuildExportRows(in)
	require.Len(t, out.Rows, 2, "found portion + normal PO portion → two rows")

	var adj, po *ExportRow
	for _, r := range out.Rows {
		if r.AdjustmentSourceTxnID == foundTransferInID {
			adj = r
		} else if r.PONumber == "PO-Z" {
			po = r
		}
	}
	require.NotNil(t, adj, "found portion must be on the adjustment row")
	require.NotNil(t, po, "normal portion must be on the PO row")
	assert.Equal(t, "5", adj.TotalTransferredIn.String())
	assert.Equal(t, "5", adj.EndingStock.String())
	assert.True(t, adj.PurchasePrice.IsZero(), "found row is zero-cost")
	assert.Equal(t, "3", po.TotalTransferredIn.String())
	assert.Equal(t, "3", po.EndingStock.String())
	// Conservation: nothing hidden — 5 + 3 = 8 total ending.
	assert.Equal(t, "8", adj.EndingStock.Add(po.EndingStock).String())
}

func TestBuildExportRows_ProductGroupingAndOrdering(t *testing.T) {
	start := ts(2026, 4, 1)
	end := ts(2026, 4, 30)

	in := ShaperInput{
		StartDate: start,
		EndDate:   end,
		Items: []*ItemInfo{
			{ItemID: 100, ProductID: 1, ProductName: "Banana", UnitName: "kg"},
			{ItemID: 101, ProductID: 2, ProductName: "Apple", UnitName: "kg"},
		},
		PeriodTxns: []*repository.InventoryTransactionWithCounter{
			newPurchaseTxn(100, 600, 1, ts(2026, 4, 1), 1),
			newPurchaseTxn(101, 601, 1, ts(2026, 4, 1), 1),
			newPurchaseTxn(101, 602, 1, ts(2026, 4, 1), 1),
		},
		POInfo: map[uint]*repository.POItemSellingPriceInfo{
			600: {POItemID: 600, POID: 90, PONumber: "PO-Z1", ProductID: 1, EffectivePrice: decPtr(2)},
			601: {POItemID: 601, POID: 91, PONumber: "PO-A1", ProductID: 2, EffectivePrice: decPtr(2)},
			602: {POItemID: 602, POID: 92, PONumber: "PO-A2", ProductID: 2, EffectivePrice: decPtr(2)},
		},
	}
	out := BuildExportRows(in)
	require.Len(t, out.Rows, 3)
	// Apple (2 rows) before Banana (1 row); within Apple, PO-A1 then PO-A2.
	assert.Equal(t, "Apple", out.Rows[0].ProductName)
	assert.Equal(t, "PO-A1", out.Rows[0].PONumber)
	assert.Equal(t, "Apple", out.Rows[1].ProductName)
	assert.Equal(t, "PO-A2", out.Rows[1].PONumber)
	assert.Equal(t, "Banana", out.Rows[2].ProductName)
}
