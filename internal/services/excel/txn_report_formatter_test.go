package excel

import (
	"bytes"
	"cim-backend/internal/models"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// createTestReport creates a complete test report with all required data.
func createTestReport() *models.TxnReportInventory {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC)

	report := &models.TxnReportInventory{
		Report: models.Report{
			Title:      "Xuất nhập tồn tháng 01/2025 - Warehouse A",
			From:       &from,
			To:         &to,
			ExportFile: &models.ExportFile{},
		},
		Inventory: &models.Inventory{
			Base: models.Base{ID: 1},
			Name: "Warehouse A",
		},
		Items: []*models.TxnReportInventoryItem{
			{
				InventoryItem: &models.InventoryItem{
					Base: models.Base{ID: 1},
					Unit: &models.Unit{Name: "kg"},
					Product: &models.Product{
						Name: "Product A",
						Unit: &models.Unit{Name: "kg"},
					},
				},
				StartQuantity:     decimal.NewFromFloat(100),
				PurchaseQuantity:  decimal.NewFromFloat(50),
				ReconcileQuantity: decimal.NewFromFloat(30),
				DisposeQuantity:   decimal.NewFromFloat(5),
				TransferQuantity:  decimal.NewFromFloat(10),
				EndQuantity:       decimal.NewFromFloat(125),
				POMap: map[uint]*models.TxnReportPOSummary{
					1: {
						OrderNumber: "PO-001",
						Status:      "completed",
						UnitPrice:   15000,
						PurchaseQuantityByDay: map[int]decimal.Decimal{
							1:  decimal.NewFromFloat(10),
							15: decimal.NewFromFloat(20),
							30: decimal.NewFromFloat(20),
						},
					},
				},
			},
		},
	}

	return report
}

// createTestReportWithMultiplePOs creates a report with an item that has multiple POs.
func createTestReportWithMultiplePOs() *models.TxnReportInventory {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC)

	report := &models.TxnReportInventory{
		Report: models.Report{
			Title:      "Xuất nhập tồn tháng 01/2025 - Warehouse B",
			From:       &from,
			To:         &to,
			ExportFile: &models.ExportFile{},
		},
		Inventory: &models.Inventory{
			Base: models.Base{ID: 1},
			Name: "Warehouse B",
		},
		Items: []*models.TxnReportInventoryItem{
			{
				InventoryItem: &models.InventoryItem{
					Base: models.Base{ID: 1},
					Unit: &models.Unit{Name: "box"},
					Product: &models.Product{
						Name: "Product B",
						Unit: &models.Unit{Name: "box"},
					},
				},
				StartQuantity:     decimal.NewFromFloat(50),
				PurchaseQuantity:  decimal.NewFromFloat(100),
				ReconcileQuantity: decimal.NewFromFloat(40),
				DisposeQuantity:   decimal.NewFromFloat(0),
				TransferQuantity:  decimal.NewFromFloat(0),
				EndQuantity:       decimal.NewFromFloat(110),
				POMap: map[uint]*models.TxnReportPOSummary{
					1: {
						OrderNumber: "PO-001",
						Status:      "completed",
						UnitPrice:   10000,
						PurchaseQuantityByDay: map[int]decimal.Decimal{
							5:  decimal.NewFromFloat(30),
							10: decimal.NewFromFloat(20),
						},
					},
					2: {
						OrderNumber: "PO-002",
						Status:      "completed",
						UnitPrice:   11000,
						PurchaseQuantityByDay: map[int]decimal.Decimal{
							20: decimal.NewFromFloat(50),
						},
					},
				},
			},
		},
	}

	return report
}

func TestFormatToXLSX_Success(t *testing.T) {
	report := createTestReport()
	formatter := NewTxnReportFormatter()

	content, err := formatter.FormatToXLSX(report)

	require.NoError(t, err)
	require.NotNil(t, content)
	assert.Greater(t, len(content), 0)

	// Verify we can open the generated Excel file
	file, err := excelize.OpenReader(bytes.NewReader(content))
	require.NoError(t, err)
	defer file.Close()

	// Verify title in row 2, column C
	titleCell, err := file.GetCellValue(txnReportSheetName, "C2")
	require.NoError(t, err)
	assert.Equal(t, "Xuất nhập tồn tháng 01/2025 - Warehouse A", titleCell)

	// Verify inventory name in row 3, column C
	invCell, err := file.GetCellValue(txnReportSheetName, "C3")
	require.NoError(t, err)
	assert.Equal(t, "Warehouse A", invCell)

	// Verify headers in row 5
	sttHeader, err := file.GetCellValue(txnReportSheetName, "A5")
	require.NoError(t, err)
	assert.Equal(t, "STT", sttHeader)

	dienGiaiHeader, err := file.GetCellValue(txnReportSheetName, "B5")
	require.NoError(t, err)
	assert.Equal(t, "Diễn giải", dienGiaiHeader)

	// Verify sub-headers in row 6
	slHeader, err := file.GetCellValue(txnReportSheetName, "E6")
	require.NoError(t, err)
	assert.Equal(t, "SL", slHeader)

	ttHeader, err := file.GetCellValue(txnReportSheetName, "F6")
	require.NoError(t, err)
	assert.Equal(t, "TT", ttHeader)

	// Verify data row 7 (first item)
	sttValue, err := file.GetCellValue(txnReportSheetName, "A7")
	require.NoError(t, err)
	assert.Equal(t, "1", sttValue)

	productName, err := file.GetCellValue(txnReportSheetName, "B7")
	require.NoError(t, err)
	assert.Equal(t, "Product A", productName)

	unitName, err := file.GetCellValue(txnReportSheetName, "C7")
	require.NoError(t, err)
	assert.Equal(t, "kg", unitName)

	unitPrice, err := file.GetCellValue(txnReportSheetName, "D7")
	require.NoError(t, err)
	assert.Equal(t, "15,000.00", unitPrice)
}

func TestFormatToXLSX_WithMultiplePOs(t *testing.T) {
	report := createTestReportWithMultiplePOs()
	formatter := NewTxnReportFormatter()

	content, err := formatter.FormatToXLSX(report)

	require.NoError(t, err)
	require.NotNil(t, content)

	// Verify we can open the generated Excel file
	file, err := excelize.OpenReader(bytes.NewReader(content))
	require.NoError(t, err)
	defer file.Close()

	// Should have 2 data rows for the same item (one per PO)
	// Row 7: First PO
	stt1, err := file.GetCellValue(txnReportSheetName, "A7")
	require.NoError(t, err)
	assert.Equal(t, "1", stt1)

	product1, err := file.GetCellValue(txnReportSheetName, "B7")
	require.NoError(t, err)
	assert.Equal(t, "Product B", product1)

	// Row 8: Second PO
	stt2, err := file.GetCellValue(txnReportSheetName, "A8")
	require.NoError(t, err)
	assert.Equal(t, "2", stt2)

	product2, err := file.GetCellValue(txnReportSheetName, "B8")
	require.NoError(t, err)
	assert.Equal(t, "Product B", product2)

	// Unit prices should be different
	price1, err := file.GetCellValue(txnReportSheetName, "D7")
	require.NoError(t, err)
	assert.Equal(t, "10,000.00", price1)

	price2, err := file.GetCellValue(txnReportSheetName, "D8")
	require.NoError(t, err)
	assert.Equal(t, "11,000.00", price2)
}

func TestFormatToXLSX_NilReport(t *testing.T) {
	formatter := NewTxnReportFormatter()

	content, err := formatter.FormatToXLSX(nil)

	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "report is nil")
}

func TestFormatToXLSX_NilExportFile(t *testing.T) {
	report := createTestReport()
	report.ExportFile = nil

	formatter := NewTxnReportFormatter()

	content, err := formatter.FormatToXLSX(report)

	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "ExportFile is nil")
}

func TestFormatToXLSX_EmptyTitle(t *testing.T) {
	report := createTestReport()
	report.Title = ""

	formatter := NewTxnReportFormatter()

	content, err := formatter.FormatToXLSX(report)

	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "title is empty")
}

func TestFormatToXLSX_MissingDateRange(t *testing.T) {
	report := createTestReport()
	report.From = nil

	formatter := NewTxnReportFormatter()

	content, err := formatter.FormatToXLSX(report)

	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "date range is missing")
}

func TestFormatToXLSX_NilInventory(t *testing.T) {
	report := createTestReport()
	report.Inventory = nil

	formatter := NewTxnReportFormatter()

	content, err := formatter.FormatToXLSX(report)

	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "inventory is nil")
}

func TestFormatToXLSX_NoItems(t *testing.T) {
	report := createTestReport()
	report.Items = []*models.TxnReportInventoryItem{}

	formatter := NewTxnReportFormatter()

	content, err := formatter.FormatToXLSX(report)

	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "no items to export")
}

func TestFormatToXLSX_ItemWithNilInventoryItem(t *testing.T) {
	report := createTestReport()
	report.Items[0].InventoryItem = nil

	formatter := NewTxnReportFormatter()

	content, err := formatter.FormatToXLSX(report)

	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "nil InventoryItem")
}

func TestFormatToXLSX_ItemWithNoPOs(t *testing.T) {
	report := createTestReport()
	report.Items[0].POMap = map[uint]*models.TxnReportPOSummary{}

	formatter := NewTxnReportFormatter()

	content, err := formatter.FormatToXLSX(report)

	require.NoError(t, err)
	require.NotNil(t, content)

	// Verify we can open the generated Excel file
	file, err := excelize.OpenReader(bytes.NewReader(content))
	require.NoError(t, err)
	defer file.Close()

	// Should still have one row with unit price = 0
	unitPrice, err := file.GetCellValue(txnReportSheetName, "D7")
	require.NoError(t, err)
	assert.Equal(t, "0.00", unitPrice)
}

func TestGetDaysInMonth(t *testing.T) {
	tests := []struct {
		name     string
		date     time.Time
		expected int
	}{
		{
			name:     "January 2025 - 31 days",
			date:     time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			expected: 31,
		},
		{
			name:     "February 2025 - 28 days",
			date:     time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC),
			expected: 28,
		},
		{
			name:     "February 2024 - 29 days (leap year)",
			date:     time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC),
			expected: 29,
		},
		{
			name:     "April 2025 - 30 days",
			date:     time.Date(2025, 4, 15, 0, 0, 0, 0, time.UTC),
			expected: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDaysInMonth(&tt.date)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatToXLSX_VerifyNoTotalsRow(t *testing.T) {
	// Test that totals row has been removed per user requirements
	report := createTestReport()
	formatter := NewTxnReportFormatter()

	content, err := formatter.FormatToXLSX(report)
	require.NoError(t, err)

	file, err := excelize.OpenReader(bytes.NewReader(content))
	require.NoError(t, err)
	defer file.Close()

	// Row 7 is the data row
	// Row 8 should NOT contain a totals row - it should be empty
	totalLabel, err := file.GetCellValue(txnReportSheetName, "B8")
	require.NoError(t, err)
	assert.NotEqual(t, "TỔNG CỘNG", totalLabel, "Totals row should have been removed")
	assert.Equal(t, "", totalLabel, "Row 8 should be empty (no totals row)")
}

func TestValidateReport_AllCases(t *testing.T) {
	formatter := NewTxnReportFormatter()

	tests := []struct {
		name          string
		setupReport   func() *models.TxnReportInventory
		expectedError string
	}{
		{
			name:          "nil report",
			setupReport:   func() *models.TxnReportInventory { return nil },
			expectedError: "report is nil",
		},
		{
			name: "nil ExportFile",
			setupReport: func() *models.TxnReportInventory {
				r := createTestReport()
				r.ExportFile = nil
				return r
			},
			expectedError: "ExportFile is nil",
		},
		{
			name: "empty title",
			setupReport: func() *models.TxnReportInventory {
				r := createTestReport()
				r.Title = ""
				return r
			},
			expectedError: "title is empty",
		},
		{
			name: "nil From date",
			setupReport: func() *models.TxnReportInventory {
				r := createTestReport()
				r.From = nil
				return r
			},
			expectedError: "date range is missing",
		},
		{
			name: "nil To date",
			setupReport: func() *models.TxnReportInventory {
				r := createTestReport()
				r.To = nil
				return r
			},
			expectedError: "date range is missing",
		},
		{
			name: "nil Inventory",
			setupReport: func() *models.TxnReportInventory {
				r := createTestReport()
				r.Inventory = nil
				return r
			},
			expectedError: "inventory is nil",
		},
		{
			name: "empty Items",
			setupReport: func() *models.TxnReportInventory {
				r := createTestReport()
				r.Items = []*models.TxnReportInventoryItem{}
				return r
			},
			expectedError: "no items to export",
		},
		{
			name: "item with nil InventoryItem",
			setupReport: func() *models.TxnReportInventory {
				r := createTestReport()
				r.Items[0].InventoryItem = nil
				return r
			},
			expectedError: "nil InventoryItem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := tt.setupReport()
			err := formatter.validateReport(report)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestFormatToXLSX_WithNilProduct(t *testing.T) {
	report := createTestReport()
	report.Items[0].Product = nil // Product can be nil

	formatter := NewTxnReportFormatter()

	content, err := formatter.FormatToXLSX(report)

	require.NoError(t, err)
	require.NotNil(t, content)

	// Verify we can open the generated Excel file
	file, err := excelize.OpenReader(bytes.NewReader(content))
	require.NoError(t, err)
	defer file.Close()

	// Product name should be empty
	productName, err := file.GetCellValue(txnReportSheetName, "B7")
	require.NoError(t, err)
	assert.Equal(t, "", productName)

	// Unit should still be populated from InventoryItem.Unit
	unit, err := file.GetCellValue(txnReportSheetName, "C7")
	require.NoError(t, err)
	assert.Equal(t, "kg", unit, "Unit should come from InventoryItem.Unit, not Product.Unit")
}
