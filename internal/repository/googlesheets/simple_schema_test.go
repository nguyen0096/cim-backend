package googlesheets

import (
	"context"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
)

// TestSimpleSchemaRead is a simple test that loads env vars and reads spreadsheet schema
func TestSimpleSchemaRead(t *testing.T) {
	// Load environment variables from .env file
	err := godotenv.Load("../../../.env")
	require.NoError(t, err, "Failed to load .env file")

	// Get environment variables
	serviceAccountFilePath := os.Getenv("GOOGLE_SERVICE_ACCOUNT")
	spreadsheetID := os.Getenv("TEST_REVENUE_EXPENSE_SPREADSHEET_ID")

	// Verify env vars are loaded
	require.NotEmpty(t, serviceAccountFilePath, "GOOGLE_SERVICE_ACCOUNT is not set in .env")
	require.NotEmpty(t, spreadsheetID, "TEST_REVENUE_EXPENSE_SPREADSHEET_ID is not set in .env")

	t.Logf("✅ Environment variables loaded successfully")
	t.Logf("   API Key length: %d", len(serviceAccountFilePath))
	t.Logf("   Spreadsheet ID: %s", spreadsheetID)

	// Create repository instance
	ctx := context.Background()
	repo := NewRevenueExpenseGoogleSheetsRepository()

	t.Cleanup(func() {
		repo.Close()
	})

	// Initialize with spreadsheet
	err = repo.InitializeWithSpreadsheet(ctx, serviceAccountFilePath, spreadsheetID)
	require.NoError(t, err, "Failed to initialize repository with spreadsheet")

	t.Logf("✅ Repository initialized successfully")

	// Read the schema
	schema := repo.GetSchema(ctx)
	require.NotNil(t, schema, "Schema should not be nil")

	// Display schema information
	t.Logf("✅ Schema read successfully!")
	t.Logf("\n📊 Spreadsheet Schema:")
	t.Logf("   File Path: %s", schema.FilePath)
	t.Logf("   File Type: %s", schema.FileType)
	t.Logf("   Number of Sheets: %d", len(schema.Sheets))

	for i, sheet := range schema.Sheets {
		t.Logf("\n   Sheet %d: %s", i+1, sheet.SheetName)
		t.Logf("      Number of Columns: %d", len(sheet.Headers))
		t.Logf("      Columns:")
		for _, header := range sheet.Headers {
			t.Logf("         - %s (%s)", header.ColumnName, header.DataType)
		}
	}

	t.Logf("\n✅ Test completed successfully!")
}
