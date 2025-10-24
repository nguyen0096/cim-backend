package googlesheets

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"cim-backend/internal/models"
	"cim-backend/pkg"
)

// RevenueExpenseGoogleSheetsRepository handles data access for revenue/expense Google Sheets operations
type RevenueExpenseGoogleSheetsRepository interface {
	InitializeWithSpreadsheet(ctx context.Context, serviceAccountFilePath string, spreadsheetID string, sheetNames ...string) error
	AddExpenses(ctx context.Context, sheetName string, expensesData []map[string]interface{}, cellColors []string) error
	GetLastExpense(ctx context.Context, sheetName string) (map[string]interface{}, error)
	GetLastTransactionDate(ctx context.Context, sheetName string) (time.Time, error)
	GetSchema(ctx context.Context) *models.FileMetadata
	VerifySpreadsheetAndSheet(ctx context.Context, serviceAccountFilePath string, spreadsheetID string, sheetName string) error
	Close() error
	ForceCacheRefresh()
}

// revenueExpenseGoogleSheetsRepository implements RevenueExpenseGoogleSheetsRepository
type revenueExpenseGoogleSheetsRepository struct {
	BaseGoogleSheetsRepository
}

// NewRevenueExpenseGoogleSheetsRepository creates a new RevenueExpenseGoogleSheetsRepository
func NewRevenueExpenseGoogleSheetsRepository() RevenueExpenseGoogleSheetsRepository {
	return &revenueExpenseGoogleSheetsRepository{}
}

// InitializeWithSpreadsheet initializes the repository with the expense/income Google Sheets using API key
// If sheetNames are provided, only those sheets will be processed
func (r *revenueExpenseGoogleSheetsRepository) InitializeWithSpreadsheet(ctx context.Context, serviceAccountFilePath string, spreadsheetID string, sheetNames ...string) error {
	return r.BaseGoogleSheetsRepository.InitializeWithSpreadsheet(ctx, serviceAccountFilePath, spreadsheetID, models.FileTypeRevenueExpense, sheetNames...)
}

// AddExpenses adds multiple expense entries to the Google Sheets
// This method is efficient as it adds all expenses in batches
func (r *revenueExpenseGoogleSheetsRepository) AddExpenses(ctx context.Context, sheetName string, expensesData []map[string]interface{}, cellColors []string) error {
	if len(expensesData) == 0 {
		return fmt.Errorf("no expenses data provided")
	}

	// Validate all expense data
	for i, expenseData := range expensesData {
		if err := ValidateData(expenseData); err != nil {
			return fmt.Errorf("invalid expense data at index %d: %w", i, err)
		}
	}

	// Get sheet data
	_, rows, err := r.GetSheetData(sheetName)
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
		if err := r.AddTransactionDateRow(sheetName, targetRow, today, detectedDateFormat); err != nil {
			return fmt.Errorf("failed to add transaction date row: %w", err)
		}
		targetRow++
	} else {
		// find the ordinal number in the last row
		lastRow, _, err := r.FindLastTransactionRow(rows)
		if err != nil {
			return fmt.Errorf("failed to find last transaction row: %w", err)
		}
		// convert the ordinal number to int
		ordinalNumberStr := getCellValueAsString(lastRow[1])
		ordinalNumber, err = strconv.Atoi(ordinalNumberStr)
		if err != nil {
			return fmt.Errorf("failed to convert ordinal number to int: %w", err)
		}
		ordinalNumber++
	}

	// Add all expense data rows
	for i, expenseData := range expensesData {
		expenseData[pkg.RevenueExpenseColumnOrdinalNumber] = ordinalNumber
		ordinalNumber++
		if err := r.AddDataRowWithColor(sheetName, targetRow, expenseData, cellColors[i]); err != nil {
			return fmt.Errorf("failed to add expense data row at index %d: %w", i, err)
		}
		targetRow++
	}

	// Invalidate cache after saving to ensure next read gets fresh data
	r.ForceCacheRefresh()

	return nil
}

// GetLastExpense retrieves the most recent expense entry from the Google Sheets
func (r *revenueExpenseGoogleSheetsRepository) GetLastExpense(ctx context.Context, sheetName string) (map[string]interface{}, error) {
	// Get sheet data
	_, rows, err := r.GetSheetData(sheetName)
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
			cellValue := getCellValueAsString(lastDataRow[header.ColumnIndex])
			if cellValue != "" {
				expenseData[header.ColumnName] = cellValue
			}
		}
	}

	return expenseData, nil
}

// GetLastTransactionDate retrieves the date of the most recent transaction from the Google Sheets
func (r *revenueExpenseGoogleSheetsRepository) GetLastTransactionDate(ctx context.Context, sheetName string) (time.Time, error) {
	return r.BaseGoogleSheetsRepository.GetLastTransactionDate(ctx, sheetName)
}

// GetSchema returns the Google Sheets schema
func (r *revenueExpenseGoogleSheetsRepository) GetSchema(ctx context.Context) *models.FileMetadata {
	return r.BaseGoogleSheetsRepository.GetSchema(ctx)
}

// Close closes the repository and releases any cached resources
func (r *revenueExpenseGoogleSheetsRepository) Close() error {
	return r.BaseGoogleSheetsRepository.Close()
}

// VerifySpreadsheetAndSheet verifies that the spreadsheet ID and sheet name exist
func (r *revenueExpenseGoogleSheetsRepository) VerifySpreadsheetAndSheet(ctx context.Context, serviceAccountFilePath string, spreadsheetID string, sheetName string) error {
	// Initialize with the spreadsheet to check if it's valid
	if err := r.InitializeWithSpreadsheet(ctx, serviceAccountFilePath, spreadsheetID); err != nil {
		return fmt.Errorf("failed to initialize with spreadsheet: %w", err)
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
		return fmt.Errorf("sheet %s not found in spreadsheet", sheetName)
	}

	return nil
}

// ForceCacheRefresh forces a cache refresh on the next access
func (r *revenueExpenseGoogleSheetsRepository) ForceCacheRefresh() {
	r.BaseGoogleSheetsRepository.ForceCacheRefresh()
}
