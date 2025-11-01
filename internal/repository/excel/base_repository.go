package excel

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/xuri/excelize/v2"
)

// BaseExcelRepository provides common functionality for Excel repositories
type BaseExcelRepository struct {
	ctx          context.Context
	fileMetadata *models.FileMetadata
	fileCache    *excelize.File
	lastModified time.Time
	cacheMutex   sync.RWMutex

	// Performance optimization cache
	cache *Cache
}

// InitializeWithFile initializes the repository with an Excel file
// If sheetNames are provided, only those sheets will be processed
func (r *BaseExcelRepository) InitializeWithFile(ctx context.Context, filePath string, fileType string, sheetNames ...string) error {
	r.ctx = ctx
	r.cache = NewCache()

	// Read the Excel file to extract actual metadata
	file, err := excelize.OpenFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	// Get sheet names
	allSheets := file.GetSheetList()
	if len(allSheets) == 0 {
		return fmt.Errorf("no sheets found in file")
	}

	// Filter sheets based on provided sheet names (if any)
	var sheetsToProcess []string
	if len(sheetNames) > 0 {
		// Create a map for quick lookup of requested sheets
		requestedSheets := make(map[string]bool)
		for _, name := range sheetNames {
			requestedSheets[name] = true
		}

		// Only include sheets that exist in the file and are requested
		for _, sheetName := range allSheets {
			if requestedSheets[sheetName] {
				sheetsToProcess = append(sheetsToProcess, sheetName)
			}
		}

		// Validate that all requested sheets exist
		if len(sheetsToProcess) != len(sheetNames) {
			var missingSheets []string
			for _, requestedSheet := range sheetNames {
				found := false
				for _, existingSheet := range allSheets {
					if existingSheet == requestedSheet {
						found = true
						break
					}
				}
				if !found {
					missingSheets = append(missingSheets, requestedSheet)
				}
			}
			if len(missingSheets) > 0 {
				return fmt.Errorf("requested sheets not found in file: %s", strings.Join(missingSheets, ", "))
			}
		}
	} else {
		// Process all sheets if no specific sheets are requested
		sheetsToProcess = allSheets
	}

	if len(sheetsToProcess) == 0 {
		return fmt.Errorf("no sheets to process")
	}

	// Process each sheet
	var sheetMetadataList []models.ExcelSheetMetadata
	for _, sheetName := range sheetsToProcess {
		sheetMetadata, err := r.extractSheetMetadata(file, sheetName)
		if err != nil {
			return fmt.Errorf("failed to extract metadata for sheet %s: %w", sheetName, err)
		}
		sheetMetadataList = append(sheetMetadataList, *sheetMetadata)
	}

	now := time.Now()
	r.fileMetadata = &models.FileMetadata{
		FileType:  fileType,
		FilePath:  filePath,
		Version:   "1.0",
		Sheets:    sheetMetadataList,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return nil
}

// GetSchema returns the Excel file schema
func (r *BaseExcelRepository) GetSchema(ctx context.Context) *models.FileMetadata {
	return r.fileMetadata
}

// GetFileAndSheetData opens the Excel file and returns file, sheet name, and rows
func (r *BaseExcelRepository) GetFileAndSheetData(sheetName string) (*excelize.File, *models.ExcelSheetMetadata, [][]string, error) {
	if err := r.validateRepositoryState(); err != nil {
		return nil, nil, nil, err
	}

	file, err := r.getCachedFile()
	if err != nil {
		return nil, nil, nil, err
	}

	// Validate that the sheet exists in metadata
	sheetMetadata := r.findSheetMetadata(sheetName)
	if sheetMetadata == nil {
		return nil, nil, nil, fmt.Errorf("sheet %s not found in metadata", sheetName)
	}

	// Check if we have cached rows and they're still valid
	r.cacheMutex.RLock()
	if r.cache.valid && len(r.cache.rows) > 0 {
		rows := r.cache.rows
		r.cacheMutex.RUnlock()
		return file, sheetMetadata, rows, nil
	}
	r.cacheMutex.RUnlock()

	// Get current rows
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get rows: %w", err)
	}

	if len(rows) == 0 {
		return nil, nil, nil, fmt.Errorf("no data found in sheet")
	}

	// Cache the rows for future use
	r.cacheMutex.Lock()
	r.cache.rows = rows
	r.cache.valid = true
	r.cacheMutex.Unlock()

	return file, sheetMetadata, rows, nil
}

// FindHeaderRow finds the row that contains column headers with caching
func (r *BaseExcelRepository) FindHeaderRow(rows [][]string) int {
	// Check if we have cached header row
	r.cacheMutex.RLock()
	if r.cache.valid && r.cache.headerRow >= 0 {
		headerRow := r.cache.headerRow
		r.cacheMutex.RUnlock()
		return headerRow
	}
	r.cacheMutex.RUnlock()

	// Find header row using common utility
	headerRow := FindHeaderRow(rows)

	// Cache the header row
	r.cacheMutex.Lock()
	r.cache.headerRow = headerRow
	r.cacheMutex.Unlock()

	return headerRow
}

// FindLastTransactionRow finds the last row with actual data (scanning from bottom up)
func (r *BaseExcelRepository) FindLastTransactionRow(rows [][]string) ([]string, int, error) {
	// Find header row
	headerRow := r.FindHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return nil, 0, fmt.Errorf("no header row found")
	}

	headerRowData := rows[headerRow]
	targetColumnIndex := -1
	for idx, cell := range headerRowData {
		if cell == pkg.RevenueExpenseColumnName {
			targetColumnIndex = idx
			break
		}
	}

	// Find the last data row (scan from bottom up, starting after the header)
	var lastRow []string
	var lastRowIndex int
	for i := len(rows) - 1; i >= headerRow+1; i-- {
		if rows[i][0] == "" && rows[i][targetColumnIndex] == "" {
			continue
		}

		lastRow = rows[i]
		lastRowIndex = i + 1
		break
	}

	if len(lastRow) == 0 {
		return nil, 0, fmt.Errorf("no transaction rows found")
	}

	return lastRow, lastRowIndex, nil
}

// FindLastDateRow finds the last row with a date in the first column
func (r *BaseExcelRepository) FindLastDateRow(rows [][]string) ([]string, error) {
	// Find header row
	headerRow := r.FindHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return nil, fmt.Errorf("no header row found")
	}

	// Find the last data row (scan from bottom up, starting after the header)
	var lastRow []string
	for i := len(rows) - 1; i >= headerRow+1; i-- {
		if len(rows[i]) == 0 {
			continue
		}

		// Check if first column contains a date-like value
		firstCol := strings.TrimSpace(rows[i][0])
		if firstCol != "" {
			// Try to parse as date to verify it's actually a date
			if _, err := ParseDateFromRow(rows[i]); err == nil {
				lastRow = rows[i]
				break
			}
		}
	}

	if len(lastRow) == 0 {
		return nil, fmt.Errorf("no date rows found")
	}

	return lastRow, nil
}

// GetLastTransactionDate retrieves the date of the most recent transaction from the Excel file
func (r *BaseExcelRepository) GetLastTransactionDate(ctx context.Context, sheetName string) (time.Time, error) {
	// Validate repository state
	if err := r.validateRepositoryState(); err != nil {
		return time.Time{}, err
	}

	// Get file and sheet data
	_, _, rows, err := r.GetFileAndSheetData(sheetName)
	if err != nil {
		return time.Time{}, err
	}

	// Find the last date row
	lastDateRow, err := r.FindLastDateRow(rows)
	if err != nil {
		return time.Time{}, err
	}

	// Parse the date
	return ParseDateFromRow(lastDateRow)
}

// AddTransactionDateRow adds a new row with today's date if needed
func (r *BaseExcelRepository) AddTransactionDateRow(file *excelize.File, sheetName string, targetRow int, today time.Time, dateFormat string) error {
	// Find the sheet metadata for the specified sheet
	sheetMetadata := r.findSheetMetadata(sheetName)
	if sheetMetadata == nil || len(sheetMetadata.Headers) == 0 {
		return nil
	}

	excelDateFormat := DetectExcelDateFormat(dateFormat)
	styleID, err := CreateCellStyle(file, excelDateFormat)
	if err != nil {
		return fmt.Errorf("failed to create date style: %w", err)
	}

	firstColumn := sheetMetadata.Headers[0]
	cellName, _ := excelize.CoordinatesToCellName(firstColumn.ColumnIndex, targetRow)

	file.SetCellStyle(sheetName, cellName, cellName, styleID)
	file.SetCellValue(sheetName, cellName, today)

	return nil
}

// AddDataRow adds data to the specified row
func (r *BaseExcelRepository) AddDataRow(file *excelize.File, sheetName string, targetRow int, data map[string]interface{}) error {
	styleID, err := CreateCellStyle(file, "")
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

		if value, exists := data[column.ColumnName]; exists {
			// Convert value to uppercase string
			valueStr := fmt.Sprintf("%v", value)
			uppercaseValue := strings.ToUpper(valueStr)
			file.SetCellValue(sheetName, cellName, uppercaseValue)
		}
	}

	return nil
}

// AddDataRowWithColor adds data to the specified row with background color
func (r *BaseExcelRepository) AddDataRowWithColor(file *excelize.File, sheetName string, targetRow int, data map[string]interface{}, cellColor string) error {
	styleID, err := CreateCellStyle(file, "")
	if err != nil {
		return fmt.Errorf("failed to create data style: %w", err)
	}

	thousandStyleID, err := CreateCellStyleWithCurrencyThousandSeparator(file)
	if err != nil {
		return fmt.Errorf("failed to create currency thousand separator style: %w", err)
	}

	// Find the sheet metadata for the specified sheet
	sheetMetadata := r.findSheetMetadata(sheetName)
	if sheetMetadata == nil {
		return fmt.Errorf("sheet %s not found in metadata", sheetName)
	}

	for _, column := range sheetMetadata.Headers {
		cellName, _ := excelize.CoordinatesToCellName(column.ColumnIndex+1, targetRow)

		value, exists := data[column.ColumnName]
		if !exists {
			continue
		}
		if column.ColumnName == pkg.RevenueExpenseColumnWater || column.ColumnName == pkg.RevenueExpenseColumnSnackAndRice {
			file.SetCellStyle(sheetName, cellName, cellName, thousandStyleID)
		} else {
			file.SetCellStyle(sheetName, cellName, cellName, styleID)
		}
		file.SetCellStyle(sheetName, cellName, cellName, styleID)

		// Convert value to uppercase string
		valueStr := fmt.Sprintf("%v", value)
		uppercaseValue := strings.ToUpper(valueStr)
		file.SetCellValue(sheetName, cellName, uppercaseValue)
	}

	if cellColor != "" {
		colorStyleID, err := CreateCellStyleWithColor(file, cellColor)
		if err != nil {
			return fmt.Errorf("failed to create color style: %w", err)
		}
		firstColumn := sheetMetadata.Headers[0]
		sttCellName, _ := excelize.CoordinatesToCellName(firstColumn.ColumnIndex+1, targetRow)
		file.SetCellStyle(sheetName, sttCellName, sttCellName, colorStyleID)
	}

	return nil
}

// Close closes the repository and releases any cached resources
func (r *BaseExcelRepository) Close() error {
	r.closeCachedFile()
	return nil
}

// ForceCacheRefresh forces a cache refresh on the next file access
func (r *BaseExcelRepository) ForceCacheRefresh() {
	r.invalidateCache()
}

// Private helper methods

// extractSheetMetadata analyzes a sheet and extracts its metadata
func (r *BaseExcelRepository) extractSheetMetadata(file *excelize.File, sheetName string) (*models.ExcelSheetMetadata, error) {
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
	headerRow := FindHeaderRow(rows)
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
		if cellValue != "" && !IsCommentOrEmpty(cellValue) {
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

// findSheetMetadata finds the metadata for a specific sheet by name
func (r *BaseExcelRepository) findSheetMetadata(sheetName string) *models.ExcelSheetMetadata {
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
func (r *BaseExcelRepository) validateRepositoryState() error {
	if r.fileMetadata == nil {
		return fmt.Errorf("repository not initialized, call InitializeWithFile first")
	}

	if len(r.fileMetadata.Sheets) == 0 {
		return fmt.Errorf("no sheets found in metadata")
	}

	return nil
}

// getCachedFile returns a cached file handle or opens a new one if needed
func (r *BaseExcelRepository) getCachedFile() (*excelize.File, error) {
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
func (r *BaseExcelRepository) invalidateCache() {
	r.cacheMutex.Lock()
	defer r.cacheMutex.Unlock()

	if r.fileCache != nil {
		r.fileCache.Close()
		r.fileCache = nil
	}

	// Invalidate all data caches
	if r.cache != nil {
		r.cache.rows = nil
		r.cache.headerRow = -1
		r.cache.valid = false
	}
}

// closeCachedFile closes the cached file if it exists
func (r *BaseExcelRepository) closeCachedFile() {
	r.cacheMutex.Lock()
	defer r.cacheMutex.Unlock()

	if r.fileCache != nil {
		r.fileCache.Close()
		r.fileCache = nil
	}
}
