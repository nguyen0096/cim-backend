package googlesheets

import (
	"context"
	"fmt"
	"time"

	"cim-backend/internal/models"
	"cim-backend/pkg"

	"google.golang.org/api/sheets/v4"
)

// RevenueExpenseGoogleSheetsRepository handles data access for revenue/expense Google Sheets operations
type RevenueExpenseGoogleSheetsRepository interface {
	InitializeWithSpreadsheet(ctx context.Context, serviceAccountFilePath string, spreadsheetID string, sheetNames ...string) error
	AddExpenses(ctx context.Context, sheetName string, expensesData []map[string]interface{}, cellColors []string) error
	GetLastExpense(ctx context.Context, sheetName string) (map[string]interface{}, error)
	FindLastTransactionRow(rows [][]interface{}) ([]interface{}, int, error)
	GetLastTransactionDate(ctx context.Context, sheetName string) (time.Time, error)
	GetSchema(ctx context.Context) *models.FileMetadata
	GetSheetData(sheetName string) (*models.ExcelSheetMetadata, [][]interface{}, error)
	VerifySpreadsheetAndSheet(ctx context.Context, serviceAccountFilePath string, spreadsheetID string, sheetName string) error
	AddNewDateRow(ctx context.Context, sheetName string, date time.Time) error
	DeleteLastNthRows(ctx context.Context, sheetName string, n int) error
	Close() error
	ForceCacheRefresh()
}

// DeleteLastNthRows deletes the last n rows from the specified sheet
func (r *revenueExpenseGoogleSheetsRepository) DeleteLastNthRows(ctx context.Context, sheetName string, n int) error {
	// Get sheet data
	_, rows, err := r.GetSheetData(sheetName)
	if err != nil {
		return fmt.Errorf("failed to get sheet data: %w", err)
	}

	if len(rows) < n {
		return fmt.Errorf("sheet has less than %d rows, cannot delete last %d rows", n, n)
	}

	// Get the sheet ID
	sheetID, err := r.getSheetID(sheetName)
	if err != nil {
		return fmt.Errorf("failed to get sheet ID: %w", err)
	}

	// Calculate the indices of the last two rows
	lastRowIndex := int64(len(rows))
	startRowIndex := lastRowIndex - int64(n)

	// Create delete request
	requests := []*sheets.Request{
		{
			DeleteDimension: &sheets.DeleteDimensionRequest{
				Range: &sheets.DimensionRange{
					SheetId:    sheetID,
					Dimension:  "ROWS",
					StartIndex: startRowIndex,
					EndIndex:   lastRowIndex,
				},
			},
		},
	}

	batchUpdateRequest := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: requests,
	}

	_, err = r.sheetsService.Spreadsheets.BatchUpdate(r.spreadsheetID, batchUpdateRequest).Do()
	if err != nil {
		return fmt.Errorf("failed to delete last %d rows: %w", n, err)
	}

	// Invalidate cache after deleting to ensure next read gets fresh data
	r.ForceCacheRefresh()

	return nil
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
		return fmt.Errorf("failed to get sheet data: %w", err)
	}

	// Find header row
	// headerRow := r.FindHeaderRow(rows)
	// if headerRow < 0 || headerRow >= len(rows) {
	// 	return fmt.Errorf("no header row found")
	// }

	// Prepare date and row information
	_, lastRowIndex, err := r.FindLastTransactionRow(rows)
	if err != nil {
		return fmt.Errorf("failed to find last transaction row: %w", err)
	}
	targetRow := lastRowIndex + 1

	// Add transaction date row if needed
	// today := pkg.GetTodayDate()
	// isTodayExists, detectedDateFormat := FindLastTransactionDateInfo(rows, headerRow, today)
	// if !isTodayExists {
	// 	if err := r.AddTransactionDateRow(sheetName, targetRow, today, detectedDateFormat); err != nil {
	// 		return fmt.Errorf("failed to add transaction date row: %w", err)
	// 	}
	// 	targetRow++
	// 	// After adding a new date row, ordinal number starts from 1
	// 	ordinalNumber = 1
	// } else {
	// 	// find the ordinal number in the last row
	// 	lastRow, _, err := r.FindLastTransactionRow(rows)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to find last transaction row: %w", err)
	// 	}

	// 	// Check if lastRow has enough columns to access the ordinal number (STT column at index 1)
	// 	if len(lastRow) < 2 {
	// 		// If no ordinal number found, start from 1
	// 		ordinalNumber = 1
	// 	} else {
	// 		// convert the ordinal number to int
	// 		ordinalNumberStr := getCellValueAsString(lastRow[1])
	// 		if ordinalNumberStr == "" {
	// 			// If ordinal number is empty, start from 1
	// 			ordinalNumber = 1
	// 		} else {
	// 			ordinalNumber, err = strconv.Atoi(ordinalNumberStr)
	// 			if err != nil {
	// 				// If conversion fails, start from 1
	// 				ordinalNumber = 1
	// 			} else {
	// 				ordinalNumber++
	// 			}
	// 		}
	// 	  }
	// }

	// Add all expense data rows
	for i, expenseData := range expensesData {
		if _, found := expenseData[pkg.RevenueExpenseColumnOrdinalNumber]; !found {
			expenseData[pkg.RevenueExpenseColumnOrdinalNumber] = 1
		}
		if err := r.AddDataRowWithColor(sheetName, targetRow, expenseData, cellColors[i]); err != nil {
			return fmt.Errorf("failed to add expense data row at index %d: %w", i, err)
		}
		targetRow++
	}

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
	lastDataRow, lastRowIndex, err := r.FindLastTransactionRow(rows)
	if err != nil {
		return nil, err
	}

	// Build the expense data map using the headers
	expenseData := make(map[string]interface{})
	expenseData["@last_transaction_row_index"] = lastRowIndex // for testing only

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

// AddNewDateRow adds a new row with the specified date to the Google Sheets
func (r *revenueExpenseGoogleSheetsRepository) AddNewDateRow(ctx context.Context, sheetName string, date time.Time) error {
	// Get sheet data
	_, rows, err := r.GetSheetData(sheetName)
	if err != nil {
		return fmt.Errorf("failed to get sheet data: %w", err)
	}

	// Find header row
	headerRow := r.FindHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return fmt.Errorf("no header row found")
	}

	// Get the date format from the last date row
	_, detectedDateFormat := FindLastTransactionDateInfo(rows, headerRow, date)
	_, lastRowIndex, err := r.FindLastTransactionRow(rows)
	if err != nil {
		return fmt.Errorf("failed to find last transaction row: %w", err)
	}

	// Calculate the target row (append at the end)
	targetRow := lastRowIndex + 1

	// Add the date row
	if err := r.AddTransactionDateRow(sheetName, targetRow, date, detectedDateFormat); err != nil {
		return fmt.Errorf("failed to add transaction date row: %w", err)
	}

	return nil
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
		// Get available sheet names for error message
		availableSheets := make([]string, len(schema.Sheets))
		for i, sheet := range schema.Sheets {
			availableSheets[i] = sheet.SheetName
		}
		return fmt.Errorf("sheet %s not found in spreadsheet. available sheets: %v", sheetName, availableSheets)
	}

	return nil
}

// ForceCacheRefresh forces a cache refresh on the next access
func (r *revenueExpenseGoogleSheetsRepository) ForceCacheRefresh() {
	r.BaseGoogleSheetsRepository.ForceCacheRefresh()
}
