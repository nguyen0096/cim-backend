package excel

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/pkg/log"

	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

// RevenueExpenseExcelRepository handles data access for revenue/expense Excel operations
type RevenueExpenseExcelRepository interface {
	InitializeWithFile(ctx context.Context, filePath string, sheetNames ...string) error
	AddExpenses(ctx context.Context, sheetName string, expensesData []map[string]interface{}, cellColors []string) error
	FindLastTransactionRow(rows [][]string) ([]string, int, error)                        // For TESTING ONLY
	GetLastExpense(ctx context.Context, sheetName string) (map[string]interface{}, error) // For TESTING ONLY
	MapRowToExpense(sheetName string, row []string) (map[string]interface{}, error)       // For TESTING ONLY
	GetLastTransactionDate(ctx context.Context, sheetName string) (time.Time, error)
	GetSchema(ctx context.Context) *models.FileMetadata
	VerifyFileAndSheet(ctx context.Context, filePath string, sheetName string) error
	AddNewDateRow(ctx context.Context, sheetName string, date time.Time) error
	Close() error
	ForceCacheRefresh()

	// FOR TESTING ONLY
	GetLastTransactionCellStyle(ctx context.Context, sheetName string) ([]int, error)
	GetFileAndSheetData(sheetName string) (*excelize.File, *models.ExcelSheetMetadata, [][]string, error)
}

// revenueExpenseExcelRepository implements RevenueExpenseExcelRepository
type revenueExpenseExcelRepository struct {
	BaseExcelRepository
}

// NewRevenueExpenseExcelRepository creates a new RevenueExpenseExcelRepository
func NewRevenueExpenseExcelRepository() RevenueExpenseExcelRepository {
	return &revenueExpenseExcelRepository{}
}

// InitializeWithFile initializes the repository with the expense/income Excel file
// If sheetNames are provided, only those sheets will be processed
func (r *revenueExpenseExcelRepository) InitializeWithFile(ctx context.Context, filePath string, sheetNames ...string) error {
	return r.BaseExcelRepository.InitializeWithFile(ctx, filePath, models.FileTypeRevenueExpense, sheetNames...)
}

// AddExpenses adds multiple expense entries to the Excel file
// This method is efficient as it only opens the file once, adds all expenses, and saves once
func (r *revenueExpenseExcelRepository) AddExpenses(ctx context.Context, sheetName string, expensesData []map[string]interface{}, cellColors []string) error {
	if len(expensesData) == 0 {
		return fmt.Errorf("no expenses data provided")
	}

	traceFields := func(step string, extra logrus.Fields) {
		fields := logrus.Fields{
			"operation":    "AddExpenses",
			"sheetName":    sheetName,
			"expense_rows": len(expensesData),
			"step":         step,
		}
		for k, v := range extra {
			fields[k] = v
		}
		log.WithContext(ctx).WithFields(fields).Info("excel revenue expense tracing")
	}

	// Validate all expense data
	validateStart := time.Now()
	for i, expenseData := range expensesData {
		if err := ValidateData(expenseData); err != nil {
			return fmt.Errorf("invalid expense data at index %d: %w", i, err)
		}
	}
	traceFields("validateData", logrus.Fields{"elapsed_ms": time.Since(validateStart).Milliseconds()})

	// Get file and sheet data
	loadStart := time.Now()
	file, _, rows, err := r.GetFileAndSheetData(sheetName)
	if err != nil {
		return err
	}
	traceFields("getFileAndSheetData", logrus.Fields{"elapsed_ms": time.Since(loadStart).Milliseconds(), "row_count": len(rows)})

	// Find header row
	headerStart := time.Now()
	headerRow := r.FindHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return fmt.Errorf("no header row found")
	}
	traceFields("findHeaderRow", logrus.Fields{"elapsed_ms": time.Since(headerStart).Milliseconds(), "header_row_index": headerRow})

	// Prepare date and row information
	var targetRow int
	var ordinalNumber int

	// Add transaction date row if needed
	// today := pkg.GetTodayDate()
	// isTodayExists, detectedDateFormat := FindLastTransactionDateInfo(rows, headerRow, today)
	// if !isTodayExists {
	// 	if err := r.AddTransactionDateRow(file, sheetName, targetRow, today, detectedDateFormat); err != nil {
	// 		return fmt.Errorf("failed to add transaction date row: %w", err)
	// 	}
	// 	targetRow++
	// } else {
	// 	// find the ordinal number in the last row
	// 	lastRow, _, err := r.FindLastTransactionRow(rows)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to find last transaction row: %w", err)
	// 	}
	// 	// convert the ordinal number to int
	// 	ordinalNumber, err = strconv.Atoi(lastRow[1])
	// 	if err != nil {
	// 		return fmt.Errorf("failed to convert ordinal number to int: %w", err)
	// 	}
	// 	ordinalNumber++
	// }

	lastRowStart := time.Now()
	lastRow, lastRowIndex, err := r.FindLastTransactionRow(rows)
	if err != nil {
		return fmt.Errorf("failed to find last transaction row: %w", err)
	}
	traceFields("findLastTransactionRow", logrus.Fields{
		"elapsed_ms":     time.Since(lastRowStart).Milliseconds(),
		"last_row_index": lastRowIndex,
	})

	// Next data row should be directly after the last transaction row (e.g. newly added date row)
	targetRow = lastRowIndex + 1

	// Check if lastRow has enough columns to access the ordinal number (STT column at index 1)
	if len(lastRow) < 2 {
		// If no ordinal number found, start from 1
		ordinalNumber = 1
	} else {
		ordinalNumber, err = strconv.Atoi(lastRow[1])
		if err != nil {
			ordinalNumber = 1
		} else {
			ordinalNumber++
		}
	}

	// Add all expense data rows
	addRowsStart := time.Now()
	for i, expenseData := range expensesData {
		ordinalNumber++
		if err := r.AddDataRowWithColor(file, sheetName, targetRow, expenseData, cellColors[i]); err != nil {
			return fmt.Errorf("failed to add expense data row at index %d: %w", i, err)
		}
		targetRow++
	}
	traceFields("addDataRows", logrus.Fields{
		"elapsed_ms": time.Since(addRowsStart).Milliseconds(),
		"start_row":  targetRow - len(expensesData),
	})

	// Save the file
	saveStart := time.Now()
	if err := file.Save(); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}
	traceFields("saveFile", logrus.Fields{"elapsed_ms": time.Since(saveStart).Milliseconds()})

	// Invalidate cache after saving to ensure next read gets fresh data
	r.ForceCacheRefresh()
	traceFields("forceCacheRefresh", logrus.Fields{})

	return nil
}

// GetLastExpense retrieves the most recent expense entry from the Excel file
func (r *revenueExpenseExcelRepository) GetLastExpense(ctx context.Context, sheetName string) (map[string]interface{}, error) {
	// Get file and sheet data
	_, _, rows, err := r.GetFileAndSheetData(sheetName)
	if err != nil {
		return nil, err
	}

	// Find the last data row
	lastDataRow, _, err := r.FindLastTransactionRow(rows)
	if err != nil {
		return nil, err
	}

	// Build the expense data map using the headers
	expenseData := make(map[string]interface{})

	// Find the sheet metadata for the specified sheet
	sheetMetadata := r.findSheetMetadata(sheetName)

	if sheetMetadata == nil {
		return nil, fmt.Errorf("sheet %s not found in metadata", sheetName)
	}

	for _, header := range sheetMetadata.Headers {
		if header.ColumnIndex < len(lastDataRow) {
			cellValue := strings.TrimSpace(lastDataRow[header.ColumnIndex])
			if cellValue != "" {
				expenseData[header.ColumnName] = cellValue
			}
		}
	}

	return expenseData, nil
}

// MapRowToExpense maps a row to an expense data map
func (r *revenueExpenseExcelRepository) MapRowToExpense(sheetName string, row []string) (map[string]interface{}, error) {
	expenseData := make(map[string]interface{})
	sheetMetadata := r.findSheetMetadata(sheetName)
	if sheetMetadata == nil {
		return nil, fmt.Errorf("sheet %s not found in metadata", sheetName)
	}

	if len(row) == 0 {
		return nil, fmt.Errorf("row is empty")
	}

	for _, header := range sheetMetadata.Headers {
		if header.ColumnIndex < len(row) {
			cellValue := strings.TrimSpace(row[header.ColumnIndex])
			if cellValue != "" {
				expenseData[header.ColumnName] = cellValue
			}
		}
	}

	return expenseData, nil
}

// TEST ONLY: get last transaction cell style
func (r *revenueExpenseExcelRepository) GetLastTransactionCellStyle(ctx context.Context, sheetName string) ([]int, error) {
	file, sheetMetadata, rows, err := r.GetFileAndSheetData(sheetName)
	if err != nil {
		return []int{}, err
	}

	// Find the last data row
	_, lastRowIndex, err := r.FindLastTransactionRow(rows)
	if err != nil {
		return []int{}, err
	}

	var styles []int
	for _, column := range sheetMetadata.Headers {
		if column.ColumnName != pkg.RevenueExpenseColumnOrdinalNumber &&
			column.ColumnName != pkg.RevenueExpenseColumnWater &&
			column.ColumnName != pkg.RevenueExpenseColumnSnackAndRice &&
			column.ColumnName != pkg.RevenueExpenseColumnName {
			continue
		}
		cellName, _ := excelize.CoordinatesToCellName(column.ColumnIndex+1, lastRowIndex)
		fmt.Printf("cellName: %s\n", cellName)
		style, err := file.GetCellStyle(sheetName, cellName)
		if err != nil {
			return []int{}, err
		}
		styles = append(styles, style)
	}

	return styles, nil
}

// FOR TESTING ONLY
func (r *revenueExpenseExcelRepository) GetFileAndSheetData(sheetName string) (*excelize.File, *models.ExcelSheetMetadata, [][]string, error) {
	return r.BaseExcelRepository.GetFileAndSheetData(sheetName)
}

// GetLastTransactionDate retrieves the date of the most recent transaction from the Excel file
func (r *revenueExpenseExcelRepository) GetLastTransactionDate(ctx context.Context, sheetName string) (time.Time, error) {
	return r.BaseExcelRepository.GetLastTransactionDate(ctx, sheetName)
}

// AddNewDateRow adds a new row with the specified date to the Excel file
func (r *revenueExpenseExcelRepository) AddNewDateRow(ctx context.Context, sheetName string, date time.Time) error {
	traceFields := func(step string, extra logrus.Fields) {
		fields := logrus.Fields{
			"operation": "AddNewDateRow",
			"sheetName": sheetName,
			"step":      step,
		}
		for k, v := range extra {
			fields[k] = v
		}
		log.WithContext(ctx).WithFields(fields).Info("excel revenue expense tracing")
	}

	// Get file and sheet data
	loadStart := time.Now()
	file, _, rows, err := r.GetFileAndSheetData(sheetName)
	if err != nil {
		return fmt.Errorf("failed to get file and sheet data: %w", err)
	}
	traceFields("getFileAndSheetData", logrus.Fields{"elapsed_ms": time.Since(loadStart).Milliseconds(), "row_count": len(rows)})

	// Find header row
	headerStart := time.Now()
	headerRow := r.FindHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return fmt.Errorf("no header row found")
	}
	traceFields("findHeaderRow", logrus.Fields{"elapsed_ms": time.Since(headerStart).Milliseconds(), "header_row_index": headerRow})

	// Get the date format from the last date row
	lastInfoStart := time.Now()
	_, detectedDateFormat := FindLastTransactionDateInfo(rows, headerRow, date)
	_, lastRowIndex, err := r.FindLastTransactionRow(rows)
	if err != nil {
		return fmt.Errorf("failed to find last transaction row: %w", err)
	}
	traceFields("determineLastRow", logrus.Fields{"elapsed_ms": time.Since(lastInfoStart).Milliseconds(), "last_row_index": lastRowIndex, "date_format": detectedDateFormat})

	// Calculate the target row (append at the end)
	targetRow := lastRowIndex + 1

	// Add the date row
	addRowStart := time.Now()
	if err := r.AddTransactionDateRow(file, sheetName, targetRow, date, detectedDateFormat); err != nil {
		return fmt.Errorf("failed to add transaction date row: %w", err)
	}
	traceFields("addTransactionDateRow", logrus.Fields{"elapsed_ms": time.Since(addRowStart).Milliseconds(), "target_row": targetRow})

	// Save the file
	saveStart := time.Now()
	if err := file.Save(); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}
	traceFields("saveFile", logrus.Fields{"elapsed_ms": time.Since(saveStart).Milliseconds()})

	// Invalidate cache after saving to ensure next read gets fresh data
	r.ForceCacheRefresh()
	traceFields("forceCacheRefresh", logrus.Fields{})

	return nil
}

// GetSchema returns the Excel file schema
func (r *revenueExpenseExcelRepository) GetSchema(ctx context.Context) *models.FileMetadata {
	return r.BaseExcelRepository.GetSchema(ctx)
}

// Close closes the repository and releases any cached resources
func (r *revenueExpenseExcelRepository) Close() error {
	return r.BaseExcelRepository.Close()
}

// VerifyFileAndSheet verifies that the filepath and sheetname exist
func (r *revenueExpenseExcelRepository) VerifyFileAndSheet(ctx context.Context, filePath string, sheetName string) error {
	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("file does not exist: %w", err)
	}

	// Initialize with the file to check if it's a valid Excel file
	if err := r.InitializeWithFile(ctx, filePath); err != nil {
		return fmt.Errorf("failed to initialize with file: %w", err)
	}

	// Check if sheet exists by getting the schema and checking if the sheet is in the metadata
	schema := r.GetSchema(ctx)
	if schema == nil {
		return fmt.Errorf("no schema available")
	}

	// Find the sheet in the metadata
	var sheetExists bool
	for _, sheet := range schema.Sheets {
		if sheet.SheetName == sheetName {
			sheetExists = true
			break
		}
	}

	if !sheetExists {
		return fmt.Errorf("sheet %s not found in file", sheetName)
	}

	return nil
}

// ForceCacheRefresh forces a cache refresh on the next file access
func (r *revenueExpenseExcelRepository) ForceCacheRefresh() {
	r.BaseExcelRepository.ForceCacheRefresh()
}
