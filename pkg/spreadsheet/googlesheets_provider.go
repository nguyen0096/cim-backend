package spreadsheet

import (
	"context"
	"fmt"
	"sync"
	"time"


	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type GoogleSheetsFileProvider struct {
	ctx                context.Context
	sheetsService      *sheets.Service
	spreadsheetID      string
	serviceAccountPath string
	cache              map[string]cachedData
	cacheMutex         sync.RWMutex
}

type cachedData struct {
	rows      [][]string
	timestamp time.Time
}

const cacheTTL = 1 * time.Minute

// NewGoogleSheetsFileProvider creates a new Google Sheets file provider
func NewGoogleSheetsFileProvider(serviceAccountPath string) *GoogleSheetsFileProvider {
	return &GoogleSheetsFileProvider{
		serviceAccountPath: serviceAccountPath,
		cache:              make(map[string]cachedData),
	}
}

// Connect initializes the Google Sheets service and sets the spreadsheet ID
// filePath should be either the full Google Sheets URL or just the spreadsheet ID
func (g *GoogleSheetsFileProvider) Connect(ctx context.Context, filePath string) error {
	g.ctx = ctx

	// Extract spreadsheet ID from URL if it's a full URL
	spreadsheetID := extractSpreadsheetID(filePath)
	if spreadsheetID == "" {
		return fmt.Errorf("google sheets provider: invalid spreadsheet URL or ID: %s", filePath)
	}
	g.spreadsheetID = spreadsheetID

	// Create Google Sheets service with service account credentials
	srv, err := sheets.NewService(ctx, option.WithCredentialsFile(g.serviceAccountPath))
	if err != nil {
		return fmt.Errorf("google sheets provider: failed to create sheets service: %w", err)
	}
	g.sheetsService = srv

	return nil
}

// Close does nothing for Google Sheets as there's no persistent connection
func (g *GoogleSheetsFileProvider) Close(ctx context.Context) error {
	// Clear cache
	g.cacheMutex.Lock()
	g.cache = make(map[string]cachedData)
	g.cacheMutex.Unlock()
	return nil
}

// GetSheetIndex returns the index of a sheet by name
func (g *GoogleSheetsFileProvider) GetSheetIndex(ctx context.Context, name string) (int, error) {
	spreadsheetData, err := g.sheetsService.Spreadsheets.Get(g.spreadsheetID).Do()
	if err != nil {
		return -1, fmt.Errorf("google sheets provider: failed to get spreadsheet: %w", err)
	}

	for idx, sheet := range spreadsheetData.Sheets {
		if sheet.Properties.Title == name {
			return idx, nil
		}
	}

	return -1, fmt.Errorf("google sheets provider: sheet %s not found", name)
}

// InsertRows inserts n rows at the specified position
func (g *GoogleSheetsFileProvider) InsertRows(ctx context.Context, sheet string, row, n int) error {
	// Get the sheet ID
	sheetID, err := g.getSheetID(ctx, sheet)
	if err != nil {
		return fmt.Errorf("google sheets provider: failed to get sheet ID: %w", err)
	}

	// Create insert dimension request
	req := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{
				InsertDimension: &sheets.InsertDimensionRequest{
					Range: &sheets.DimensionRange{
						SheetId:    sheetID,
						Dimension:  "ROWS",
						StartIndex: int64(row - 1), // Google Sheets API uses 0-based indexing
						EndIndex:   int64(row - 1 + n),
					},
				},
			},
		},
	}

	_, err = g.sheetsService.Spreadsheets.BatchUpdate(g.spreadsheetID, req).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("google sheets provider: failed to insert rows: %w", err)
	}

	// Invalidate cache for this sheet
	g.invalidateCache(sheet)

	return nil
}

// SetCellValue sets the value of a cell
func (g *GoogleSheetsFileProvider) SetCellValue(ctx context.Context, sheet, cell string, value interface{}) error {
	cellRange := fmt.Sprintf("%s!%s", sheet, cell)

	valueRange := &sheets.ValueRange{
		Values: [][]interface{}{{value}},
	}

	_, err := g.sheetsService.Spreadsheets.Values.Update(
		g.spreadsheetID,
		cellRange,
		valueRange,
	).ValueInputOption("USER_ENTERED").Context(ctx).Do()

	if err != nil {
		return fmt.Errorf("google sheets provider: failed to set cell value: %w", err)
	}

	// Invalidate cache for this sheet
	g.invalidateCache(sheet)

	return nil
}

// GetCellValue gets the value of a cell
func (g *GoogleSheetsFileProvider) GetCellValue(ctx context.Context, sheet, cell string) (string, error) {
	cellRange := fmt.Sprintf("%s!%s", sheet, cell)

	resp, err := g.sheetsService.Spreadsheets.Values.Get(g.spreadsheetID, cellRange).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("google sheets provider: failed to get cell value: %w", err)
	}

	if len(resp.Values) == 0 || len(resp.Values[0]) == 0 {
		return "", nil
	}

	// Convert interface{} to string
	if val, ok := resp.Values[0][0].(string); ok {
		return val, nil
	}
	return fmt.Sprintf("%v", resp.Values[0][0]), nil
}

// GetRows gets all rows in a sheet with caching
func (g *GoogleSheetsFileProvider) GetRows(ctx context.Context, sheet string) ([][]string, error) {
	// Check cache first
	g.cacheMutex.RLock()
	if cached, ok := g.cache[sheet]; ok {
		if time.Since(cached.timestamp) < cacheTTL {
			g.cacheMutex.RUnlock()
			return cached.rows, nil
		}
	}
	g.cacheMutex.RUnlock()

	// Fetch from API
	rangeStr := fmt.Sprintf("%s!A1:ZZ", sheet) // Adjust range as needed

	resp, err := g.sheetsService.Spreadsheets.Values.Get(g.spreadsheetID, rangeStr).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("google sheets provider: failed to get rows: %w", err)
	}

	// Convert [][]interface{} to [][]string
	rows := make([][]string, len(resp.Values))
	for i, row := range resp.Values {
		rows[i] = make([]string, len(row))
		for j, cell := range row {
			if cell == nil {
				rows[i][j] = ""
			} else if str, ok := cell.(string); ok {
				rows[i][j] = str
			} else {
				rows[i][j] = fmt.Sprintf("%v", cell)
			}
		}
	}

	// Update cache
	g.cacheMutex.Lock()
	g.cache[sheet] = cachedData{
		rows:      rows,
		timestamp: time.Now(),
	}
	g.cacheMutex.Unlock()

	return rows, nil
}

// GetMergeCells gets all merged cells in a sheet
func (g *GoogleSheetsFileProvider) GetMergeCells(ctx context.Context, sheet string) ([]MergeCell, error) {
	spreadsheetData, err := g.sheetsService.Spreadsheets.Get(g.spreadsheetID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("google sheets provider: failed to get spreadsheet: %w", err)
	}

	// Find the sheet
	var targetSheet *sheets.Sheet
	for _, s := range spreadsheetData.Sheets {
		if s.Properties.Title == sheet {
			targetSheet = s
			break
		}
	}

	if targetSheet == nil {
		return nil, fmt.Errorf("google sheets provider: sheet %s not found", sheet)
	}

	// Extract merge cells
	var mergeCells []MergeCell
	if targetSheet.Merges != nil {
		for _, merge := range targetSheet.Merges {
			mergeCells = append(mergeCells, MergeCell{
				StartRow: int(merge.StartRowIndex) + 1,    // Convert to 1-based
				StartCol: int(merge.StartColumnIndex) + 1, // Convert to 1-based
				EndRow:   int(merge.EndRowIndex),          // EndRow is exclusive in API, but we want inclusive
				EndCol:   int(merge.EndColumnIndex),       // EndCol is exclusive in API, but we want inclusive
			})
		}
	}

	return mergeCells, nil
}

// Helper functions

func (g *GoogleSheetsFileProvider) getSheetID(ctx context.Context, sheetName string) (int64, error) {
	spreadsheetData, err := g.sheetsService.Spreadsheets.Get(g.spreadsheetID).Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	for _, sheet := range spreadsheetData.Sheets {
		if sheet.Properties.Title == sheetName {
			return sheet.Properties.SheetId, nil
		}
	}

	return 0, fmt.Errorf("sheet %s not found", sheetName)
}

func (g *GoogleSheetsFileProvider) invalidateCache(sheet string) {
	g.cacheMutex.Lock()
	delete(g.cache, sheet)
	g.cacheMutex.Unlock()
}

// extractSpreadsheetID extracts the spreadsheet ID from a URL or returns the input if it's already an ID
func extractSpreadsheetID(input string) string {
	// If it's a URL, extract the ID
	// Format: https://docs.google.com/spreadsheets/d/{SPREADSHEET_ID}/edit...
	const prefix = "docs.google.com/spreadsheets/d/"

	if idx := indexOf(input, prefix); idx != -1 {
		start := idx + len(prefix)
		end := start
		for end < len(input) && input[end] != '/' && input[end] != '#' && input[end] != '?' {
			end++
		}
		return input[start:end]
	}

	// If it doesn't contain the URL pattern, assume it's already an ID
	return input
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
