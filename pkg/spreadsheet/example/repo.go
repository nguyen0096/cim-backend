package integration

import (
	"cim-backend/internal/config"
	"cim-backend/internal/models"
	"cim-backend/pkg/spreadsheet"
	"context"
	"fmt"
	"strconv"
)

type InventoryGoogleSheetsRepository interface {
	// Connect initializes the connection to the Google Sheets inventory spreadsheet
	Connect(ctx context.Context, fileConfig spreadsheet.FileConfig) error
	// Close closes the connection to the spreadsheet
	Close(ctx context.Context) error
	// UpsertRow upserts a row in the inventory sheet
	UpsertRow(
		ctx context.Context,
		sheetInternalID spreadsheet.SheetInternalID,
		indexColHeaderStr spreadsheet.HeaderBranchStr,
		indexValue string,
		rowData map[spreadsheet.HeaderBranchStr]interface{},
	) error
	// ReadDataRows reads all data rows from a specified sheet
	ReadDataRows(ctx context.Context, sheetName string) ([][]string, error)
	// ReadInventoryData reads and maps inventory data from a sheet to DTO models
	ReadInventoryData(ctx context.Context, sheetInternalID spreadsheet.SheetInternalID) (*models.InventoryImportSheetDTO, error)
}

// InventoryColumnMapping defines the mapping from spreadsheet columns to DTO fields
type InventoryColumnMapping struct {
	ProductName     spreadsheet.HeaderBranchStr
	Unit            spreadsheet.HeaderBranchStr
	Price           spreadsheet.HeaderBranchStr
	InitialQuantity spreadsheet.HeaderBranchStr
	Days            []spreadsheet.HeaderBranchStr // Days 1-31
}

// inventoryGoogleSheetsRepository implements InventoryGoogleSheetsRepository
type inventoryGoogleSheetsRepository struct {
	ctx           context.Context
	file          *spreadsheet.File
	config        *config.GoogleSheetsConfig
	columnMapping *InventoryColumnMapping
}

// NewInventoryGoogleSheetsRepository creates a new InventoryGoogleSheetsRepository
func NewInventoryGoogleSheetsRepository(
	ctx context.Context,
	cfg *config.GoogleSheetsConfig,
) InventoryGoogleSheetsRepository {
	return &inventoryGoogleSheetsRepository{
		ctx:           ctx,
		config:        cfg,
		columnMapping: getInventoryColumnMapping(),
	}
}

// getInventoryColumnMapping returns the column mapping configuration
func getInventoryColumnMapping() *InventoryColumnMapping {
	// Create Days array (1-31)
	days := make([]spreadsheet.HeaderBranchStr, 31)
	for i := 0; i < 31; i++ {
		days[i] = spreadsheet.HeaderBranchStr(
			fmt.Sprintf("Ngày.%d.SL", i+1))
	}

	return &InventoryColumnMapping{
		ProductName:     "DIỄN GIẢI",
		Unit:            "ĐVT",
		Price:           "Đơn giá",
		InitialQuantity: "Tồn đầu.SL",
		Days:            days,
	}
}

// Connect initializes the connection to the Google Sheets inventory spreadsheet
func (r *inventoryGoogleSheetsRepository) Connect(ctx context.Context, fileConfig spreadsheet.FileConfig) error {
	// Create Google Sheets provider with spreadsheet ID and service account path
	prov := spreadsheet.NewGoogleSheetsFileProvider(r.config.InventorySpreadsheetID, r.config.ServiceAccountPath)

	// Create file with provider
	file, err := spreadsheet.NewFile(fileConfig, prov)
	if err != nil {
		return fmt.Errorf("failed to create Google Sheets file: %w", err)
	}

	// Connect to the spreadsheet
	if err := file.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to Google Sheets: %w", err)
	}

	r.file = file
	return nil
}

// Close closes the connection to the spreadsheet
func (r *inventoryGoogleSheetsRepository) Close(ctx context.Context) error {
	if r.file == nil {
		return nil
	}
	return r.file.Close(ctx)
}

// UpsertRow upserts a row in the inventory sheet
func (r *inventoryGoogleSheetsRepository) UpsertRow(
	ctx context.Context,
	sheetInternalID spreadsheet.SheetInternalID,
	indexColHeaderStr spreadsheet.HeaderBranchStr,
	indexValue string,
	rowData map[spreadsheet.HeaderBranchStr]interface{},
) error {
	if r.file == nil {
		return fmt.Errorf("file not connected")
	}
	return r.file.UpsertRow(ctx, sheetInternalID, indexColHeaderStr, indexValue, rowData)
}

// ReadDataRows reads all data rows from a specified sheet
func (r *inventoryGoogleSheetsRepository) ReadDataRows(ctx context.Context, sheetName string) ([][]string, error) {
	if r.file == nil {
		return nil, fmt.Errorf("file not connected")
	}

	// Get all rows from the sheet using the provider
	rows, err := r.file.Provider.GetRows(ctx, sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read rows from sheet %s: %w", sheetName, err)
	}

	return rows, nil
}

// ReadInventoryData reads and maps inventory data from a sheet to DTO models
func (r *inventoryGoogleSheetsRepository) ReadInventoryData(ctx context.Context, sheetInternalID spreadsheet.SheetInternalID) (*models.InventoryImportSheetDTO, error) {
	if r.file == nil {
		return nil, fmt.Errorf("file not connected")
	}

	if r.columnMapping == nil {
		return nil, fmt.Errorf("column mapping not configured")
	}

	// Get the sheet by internal ID
	sheet, ok := r.file.Sheets[sheetInternalID]
	if !ok {
		return nil, fmt.Errorf("sheet with internal ID %s not found", sheetInternalID)
	}

	// Get all rows from the sheet
	rows, err := r.file.Provider.GetRows(ctx, sheet.SheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read rows from sheet %s: %w", sheet.SheetName, err)
	}

	// Parse the data rows into DTO
	result := &models.InventoryImportSheetDTO{
		Items: []models.InventoryImportDTO{},
	}

	// Iterate through data rows (starting from DataStartRow)
	for rowIndex := sheet.DataStartRow - 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]

		// Skip empty rows
		if isEmptyRow(row) {
			continue
		}

		item := models.InventoryImportDTO{
			QuantityByDay: make(map[int]float64),
		}

		// Map Product Name
		if r.columnMapping.ProductName != "" {
			col, err := sheet.GetColByExactHeaders(sheetInternalID, r.columnMapping.ProductName.ToBranch())
			if err == nil && col-1 < len(row) {
				item.ProductName = row[col-1]
			}
		}

		// Map Unit
		if r.columnMapping.Unit != "" {
			col, err := sheet.GetColByExactHeaders(sheetInternalID, r.columnMapping.Unit.ToBranch())
			if err == nil && col-1 < len(row) {
				item.Unit = row[col-1]
			}
		}

		// Map Price
		if r.columnMapping.Price != "" {
			col, err := sheet.GetColByExactHeaders(sheetInternalID, r.columnMapping.Price.ToBranch())
			if err == nil && col-1 < len(row) {
				if price, err := parseFloat(row[col-1]); err == nil {
					item.Price = price
				}
			}
		}

		// Map Initial Quantity
		if r.columnMapping.InitialQuantity != "" {
			col, err := sheet.GetColByExactHeaders(sheetInternalID, r.columnMapping.InitialQuantity.ToBranch())
			if err == nil && col-1 < len(row) {
				if qty, err := parseFloat(row[col-1]); err == nil {
					item.InitialQuantity = qty
				}
			}
		}

		// Map Quantity by Days (1-31)
		for day := 1; day <= 31; day++ {
			dayIndex := day - 1
			if dayIndex < len(r.columnMapping.Days) && r.columnMapping.Days[dayIndex] != "" {
				col, err := sheet.GetColByExactHeaders(sheetInternalID, r.columnMapping.Days[dayIndex].ToBranch())
				if err == nil && col-1 < len(row) {
					if qty, err := parseFloat(row[col-1]); err == nil {
						item.QuantityByDay[day] = qty
					}
				}
			}
		}

		result.Items = append(result.Items, item)
	}

	return result, nil
}

// Helper function to check if a row is empty
func isEmptyRow(row []string) bool {
	for _, cell := range row {
		if cell != "" {
			return false
		}
	}
	return true
}

// Helper function to parse float from string
func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}
