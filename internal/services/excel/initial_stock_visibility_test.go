package excel

import (
	"bytes"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
)

// sheetRowsOf reads back the rendered worksheet.
func sheetRowsOf(t *testing.T, content []byte) [][]string {
	t.Helper()
	file, err := excelize.OpenReader(bytes.NewReader(content))
	require.NoError(t, err)
	defer func() { _ = file.Close() }()
	rows, err := file.GetRows(txnReportSheetName)
	require.NoError(t, err)
	return rows
}

// newInitialTxn builds an opening-stock layer: no purchase-order item, no counter,
// flagged as an adjustment so it keys its own row family.
func newInitialTxn(id, itemID uint, qty float64, at time.Time) *repository.InventoryTransactionWithCounter {
	return &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:            models.Base{ID: id, CreatedAt: at},
			InventoryItemID: itemID,
			TransactionType: models.InventoryTransactionTypeInitial,
			Quantity:        decimal.NewFromFloat(qty),
			Price:           0,
			IsAdjustment:    true,
		},
	}
}

// An opening-stock layer keys its own row family in the in/out export, labelled
// Tồn đầu kỳ, carrying its load as the window "in" and every consume drawn from it
// as the window "out" — so the row foots on real movement instead of dropping it.
func TestBuildExportRows_InitialLayerKeysItsOwnFootingRow(t *testing.T) {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC)
	const itemID, poiID uint = 41, 7

	purchase := newPurchaseTxn(itemID, poiID, 20, ts(2026, time.July, 3), 10)
	purchase.ID = 100
	sellFromPurchase := newConsumeTxn(itemID, 5, ts(2026, time.July, 10),
		models.InventoryTransactionTypeSell, uintPtr(poiID))
	sellFromPurchase.ID = 101

	// The opening-stock layer and a sell drawn from it. The sell's counter is the
	// initial layer, so it carries no counter POI and can only be attributed by it.
	initial := newInitialTxn(200, itemID, 30, ts(2026, time.July, 5))
	sellFromInitial := newConsumeTxn(itemID, 4, ts(2026, time.July, 12),
		models.InventoryTransactionTypeSell, nil)
	sellFromInitial.ID = 201
	counter := initial.ID
	sellFromInitial.CounterTransactionID = &counter

	rows := BuildExportRows(ShaperInput{
		StartDate:   start,
		EndDate:     end,
		InventoryID: 3,
		Items: []*ItemInfo{
			{ItemID: itemID, ProductID: 9, ProductName: "CÀ PHÊ", UnitName: "KG"},
		},
		PeriodTxns: []*repository.InventoryTransactionWithCounter{
			purchase, sellFromPurchase, initial, sellFromInitial,
		},
		POInfo: map[uint]*repository.POItemSellingPriceInfo{
			poiID: {POItemID: poiID, POID: 1, PONumber: "PO-1", EffectivePrice: decPtr(15)},
		},
	})
	require.NotNil(t, rows)
	require.Len(t, rows.Rows, 2, "one PO-item row and one opening-stock row")

	var poRow, openingRow *ExportRow
	for _, row := range rows.Rows {
		if row.AdjustmentSourceTxnID == initial.ID {
			openingRow = row
			continue
		}
		poRow = row
	}
	require.NotNil(t, openingRow, "the opening-stock layer must key its own row")
	require.NotNil(t, poRow)

	// Labelled as opening stock, never as a counting correction.
	assert.Equal(t, models.OpeningStockCategoryLabel, openingRow.PONumber)
	assert.NotEqual(t, models.AdjustmentCategoryLabel, openingRow.PONumber,
		"opening stock is not a count correction and must not claim to be one")
	assert.True(t, openingRow.PurchasePrice.IsZero(), "opening stock carries no cost")

	// The load is the window in, and the sell drawn from it is the window out.
	assert.True(t, openingRow.TotalPurchasedAmount.Equal(decimal.NewFromInt(30)),
		"the load must be the window in, got %s", openingRow.TotalPurchasedAmount)
	assert.True(t, openingRow.SubtotalSold.Equal(decimal.NewFromInt(4)),
		"the sell drawn from opening stock must land on its own row, got %s", openingRow.SubtotalSold)

	// Both rows foot, and neither leaks into the other.
	for _, row := range []*ExportRow{openingRow, poRow} {
		in := row.TotalPurchasedAmount.Add(row.TotalTransferredIn)
		out := row.SubtotalSold.Add(row.TotalDisposedAmount).Add(row.TotalTransferredOut)
		assert.True(t, row.BeginningStock.Add(in).Sub(out).Equal(row.EndingStock),
			"row %q must foot: begin %s + in %s - out %s != end %s",
			row.PONumber, row.BeginningStock, in, out, row.EndingStock)
	}
	assert.True(t, openingRow.EndingStock.Equal(decimal.NewFromInt(26)),
		"opening row ending 30-4, got %s", openingRow.EndingStock)
	assert.True(t, poRow.SubtotalSold.Equal(decimal.NewFromInt(5)),
		"PO row keeps only its own sell, got %s", poRow.SubtotalSold)
	assert.True(t, poRow.EndingStock.Equal(decimal.NewFromInt(15)),
		"PO row ending 20-5, got %s", poRow.EndingStock)
}

// A historical opening-stock layer carries its remaining units into beginning stock
// on its own row, so a month that only draws it down still foots.
func TestBuildExportRows_HistoricalInitialLayerCarriesBeginningStock(t *testing.T) {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC)
	const itemID, poiID uint = 41, 7

	histPurchase := newPurchaseTxn(itemID, poiID, 12, ts(2026, time.June, 3), 10)
	histPurchase.ID = 100
	histInitial := newInitialTxn(200, itemID, 50, ts(2026, time.June, 5))

	// This month draws 6 out of the historical opening-stock layer.
	sell := newConsumeTxn(itemID, 6, ts(2026, time.July, 9),
		models.InventoryTransactionTypeSell, nil)
	sell.ID = 201
	counter := histInitial.ID
	sell.CounterTransactionID = &counter

	rows := BuildExportRows(ShaperInput{
		StartDate:   start,
		EndDate:     end,
		InventoryID: 3,
		Items: []*ItemInfo{
			{ItemID: itemID, ProductID: 9, ProductName: "CÀ PHÊ", UnitName: "KG"},
		},
		HistoricalTxns: []*repository.InventoryTransactionWithCounter{histPurchase, histInitial},
		PeriodTxns:     []*repository.InventoryTransactionWithCounter{sell},
		POInfo: map[uint]*repository.POItemSellingPriceInfo{
			poiID: {POItemID: poiID, POID: 1, PONumber: "PO-1", EffectivePrice: decPtr(15)},
		},
	})
	require.NotNil(t, rows)

	var openingRow *ExportRow
	for _, row := range rows.Rows {
		if row.AdjustmentSourceTxnID == histInitial.ID {
			openingRow = row
		}
	}
	require.NotNil(t, openingRow, "a historical opening-stock layer with activity must emit a row")
	assert.Equal(t, models.OpeningStockCategoryLabel, openingRow.PONumber)
	assert.True(t, openingRow.BeginningStock.Equal(decimal.NewFromInt(50)),
		"beginning stock must carry the loaded units, got %s", openingRow.BeginningStock)
	assert.True(t, openingRow.SubtotalSold.Equal(decimal.NewFromInt(6)),
		"the drawdown must be visible, got %s", openingRow.SubtotalSold)
	assert.True(t, openingRow.EndingStock.Equal(decimal.NewFromInt(44)),
		"50 - 6 must foot, got %s", openingRow.EndingStock)
}

// The monthly Excel txn report renders an opening-stock source layer under its own
// label, rather than omitting it or borrowing the count-correction label.
func TestFormatToXLSX_RendersInitialSourceItemUnderOpeningStockLabel(t *testing.T) {
	product := &models.Product{Base: models.Base{ID: 9}, Name: "CÀ PHÊ"}
	item := &models.InventoryItem{Base: models.Base{ID: 41}, ProductID: 9, Product: product}

	initialItem := &models.TxnReportInventoryItem{
		InventoryItem: item,
		SourceTransaction: &models.InventoryTransaction{
			Base:            models.Base{ID: 200},
			TransactionType: models.InventoryTransactionTypeInitial,
			Quantity:        decimal.NewFromInt(30),
			IsAdjustment:    true,
		},
		POMap:                 map[uint]*models.TxnReportPOSummary{},
		PurchaseQuantityByDay: map[int]decimal.Decimal{},
	}
	purchaseItem := &models.TxnReportInventoryItem{
		InventoryItem: item,
		SourceTransaction: &models.InventoryTransaction{
			Base:            models.Base{ID: 100},
			TransactionType: models.InventoryTransactionTypePurchase,
			Quantity:        decimal.NewFromInt(20),
			Price:           10,
		},
		POMap:                 map[uint]*models.TxnReportPOSummary{},
		PurchaseQuantityByDay: map[int]decimal.Decimal{},
	}

	from := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	render := func(items []*models.TxnReportInventoryItem) [][]string {
		report := &models.TxnReportInventory{
			Report: models.Report{
				Title:      "Xuất nhập tồn tháng 07/2026 - KHO",
				From:       &from,
				To:         &to,
				ExportFile: &models.ExportFile{},
			},
			Inventory: &models.Inventory{Base: models.Base{ID: 3}, Name: "KHO"},
			Items:     items,
		}
		data, err := NewTxnReportFormatter().FormatToXLSX(report)
		require.NoError(t, err)
		return sheetRowsOf(t, data)
	}

	withInitial := render([]*models.TxnReportInventoryItem{purchaseItem, initialItem})
	purchaseOnly := render([]*models.TxnReportInventoryItem{purchaseItem})

	assert.Equal(t, len(purchaseOnly)+1, len(withInitial),
		"an opening-stock source item must add exactly one row")

	var cells []string
	for _, row := range withInitial {
		cells = append(cells, row...)
	}
	assert.Contains(t, cells, models.OpeningStockCategoryLabel,
		"the opening-stock row must carry its own label")
	assert.NotContains(t, cells, models.AdjustmentCategoryLabel,
		"it must not borrow the count-correction label")
	assert.Contains(t, cells, "30.00", "the loaded quantity must appear on the row")
}
