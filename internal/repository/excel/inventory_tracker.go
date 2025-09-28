package excel

import (
	"context"
	"fmt"
	"import-export-backend/internal/models"
	"import-export-backend/pkg"
	"strings"
	"time"
)

// InventoryTrackerExcelRepository handles data access for inventory tracking Excel operations
type InventoryTrackerExcelRepository interface {
	InitializeWithFile(ctx context.Context, filePath string) error
	AddInventoryEntry(ctx context.Context, sheetName string, inventoryData map[string]interface{}) error
	GetLastInventoryEntry(ctx context.Context, sheetName string) (map[string]interface{}, error)
	GetLastTransactionDate(ctx context.Context, sheetName string) (time.Time, error)
	DeleteLastNRows(ctx context.Context, sheetName string, n int) error
	GetSchema(ctx context.Context) *models.FileMetadata
	Close() error
	ForceCacheRefresh()
}

// inventoryTrackerExcelRepository implements InventoryTrackerExcelRepository
type inventoryTrackerExcelRepository struct {
	BaseExcelRepository
}

// NewInventoryTrackerExcelRepository creates a new InventoryTrackerExcelRepository
func NewInventoryTrackerExcelRepository() InventoryTrackerExcelRepository {
	return &inventoryTrackerExcelRepository{}
}

// InitializeWithFile initializes the repository with the inventory tracking Excel file
func (r *inventoryTrackerExcelRepository) InitializeWithFile(ctx context.Context, filePath string) error {
	return r.BaseExcelRepository.InitializeWithFile(ctx, filePath, models.FileTypeInventoryTracker)
}

// AddInventoryEntry adds a new inventory entry to the Excel file
func (r *inventoryTrackerExcelRepository) AddInventoryEntry(ctx context.Context, sheetName string, inventoryData map[string]interface{}) error {
	// Validate inventory data
	if err := ValidateData(inventoryData); err != nil {
		return err
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

	// Add transaction date row if needed
	if !isTodayExists {
		if err := r.AddTransactionDateRow(file, sheetName, targetRow, today, detectedDateFormat); err != nil {
			return fmt.Errorf("failed to add transaction date row: %w", err)
		}
		targetRow++
	}

	// Add inventory data row
	if err := r.AddDataRow(file, sheetName, targetRow, inventoryData); err != nil {
		return fmt.Errorf("failed to add inventory data row: %w", err)
	}

	// Save the file
	if err := file.Save(); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	// Invalidate cache after saving to ensure next read gets fresh data
	r.ForceCacheRefresh()

	return nil
}

// GetLastInventoryEntry retrieves the most recent inventory entry from the Excel file
func (r *inventoryTrackerExcelRepository) GetLastInventoryEntry(ctx context.Context, sheetName string) (map[string]interface{}, error) {
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

	// Build the inventory data map using the headers
	inventoryData := make(map[string]interface{})

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
				inventoryData[header.ColumnName] = cellValue
			}
		}
	}

	return inventoryData, nil
}

// GetLastTransactionDate retrieves the date of the most recent transaction from the Excel file
func (r *inventoryTrackerExcelRepository) GetLastTransactionDate(ctx context.Context, sheetName string) (time.Time, error) {
	return r.BaseExcelRepository.GetLastTransactionDate(ctx, sheetName)
}

// DeleteLastNRows removes the last n data rows from the Excel file
func (r *inventoryTrackerExcelRepository) DeleteLastNRows(ctx context.Context, sheetName string, n int) error {
	return r.BaseExcelRepository.DeleteLastNRows(ctx, sheetName, n)
}

// GetSchema returns the Excel file schema
func (r *inventoryTrackerExcelRepository) GetSchema(ctx context.Context) *models.FileMetadata {
	return r.BaseExcelRepository.GetSchema(ctx)
}

// Close closes the repository and releases any cached resources
func (r *inventoryTrackerExcelRepository) Close() error {
	return r.BaseExcelRepository.Close()
}

// ForceCacheRefresh forces a cache refresh on the next file access
func (r *inventoryTrackerExcelRepository) ForceCacheRefresh() {
	r.BaseExcelRepository.ForceCacheRefresh()
}
