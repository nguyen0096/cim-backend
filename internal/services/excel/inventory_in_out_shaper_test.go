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
	histPurchase := func(itemID, poiID uint, qty float64, at time.Time) *models.InventoryTransaction {
		poi := poiID
		return &models.InventoryTransaction{
			Base:                models.Base{ID: 0, CreatedAt: at},
			InventoryItemID:     itemID,
			TransactionType:     models.InventoryTransactionTypePurchase,
			Quantity:            decimal.NewFromFloat(qty),
			PurchaseOrderItemID: &poi,
		}
	}
	// Consume historically — PurchaseOrderItemID is the *resolved* source POI.
	histConsume := func(itemID, poiID uint, qty float64, at time.Time) *models.InventoryTransaction {
		poi := poiID
		return &models.InventoryTransaction{
			Base:                models.Base{ID: 0, CreatedAt: at},
			InventoryItemID:     itemID,
			TransactionType:     models.InventoryTransactionTypeSell,
			Quantity:            decimal.NewFromFloat(qty),
			PurchaseOrderItemID: &poi,
		}
	}

	in := ShaperInput{
		StartDate: start,
		EndDate:   end,
		Items: []*ItemInfo{
			{ItemID: 100, ProductID: 1, ProductName: "Widget", UnitName: "kg"},
		},
		HistoricalTxns: []*models.InventoryTransaction{
			histPurchase(100, 300, 10, ts(2026, 3, 1)),
			histConsume(100, 300, 10, ts(2026, 3, 20)),
			histPurchase(100, 301, 8, ts(2026, 3, 5)),
			histConsume(100, 301, 3, ts(2026, 3, 25)),
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
	histPurchase := func(itemID, poiID uint, qty float64, at time.Time) *models.InventoryTransaction {
		poi := poiID
		return &models.InventoryTransaction{
			Base:                models.Base{ID: 0, CreatedAt: at},
			InventoryItemID:     itemID,
			TransactionType:     models.InventoryTransactionTypePurchase,
			Quantity:            decimal.NewFromFloat(qty),
			PurchaseOrderItemID: &poi,
		}
	}

	in := ShaperInput{
		StartDate: start,
		EndDate:   end,
		Items: []*ItemInfo{
			{ItemID: 100, ProductID: 1, ProductName: "Widget", UnitName: "kg"},
		},
		HistoricalTxns: []*models.InventoryTransaction{
			histPurchase(100, 700, 10, ts(2026, 3, 15)),
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
