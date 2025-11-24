package googlesheets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// TestListAvailableSheets lists all sheets in the inventory spreadsheet for debugging
func TestListAvailableSheets(t *testing.T) {
	t.Skip("Skipping Google Sheets test on CI/CD")

	t.Run("TestListAvailableSheets", func(t *testing.T) {
		// Adjust service account path to be relative to test directory
		serviceAccountFilePath := GOOGLE_SERVICE_ACCOUNT_FILE_PATH

		// Create context
		ctx := context.Background()

		// Create Google Sheets service directly
		srv, err := sheets.NewService(ctx, option.WithCredentialsFile(serviceAccountFilePath))
		require.NoError(t, err, "Failed to create sheets service")

		// Get spreadsheet metadata using the Fields parameter to limit the response
		spreadsheet, err := srv.Spreadsheets.Get(TEST_INVENTORY_SPREADSHEET_ID).
			Fields("properties(title),sheets(properties(title,sheetId))").
			Do()
		require.NoError(t, err, "Failed to get spreadsheet")

		t.Logf("Spreadsheet title: %s", spreadsheet.Properties.Title)
		t.Logf("Available sheets:")
		for i, sheet := range spreadsheet.Sheets {
			t.Logf("  %d. %s (ID: %d)", i+1, sheet.Properties.Title, sheet.Properties.SheetId)
		}
	})
}
