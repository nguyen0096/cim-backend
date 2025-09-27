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
	AddExpense(ctx context.Context, expenseData map[string]interface{}) error
	GetLastExpense(ctx context.Context) (map[string]interface{}, error)
	GetLastTransactionDate(ctx context.Context) (time.Time, error)
	DeleteLastNRows(ctx context.Context, n int) error
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

// AddExpense adds a new expense entry to the Excel file using ExcelWriter
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
	detectedDateFormat := availableDateFormats[0]

	// Helper function to determine if a date string matches today's date in any supported format
	isToday := func(dateStr string) bool {
		for _, dateFormat := range availableDateFormats {
			// try parse datestr
			date, err := time.Parse(dateFormat, dateStr)
			if err != nil {
				continue
			}

			detectedDateFormat = dateFormat

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

	// Helper function to detect Excel date format from existing data
	detectExcelDateFormat := func(goDateFormat string) string {
		switch goDateFormat {
		case "1/02/2006", "01/02/2006":
			return "mm/dd/yyyy"
		case "02/1/2006", "02/01/2006":
			return "dd/mm/yyyy"
		case "1-02-2006", "01-02-2006":
			return "mm-dd-yyyy"
		case "02-01-2006", "02-1-2006":
			return "dd-mm-yyyy"
		case "2006-1-02", "2006-01-02":
			return "yyyy-mm-dd"
		case "2006/1/02", "2006/01/02":
			return "yyyy/mm/dd"
		default:
			return "mm/dd/yyyy" // default fallback
		}
	}

	// If no row with today's date exists, create a new row
	if isNewTransactionDateRequired {
		if len(r.fileMetadata.Sheets[0].Headers) > 0 {
			excelDateFormat := detectExcelDateFormat(detectedDateFormat)
			styleID, err := file.NewStyle(&excelize.Style{
				Font: &excelize.Font{
					Family: "Times New Roman",
					Bold:   true,
				},
				Border: []excelize.Border{
					{Type: "left", Color: "000000", Style: 1},
					{Type: "top", Color: "000000", Style: 1},
					{Type: "bottom", Color: "000000", Style: 1},
					{Type: "right", Color: "000000", Style: 1},
				},
				CustomNumFmt: &excelDateFormat,
			})
			if err != nil {
				return fmt.Errorf("failed to create style: %w", err)
			}

			firstColumn := r.fileMetadata.Sheets[0].Headers[0]
			cellName, _ := excelize.CoordinatesToCellName(firstColumn.ColumnIndex, targetRow)
			file.SetCellStyle(sheetName, cellName, cellName, styleID)
			file.SetCellValue(sheetName, cellName, today)
		}
		targetRow++
	}

	styleID, err := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Family: "Times New Roman",
			Bold:   true,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	// Add the expense data to the target row
	for _, column := range r.fileMetadata.Sheets[0].Headers {
		cellName, _ := excelize.CoordinatesToCellName(column.ColumnIndex+1, targetRow)
		file.SetCellStyle(sheetName, cellName, cellName, styleID)
		if value, exists := expenseData[column.ColumnName]; exists {

			// Convert value to uppercase string
			valueStr := fmt.Sprintf("%v", value)
			uppercaseValue := strings.ToUpper(valueStr)

			// Set cell value and apply formatting
			file.SetCellValue(sheetName, cellName, uppercaseValue)
		}
	}

	// Save the file
	err = file.Save()
	if err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

// GetLastExpense retrieves the most recent expense entry from the Excel file
func (r *revenueExpenseExcelRepository) GetLastExpense(ctx context.Context) (map[string]interface{}, error) {
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

	// Get current rows
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows: %w", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("no data found in sheet")
	}

	// Find header row
	headerRow := r.findHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return nil, fmt.Errorf("no header row found")
	}

	// Find the last data row (scan from bottom up, starting after the header)
	var lastDataRow []string
	for i := len(rows) - 1; i >= headerRow+1; i-- {
		if len(rows[i]) == 0 {
			continue
		}

		// Check if this row has any non-empty data
		hasData := false
		for _, cell := range rows[i] {
			if strings.TrimSpace(cell) != "" {
				hasData = true
				break
			}
		}

		if hasData {
			lastDataRow = rows[i]
			break
		}
	}

	if len(lastDataRow) == 0 {
		return nil, fmt.Errorf("no data rows found")
	}

	// Build the expense data map using the headers
	expenseData := make(map[string]interface{})
	headers := r.fileMetadata.Sheets[0].Headers

	for _, header := range headers {
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
func (r *revenueExpenseExcelRepository) GetLastTransactionDate(ctx context.Context) (time.Time, error) {
	if r.fileMetadata == nil {
		return time.Time{}, fmt.Errorf("repository not initialized, call InitializeWithFile first")
	}

	file, err := excelize.OpenFile(r.fileMetadata.FilePath)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Use the first sheet from metadata
	if len(r.fileMetadata.Sheets) == 0 {
		return time.Time{}, fmt.Errorf("no sheets found in metadata")
	}

	sheetName := r.fileMetadata.Sheets[0].SheetName

	// Get current rows
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get rows: %w", err)
	}

	if len(rows) == 0 {
		return time.Time{}, fmt.Errorf("no data found in sheet")
	}

	// Find header row
	headerRow := r.findHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return time.Time{}, fmt.Errorf("no header row found")
	}

	// Available date formats that might be used in Excel
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
		"2006-01-02 15:04:05",
		"02/01/2006 15:04",
		"01/02/2006 15:04",
	}

	// Find the last transaction date (scan from bottom up, starting after the header)
	for i := len(rows) - 1; i >= headerRow+1; i-- {
		if len(rows[i]) == 0 {
			continue
		}

		// Check if this row has any non-empty data
		hasData := false
		for _, cell := range rows[i] {
			if strings.TrimSpace(cell) != "" {
				hasData = true
				break
			}
		}

		if !hasData {
			continue
		}

		// Get the date from the first column (assuming first column contains dates)
		if len(rows[i]) > 0 {
			dateValue := strings.TrimSpace(rows[i][0])
			if dateValue != "" {
				// Try to parse the date using various formats
				for _, dateFormat := range availableDateFormats {
					if parsedDate, err := time.Parse(dateFormat, dateValue); err == nil {
						return parsedDate, nil
					}
				}
			}
		}
	}

	return time.Time{}, fmt.Errorf("no transaction date found")
}

// DeleteLastNRows removes the last n data rows from the Excel file
func (r *revenueExpenseExcelRepository) DeleteLastNRows(ctx context.Context, n int) error {
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

	// Find all data rows (rows with actual data after the header)
	var dataRowIndices []int
	for i := headerRow + 1; i < len(rows); i++ {
		if len(rows[i]) == 0 {
			continue
		}

		// Check if this row has any non-empty data
		hasData := false
		for _, cell := range rows[i] {
			if strings.TrimSpace(cell) != "" {
				hasData = true
				break
			}
		}

		if hasData {
			dataRowIndices = append(dataRowIndices, i)
		}
	}

	// Check if we have at least n data rows to delete
	if len(dataRowIndices) < n {
		return fmt.Errorf("not enough data rows to delete (found %d, need at least %d)", len(dataRowIndices), n)
	}

	// Get the last n data row indices (1-based for Excel)
	lastNRowIndices := dataRowIndices[len(dataRowIndices)-n:]

	// Delete the rows in reverse order to maintain correct indices
	for i := len(lastNRowIndices) - 1; i >= 0; i-- {
		rowIndex := lastNRowIndices[i] + 1 // Convert to 1-based index for Excel
		err := file.RemoveRow(sheetName, rowIndex)
		if err != nil {
			return fmt.Errorf("failed to delete row %d: %w", rowIndex, err)
		}
	}

	// Save the file
	err = file.Save()
	if err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
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
		"2006-01-02 15:04:05",
		"02/01/2006 15:04",
		"01/02/2006 15:04",
	}

	trimmedDate := strings.TrimSpace(dateValue)
	if trimmedDate == "" {
		return false
	}

	for _, dateFormat := range availableDateFormats {
		// try parse dateValue
		date, err := time.Parse(dateFormat, trimmedDate)
		if err != nil {
			continue
		}

		// Compare dates by truncating time components and comparing only date parts
		dateOnly := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
		todayOnly := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())

		if dateOnly.Equal(todayOnly) {
			return true
		}
	}
	return false
}
