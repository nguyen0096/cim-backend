package googlesheets

import (
	"context"
	"fmt"

	"cim-backend/internal/config"
	"cim-backend/pkg/spreadsheet"
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
		indexColHeaderStr spreadsheet.TreeHeaderStr,
		indexValue string,
		rowData map[spreadsheet.TreeHeaderStr]interface{},
	) error
}

// inventoryGoogleSheetsRepository implements InventoryGoogleSheetsRepository
type inventoryGoogleSheetsRepository struct {
	ctx    context.Context
	file   *spreadsheet.File
	config *config.GoogleSheetsConfig
}

// NewInventoryGoogleSheetsRepository creates a new InventoryGoogleSheetsRepository
func NewInventoryGoogleSheetsRepository(
	ctx context.Context,
	cfg *config.GoogleSheetsConfig,
) InventoryGoogleSheetsRepository {
	return &inventoryGoogleSheetsRepository{
		ctx:    ctx,
		config: cfg,
	}
}

// Connect initializes the connection to the Google Sheets inventory spreadsheet
func (r *inventoryGoogleSheetsRepository) Connect(ctx context.Context, fileConfig spreadsheet.FileConfig) error {
	// Create Google Sheets provider
	prov := spreadsheet.NewGoogleSheetsFileProvider(r.config.ServiceAccountPath)

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
	indexColHeaderStr spreadsheet.TreeHeaderStr,
	indexValue string,
	rowData map[spreadsheet.TreeHeaderStr]interface{},
) error {
	if r.file == nil {
		return fmt.Errorf("file not connected")
	}
	return r.file.UpsertRow(ctx, sheetInternalID, indexColHeaderStr, indexValue, rowData)
}
