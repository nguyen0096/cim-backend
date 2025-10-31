package googlesheets

import (
	"context"
	"testing"

	"cim-backend/internal/config"

	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// TestListAvailableSheets lists all sheets in the inventory spreadsheet for debugging
func TestListAvailableSheets(t *testing.T) {
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

	// Create Google Sheets service directly
	srv, err := sheets.NewService(ctx, option.WithCredentialsFile(cfg.GoogleSheets.ServiceAccountPath))
	require.NoError(t, err, "Failed to create sheets service")

	// Get spreadsheet metadata using the Fields parameter to limit the response
	spreadsheet, err := srv.Spreadsheets.Get(cfg.GoogleSheets.InventorySpreadsheetID).
		Fields("properties(title),sheets(properties(title,sheetId))").
		Do()
	require.NoError(t, err, "Failed to get spreadsheet")

	t.Logf("Spreadsheet title: %s", spreadsheet.Properties.Title)
	t.Logf("Available sheets:")
	for i, sheet := range spreadsheet.Sheets {
		t.Logf("  %d. %s (ID: %d)", i+1, sheet.Properties.Title, sheet.Properties.SheetId)
	}
}
