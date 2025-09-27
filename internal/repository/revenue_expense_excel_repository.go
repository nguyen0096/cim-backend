package repository

import (
	"context"
	"fmt"
	"import-export-backend/internal/models"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// RevenueExpenseExcelRepository handles data access for revenue/expense Excel operations
type RevenueExpenseExcelRepository interface {
	InitializeWithFile(ctx context.Context, filePath string) error
	ReadAllExpenses(ctx context.Context) ([]map[string]interface{}, error)
	AddExpense(ctx context.Context, expenseData map[string]interface{}) error
	AddMultipleExpenses(ctx context.Context, expenses []map[string]interface{}) error
	SearchExpenses(ctx context.Context, criteria map[string]interface{}) ([]map[string]interface{}, error)
	GetSchema(ctx context.Context) *models.FileMetadata
}

// revenueExpenseExcelRepository implements RevenueExpenseExcelRepository
type revenueExpenseExcelRepository struct {
	ctx          context.Context
	fileMetadata *models.FileMetadata
}

// NewRevenueExpenseExcelRepository creates a new RevenueExpenseExcelRepository
func NewRevenueExpenseExcelRepository() RevenueExpenseExcelRepository {
	return &revenueExpenseExcelRepository{}
}

// InitializeWithFile initializes the repository with the expense/income Excel file
func (r *revenueExpenseExcelRepository) InitializeWithFile(ctx context.Context, filePath string) error {
	r.ctx = ctx

	// Read the Excel file to extract actual metadata
	file, err := excelize.OpenFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	// Get sheet names
	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return fmt.Errorf("no sheets found in file")
	}

	// Process each sheet
	var sheetMetadataList []models.ExcelSheetMetadata
	for _, sheetName := range sheets {
		sheetMetadata, err := r.extractSheetMetadata(file, sheetName)
		if err != nil {
			return fmt.Errorf("failed to extract metadata for sheet %s: %w", sheetName, err)
		}
		sheetMetadataList = append(sheetMetadataList, *sheetMetadata)
	}

	now := time.Now()
	r.fileMetadata = &models.FileMetadata{
		FileType:  models.FileTypeRevenueExpense,
		FilePath:  filePath,
		Version:   "1.0",
		Sheets:    sheetMetadataList,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return nil
}

// ReadAllExpenses reads all expense entries from the Excel file
func (r *revenueExpenseExcelRepository) ReadAllExpenses(ctx context.Context) ([]map[string]interface{}, error) {
	if r.fileMetadata == nil {
		return nil, fmt.Errorf("repository not initialized, call InitializeWithFile first")
	}

	file, err := excelize.OpenFile(r.fileMetadata.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Use the first sheet from metadata
	if len(r.fileMetadata.Sheets) == 0 {
		return nil, fmt.Errorf("no sheets found in metadata")
	}

	sheetName := r.fileMetadata.Sheets[0].SheetName
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows from sheet %s: %w", sheetName, err)
	}

	if len(rows) == 0 {
		return []map[string]interface{}{}, nil
	}

	// Find header row
	headerRow := r.findHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return nil, fmt.Errorf("no header row found")
	}

	// Extract headers
	headers := rows[headerRow]
	var result []map[string]interface{}

	// Process data rows
	for i := headerRow + 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}

		rowData := make(map[string]interface{})
		for j, cellValue := range row {
			if j < len(headers) && headers[j] != "" {
				rowData[headers[j]] = cellValue
			}
		}
		result = append(result, rowData)
	}

	return result, nil
}

// AddExpense adds a new expense entry to the Excel file
func (r *revenueExpenseExcelRepository) AddExpense(ctx context.Context, expenseData map[string]interface{}) error {
	if r.fileMetadata == nil {
		return fmt.Errorf("repository not initialized, call InitializeWithFile first")
	}

	file, err := excelize.OpenFile(r.fileMetadata.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Use the first sheet from metadata
	if len(r.fileMetadata.Sheets) == 0 {
		return fmt.Errorf("no sheets found in metadata")
	}

	sheetName := r.fileMetadata.Sheets[0].SheetName

	// Get current rows
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("failed to get rows: %w", err)
	}

	if len(rows) == 0 {
		return fmt.Errorf("no data found in sheet")
	}

	// Find header row
	headerRow := r.findHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return fmt.Errorf("no header row found")
	}

	// Get today's date in various formats that might be used in Excel, with time set to zero
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	availableDateFormats := []string{
		"1/02/2006",
		"01/02/2006",
		"02/1/2006",
		"02/01/2006",
		"1-02-2006",
		"01-02-2006",
		"02-01-2006",
		"02-1-2006",
		"2006-1-02",
		"2006-01-02",
		"2006/1/02",
		"2006/01/02",
	}

	// Check if a row with today's date already exists
	isNewTransactionDateRequired := false

	// Helper function to determine if a date string matches today's date in any supported format
	isToday := func(dateStr string) bool {
		for _, dateFormat := range availableDateFormats {
			// try parse datestr
			date, err := time.Parse(dateFormat, dateStr)
			if err != nil {
				continue
			}

			if date.Format(dateFormat) == today.Format(dateFormat) {
				return true
			}
		}
		return false
	}

	// Scan rows from bottom up, starting after the header, to find today's date
	for i := len(rows) - 1; i >= headerRow+1; i-- {
		if len(rows[i]) == 0 {
			continue
		}

		dateValue := strings.TrimSpace(rows[i][0])
		if dateValue == "" {
			continue
		}

		isNewTransactionDateRequired = !isToday(dateValue)

		break
	}

	targetRow := len(rows) + 1

	// If no row with today's date exists, create a new row
	if isNewTransactionDateRequired {
		// First, add today's date to the first column (use standard format)
		if len(r.fileMetadata.Sheets[0].Headers) > 0 {
			firstColumn := r.fileMetadata.Sheets[0].Headers[0]
			cellName, _ := excelize.CoordinatesToCellName(firstColumn.ColumnIndex, targetRow)
			file.SetCellValue(sheetName, cellName, today)
		}
		targetRow++
	}

	// Add the expense data to the target row
	for _, column := range r.fileMetadata.Sheets[0].Headers {
		if value, exists := expenseData[column.ColumnName]; exists {
			cellName, _ := excelize.CoordinatesToCellName(column.ColumnIndex+1, targetRow)
			file.SetCellValue(sheetName, cellName, value)
		}
	}

	// Save the file
	err = file.Save()
	if err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

// AddMultipleExpenses adds multiple expense entries at once
func (r *revenueExpenseExcelRepository) AddMultipleExpenses(ctx context.Context, expenses []map[string]interface{}) error {
	if r.fileMetadata == nil {
		return fmt.Errorf("repository not initialized, call InitializeWithFile first")
	}

	file, err := excelize.OpenFile(r.fileMetadata.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Use the first sheet from metadata
	if len(r.fileMetadata.Sheets) == 0 {
		return fmt.Errorf("no sheets found in metadata")
	}

	sheetName := r.fileMetadata.Sheets[0].SheetName

	// Get current rows
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("failed to get rows: %w", err)
	}

	if len(rows) == 0 {
		return fmt.Errorf("no data found in sheet")
	}

	// Find header row
	headerRow := r.findHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return fmt.Errorf("no header row found")
	}

	// Get today's date in various formats that might be used in Excel
	today := time.Now()
	todayFormats := []string{
		today.Format("01/02/2006"),
		today.Format("01-02-2006"),
		today.Format("2006-01-02"),
		today.Format("2006/01/02"),
		today.Format("02-01-2006"),
		today.Format("02/01/2006"),
		today.Format("2006-01-02 15:04:05"),
		today.Format("02/01/2006 15:04"),
	}

	// Find if there's already a row with today's date
	targetRow := -1
	for i := headerRow + 1; i < len(rows); i++ {
		if len(rows[i]) > 0 {
			// Check the first column (date column) for today's date
			dateValue := strings.TrimSpace(rows[i][0])
			for _, todayFormat := range todayFormats {
				if dateValue == todayFormat {
					targetRow = i + 1 // Excel rows are 1-indexed
					break
				}
			}
			if targetRow != -1 {
				break
			}
		}
	}

	// If no row with today's date exists, create a new row
	if targetRow == -1 {
		targetRow = len(rows) + 1

		// First, add today's date to the first column (use standard format)
		if len(r.fileMetadata.Sheets[0].Headers) > 0 {
			firstColumn := r.fileMetadata.Sheets[0].Headers[0]
			cellName, _ := excelize.CoordinatesToCellName(firstColumn.ColumnIndex+1, targetRow)
			file.SetCellValue(sheetName, cellName, today.Format("2006-01-02"))
		}
	}

	// Add all expenses to the target row (or create additional rows if needed)
	currentRow := targetRow
	for i, expense := range expenses {
		// For multiple expenses, if we're not on the first expense, we might need new rows
		if i > 0 {
			currentRow = len(rows) + 1 + i
			// Add today's date to the new row (use standard format)
			if len(r.fileMetadata.Sheets[0].Headers) > 0 {
				firstColumn := r.fileMetadata.Sheets[0].Headers[0]
				cellName, _ := excelize.CoordinatesToCellName(firstColumn.ColumnIndex+1, currentRow)
				file.SetCellValue(sheetName, cellName, today.Format("2006-01-02"))
			}
		}

		// Add the expense data to the current row
		for _, column := range r.fileMetadata.Sheets[0].Headers {
			if value, exists := expense[column.ColumnName]; exists {
				cellName, _ := excelize.CoordinatesToCellName(column.ColumnIndex+1, currentRow)
				file.SetCellValue(sheetName, cellName, value)
			}
		}
	}

	// Save the file
	err = file.Save()
	if err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

// SearchExpenses returns expenses matching search criteria
func (r *revenueExpenseExcelRepository) SearchExpenses(ctx context.Context, criteria map[string]interface{}) ([]map[string]interface{}, error) {
	allExpenses, err := r.ReadAllExpenses(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read expenses: %w", err)
	}

	var filteredExpenses []map[string]interface{}
	for _, expense := range allExpenses {
		if r.matchesCriteria(expense, criteria) {
			filteredExpenses = append(filteredExpenses, expense)
		}
	}

	return filteredExpenses, nil
}

// GetSchema returns the Excel file schema
func (r *revenueExpenseExcelRepository) GetSchema(ctx context.Context) *models.FileMetadata {
	return r.fileMetadata
}

// extractSheetMetadata analyzes a sheet and extracts its metadata
func (r *revenueExpenseExcelRepository) extractSheetMetadata(file *excelize.File, sheetName string) (*models.ExcelSheetMetadata, error) {
	// Get all rows from the sheet
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows: %w", err)
	}

	if len(rows) == 0 {
		return &models.ExcelSheetMetadata{
			SheetName: sheetName,
			Headers:   []models.ExcelColumnMetadata{},
		}, nil
	}

	// Find the header row
	headerRow := r.findHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return &models.ExcelSheetMetadata{
			SheetName: sheetName,
			Headers:   []models.ExcelColumnMetadata{},
		}, nil
	}

	// Extract headers
	var headers []models.ExcelColumnMetadata
	headerCells := rows[headerRow]

	for i, cellValue := range headerCells {
		if cellValue != "" && !r.isCommentOrEmpty(cellValue) {
			header := models.ExcelColumnMetadata{
				ColumnIndex: i,
				ColumnName:  cellValue,
				DataType:    "string", // Default to string for now
				Required:    false,
			}
			headers = append(headers, header)
		}
	}

	return &models.ExcelSheetMetadata{
		SheetName: sheetName,
		Headers:   headers,
	}, nil
}

// findHeaderRow finds the row that contains column headers
func (r *revenueExpenseExcelRepository) findHeaderRow(rows [][]string) int {
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}

		// Check if this row looks like a header row
		headerCount := 0
		for _, cell := range row {
			if cell != "" && !r.isCommentOrEmpty(cell) {
				headerCount++
			}
		}

		// If we have at least 3 non-empty cells, consider it a header row
		if headerCount >= 3 {
			return i
		}
	}

	return -1
}

// isCommentOrEmpty checks if a cell value is a comment or empty
func (r *revenueExpenseExcelRepository) isCommentOrEmpty(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//")
}

// matchesCriteria helper function to check if an expense matches search criteria
func (r *revenueExpenseExcelRepository) matchesCriteria(expense, criteria map[string]interface{}) bool {
	for key, expectedValue := range criteria {
		if actualValue, exists := expense[key]; !exists || actualValue != expectedValue {
			return false
		}
	}
	return true
}

// isTodayDate checks if the given date string matches today's date in any common format
func (r *revenueExpenseExcelRepository) isTodayDate(dateValue string) bool {
	today := time.Now()
	todayFormats := []string{
		today.Format("2006-01-02"),
		today.Format("02/01/2006"),
		today.Format("01/02/2006"),
		today.Format("2006/01/02"),
		today.Format("02-01-2006"),
		today.Format("01-02-2006"),
		today.Format("2006-01-02 15:04:05"),
		today.Format("02/01/2006 15:04"),
	}

	trimmedDate := strings.TrimSpace(dateValue)
	for _, todayFormat := range todayFormats {
		if trimmedDate == todayFormat {
			return true
		}
	}
	return false
}
