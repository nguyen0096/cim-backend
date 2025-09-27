package repository

import (
	"context"
	"fmt"
	"import-export-backend/internal/models"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/xuri/excelize/v2"
)

// Constants for date formats and Excel styling
var (
	// Available date formats for parsing Excel dates
	availableDateFormats = []string{
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

	// Default Excel cell style configuration
	defaultCellStyle = &excelize.Style{
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
	}
)

// RevenueExpenseExcelRepository handles data access for revenue/expense Excel operations
type RevenueExpenseExcelRepository interface {
	InitializeWithFile(ctx context.Context, filePath string) error
	AddExpense(ctx context.Context, sheetName string, expenseData map[string]interface{}) error
	GetLastExpense(ctx context.Context, sheetName string) (map[string]interface{}, error)
	GetLastTransactionDate(ctx context.Context, sheetName string) (time.Time, error)
	DeleteLastNRows(ctx context.Context, sheetName string, n int) error
	GetSchema(ctx context.Context) *models.FileMetadata
	Close() error
	ForceCacheRefresh()
}

// revenueExpenseExcelRepository implements RevenueExpenseExcelRepository
type revenueExpenseExcelRepository struct {
	ctx          context.Context
	fileMetadata *models.FileMetadata
	fileCache    *excelize.File
	lastModified time.Time
	cacheMutex   sync.RWMutex

	// Performance optimization caches
	rowsCache      [][]string
	headerRowCache int
	cacheValid     bool
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

// createCellStyle creates a new Excel cell style with optional date format
func (r *revenueExpenseExcelRepository) createCellStyle(file *excelize.File, dateFormat string) (int, error) {
	style := *defaultCellStyle // Copy the default style
	if dateFormat != "" {
		style.CustomNumFmt = &dateFormat
	}

	styleID, err := file.NewStyle(&style)
	if err != nil {
		return 0, fmt.Errorf("failed to create style: %w", err)
	}
	return styleID, nil
}

// detectExcelDateFormat converts Go time format to Excel date format
func (r *revenueExpenseExcelRepository) detectExcelDateFormat(goDateFormat string) string {
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

// findLastTransactionDateInfo finds the last transaction date and its format
func (r *revenueExpenseExcelRepository) findLastTransactionDateInfo(rows [][]string, headerRow int, today time.Time) (bool, string) {
	detectedDateFormat := availableDateFormats[0] // default format
	// Scan rows from bottom up, starting after the header
	for i := len(rows) - 1; i >= headerRow+1; i-- {
		if len(rows[i]) == 0 {
			continue
		}

		dateValue := strings.TrimSpace(rows[i][0])
		if dateValue == "" {
			continue
		}

		for _, dateFormat := range availableDateFormats {
			// try parse datestr
			parsedDate, err := time.Parse(dateFormat, dateValue)
			if err != nil {
				continue
			}

			detectedDateFormat = dateFormat

			if parsedDate.Format(dateFormat) == today.Format(dateFormat) {
				return true, detectedDateFormat
			}
		}

		break
	}

	return false, detectedDateFormat
}

// addTransactionDateRow adds a new row with today's date if needed
func (r *revenueExpenseExcelRepository) addTransactionDateRow(file *excelize.File, sheetName string, targetRow int, today time.Time, dateFormat string) error {
	// Find the sheet metadata for the specified sheet
	sheetMetadata := r.findSheetMetadata(sheetName)
	if sheetMetadata == nil || len(sheetMetadata.Headers) == 0 {
		return nil
	}

	excelDateFormat := r.detectExcelDateFormat(dateFormat)
	styleID, err := r.createCellStyle(file, excelDateFormat)
	if err != nil {
		return fmt.Errorf("failed to create date style: %w", err)
	}

	firstColumn := sheetMetadata.Headers[0]
	cellName, _ := excelize.CoordinatesToCellName(firstColumn.ColumnIndex, targetRow)

	file.SetCellStyle(sheetName, cellName, cellName, styleID)
	file.SetCellValue(sheetName, cellName, today)

	return nil
}

// addExpenseDataRow adds the expense data to the specified row
func (r *revenueExpenseExcelRepository) addExpenseDataRow(file *excelize.File, sheetName string, targetRow int, expenseData map[string]interface{}) error {
	styleID, err := r.createCellStyle(file, "")
	if err != nil {
		return fmt.Errorf("failed to create data style: %w", err)
	}

	// Find the sheet metadata for the specified sheet
	sheetMetadata := r.findSheetMetadata(sheetName)
	if sheetMetadata == nil {
		return fmt.Errorf("sheet %s not found in metadata", sheetName)
	}

	for _, column := range sheetMetadata.Headers {
		cellName, _ := excelize.CoordinatesToCellName(column.ColumnIndex+1, targetRow)
		file.SetCellStyle(sheetName, cellName, cellName, styleID)

		if value, exists := expenseData[column.ColumnName]; exists {
			// Convert value to uppercase string
			valueStr := fmt.Sprintf("%v", value)
			uppercaseValue := strings.ToUpper(valueStr)
			file.SetCellValue(sheetName, cellName, uppercaseValue)
		}
	}

	return nil
}

// findSheetMetadata finds the metadata for a specific sheet by name
func (r *revenueExpenseExcelRepository) findSheetMetadata(sheetName string) *models.ExcelSheetMetadata {
	if r.fileMetadata == nil {
		return nil
	}

	for _, sheet := range r.fileMetadata.Sheets {
		if sheet.SheetName == sheetName {
			return &sheet
		}
	}
	return nil
}

// validateRepositoryState validates that the repository is properly initialized
func (r *revenueExpenseExcelRepository) validateRepositoryState() error {
	if r.fileMetadata == nil {
		return fmt.Errorf("repository not initialized, call InitializeWithFile first")
	}

	if len(r.fileMetadata.Sheets) == 0 {
		return fmt.Errorf("no sheets found in metadata")
	}

	return nil
}

// AddExpense adds a new expense entry to the Excel file
func (r *revenueExpenseExcelRepository) AddExpense(ctx context.Context, sheetName string, expenseData map[string]interface{}) error {
	// Validate repository state and expense data
	if err := r.validateRepositoryState(); err != nil {
		return err
	}
	if err := r.validateExpenseData(expenseData); err != nil {
		return err
	}

	// Get file and sheet data
	file, _, rows, err := r.getFileAndSheetData(sheetName)
	if err != nil {
		return err
	}
	// Note: Don't close file here as it's cached

	// Find header row
	headerRow := r.findHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return fmt.Errorf("no header row found")
	}

	// Prepare date and row information
	today := r.getTodayDate()
	isTodayExists, detectedDateFormat := r.findLastTransactionDateInfo(rows, headerRow, today)
	targetRow := len(rows) + 1

	// Add transaction date row if needed
	if !isTodayExists {
		if err := r.addTransactionDateRow(file, sheetName, targetRow, today, detectedDateFormat); err != nil {
			return fmt.Errorf("failed to add transaction date row: %w", err)
		}
		targetRow++
	}

	// Add expense data row
	if err := r.addExpenseDataRow(file, sheetName, targetRow, expenseData); err != nil {
		return fmt.Errorf("failed to add expense data row: %w", err)
	}

	// Save the file
	if err := file.Save(); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	// Invalidate cache after saving to ensure next read gets fresh data
	r.invalidateCache()

	return nil
}

// GetLastExpense retrieves the most recent expense entry from the Excel file
func (r *revenueExpenseExcelRepository) GetLastExpense(ctx context.Context, sheetName string) (map[string]interface{}, error) {
	// Validate repository state
	if err := r.validateRepositoryState(); err != nil {
		return nil, err
	}

	// Get file and sheet data
	_, _, rows, err := r.getFileAndSheetData(sheetName)
	if err != nil {
		return nil, err
	}
	// Note: Don't close file here as it's cached

	// Find the last data row
	lastDataRow, err := r.findLastTransactionRow(rows)
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

// GetLastTransactionDate retrieves the date of the most recent transaction from the Excel file
func (r *revenueExpenseExcelRepository) GetLastTransactionDate(ctx context.Context, sheetName string) (time.Time, error) {
	// Validate repository state
	if err := r.validateRepositoryState(); err != nil {
		return time.Time{}, err
	}

	// Get file and sheet data
	_, _, rows, err := r.getFileAndSheetData(sheetName)
	if err != nil {
		return time.Time{}, err
	}
	// Note: Don't close file here as it's cached

	// Find the last data row
	lastDateRow, err := r.findLastDateRow(rows)
	if err != nil {
		return time.Time{}, err
	}

	// Try to parse the date using various formats
	for _, dateFormat := range availableDateFormats {
		if parsedDate, err := time.Parse(dateFormat, lastDateRow[0]); err == nil {
			return parsedDate, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid date format found")
}

// DeleteLastNRows removes the last n data rows from the Excel file
func (r *revenueExpenseExcelRepository) DeleteLastNRows(ctx context.Context, sheetName string, n int) error {
	if r.fileMetadata == nil {
		return fmt.Errorf("repository not initialized, call InitializeWithFile first")
	}

	// Get file and sheet data
	file, _, rows, err := r.getFileAndSheetData(sheetName)
	if err != nil {
		return err
	}
	// Note: Don't close file here as it's cached

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

	// Invalidate cache after saving to ensure next read gets fresh data
	r.invalidateCache()

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
	// Check if we have cached header row
	r.cacheMutex.RLock()
	if r.cacheValid && r.headerRowCache >= 0 {
		headerRow := r.headerRowCache
		r.cacheMutex.RUnlock()
		return headerRow
	}
	r.cacheMutex.RUnlock()

	// Find header row
	headerRow := -1
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
			headerRow = i
			break
		}
	}

	// Cache the header row
	r.cacheMutex.Lock()
	r.headerRowCache = headerRow
	r.cacheMutex.Unlock()

	return headerRow
}

// isCommentOrEmpty checks if a cell value is a comment or empty
func (r *revenueExpenseExcelRepository) isCommentOrEmpty(value string) bool {
	// Fast path for empty strings
	if len(value) == 0 {
		return true
	}

	// Fast path for strings that are just whitespace
	trimmed := strings.TrimSpace(value)
	if len(trimmed) == 0 {
		return true
	}

	// Check for comments
	return strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//")
}

// hasNonEmptyData checks if a row has any non-empty data (optimized)
func (r *revenueExpenseExcelRepository) hasNonEmptyData(row []string) bool {
	for _, cell := range row {
		if len(cell) > 0 && strings.TrimSpace(cell) != "" {
			return true
		}
	}
	return false
}

// getFileAndSheetData opens the Excel file and returns file, sheet name, and rows
func (r *revenueExpenseExcelRepository) getFileAndSheetData(sheetName string) (*excelize.File, string, [][]string, error) {
	file, err := r.getCachedFile()
	if err != nil {
		return nil, "", nil, err
	}

	// Validate that the sheet exists in metadata
	sheetMetadata := r.findSheetMetadata(sheetName)
	if sheetMetadata == nil {
		return nil, "", nil, fmt.Errorf("sheet %s not found in metadata", sheetName)
	}

	// Check if we have cached rows and they're still valid
	r.cacheMutex.RLock()
	if r.cacheValid && len(r.rowsCache) > 0 {
		rows := r.rowsCache
		r.cacheMutex.RUnlock()
		return file, sheetName, rows, nil
	}
	r.cacheMutex.RUnlock()

	// Get current rows
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to get rows: %w", err)
	}

	if len(rows) == 0 {
		return nil, "", nil, fmt.Errorf("no data found in sheet")
	}

	// Cache the rows for future use
	r.cacheMutex.Lock()
	r.rowsCache = rows
	r.cacheValid = true
	r.cacheMutex.Unlock()

	return file, sheetName, rows, nil
}

// findLastTransactionRow finds the last row with actual data (scanning from bottom up)
func (r *revenueExpenseExcelRepository) findLastTransactionRow(rows [][]string) ([]string, error) {
	// Find header row
	headerRow := r.findHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return nil, fmt.Errorf("no header row found")
	}

	// Find the last data row (scan from bottom up, starting after the header)
	var lastRow []string
	for i := len(rows) - 1; i >= headerRow+1; i-- {
		if len(rows[i]) == 0 {
			continue
		}

		// Check if this row has any non-empty data (optimized)
		if r.hasNonEmptyData(rows[i]) {
			lastRow = rows[i]
			break
		}
	}

	if len(lastRow) == 0 {
		return nil, fmt.Errorf("no transaction rows found")
	}

	return lastRow, nil
}

func (r *revenueExpenseExcelRepository) findLastDateRow(rows [][]string) ([]string, error) {
	// Find header row
	headerRow := r.findHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return nil, fmt.Errorf("no header row found")
	}

	// Find the last data row (scan from bottom up, starting after the header)
	var lastRow []string
	for i := len(rows) - 1; i >= headerRow+1; i-- {
		if len(rows[i]) == 0 {
			continue
		}

		if strings.TrimSpace(rows[i][0]) != "" {
			lastRow = rows[i]
			break
		}
	}

	if len(lastRow) == 0 {
		return nil, fmt.Errorf("no date rows found")
	}

	return lastRow, nil
}

// validateExpenseData validates the expense data before adding it
func (r *revenueExpenseExcelRepository) validateExpenseData(expenseData map[string]interface{}) error {
	if expenseData == nil {
		return fmt.Errorf("expense data cannot be nil")
	}

	if len(expenseData) == 0 {
		return fmt.Errorf("expense data cannot be empty")
	}

	// Check if at least one field has a non-empty value
	hasValidData := false
	for _, value := range expenseData {
		if valueStr := fmt.Sprintf("%v", value); strings.TrimSpace(valueStr) != "" {
			hasValidData = true
			break
		}
	}

	if !hasValidData {
		return fmt.Errorf("expense data must contain at least one non-empty value")
	}

	return nil
}

// getTodayDate returns today's date with time set to midnight
func (r *revenueExpenseExcelRepository) getTodayDate() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// getCachedFile returns a cached file handle or opens a new one if needed
func (r *revenueExpenseExcelRepository) getCachedFile() (*excelize.File, error) {
	r.cacheMutex.Lock()
	defer r.cacheMutex.Unlock()

	// Check if file exists and get its modification time
	fileInfo, err := os.Stat(r.fileMetadata.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// If we have a cached file and it's still valid, return it
	if r.fileCache != nil && !fileInfo.ModTime().After(r.lastModified) {
		return r.fileCache, nil
	}

	// Close existing cached file if it exists
	if r.fileCache != nil {
		r.fileCache.Close()
	}

	// Open new file
	file, err := excelize.OpenFile(r.fileMetadata.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Cache the file and update metadata
	r.fileCache = file
	r.lastModified = fileInfo.ModTime()

	return file, nil
}

// invalidateCache marks the file cache as invalid, forcing a reload on next access
func (r *revenueExpenseExcelRepository) invalidateCache() {
	r.cacheMutex.Lock()
	defer r.cacheMutex.Unlock()

	if r.fileCache != nil {
		r.fileCache.Close()
		r.fileCache = nil
	}

	// Invalidate all data caches
	r.rowsCache = nil
	r.headerRowCache = -1
	r.cacheValid = false
}

// closeCachedFile closes the cached file if it exists
func (r *revenueExpenseExcelRepository) closeCachedFile() {
	r.cacheMutex.Lock()
	defer r.cacheMutex.Unlock()

	if r.fileCache != nil {
		r.fileCache.Close()
		r.fileCache = nil
	}
}

// Close closes the repository and releases any cached resources
func (r *revenueExpenseExcelRepository) Close() error {
	r.closeCachedFile()
	return nil
}

// ForceCacheRefresh forces a cache refresh on the next file access
func (r *revenueExpenseExcelRepository) ForceCacheRefresh() {
	r.invalidateCache()
}
