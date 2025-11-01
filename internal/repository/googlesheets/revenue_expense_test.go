package googlesheets

import (
	"cim-backend/pkg"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
)

const (
	// TestRevenueExpenseSpreadsheetID is the ID of the test Google Sheets spreadsheet
	// This should be set via environment variable: TEST_REVENUE_EXPENSE_SPREADSHEET_ID
	TestRevenueExpenseSpreadsheetID = "TEST_REVENUE_EXPENSE_SPREADSHEET_ID"

	// TestGoogleAPIKey is the Google API key for accessing Google Sheets
	// This should be set via environment variable: GOOGLE_SERVICE_ACCOUNT
	TestGoogleAPIKey = "GOOGLE_SERVICE_ACCOUNT"
)

func TestRevenueExpenseGoogleSheetsRepository(t *testing.T) {
	// Load environment variables from .env file
	// Try loading from multiple locations (current dir, parent dirs, project root)
	if err := godotenv.Load("../../../.env"); err != nil {
		t.Logf("Warning: Could not load .env file: %v", err)
	}

	// Check if test environment variables are set
	spreadsheetID := os.Getenv(TestRevenueExpenseSpreadsheetID)
	if spreadsheetID == "" {
		t.Skip("Skipping Google Sheets test: TEST_REVENUE_EXPENSE_SPREADSHEET_ID not set")
	}

	serviceAccountFilePath := os.Getenv(TestGoogleAPIKey)
	if serviceAccountFilePath == "" {
		t.Skip("Skipping Google Sheets test: GOOGLE_SERVICE_ACCOUNT not set")
	}

	t.Log("=== Google Sheets test: handling with THU CHI Google Sheets ===")
	t.Logf("Spreadsheet ID: %s", spreadsheetID)
	ctx := context.Background()
	sheetName := "TIỀN MẶT "
	revenueExpenseGoogleSheetsRepo := NewRevenueExpenseGoogleSheetsRepository()

	t.Cleanup(func() {
		t.Log("Cleaning up...")
		// Delete two last rows
		revenueExpenseGoogleSheetsRepo.DeleteLastNthRows(ctx, sheetName, 2)
		revenueExpenseGoogleSheetsRepo.Close()
		t.Log("✅ Cleanup completed")
	})

	// Initialize with the spreadsheet
	err := revenueExpenseGoogleSheetsRepo.InitializeWithSpreadsheet(ctx, serviceAccountFilePath, spreadsheetID)
	require.Nil(t, err)

	// Force cache refresh to ensure we read fresh data
	revenueExpenseGoogleSheetsRepo.ForceCacheRefresh()

	t.Log("=== Service initialized successfully, getting schema ===")

	// Get and display schema
	schema := revenueExpenseGoogleSheetsRepo.GetSchema(ctx)
	t.Logf("\n📄 Spreadsheet ID: %s\n", schema.FilePath)
	t.Logf("📋 File type: %s\n", schema.FileType)
	t.Logf("📊 Number of sheets: %d\n", len(schema.Sheets))
	if len(schema.Sheets) > 0 {
		t.Logf("   First sheet: %s with %d columns\n",
			schema.Sheets[0].SheetName,
			len(schema.Sheets[0].Headers),
		)
		t.Logf("   Columns: ")
		for i, header := range schema.Sheets[0].Headers {
			t.Logf("%s", header.ColumnName)
			if i < len(schema.Sheets[0].Headers)-1 {
				t.Logf(", ")
			}
		}
	}

	// Display detected columns to show what the Google Sheets actually contains
	t.Log("\n📋 Available columns for writing data:")
	t.Logf("   Total columns detected: %d\n", len(schema.Sheets[0].Headers))
	for i, header := range schema.Sheets[0].Headers {
		requiredText := ""
		if header.Required {
			requiredText = " (REQUIRED)"
		}
		t.Logf("   %d. %s [%s]%s\n", i+1, header.ColumnName, header.DataType, requiredText)
	}

	t.Log("\n=== Test Add First Expense ===")

	for _, header := range schema.Sheets[0].Headers {
		switch header.ColumnName {
		case "THÁNG 04+05/2023":
			t.Logf("   %s → Date/period description\n", header.ColumnName)
		case "CƠM":
			t.Logf("   %s → Rice/meal expenses\n", header.ColumnName)
		case "ĂN NHẸ":
			t.Logf("   %s → Light meal/snack expenses\n", header.ColumnName)
		case "NƯỚC":
			t.Logf("   %s → Water/beverage expenses\n", header.ColumnName)
		case "DƯ,THIẾU":
			t.Logf("   %s → Surplus/deficit amount\n", header.ColumnName)
		case "TỔNG CỘNG":
			t.Logf("   %s → Total amount\n", header.ColumnName)
		default:
			t.Logf("   %s → %s field\n", header.ColumnName, header.ColumnName)
		}
	}

	// Attempt to add expense using detected columns - test what actually works
	sampleExpense := map[string]interface{}{
		pkg.RevenueExpenseColumnName:  "Test expense",
		pkg.RevenueExpenseColumnWater: 15000,
	}

	// Add expense
	err = revenueExpenseGoogleSheetsRepo.AddExpenses(ctx, sheetName, []map[string]interface{}{sampleExpense}, []string{pkg.RevenueExpenseColumnWaterColor})
	require.Nil(t, err, "Failed to add expense")
	t.Log("✅ Sample expense added successfully")

	// Try to read last expense
	lastExpense, err := revenueExpenseGoogleSheetsRepo.GetLastExpense(ctx, sheetName)
	require.Nil(t, err, "Failed to get last expense")

	// Compare last expense with expected expense
	t.Log("\n🔍 Comparing last expense with expected expense...")
	expectedExpense := map[string]interface{}{
		pkg.RevenueExpenseColumnName:  "TEST EXPENSE",
		pkg.RevenueExpenseColumnWater: "15.000",
		// Note: STT (ordinal number) is not checked because it depends on existing data in the sheet
	}
	compareExpenses(t, expectedExpense, lastExpense)

	// Force cache refresh before reading transaction date
	revenueExpenseGoogleSheetsRepo.ForceCacheRefresh()

	sampleExpense2 := map[string]interface{}{
		pkg.RevenueExpenseColumnName:          "Test expense 2",
		pkg.RevenueExpenseColumnSnackAndRice:  1000000,
		pkg.RevenueExpenseColumnOrdinalNumber: 2,
	}

	// Add another expense
	t.Log("🔄 Adding sample expense using detected columns...")
	err = revenueExpenseGoogleSheetsRepo.AddExpenses(ctx, sheetName,
		[]map[string]interface{}{sampleExpense2},
		[]string{pkg.RevenueExpenseColumnSnackAndRiceColor})
	require.Nil(t, err, "Failed to add second expense")
	t.Log("✅ Sample expense added successfully")

	// Try to read last expense
	t.Log("🔄 Reading last expense...")
	lastExpense2, err := revenueExpenseGoogleSheetsRepo.GetLastExpense(ctx, sheetName)
	require.Nil(t, err, "Failed to get last expense")
	t.Logf("✅ Last expense 2 retrieved: %v\n", lastExpense2)

	// Compare last expense with expected expense
	t.Log("\n🔍 Comparing last expense with expected expense...")
	expectedExpense2 := map[string]interface{}{
		pkg.RevenueExpenseColumnName:         "TEST EXPENSE 2",
		pkg.RevenueExpenseColumnSnackAndRice: "1000000",
		// Note: STT (ordinal number) is not checked because it depends on existing data in the sheet
	}
	compareExpenses(t, expectedExpense2, lastExpense2)

	t.Log("✅ Thu chi Google Sheets repository test completed successfully")
}

func TestAddNewDateRow_GoogleSheets(t *testing.T) {
	t.Log("=== Google Sheets test: Add New Date Row (Thu Chi) ===")

	ctx := context.Background()
	sheetName := "TIỀN MẶT"
	if err := godotenv.Load("../../../.env"); err != nil {
		t.Logf("Warning: Could not load .env file: %v", err)
	}

	// Use fresh repo instance for this test to avoid side-effects with shared cache
	revenueExpenseGoogleSheetsRepo := NewRevenueExpenseGoogleSheetsRepository()

	// Use environment variables to pull service account and spreadsheet configs
	serviceAccountFile := os.Getenv(TestGoogleAPIKey)
	spreadsheetID := os.Getenv(TestRevenueExpenseSpreadsheetID)

	// Initialize with test service account and spreadsheet
	err := revenueExpenseGoogleSheetsRepo.InitializeWithSpreadsheet(ctx, serviceAccountFile, spreadsheetID, sheetName)
	require.Nil(t, err, "Failed to initialize repository")

	t.Cleanup(func() {
		t.Log("Cleaning up after AddNewDateRow test...")
		revenueExpenseGoogleSheetsRepo.DeleteLastNthRows(ctx, sheetName, 1)
		revenueExpenseGoogleSheetsRepo.Close()
	})

	// Choose a new date (use today's date for testing)
	newDate := pkg.GetTodayDate()

	// Add new date row
	t.Logf("Adding new date row: %v", newDate)
	err = revenueExpenseGoogleSheetsRepo.AddNewDateRow(ctx, sheetName, newDate)
	require.Nil(t, err, "Failed to add new date row")

	// Force cache refresh to ensure read after write
	revenueExpenseGoogleSheetsRepo.ForceCacheRefresh()

	// Get last transaction date to verify
	t.Log("Verifying last transaction date matches inserted date...")
	lastDate, err := revenueExpenseGoogleSheetsRepo.GetLastTransactionDate(ctx, sheetName)
	require.Nil(t, err, "Failed to get last transaction date")
	require.Equal(t, newDate.Format("2006-01-02"), lastDate.Format("2006-01-02"), "Last transaction date mismatch")

	t.Logf("✅ AddNewDateRow and last transaction date verified for date: %v", lastDate.Format("2006-01-02"))
}

// compareExpenses compares two expense maps and returns true if they match
// It only checks that all expected keys exist in actual with matching values
// Extra keys in actual are allowed (e.g., STT which depends on existing data)
func compareExpenses(t *testing.T, expected, actual map[string]interface{}) bool {
	// Check if all expected keys exist in actual and have matching values
	for key, expectedValue := range expected {
		if expectedValue == nil || expectedValue == "" {
			continue
		}
		actualValue, exists := actual[key]
		if !exists {
			require.FailNowf(t, "Missing key in actual", "key: %s", key)
		}

		// Convert both values to strings for comparison (since Google Sheets values are strings)
		expectedStr := fmt.Sprintf("%v", expectedValue)
		actualStr := fmt.Sprintf("%v", actualValue)

		if expectedStr != actualStr {
			require.FailNowf(t, "Value mismatch for key", "key: %s, expected '%s', got '%s'", key, expectedStr, actualStr)
		}
	}

	// Note: We don't check for extra keys in actual because some fields like STT (ordinal number)
	// depend on existing data in the sheet and are not deterministic in tests

	return true
}
