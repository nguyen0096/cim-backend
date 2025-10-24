package googlesheets

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// BaseGoogleSheetsRepository provides common functionality for Google Sheets repositories
type BaseGoogleSheetsRepository struct {
	ctx           context.Context
	fileMetadata  *models.FileMetadata
	sheetsService *sheets.Service
	spreadsheetID string
	lastFetchTime time.Time
	cacheMutex    sync.RWMutex
	cache         *Cache
}

// InitializeWithSpreadsheet initializes the repository with a Google Sheets spreadsheet using service account
// If sheetNames are provided, only those sheets will be processed
func (r *BaseGoogleSheetsRepository) InitializeWithSpreadsheet(ctx context.Context, serviceAccountFilePath string, spreadsheetID string, fileType string, sheetNames ...string) error {
	r.ctx = ctx
	r.cache = NewCache()
	r.spreadsheetID = spreadsheetID

	// Create Google Sheets service with service account credentials
	srv, err := sheets.NewService(ctx, option.WithCredentialsFile(serviceAccountFilePath))
	if err != nil {
		return fmt.Errorf("failed to create sheets service: %w", err)
	}
	r.sheetsService = srv

	// Get spreadsheet metadata to get sheet names
	spreadsheet, err := r.sheetsService.Spreadsheets.Get(spreadsheetID).Do()
	if err != nil {
		return fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	// Get sheet names from the spreadsheet
	var allSheets []string
	for _, sheet := range spreadsheet.Sheets {
		allSheets = append(allSheets, sheet.Properties.Title)
	}

	if len(allSheets) == 0 {
		return fmt.Errorf("no sheets found in spreadsheet")
	}

	// Filter sheets based on provided sheet names (if any)
	var sheetsToProcess []string
	if len(sheetNames) > 0 {
		// Create a map for quick lookup of requested sheets
		requestedSheets := make(map[string]bool)
		for _, name := range sheetNames {
			requestedSheets[name] = true
		}

		// Only include sheets that exist in the spreadsheet and are requested
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
				return fmt.Errorf("requested sheets not found in spreadsheet: %s", strings.Join(missingSheets, ", "))
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
		sheetMetadata, err := r.extractSheetMetadata(sheetName)
		if err != nil {
			return fmt.Errorf("failed to extract metadata for sheet %s: %w", sheetName, err)
		}
		sheetMetadataList = append(sheetMetadataList, *sheetMetadata)
	}

	now := time.Now()
	r.fileMetadata = &models.FileMetadata{
		FileType:  fileType,
		FilePath:  spreadsheetID, // Use spreadsheet ID as file path
		Version:   "1.0",
		Sheets:    sheetMetadataList,
		CreatedAt: now,
		UpdatedAt: now,
	}

	r.lastFetchTime = now

	return nil
}

// GetSchema returns the Google Sheets schema
func (r *BaseGoogleSheetsRepository) GetSchema(ctx context.Context) *models.FileMetadata {
	return r.fileMetadata
}

// GetSheetData gets sheet data with caching
func (r *BaseGoogleSheetsRepository) GetSheetData(sheetName string) (*models.ExcelSheetMetadata, [][]interface{}, error) {
	if err := r.validateRepositoryState(); err != nil {
		return nil, nil, err
	}

	// Validate that the sheet exists in metadata
	sheetMetadata := r.findSheetMetadata(sheetName)
	if sheetMetadata == nil {
		return nil, nil, fmt.Errorf("sheet %s not found in metadata", sheetName)
	}

	// Check if we have cached rows and they're still valid (cache for 1 minute)
	r.cacheMutex.RLock()
	if r.cache.valid && len(r.cache.rows) > 0 && time.Since(r.lastFetchTime) < time.Minute {
		rows := r.cache.rows
		r.cacheMutex.RUnlock()
		return sheetMetadata, rows, nil
	}
	r.cacheMutex.RUnlock()

	// Fetch data from Google Sheets
	resp, err := r.sheetsService.Spreadsheets.Values.Get(r.spreadsheetID, sheetName).Do()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get sheet data: %w", err)
	}

	if len(resp.Values) == 0 {
		return nil, nil, fmt.Errorf("no data found in sheet")
	}

	// Cache the rows for future use
	r.cacheMutex.Lock()
	r.cache.rows = resp.Values
	r.cache.valid = true
	r.lastFetchTime = time.Now()
	r.cacheMutex.Unlock()

	return sheetMetadata, resp.Values, nil
}

// FindHeaderRow finds the row that contains column headers with caching
func (r *BaseGoogleSheetsRepository) FindHeaderRow(rows [][]interface{}) int {
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
func (r *BaseGoogleSheetsRepository) FindLastTransactionRow(rows [][]interface{}) ([]interface{}, int, error) {
	// Find header row
	headerRow := r.FindHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return nil, 0, fmt.Errorf("no header row found")
	}

	// Find the last data row (scan from bottom up, starting after the header)
	var lastRow []interface{}
	var lastRowIndex int
	for i := len(rows) - 1; i >= headerRow+1; i-- {
		if len(rows[i]) == 0 {
			continue
		}

		// Check if this row has any non-empty data (optimized)
		if HasNonEmptyData(rows[i]) {
			lastRow = rows[i]
			lastRowIndex = i
			break
		}
	}

	if len(lastRow) == 0 {
		return nil, 0, fmt.Errorf("no transaction rows found")
	}

	return lastRow, lastRowIndex, nil
}

// FindLastDateRow finds the last row with a date in the first column
func (r *BaseGoogleSheetsRepository) FindLastDateRow(rows [][]interface{}) ([]interface{}, error) {
	// Find header row
	headerRow := r.FindHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return nil, fmt.Errorf("no header row found")
	}

	// Find the last data row (scan from bottom up, starting after the header)
	var lastRow []interface{}
	for i := len(rows) - 1; i >= headerRow+1; i-- {
		if len(rows[i]) == 0 {
			continue
		}

		// Check if first column contains a date-like value
		firstCol := getCellValueAsString(rows[i][0])
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

// GetLastTransactionDate retrieves the date of the most recent transaction from the Google Sheets
func (r *BaseGoogleSheetsRepository) GetLastTransactionDate(ctx context.Context, sheetName string) (time.Time, error) {
	// Validate repository state
	if err := r.validateRepositoryState(); err != nil {
		return time.Time{}, err
	}

	// Get sheet data
	_, rows, err := r.GetSheetData(sheetName)
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
func (r *BaseGoogleSheetsRepository) AddTransactionDateRow(sheetName string, targetRow int, today time.Time, dateFormat string) error {
	// Find the sheet metadata for the specified sheet
	sheetMetadata := r.findSheetMetadata(sheetName)
	if sheetMetadata == nil || len(sheetMetadata.Headers) == 0 {
		return nil
	}

	// Format the date according to the detected format
	dateStr := today.Format(dateFormat)

	// The date column is ALWAYS at column index 0 (column A), regardless of where headers start
	// This is because the date row has no header - it's in column A before the other data columns
	cellRange := fmt.Sprintf("%s!A%d", sheetName, targetRow)

	valueRange := &sheets.ValueRange{
		Values: [][]interface{}{{dateStr}},
	}

	// Use Append instead of Update to allow creating new rows beyond sheet limits
	_, err := r.sheetsService.Spreadsheets.Values.Append(r.spreadsheetID, cellRange, valueRange).
		ValueInputOption("USER_ENTERED").
		Do()

	if err != nil {
		return fmt.Errorf("failed to add transaction date row: %w", err)
	}

	return nil
}

// AddDataRow adds data to the specified row
func (r *BaseGoogleSheetsRepository) AddDataRow(sheetName string, targetRow int, data map[string]interface{}) error {
	// Find the sheet metadata for the specified sheet
	sheetMetadata := r.findSheetMetadata(sheetName)
	if sheetMetadata == nil {
		return fmt.Errorf("sheet %s not found in metadata", sheetName)
	}

	// Create a row with empty cells
	row := make([]interface{}, len(sheetMetadata.Headers))

	for _, column := range sheetMetadata.Headers {
		if value, exists := data[column.ColumnName]; exists {
			// Convert value to uppercase string
			valueStr := fmt.Sprintf("%v", value)
			uppercaseValue := strings.ToUpper(valueStr)
			row[column.ColumnIndex] = uppercaseValue
		}
	}

	// Build A1 notation for the range
	cellRange := fmt.Sprintf("%s!A%d", sheetName, targetRow)

	valueRange := &sheets.ValueRange{
		Values: [][]interface{}{row},
	}

	_, err := r.sheetsService.Spreadsheets.Values.Append(r.spreadsheetID, cellRange, valueRange).
		ValueInputOption("USER_ENTERED").
		Do()

	if err != nil {
		return fmt.Errorf("failed to add data row: %w", err)
	}

	return nil
}

// AddDataRowWithColor adds data to the specified row with background color
func (r *BaseGoogleSheetsRepository) AddDataRowWithColor(sheetName string, targetRow int, data map[string]interface{}, cellColor string) error {
	// Find the sheet metadata for the specified sheet
	sheetMetadata := r.findSheetMetadata(sheetName)
	if sheetMetadata == nil {
		return fmt.Errorf("sheet %s not found in metadata", sheetName)
	}

	// Create a row with empty cells
	row := make([]interface{}, len(sheetMetadata.Headers))

	for _, column := range sheetMetadata.Headers {
		value, exists := data[column.ColumnName]
		if !exists {
			continue
		}

		// Check if this is a currency column that needs thousand separator
		if column.ColumnName == pkg.RevenueExpenseColumnWater || column.ColumnName == pkg.RevenueExpenseColumnSnackAndRice {
			// Format with thousand separator
			switch v := value.(type) {
			case int:
				row[column.ColumnIndex] = fmt.Sprintf("%d", v)
			case int64:
				row[column.ColumnIndex] = fmt.Sprintf("%d", v)
			case float64:
				row[column.ColumnIndex] = fmt.Sprintf("%.0f", v)
			default:
				row[column.ColumnIndex] = strings.ToUpper(fmt.Sprintf("%v", value))
			}
		} else {
			// Convert value to uppercase string
			valueStr := fmt.Sprintf("%v", value)
			uppercaseValue := strings.ToUpper(valueStr)
			row[column.ColumnIndex] = uppercaseValue
		}
	}

	// Build A1 notation for the range
	cellRange := fmt.Sprintf("%s!A%d", sheetName, targetRow)

	valueRange := &sheets.ValueRange{
		Values: [][]interface{}{row},
	}

	_, err := r.sheetsService.Spreadsheets.Values.Append(r.spreadsheetID, cellRange, valueRange).
		ValueInputOption("USER_ENTERED").
		Do()

	if err != nil {
		return fmt.Errorf("failed to add data row: %w", err)
	}

	// Apply color formatting if provided
	if cellColor != "" {
		// Convert hex color to RGB
		rgb, err := hexToRGB(cellColor)
		if err == nil {
			// Get the sheet ID
			sheetID, err := r.getSheetID(sheetName)
			if err != nil {
				return fmt.Errorf("failed to get sheet ID: %w", err)
			}

			firstColumn := sheetMetadata.Headers[0]

			// Apply background color to the first cell (STT column)
			requests := []*sheets.Request{
				{
					RepeatCell: &sheets.RepeatCellRequest{
						Range: &sheets.GridRange{
							SheetId:          sheetID,
							StartRowIndex:    int64(targetRow - 1),
							EndRowIndex:      int64(targetRow),
							StartColumnIndex: int64(firstColumn.ColumnIndex),
							EndColumnIndex:   int64(firstColumn.ColumnIndex + 1),
						},
						Cell: &sheets.CellData{
							UserEnteredFormat: &sheets.CellFormat{
								BackgroundColor: &sheets.Color{
									Red:   rgb[0],
									Green: rgb[1],
									Blue:  rgb[2],
								},
							},
						},
						Fields: "userEnteredFormat.backgroundColor",
					},
				},
			}

			batchUpdateRequest := &sheets.BatchUpdateSpreadsheetRequest{
				Requests: requests,
			}

			_, err = r.sheetsService.Spreadsheets.BatchUpdate(r.spreadsheetID, batchUpdateRequest).Do()
			if err != nil {
				return fmt.Errorf("failed to apply color formatting: %w", err)
			}
		}
	}

	return nil
}

// Close closes the repository and releases any cached resources
func (r *BaseGoogleSheetsRepository) Close() error {
	r.invalidateCache()
	return nil
}

// ForceCacheRefresh forces a cache refresh on the next access
func (r *BaseGoogleSheetsRepository) ForceCacheRefresh() {
	r.invalidateCache()
}

// Private helper methods

// extractSheetMetadata analyzes a sheet and extracts its metadata
func (r *BaseGoogleSheetsRepository) extractSheetMetadata(sheetName string) (*models.ExcelSheetMetadata, error) {
	// Get all rows from the sheet
	resp, err := r.sheetsService.Spreadsheets.Values.Get(r.spreadsheetID, sheetName).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows: %w", err)
	}

	rows := resp.Values
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

	for i, cell := range headerCells {
		cellValue := getCellValueAsString(cell)
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
func (r *BaseGoogleSheetsRepository) findSheetMetadata(sheetName string) *models.ExcelSheetMetadata {
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
func (r *BaseGoogleSheetsRepository) validateRepositoryState() error {
	if r.fileMetadata == nil {
		return fmt.Errorf("repository not initialized, call InitializeWithSpreadsheet first")
	}

	if len(r.fileMetadata.Sheets) == 0 {
		return fmt.Errorf("no sheets found in metadata")
	}

	if r.sheetsService == nil {
		return fmt.Errorf("sheets service not initialized")
	}

	return nil
}

// invalidateCache marks the cache as invalid, forcing a reload on next access
func (r *BaseGoogleSheetsRepository) invalidateCache() {
	r.cacheMutex.Lock()
	defer r.cacheMutex.Unlock()

	// Invalidate all data caches
	if r.cache != nil {
		r.cache.rows = nil
		r.cache.headerRow = -1
		r.cache.valid = false
	}
}

// getSheetID gets the sheet ID by sheet name
func (r *BaseGoogleSheetsRepository) getSheetID(sheetName string) (int64, error) {
	spreadsheet, err := r.sheetsService.Spreadsheets.Get(r.spreadsheetID).Do()
	if err != nil {
		return 0, fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	for _, sheet := range spreadsheet.Sheets {
		if sheet.Properties.Title == sheetName {
			return sheet.Properties.SheetId, nil
		}
	}

	return 0, fmt.Errorf("sheet %s not found", sheetName)
}

// hexToRGB converts hex color to RGB values (0.0-1.0)
func hexToRGB(hex string) ([]float64, error) {
	// Remove '#' if present
	hex = strings.TrimPrefix(hex, "#")

	if len(hex) != 6 {
		return nil, fmt.Errorf("invalid hex color: %s", hex)
	}

	var r, g, b int
	_, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	if err != nil {
		return nil, fmt.Errorf("failed to parse hex color: %w", err)
	}

	return []float64{
		float64(r) / 255.0,
		float64(g) / 255.0,
		float64(b) / 255.0,
	}, nil
}
