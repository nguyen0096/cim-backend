// Package excel provides Excel file formatting utilities for inventory reports.
//
// # Debug Mode
//
// To enable debug mode and save generated Excel files to disk during runtime:
//
//  1. Set DEBUG_SAVE_EXCEL = true (line 32)
//  2. Run your application
//  3. Check console for: [DEBUG] Excel file saved to: /path/to/your-binary/...
//  4. Open the file in Excel/LibreOffice to inspect
//  5. Set DEBUG_SAVE_EXCEL = false before committing
//
// Debug files are saved in the same directory as your executable with timestamps.
package excel

import (
	"cim-backend/internal/models"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

const txnReportSheetName = "Sheet1"

// DEBUG_SAVE_EXCEL enables saving Excel files to disk for debugging runtime sessions.
// Set to true to save all generated files in the same directory as the executable.
// Files are named: {report-title}-{timestamp}.xlsx
// Remember to set back to false before committing!
const DEBUG_SAVE_EXCEL = false

// TxnReportFormatter formats inventory transaction reports as Excel files.
// It's a pure data transformation component with no database dependencies.
type TxnReportFormatter struct {
	// No dependencies - pure data transformation
}

// NewTxnReportFormatter creates a new formatter instance.
func NewTxnReportFormatter() *TxnReportFormatter {
	return &TxnReportFormatter{}
}

// FormatToXLSX converts a TxnReportInventory to Excel format and returns the bytes.
// This is a pure function with no side effects or database access.
func (f *TxnReportFormatter) FormatToXLSX(report *models.TxnReportInventory) ([]byte, error) {
	// Validate input data first
	if err := f.validateReport(report); err != nil {
		return nil, err
	}

	// Create Excel file
	file := excelize.NewFile()
	defer file.Close()

	// Calculate days in month
	daysInMonth := getDaysInMonth(report.From)

	// Write metadata rows (1-4)
	if err := f.writeMetadataRows(file, report.Title, report.Inventory.Name); err != nil {
		return nil, err
	}

	// Write headers (row 5)
	if err := f.writeHeaders(file, daysInMonth); err != nil {
		return nil, err
	}

	// Write data rows (starting row 6)
	if err := f.writeDataRows(file, report.Items, daysInMonth); err != nil {
		return nil, err
	}

	// Apply column widths (removed totals row per requirements)
	f.applyColumnWidths(file, daysInMonth)

	// Debug: Save file to disk if DEBUG_SAVE_EXCEL is enabled
	// Set DEBUG_SAVE_EXCEL = true at the top of this file to enable
	if DEBUG_SAVE_EXCEL {
		if err := f.debugSaveFile(file, report.Title); err != nil {
			// Log error but don't fail - debugging shouldn't break production
			fmt.Printf("[WARNING] Failed to save debug Excel file: %v\n", err)
		}
	}

	// Convert to bytes
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to write Excel to buffer: %w", err)
	}

	return buffer.Bytes(), nil
}

// validateReport checks if all required data is present and returns clear error messages.
func (f *TxnReportFormatter) validateReport(report *models.TxnReportInventory) error {
	if report == nil {
		return fmt.Errorf("report is nil")
	}

	if report.ExportFile == nil {
		return fmt.Errorf("report.ExportFile is nil")
	}

	if report.Title == "" {
		return fmt.Errorf("report title is empty")
	}

	if report.From == nil || report.To == nil {
		return fmt.Errorf("report date range is missing (From or To is nil)")
	}

	if report.Inventory == nil {
		return fmt.Errorf("report inventory is nil")
	}

	if len(report.Items) == 0 {
		return fmt.Errorf("report has no items to export")
	}

	// Validate each item has required data
	for i, item := range report.Items {
		if item.InventoryItem == nil {
			return fmt.Errorf("item at index %d has nil InventoryItem", i)
		}
		// Product can be nil - will show empty name
		// PurchaseQuantityByDay can be empty - will show zeros
	}

	return nil
}

// writeMetadataRows writes rows 1-4 (padding and metadata).
func (f *TxnReportFormatter) writeMetadataRows(file *excelize.File, title, inventoryName string) error {
	// Create styles
	titleStyle, err := f.createTitleStyle(file)
	if err != nil {
		return fmt.Errorf("failed to create title style: %w", err)
	}

	inventoryStyle, err := f.createInventoryNameStyle(file)
	if err != nil {
		return fmt.Errorf("failed to create inventory style: %w", err)
	}

	// Row 1: Empty (padding)

	// Row 2: Title in column C (bold, size 16)
	if err := file.SetCellValue(txnReportSheetName, "C2", title); err != nil {
		return fmt.Errorf("failed to set title: %w", err)
	}
	if err := file.SetCellStyle(txnReportSheetName, "C2", "C2", titleStyle); err != nil {
		return fmt.Errorf("failed to set title style: %w", err)
	}

	// Row 3: Inventory name in column C (bold, size 14)
	if err := file.SetCellValue(txnReportSheetName, "C3", inventoryName); err != nil {
		return fmt.Errorf("failed to set inventory name: %w", err)
	}
	if err := file.SetCellStyle(txnReportSheetName, "C3", "C3", inventoryStyle); err != nil {
		return fmt.Errorf("failed to set inventory style: %w", err)
	}

	// Row 4: Empty (padding)

	return nil
}

// writeHeaders writes rows 5-6 with 2-level header structure and colored backgrounds.
func (f *TxnReportFormatter) writeHeaders(file *excelize.File, daysInMonth int) error {
	// Create all required styles
	styleWhite, err := f.createHeaderStyle(file)
	if err != nil {
		return fmt.Errorf("failed to create white header style: %w", err)
	}
	styleGreen, err := f.createHeaderStyleGreen(file)
	if err != nil {
		return fmt.Errorf("failed to create green header style: %w", err)
	}
	styleCyan, err := f.createHeaderStyleCyan(file)
	if err != nil {
		return fmt.Errorf("failed to create cyan header style: %w", err)
	}
	stylePink, err := f.createHeaderStylePink(file)
	if err != nil {
		return fmt.Errorf("failed to create pink header style: %w", err)
	}
	styleGray, err := f.createHeaderStyleGray(file)
	if err != nil {
		return fmt.Errorf("failed to create gray header style: %w", err)
	}

	col := 1
	row := 5

	// Helper to write and merge header with custom style
	writeHeader := func(text string, cols int, style int) error {
		startCell, _ := excelize.CoordinatesToCellName(col, row)
		endCell, _ := excelize.CoordinatesToCellName(col+cols-1, row)

		if err := file.SetCellValue(txnReportSheetName, startCell, text); err != nil {
			return err
		}

		if cols > 1 {
			if err := file.MergeCell(txnReportSheetName, startCell, endCell); err != nil {
				return err
			}
		}

		// Use the helper to apply borders to all cells in merged range
		if err := setMergedCellBorders(file, startCell, endCell, style); err != nil {
			return err
		}

		col += cols
		return nil
	}

	// Single-column headers (white background)
	if err := writeHeader("STT", 1, styleWhite); err != nil {
		return fmt.Errorf("failed to write STT header: %w", err)
	}
	if err := writeHeader("Diễn giải", 1, styleWhite); err != nil {
		return fmt.Errorf("failed to write Diễn giải header: %w", err)
	}
	if err := writeHeader("ĐVT", 1, styleWhite); err != nil {
		return fmt.Errorf("failed to write ĐVT header: %w", err)
	}
	if err := writeHeader("Nguồn", 1, styleWhite); err != nil {
		return fmt.Errorf("failed to write Nguồn header: %w", err)
	}
	if err := writeHeader("Ngày nhận", 1, styleWhite); err != nil {
		return fmt.Errorf("failed to write Ngày nhận header: %w", err)
	}
	if err := writeHeader("Đơn giá", 1, styleWhite); err != nil {
		return fmt.Errorf("failed to write Đơn giá header: %w", err)
	}

	// Tồn đầu (2 sub-columns: SL, TT) - Blue
	if err := writeHeader("Tồn đầu", 2, styleCyan); err != nil {
		return fmt.Errorf("failed to write Tồn đầu header: %w", err)
	}

	// SL nguồn (2 sub-columns) - Green
	if err := writeHeader("SL nguồn", 2, styleGreen); err != nil {
		return fmt.Errorf("failed to write SL nguồn header: %w", err)
	}

	// Đã tiêu thụ (2 sub-columns) - Pink (same as Chi tiết tiêu thụ)
	if err := writeHeader("Đã tiêu thụ", 2, stylePink); err != nil {
		return fmt.Errorf("failed to write Đã tiêu thụ header: %w", err)
	}

	// Còn lại (2 sub-columns) - Gray
	if err := writeHeader("Còn lại", 2, styleGray); err != nil {
		return fmt.Errorf("failed to write Còn lại header: %w", err)
	}

	// Chi tiết tiêu thụ (4 sub-columns: Loại | Ngày | SL | TT) - Pink (same as Đã tiêu thụ)
	if err := writeHeader("Chi tiết tiêu thụ", 4, stylePink); err != nil {
		return fmt.Errorf("failed to write Chi tiết tiêu thụ header: %w", err)
	}

	// Now write sub-headers on row 6
	col = 7 // Start after STT, Diễn giải, ĐVT, Nguồn, Ngày nhận, Đơn giá
	row = 6

	// Tồn đầu sub-headers (SL, TT)
	for i := 0; i < 2; i++ {
		subHeader := "SL"
		if i == 1 {
			subHeader = "TT"
		}
		cell, _ := excelize.CoordinatesToCellName(col, row)
		if err := file.SetCellValue(txnReportSheetName, cell, subHeader); err != nil {
			return fmt.Errorf("failed to write sub-header: %w", err)
		}
		if err := file.SetCellStyle(txnReportSheetName, cell, cell, styleWhite); err != nil {
			return err
		}
		col++
	}

	// SL nguồn sub-headers (SL, TT)
	for i := 0; i < 2; i++ {
		subHeader := "SL"
		if i == 1 {
			subHeader = "TT"
		}
		cell, _ := excelize.CoordinatesToCellName(col, row)
		if err := file.SetCellValue(txnReportSheetName, cell, subHeader); err != nil {
			return fmt.Errorf("failed to write sub-header: %w", err)
		}
		if err := file.SetCellStyle(txnReportSheetName, cell, cell, styleWhite); err != nil {
			return err
		}
		col++
	}

	// Đã tiêu thụ sub-headers (SL, TT)
	for i := 0; i < 2; i++ {
		subHeader := "SL"
		if i == 1 {
			subHeader = "TT"
		}
		cell, _ := excelize.CoordinatesToCellName(col, row)
		if err := file.SetCellValue(txnReportSheetName, cell, subHeader); err != nil {
			return fmt.Errorf("failed to write sub-header: %w", err)
		}
		if err := file.SetCellStyle(txnReportSheetName, cell, cell, styleWhite); err != nil {
			return err
		}
		col++
	}

	// Còn lại sub-headers (SL, TT)
	for i := 0; i < 2; i++ {
		subHeader := "SL"
		if i == 1 {
			subHeader = "TT"
		}
		cell, _ := excelize.CoordinatesToCellName(col, row)
		if err := file.SetCellValue(txnReportSheetName, cell, subHeader); err != nil {
			return fmt.Errorf("failed to write sub-header: %w", err)
		}
		if err := file.SetCellStyle(txnReportSheetName, cell, cell, styleWhite); err != nil {
			return err
		}
		col++
	}

	// Chi tiết tiêu thụ sub-headers (Loại, Ngày, SL, TT)
	chiTietSubHeaders := []string{"Loại", "Ngày", "SL", "TT"}
	for _, subHeader := range chiTietSubHeaders {
		cell, _ := excelize.CoordinatesToCellName(col, row)
		if err := file.SetCellValue(txnReportSheetName, cell, subHeader); err != nil {
			return fmt.Errorf("failed to write Chi tiết tiêu thụ sub-header: %w", err)
		}
		if err := file.SetCellStyle(txnReportSheetName, cell, cell, styleWhite); err != nil {
			return err
		}
		col++
	}

	// Merge single-column headers across both header rows
	for _, col := range []int{1, 2, 3, 4, 5, 6} {
		startCell, _ := excelize.CoordinatesToCellName(col, 5)
		endCell, _ := excelize.CoordinatesToCellName(col, 6)
		if err := file.MergeCell(txnReportSheetName, startCell, endCell); err != nil {
			return fmt.Errorf("failed to merge single column header: %w", err)
		}
		// Apply style to both cells in the merged range
		if err := setMergedCellBorders(file, startCell, endCell, styleWhite); err != nil {
			return fmt.Errorf("failed to set borders on merged header: %w", err)
		}
	}

	return nil
}

// writeDataRows writes all data rows.
func (f *TxnReportFormatter) writeDataRows(file *excelize.File, items []*models.TxnReportInventoryItem, daysInMonth int) error {
	rowNum := 7 // Start after headers (rows 5-6)
	serialNum := 1

	dataStyle, err := f.createDataStyle(file)
	if err != nil {
		return fmt.Errorf("failed to create data style: %w", err)
	}

	dataStyleBold, err := f.createDataStyleBold(file)
	if err != nil {
		return fmt.Errorf("failed to create bold data style: %w", err)
	}

	numberStyle, err := f.createNumberStyle(file)
	if err != nil {
		return fmt.Errorf("failed to create number style: %w", err)
	}

	slStyle, err := f.createSLStyle(file)
	if err != nil {
		return fmt.Errorf("failed to create SL style: %w", err)
	}

	for _, item := range items {
		// Check if this is source transaction view
		if item.SourceTransaction != nil {
			if err := f.writeSourceTransactionRows(file, rowNum, serialNum, item, dataStyle, dataStyleBold, numberStyle, slStyle); err != nil {
				return err
			}
			rowNum += 1 + len(item.ConsumeDetails)
			serialNum++
			continue
		}

		// Legacy PO-grouped view
		// If no POs, create single row with unit price = 0
		if len(item.POMap) == 0 {
			if err := f.writeItemPORow(file, rowNum, serialNum, item, nil, true, daysInMonth, dataStyle, dataStyleBold, numberStyle, slStyle); err != nil {
				return err
			}
			rowNum++
			serialNum++
			continue
		}

		// Create one row per PO
		isFirstPO := true
		for _, poSummary := range item.POMap {
			if err := f.writeItemPORow(file, rowNum, serialNum, item, poSummary, isFirstPO, daysInMonth, dataStyle, dataStyleBold, numberStyle, slStyle); err != nil {
				return err
			}
			rowNum++
			serialNum++
			isFirstPO = false
		}
	}

	return nil
}

// writeItemPORow writes a single row for an inventory item + PO combination.
func (f *TxnReportFormatter) writeItemPORow(
	file *excelize.File,
	rowNum, serialNum int,
	item *models.TxnReportInventoryItem,
	poSummary *models.TxnReportPOSummary,
	isFirstPO bool,
	daysInMonth int,
	dataStyle, dataStyleBold, numberStyle, slStyle int,
) error {
	col := 1

	// Helper to set cell value with style
	setCell := func(value interface{}, style int) error {
		cell, _ := excelize.CoordinatesToCellName(col, rowNum)
		if err := file.SetCellValue(txnReportSheetName, cell, value); err != nil {
			return err
		}
		if err := file.SetCellStyle(txnReportSheetName, cell, cell, style); err != nil {
			return err
		}
		col++
		return nil
	}

	// Helper to set numeric cell with number format (SL columns use slStyle)
	setNumCell := func(value decimal.Decimal, showOnlyOnFirstPO bool) error {
		cell, _ := excelize.CoordinatesToCellName(col, rowNum)

		if showOnlyOnFirstPO && !isFirstPO {
			// Leave empty on subsequent PO rows, use SL style
			if err := file.SetCellStyle(txnReportSheetName, cell, cell, slStyle); err != nil {
				return err
			}
		} else {
			floatVal, _ := value.Float64()
			if err := file.SetCellValue(txnReportSheetName, cell, floatVal); err != nil {
				return err
			}
			if err := file.SetCellStyle(txnReportSheetName, cell, cell, slStyle); err != nil {
				return err
			}
		}
		col++
		return nil
	}

	// Helper for value columns (TT = SL × UnitPrice)
	setValueCell := func(quantity decimal.Decimal, unitPrice float64, showOnlyOnFirstPO bool) error {
		cell, _ := excelize.CoordinatesToCellName(col, rowNum)

		if showOnlyOnFirstPO && !isFirstPO {
			// Leave empty
			if err := file.SetCellStyle(txnReportSheetName, cell, cell, numberStyle); err != nil {
				return err
			}
		} else {
			qtyFloat, _ := quantity.Float64()
			value := qtyFloat * unitPrice
			if err := file.SetCellValue(txnReportSheetName, cell, value); err != nil {
				return err
			}
			if err := file.SetCellStyle(txnReportSheetName, cell, cell, numberStyle); err != nil {
				return err
			}
		}
		col++
		return nil
	}

	// Column A: STT
	if err := setCell(serialNum, dataStyle); err != nil {
		return fmt.Errorf("failed to set STT: %w", err)
	}

	// Column B: Diễn giải (product name) - use bold style
	productName := ""
	if item.Product != nil {
		productName = item.Product.Name
	}
	if err := setCell(productName, dataStyleBold); err != nil {
		return fmt.Errorf("failed to set product name: %w", err)
	}

	// Column C: ĐVT (unit) - use inventory_item.Unit, not product.Unit
	unitName := ""
	if item.InventoryItem != nil && item.InventoryItem.Unit != nil {
		unitName = item.InventoryItem.Unit.Name
	}
	if err := setCell(unitName, dataStyle); err != nil {
		return fmt.Errorf("failed to set unit: %w", err)
	}

	// Column D: Đơn giá (unit price from PO)
	unitPrice := 0.0
	if poSummary != nil {
		unitPrice = poSummary.UnitPrice
	}
	if err := setCell(unitPrice, numberStyle); err != nil {
		return fmt.Errorf("failed to set unit price: %w", err)
	}

	// Columns E-F: Tồn đầu SL/TT (show only on first PO row)
	if err := setNumCell(item.StartQuantity, true); err != nil {
		return fmt.Errorf("failed to set start quantity: %w", err)
	}
	if err := setValueCell(item.StartQuantity, unitPrice, true); err != nil {
		return fmt.Errorf("failed to set start value: %w", err)
	}

	// Daily columns (Ngày 1-31) SL/TT
	totalImport := decimal.Zero
	for day := 1; day <= daysInMonth; day++ {
		dayQty := decimal.Zero
		if poSummary != nil {
			if qty, exists := poSummary.PurchaseQuantityByDay[day]; exists {
				dayQty = qty
				totalImport = totalImport.Add(qty)
			}
		}

		// SL column - use slStyle for #FFCC99 background
		cell, _ := excelize.CoordinatesToCellName(col, rowNum)
		if !dayQty.IsZero() {
			floatVal, _ := dayQty.Float64()
			if err := file.SetCellValue(txnReportSheetName, cell, floatVal); err != nil {
				return err
			}
		}
		if err := file.SetCellStyle(txnReportSheetName, cell, cell, slStyle); err != nil {
			return err
		}
		col++

		// TT column (value) - use numberStyle for white background
		cell, _ = excelize.CoordinatesToCellName(col, rowNum)
		if !dayQty.IsZero() {
			qtyFloat, _ := dayQty.Float64()
			value := qtyFloat * unitPrice
			if err := file.SetCellValue(txnReportSheetName, cell, value); err != nil {
				return err
			}
		}
		if err := file.SetCellStyle(txnReportSheetName, cell, cell, numberStyle); err != nil {
			return err
		}
		col++
	}

	// Tổng nhập SL/TT
	if err := setNumCell(totalImport, false); err != nil {
		return fmt.Errorf("failed to set total import quantity: %w", err)
	}
	if err := setValueCell(totalImport, unitPrice, false); err != nil {
		return fmt.Errorf("failed to set total import value: %w", err)
	}

	// Tiêu thụ SL/TT (show only on first PO row)
	if err := setNumCell(item.ReconcileQuantity, true); err != nil {
		return fmt.Errorf("failed to set reconcile quantity: %w", err)
	}
	if err := setValueCell(item.ReconcileQuantity, unitPrice, true); err != nil {
		return fmt.Errorf("failed to set reconcile value: %w", err)
	}

	// Hủy SL/TT (show only on first PO row)
	if err := setNumCell(item.DisposeQuantity, true); err != nil {
		return fmt.Errorf("failed to set dispose quantity: %w", err)
	}
	if err := setValueCell(item.DisposeQuantity, unitPrice, true); err != nil {
		return fmt.Errorf("failed to set dispose value: %w", err)
	}

	// Xuất kho SL/TT (show only on first PO row) - display as negative
	transferQuantity := item.TransferQuantity.Neg()
	if err := setNumCell(transferQuantity, true); err != nil {
		return fmt.Errorf("failed to set transfer quantity: %w", err)
	}
	if err := setValueCell(transferQuantity, unitPrice, true); err != nil {
		return fmt.Errorf("failed to set transfer value: %w", err)
	}

	// Tồn cuối SL/TT (show only on first PO row)
	// EndQuantity = StartQuantity + TotalImport - Reconcile - Dispose - Transfer
	endQuantity := item.StartQuantity.
		Add(item.PurchaseQuantity).
		Sub(item.ReconcileQuantity).
		Sub(item.DisposeQuantity).
		Add(item.TransferQuantity) // Transfer can be negative (net out)

	if err := setNumCell(endQuantity, true); err != nil {
		return fmt.Errorf("failed to set end quantity: %w", err)
	}
	if err := setValueCell(endQuantity, unitPrice, true); err != nil {
		return fmt.Errorf("failed to set end value: %w", err)
	}

	return nil
}

// writeSourceTransactionRows writes rows for source transaction view
func (f *TxnReportFormatter) writeSourceTransactionRows(
	file *excelize.File,
	rowNum, serialNum int,
	item *models.TxnReportInventoryItem,
	dataStyle, dataStyleBold, numberStyle, slStyle int,
) error {
	source := item.SourceTransaction

	// Helper to set cell with style
	setCell := func(row, col int, value interface{}, style int) error {
		cell, _ := excelize.CoordinatesToCellName(col, row)
		if err := file.SetCellValue(txnReportSheetName, cell, value); err != nil {
			return err
		}
		return file.SetCellStyle(txnReportSheetName, cell, cell, style)
	}

	// Get source type display name
	sourceType := ""
	unitPrice := source.Price
	if source.TransactionType == models.InventoryTransactionTypePurchase {
		sourceType = "Mua hàng"
	} else if source.TransactionType == models.InventoryTransactionTypeTransferIn {
		sourceType = "Chuyển kho"
	}

	// Consumed quantity from source transaction
	consumedQty := source.ConsumedQuantity
	remainingQty := source.Quantity.Sub(consumedQty)

	// First row: source transaction details
	col := 1

	// STT
	if err := setCell(rowNum, col, serialNum, dataStyle); err != nil {
		return err
	}
	col++

	// Product name
	productName := ""
	if item.Product != nil {
		productName = item.Product.Name
	}
	if err := setCell(rowNum, col, productName, dataStyleBold); err != nil {
		return err
	}
	col++

	// Unit
	unitName := ""
	if item.InventoryItem != nil && item.InventoryItem.Unit != nil {
		unitName = item.InventoryItem.Unit.Name
	}
	if err := setCell(rowNum, col, unitName, dataStyle); err != nil {
		return err
	}
	col++

	// Source type
	if err := setCell(rowNum, col, sourceType, dataStyle); err != nil {
		return err
	}
	col++

	// Receive date
	if err := setCell(rowNum, col, source.CreatedAt.Format("02/01/2006"), dataStyle); err != nil {
		return err
	}
	col++

	// Unit price
	if err := setCell(rowNum, col, unitPrice, numberStyle); err != nil {
		return err
	}
	col++

	// Opening quantity (SL, TT)
	qtyFloat, _ := item.StartQuantity.Float64()
	if err := setCell(rowNum, col, qtyFloat, slStyle); err != nil {
		return err
	}
	col++
	if err := setCell(rowNum, col, qtyFloat*unitPrice, numberStyle); err != nil {
		return err
	}
	col++

	// Source quantity (SL, TT)
	sourceQtyFloat, _ := source.Quantity.Float64()
	if err := setCell(rowNum, col, sourceQtyFloat, slStyle); err != nil {
		return err
	}
	col++
	if err := setCell(rowNum, col, sourceQtyFloat*unitPrice, numberStyle); err != nil {
		return err
	}
	col++

	// Total consumed quantity (SL, TT) - sum of all consume details
	totalConsumedQty := decimal.Zero
	for _, consume := range item.ConsumeDetails {
		totalConsumedQty = totalConsumedQty.Add(consume.Quantity)
	}
	totalConsumedQtyFloat, _ := totalConsumedQty.Float64()
	if err := setCell(rowNum, col, totalConsumedQtyFloat, slStyle); err != nil {
		return err
	}
	col++
	if err := setCell(rowNum, col, totalConsumedQtyFloat*unitPrice, numberStyle); err != nil {
		return err
	}
	col++

	// Remaining quantity (SL, TT)
	remainingQtyFloat, _ := remainingQty.Float64()
	if err := setCell(rowNum, col, remainingQtyFloat, slStyle); err != nil {
		return err
	}
	col++
	if err := setCell(rowNum, col, remainingQtyFloat*unitPrice, numberStyle); err != nil {
		return err
	}
	col++

	// Tiêu thụ columns: Loại, Ngày, SL, TT (empty for source row - shows totals above)
	setCell(rowNum, col, "", dataStyle)
	col++
	setCell(rowNum, col, "", dataStyle)
	col++
	setCell(rowNum, col, "", slStyle)
	col++
	setCell(rowNum, col, "", numberStyle)

	// Write consume detail rows
	for _, consume := range item.ConsumeDetails {
		rowNum++
		col = 1

		// Leave STT, Product, Unit, Source, Receive Date, Unit Price empty
		for j := 0; j < 6; j++ {
			setCell(rowNum, col, "", dataStyle)
			col++
		}

		// Leave opening, source qty empty (4 columns)
		for j := 0; j < 4; j++ {
			if j%2 == 0 {
				setCell(rowNum, col, "", slStyle)
			} else {
				setCell(rowNum, col, "", numberStyle)
			}
			col++
		}

		// Leave "Đã tiêu thụ" total empty (2 columns)
		setCell(rowNum, col, "", slStyle)
		col++
		setCell(rowNum, col, "", numberStyle)
		col++

		// Leave "Còn lại" empty (2 columns)
		setCell(rowNum, col, "", slStyle)
		col++
		setCell(rowNum, col, "", numberStyle)
		col++

		// Tiêu thụ detail columns: Loại, Ngày, SL, TT
		// Consume type
		consumeType := ""
		switch consume.TransactionType {
		case models.InventoryTransactionTypeSell:
			consumeType = "Tiêu thụ"
		case models.InventoryTransactionTypeDisposal:
			consumeType = "Hủy"
		case models.InventoryTransactionTypeTransferOut:
			consumeType = "Xuất kho"
		}
		setCell(rowNum, col, consumeType, dataStyle)
		col++

		// Consume date
		setCell(rowNum, col, consume.CreatedAt.Format("02/01/2006"), dataStyle)
		col++

		// Consume quantity (SL, TT)
		consumeQtyFloat, _ := consume.Quantity.Float64()
		setCell(rowNum, col, consumeQtyFloat, slStyle)
		col++
		setCell(rowNum, col, consumeQtyFloat*unitPrice, numberStyle)
	}

	return nil
}

// applyColumnWidths sets appropriate column widths for readability.
func (f *TxnReportFormatter) applyColumnWidths(file *excelize.File, daysInMonth int) {
	file.SetColWidth(txnReportSheetName, "A", "A", 5)  // STT
	file.SetColWidth(txnReportSheetName, "B", "B", 30) // Diễn giải
	file.SetColWidth(txnReportSheetName, "C", "C", 10) // ĐVT
	file.SetColWidth(txnReportSheetName, "D", "D", 15) // Nguồn
	file.SetColWidth(txnReportSheetName, "E", "E", 12) // Ngày nhận
	file.SetColWidth(txnReportSheetName, "F", "F", 12) // Đơn giá

	// SL columns (G, I, K, M, Q) - width 12
	file.SetColWidth(txnReportSheetName, "G", "G", 12) // Tồn đầu SL
	file.SetColWidth(txnReportSheetName, "I", "I", 12) // SL nguồn SL
	file.SetColWidth(txnReportSheetName, "K", "K", 12) // Đã tiêu thụ SL
	file.SetColWidth(txnReportSheetName, "M", "M", 12) // Còn lại SL
	file.SetColWidth(txnReportSheetName, "Q", "Q", 12) // Chi tiết tiêu thụ SL

	// TT columns (H, J, L, N, R) - width 15 (bigger and consistent)
	file.SetColWidth(txnReportSheetName, "H", "H", 15) // Tồn đầu TT
	file.SetColWidth(txnReportSheetName, "J", "J", 15) // SL nguồn TT
	file.SetColWidth(txnReportSheetName, "L", "L", 15) // Đã tiêu thụ TT
	file.SetColWidth(txnReportSheetName, "N", "N", 15) // Còn lại TT
	file.SetColWidth(txnReportSheetName, "R", "R", 15) // Chi tiết tiêu thụ TT

	// Chi tiết tiêu thụ detail columns (O, P)
	file.SetColWidth(txnReportSheetName, "O", "O", 12) // Loại tiêu thụ
	file.SetColWidth(txnReportSheetName, "P", "P", 12) // Ngày tiêu thụ
}

// Style creation methods

func (f *TxnReportFormatter) createTitleStyle(file *excelize.File) (int, error) {
	return file.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   16,
			Family: "Calibri",
		},
		Alignment: &excelize.Alignment{
			Horizontal: "left",
			Vertical:   "center",
		},
	})
}

func (f *TxnReportFormatter) createInventoryNameStyle(file *excelize.File) (int, error) {
	return file.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   14,
			Family: "Calibri",
		},
		Alignment: &excelize.Alignment{
			Horizontal: "left",
			Vertical:   "center",
		},
	})
}

// Header style creators - different colors for different header types
func (f *TxnReportFormatter) createHeaderStyle(file *excelize.File) (int, error) {
	// Default white background for headers like STT, Diễn giải, ĐVT, Đơn giá
	return file.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   14, // Changed to 14px
			Family: "Calibri",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"FFFFFF"}, // White
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
	})
}

func (f *TxnReportFormatter) createHeaderStylePink(file *excelize.File) (int, error) {
	// #FFB6C1 (light pink) for "Đã tiêu thụ" and "Chi tiết tiêu thụ"
	return file.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   14,
			Family: "Calibri",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"FFB6C1"},
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
	})
}

func (f *TxnReportFormatter) createHeaderStyleGreen(file *excelize.File) (int, error) {
	// #90EE90 (light green) for "SL nguồn"
	return file.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   14,
			Family: "Calibri",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"90EE90"},
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
	})
}

func (f *TxnReportFormatter) createHeaderStyleCyan(file *excelize.File) (int, error) {
	// #ADD8E6 (light blue) for "Tồn đầu"
	return file.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   14,
			Family: "Calibri",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"ADD8E6"},
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
	})
}

func (f *TxnReportFormatter) createHeaderStyleGray(file *excelize.File) (int, error) {
	// #D3D3D3 (light gray) for "Còn lại"
	return file.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   14,
			Family: "Calibri",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"D3D3D3"},
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
	})
}

func (f *TxnReportFormatter) createDataStyle(file *excelize.File) (int, error) {
	return file.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Family: "Calibri",
			Size:   14, // Font size 14 for all data cells
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"FFFFFF"}, // White background
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Vertical: "center",
		},
	})
}

// createDataStyleBold creates a bold style for text data cells (Diễn giải column).
func (f *TxnReportFormatter) createDataStyleBold(file *excelize.File) (int, error) {
	return file.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Family: "Calibri",
			Size:   14,   // Font size 14 for all data cells
			Bold:   true, // Bold for Diễn giải column
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"FFFFFF"}, // White background
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Vertical: "center",
		},
	})
}

func (f *TxnReportFormatter) createNumberStyle(file *excelize.File) (int, error) {
	return file.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Family: "Calibri",
			Size:   14, // Font size 14 for all data cells
		},
		CustomNumFmt: stringPtr("#,##0.00"),
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"FFFFFF"}, // White background
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Horizontal: "right",
			Vertical:   "center",
		},
	})
}

// createSLStyle creates a style for SL columns with white background and font size 14.
func (f *TxnReportFormatter) createSLStyle(file *excelize.File) (int, error) {
	return file.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Family: "Calibri",
			Size:   14, // Font size 14 for all data cells
		},
		CustomNumFmt: stringPtr("#,##0.00"),
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"FFFFFF"}, // White background (removed orange)
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Horizontal: "right",
			Vertical:   "center",
		},
	})
}

// Utility functions

// setMergedCellBorders applies borders to all cells in a merged range.
// By default, Excel only applies borders to the first cell when merging,
// but we need borders on all cells for proper display.
func setMergedCellBorders(file *excelize.File, startCell, endCell string, styleID int) error {
	startCol, startRow, err := excelize.CellNameToCoordinates(startCell)
	if err != nil {
		return err
	}
	endCol, endRow, err := excelize.CellNameToCoordinates(endCell)
	if err != nil {
		return err
	}

	// Apply style to all cells in the range
	for row := startRow; row <= endRow; row++ {
		for col := startCol; col <= endCol; col++ {
			cell, _ := excelize.CoordinatesToCellName(col, row)
			if err := file.SetCellStyle(txnReportSheetName, cell, cell, styleID); err != nil {
				return err
			}
		}
	}
	return nil
}

// getDaysInMonth returns the number of days in the month of the given time.
func getDaysInMonth(t *time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
}

// stringPtr returns a pointer to the given string.
func stringPtr(s string) *string {
	return &s
}

// debugSaveFile saves the Excel file to disk for debugging purposes.
// Files are saved in the same directory as the executable binary.
func (f *TxnReportFormatter) debugSaveFile(file *excelize.File, title string) error {
	// Get the directory where the executable is located
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	execDir := filepath.Dir(execPath)

	// Generate filename from title and timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(execDir, fmt.Sprintf("%s-%s.xlsx", sanitizeFilename(title), timestamp))

	// Save file
	if err := file.SaveAs(filename); err != nil {
		return fmt.Errorf("failed to save debug file: %w", err)
	}

	// Log the file path (will appear in console/logs)
	fmt.Printf("\n[DEBUG] Excel file saved to: %s\n\n", filename)

	return nil
}

// sanitizeFilename removes invalid characters from filename.
func sanitizeFilename(name string) string {
	// Replace spaces and special characters with underscores
	result := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result += string(r)
		} else if r == ' ' {
			result += "_"
		}
	}
	if len(result) > 50 {
		result = result[:50]
	}
	return result
}
