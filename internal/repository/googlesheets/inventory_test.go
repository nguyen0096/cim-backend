package googlesheets

import (
	"context"
	"testing"

	"cim-backend/internal/config"
	"cim-backend/pkg/spreadsheet"

	"github.com/stretchr/testify/require"
)

func TestNewInventoryGoogleSheetsRepository(t *testing.T) {
	// Load config with .env file path
	cfg := config.Load("../../../.env")

	// Debug: Log the loaded config values
	t.Logf("ServiceAccountPath: '%s'", cfg.GoogleSheets.ServiceAccountPath)
	t.Logf("InventorySpreadsheetID: '%s'", cfg.GoogleSheets.InventorySpreadsheetID)

	// Check if Google Sheets config is set
	if cfg.GoogleSheets.ServiceAccountPath == "" {
		t.Skip("Skipping test: GOOGLE_SERVICE_ACCOUNT not set")
	}

	if cfg.GoogleSheets.InventorySpreadsheetID == "" {
		t.Skip("Skipping test: INVENTORY_SPREADSHEET_ID not set")
	}

	// Create context
	ctx := context.Background()

	// Test: Create new repository
	repo := NewInventoryGoogleSheetsRepository(ctx, &cfg.GoogleSheets)

	// Assertions
	require.NotNil(t, repo, "Repository should not be nil")
}

func TestInventoryGoogleSheetsRepository_Connect(t *testing.T) {
	// Load config with .env file path
	cfg := config.Load("../../../.env")

	// Check if Google Sheets config is set
	if cfg.GoogleSheets.ServiceAccountPath == "" {
		t.Skip("Skipping test: GOOGLE_SERVICE_ACCOUNT not set")
	}

	if cfg.GoogleSheets.InventorySpreadsheetID == "" {
		t.Skip("Skipping test: INVENTORY_SPREADSHEET_ID not set")
	}

	// Create context
	ctx := context.Background()

	// Create repository
	repo := NewInventoryGoogleSheetsRepository(ctx, &cfg.GoogleSheets)
	require.NotNil(t, repo)

	// Create a minimal FileConfig for testing connectivity
	// Note: FilePath is no longer used for Google Sheets since spreadsheet ID
	// is now passed to the provider constructor, but we keep it for validation
	fileConfig := spreadsheet.FileConfig{
		FilePath: cfg.GoogleSheets.InventorySpreadsheetID,
		SheetConfigs: []spreadsheet.SheetConfig{
			{
				InternalID:     "test_sheet",
				NamePattern:    "Sheet1", // Adjust this to match an actual sheet name
				HeaderStartRow: 1,
				HeaderStartCol: 1,
				HeaderHeight:   1,
			},
		},
	}

	// Test: Connect to the spreadsheet
	err := repo.Connect(ctx, fileConfig)
	require.NoError(t, err, "Connect should not return an error")

	// Setup cleanup
	t.Cleanup(func() {
		err := repo.Close(ctx)
		if err != nil {
			t.Logf("Warning: Error closing repository: %v", err)
		}
	})
}

func TestInventoryGoogleSheetsRepository_ConnectAndClose(t *testing.T) {
	// Load config with .env file path
	cfg := config.Load("../../../.env")

	// Check if Google Sheets config is set
	if cfg.GoogleSheets.ServiceAccountPath == "" {
		t.Skip("Skipping test: GOOGLE_SERVICE_ACCOUNT not set")
	}

	if cfg.GoogleSheets.InventorySpreadsheetID == "" {
		t.Skip("Skipping test: INVENTORY_SPREADSHEET_ID not set")
	}

	// Create context
	ctx := context.Background()

	// Create repository
	repo := NewInventoryGoogleSheetsRepository(ctx, &cfg.GoogleSheets)
	require.NotNil(t, repo)

	// Create a minimal FileConfig
	fileConfig := spreadsheet.FileConfig{
		FilePath: cfg.GoogleSheets.InventorySpreadsheetID,
		SheetConfigs: []spreadsheet.SheetConfig{
			{
				InternalID:     "test_sheet",
				NamePattern:    "Sheet1",
				HeaderStartRow: 1,
				HeaderStartCol: 1,
				HeaderHeight:   1,
			},
		},
	}

	// Test: Connect
	err := repo.Connect(ctx, fileConfig)
	require.NoError(t, err, "Connect should not return an error")

	// Test: Close
	err = repo.Close(ctx)
	require.NoError(t, err, "Close should not return an error")

	// Test: Close again (should handle gracefully)
	err = repo.Close(ctx)
	require.NoError(t, err, "Close should handle being called multiple times")
}
