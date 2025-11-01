package googlesheets

import (
	"context"
	"testing"
	"time"

	"cim-backend/internal/config"
	"cim-backend/pkg/spreadsheet"

	"github.com/stretchr/testify/require"
)

const (
	// Test configuration constants
	testEnvFilePath          = "../../../.env"
	testServiceAccountPath   = "../../../google-service-account.json"
	testSheetInternalID      = "inventory_sheet"
	testSheetNamePattern     = "THANG {MM}"
	testHeaderStartRow       = 5
	testHeaderStartCol       = 1
	testHeaderHeight         = 3
	testMaxDaysToShowInLog   = 5
	testFloatPrecision       = 2
)

const (
	// Test skip messages
	skipMessageServiceAccount = "Skipping test: GOOGLE_SERVICE_ACCOUNT not set"
	skipMessageSpreadsheetID  = "Skipping test: INVENTORY_SPREADSHEET_ID not set"
)

func TestNewInventoryGoogleSheetsRepository(t *testing.T) {
	// Load config with .env file path
	cfg := config.Load(testEnvFilePath)

	// Debug: Log the loaded config values
	t.Logf("ServiceAccountPath: '%s'", cfg.GoogleSheets.ServiceAccountPath)
	t.Logf("InventorySpreadsheetID: '%s'", cfg.GoogleSheets.InventorySpreadsheetID)

	// Check if Google Sheets config is set
	if cfg.GoogleSheets.ServiceAccountPath == "" {
		t.Skip(skipMessageServiceAccount)
	}

	if cfg.GoogleSheets.InventorySpreadsheetID == "" {
		t.Skip(skipMessageSpreadsheetID)
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
	cfg := config.Load(testEnvFilePath)

	// Check if Google Sheets config is set
	if cfg.GoogleSheets.ServiceAccountPath == "" {
		t.Skip(skipMessageServiceAccount)
	}

	if cfg.GoogleSheets.InventorySpreadsheetID == "" {
		t.Skip(skipMessageSpreadsheetID)
	}

	// Adjust service account path to be relative to test directory
	cfg.GoogleSheets.ServiceAccountPath = testServiceAccountPath

	// Create context
	ctx := context.Background()

	// Create repository
	repo := NewInventoryGoogleSheetsRepository(ctx, &cfg.GoogleSheets)
	require.NotNil(t, repo)

	// Create FileConfig with proper sheet configuration using dynamic time-based parameters
	fileConfig := spreadsheet.FileConfig{
		FilePath: cfg.GoogleSheets.InventorySpreadsheetID,
		SheetConfigs: []spreadsheet.SheetConfig{
			{
				InternalID:        testSheetInternalID,
				NamePattern:       testSheetNamePattern,
				NameParamResolver: spreadsheet.WithSheetNameTimeParams(time.Now()),
				HeaderStartRow:    testHeaderStartRow,
				HeaderStartCol:    testHeaderStartCol,
				HeaderHeight:      testHeaderHeight,
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
	cfg := config.Load(testEnvFilePath)

	// Check if Google Sheets config is set
	if cfg.GoogleSheets.ServiceAccountPath == "" {
		t.Skip(skipMessageServiceAccount)
	}

	if cfg.GoogleSheets.InventorySpreadsheetID == "" {
		t.Skip(skipMessageSpreadsheetID)
	}

	// Adjust service account path to be relative to test directory
	cfg.GoogleSheets.ServiceAccountPath = testServiceAccountPath

	// Create context
	ctx := context.Background()

	// Create repository
	repo := NewInventoryGoogleSheetsRepository(ctx, &cfg.GoogleSheets)
	require.NotNil(t, repo)

	// Create FileConfig with proper sheet configuration using dynamic time-based parameters
	fileConfig := spreadsheet.FileConfig{
		FilePath: cfg.GoogleSheets.InventorySpreadsheetID,
		SheetConfigs: []spreadsheet.SheetConfig{
			{
				InternalID:        testSheetInternalID,
				NamePattern:       testSheetNamePattern,
				NameParamResolver: spreadsheet.WithSheetNameTimeParams(time.Now()),
				HeaderStartRow:    testHeaderStartRow,
				HeaderStartCol:    testHeaderStartCol,
				HeaderHeight:      testHeaderHeight,
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

func TestInventoryGoogleSheetsRepository_ReadInventoryData(t *testing.T) {
	// Load config with .env file path
	cfg := config.Load(testEnvFilePath)

	// Check if Google Sheets config is set
	if cfg.GoogleSheets.ServiceAccountPath == "" {
		t.Skip(skipMessageServiceAccount)
	}

	if cfg.GoogleSheets.InventorySpreadsheetID == "" {
		t.Skip(skipMessageSpreadsheetID)
	}

	// Adjust service account path to be relative to test directory
	cfg.GoogleSheets.ServiceAccountPath = testServiceAccountPath

	// Create context
	ctx := context.Background()

	// Create repository
	repo := NewInventoryGoogleSheetsRepository(ctx, &cfg.GoogleSheets)
	require.NotNil(t, repo)

	// Create FileConfig with proper sheet configuration using dynamic time-based parameters
	// Use November for testing since test data is in November
	testTime := time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)
	fileConfig := spreadsheet.FileConfig{
		FilePath: cfg.GoogleSheets.InventorySpreadsheetID,
		SheetConfigs: []spreadsheet.SheetConfig{
			{
				InternalID:        testSheetInternalID,
				NamePattern:       testSheetNamePattern,
				NameParamResolver: spreadsheet.WithSheetNameTimeParams(testTime),
				HeaderStartRow:    testHeaderStartRow,
				HeaderStartCol:    testHeaderStartCol,
				HeaderHeight:      testHeaderHeight,
			},
		},
	}

	// Test: Connect to the spreadsheet
	t.Logf("Looking for sheet with pattern: %s (resolved to: %s)", testSheetNamePattern,
		fileConfig.SheetConfigs[0].NamePattern.Parse(fileConfig.SheetConfigs[0].GetResolvedParams()))

	err := repo.Connect(ctx, fileConfig)
	require.NoError(t, err, "Connect should not return an error")

	// Setup cleanup
	defer func() {
		spreadsheet.LogAPIRequestSummary()
		err := repo.Close(ctx)
		require.NoError(t, err, "Close should not return an error")
	}()

	// Test: Read inventory data
	data, err := repo.ReadInventoryData(ctx, testSheetInternalID)
	require.NoError(t, err, "ReadInventoryData should not return an error")
	require.NotNil(t, data, "Data should not be nil")

	// Log the number of items retrieved
	t.Logf("Retrieved %d inventory items", len(data.Items))

	// If there are items, log details of the first few items
	if len(data.Items) > 0 {
		t.Logf("First item:")
		t.Logf("  Product Name: %s", data.Items[0].ProductName)
		t.Logf("  Unit: %s", data.Items[0].Unit)
		t.Logf("  Price: %.*f", testFloatPrecision, data.Items[0].Price)
		t.Logf("  Initial Quantity: %.*f", testFloatPrecision, data.Items[0].InitialQuantity)
		t.Logf("  Quantity by days (showing first %d days):", testMaxDaysToShowInLog)
		for day := 1; day <= testMaxDaysToShowInLog; day++ {
			if qty, ok := data.Items[0].QuantityByDay[day]; ok {
				t.Logf("    Day %d: %.*f", day, testFloatPrecision, qty)
			}
		}

		if len(data.Items) > 1 {
			t.Logf("\nSecond item:")
			t.Logf("  Product Name: %s", data.Items[1].ProductName)
			t.Logf("  Unit: %s", data.Items[1].Unit)
			t.Logf("  Price: %.*f", testFloatPrecision, data.Items[1].Price)
		}
	}
}
