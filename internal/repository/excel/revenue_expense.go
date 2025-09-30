package excel

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"import-export-backend/internal/models"
	"import-export-backend/pkg"
)

// RevenueExpenseExcelRepository handles data access for revenue/expense Excel operations
type RevenueExpenseExcelRepository interface {
	InitializeWithFile(ctx context.Context, filePath string, sheetNames ...string) error
	AddExpenses(ctx context.Context, sheetName string, expensesData []map[string]interface{}) error
	GetLastExpense(ctx context.Context, sheetName string) (map[string]interface{}, error)
	GetLastTransactionDate(ctx context.Context, sheetName string) (time.Time, error)
	GetSchema(ctx context.Context) *models.FileMetadata
	VerifyFileAndSheet(ctx context.Context, filePath string, sheetName string) error
	Close() error
	ForceCacheRefresh()
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
func (r *revenueExpenseExcelRepository) AddExpenses(ctx context.Context, sheetName string, expensesData []map[string]interface{}) error {
	if len(expensesData) == 0 {
		return fmt.Errorf("no expenses data provided")
	}

	// Validate all expense data
	for i, expenseData := range expensesData {
		if err := ValidateData(expenseData); err != nil {
			return fmt.Errorf("invalid expense data at index %d: %w", i, err)
		}
	}

	// Get file and sheet data
	file, _, rows, err := r.GetFileAndSheetData(sheetName)
	if err != nil {
		return err
	}

	// Find header row
	headerRow := r.FindHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return fmt.Errorf("no header row found")
	}

	// Prepare date and row information
	today := pkg.GetTodayDate()
	isTodayExists, detectedDateFormat := FindLastTransactionDateInfo(rows, headerRow, today)
	targetRow := len(rows) + 1

	ordinalNumber := 1

	// Add transaction date row if needed
	if !isTodayExists {
		if err := r.AddTransactionDateRow(file, sheetName, targetRow, today, detectedDateFormat); err != nil {
			return fmt.Errorf("failed to add transaction date row: %w", err)
		}
		targetRow++
	} else {
		// find the ordinal number in the last row
		lastRow, err := r.FindLastTransactionRow(rows)
		if err != nil {
			return fmt.Errorf("failed to find last transaction row: %w", err)
		}
		// convert the ordinal number to int
		ordinalNumber, err = strconv.Atoi(lastRow[1])
		if err != nil {
			return fmt.Errorf("failed to convert ordinal number to int: %w", err)
		}
		ordinalNumber++
	}

	// Add all expense data rows
	for i, expenseData := range expensesData {
		expenseData["STT"] = ordinalNumber
		ordinalNumber++
		if err := r.AddDataRow(file, sheetName, targetRow, expenseData); err != nil {
			return fmt.Errorf("failed to add expense data row at index %d: %w", i, err)
		}
		targetRow++
	}

	// Save the file
	if err := file.Save(); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	// Invalidate cache after saving to ensure next read gets fresh data
	r.ForceCacheRefresh()

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
	lastDataRow, err := r.FindLastTransactionRow(rows)
	if err != nil {
		return nil, err
	}

	// Build the expense data map using the headers
	expenseData := make(map[string]interface{})

	// Get the schema to access sheet metadata
	schema := r.GetSchema(ctx)
	if schema == nil {
		return nil, fmt.Errorf("no schema available")
	}

	// Find the sheet metadata for the specified sheet
	var sheetMetadata *models.ExcelSheetMetadata
	for _, sheet := range schema.Sheets {
		if sheet.SheetName == sheetName {
			sheetMetadata = &sheet
			break
		}
	}

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

// GetLastTransactionDate retrieves the date of the most recent transaction from the Excel file
func (r *revenueExpenseExcelRepository) GetLastTransactionDate(ctx context.Context, sheetName string) (time.Time, error) {
	return r.BaseExcelRepository.GetLastTransactionDate(ctx, sheetName)
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
