package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRevenueExpenseExcelRepository tests the Excel handling classes with the Thu chi.xlsx file
func TestRevenueExpenseExcelRepository(t *testing.T) {
	iterations := 2
	for i := 0; i < iterations; i++ {
		t.Run(fmt.Sprintf("Iteration %d", i+1), func(t *testing.T) {
			testRevenueExpenseExcelRepository(t)
		})
	}
}

func testRevenueExpenseExcelRepository(t *testing.T) {
	t.Log("=== Excel test: handling with THU CHI Excel ===")
	ctx := context.Background()
	sheetName := "TIỀN MẶT"
	revenueExpenseExcelRepo := NewRevenueExpenseExcelRepository()
	numberOfRowsToDelete := 0
	t.Cleanup(func() {
		t.Log("Cleaning up...")
		err := revenueExpenseExcelRepo.DeleteLastNRows(ctx, sheetName, numberOfRowsToDelete)
		require.Nil(t, err)
		t.Log("✅ Rows deleted successfully")
	})

	// Initialize with the Thu chi.xlsx file
	err := revenueExpenseExcelRepo.InitializeWithFile(ctx, "testdata/Thu chi.xlsx")
	require.Nil(t, err)

	// Force cache refresh to ensure we read fresh data
	revenueExpenseExcelRepo.ForceCacheRefresh()

	t.Log("✅ Service initialized successfully")

	// Get and display schema
	schema := revenueExpenseExcelRepo.GetSchema(ctx)
	t.Logf("\n📄 File path: %s\n", schema.FilePath)
	t.Logf("📋 File type: %s\n", schema.FileType)
	t.Logf("📊 Number of sheets: %d\n", len(schema.Sheets))
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

	t.Log("\n=== Test Methods Functionality ===")

	// Display detected columns to show what the Excel file actually contains
	t.Log("\n📋 Available columns for writing data:")
	t.Logf("   Total columns detected: %d\n", len(schema.Sheets[0].Headers))
	for i, header := range schema.Sheets[0].Headers {
		requiredText := ""
		if header.Required {
			requiredText = " (REQUIRED)"
		}
		t.Logf("   %d. %s [%s]%s\n", i+1, header.ColumnName, header.DataType, requiredText)
	}

	// Show column mapping for easier understanding
	t.Log("\n🔍 Column mapping for data entry:")
	for _, header := range schema.Sheets[0].Headers {
		switch header.ColumnName {
		case "STT":
			t.Logf("   %s → Serial number/row index\n", header.ColumnName)
		case "DIỄN GIẢI":
			t.Logf("   %s → Description/explanation\n", header.ColumnName)
		case "THU":
			t.Logf("   %s → Revenue/income amount\n", header.ColumnName)
		case "CHI KHÁC", "CHI":
			t.Logf("   %s → Expense amount\n", header.ColumnName)
		default:
			t.Logf("   %s → %s field\n", header.ColumnName, header.ColumnName)
		}
	}
	// Attempt to add expense using detected columns
	sampleExpense := map[string]interface{}{}
	for _, header := range schema.Sheets[0].Headers {
		switch header.ColumnName {
		case "STT":
			sampleExpense[header.ColumnName] = "1"
		case "DIỄN GIẢI":
			sampleExpense[header.ColumnName] = "Sample Transaction"
		case "THU":
			sampleExpense[header.ColumnName] = "0"
		case "CHI KHÁC":
			sampleExpense[header.ColumnName] = "50000"
		default:
			sampleExpense[header.ColumnName] = ""
		}
	}

	// Read last transaction date
	t.Log("🔄 Reading last transaction date...")
	lastTransactionDate, err := revenueExpenseExcelRepo.GetLastTransactionDate(ctx, sheetName)
	require.Nil(t, err)
	t.Logf("✅ Last transaction date retrieved: %v\n", lastTransactionDate)

	// Verify that last transaction date is not today
	require.NotEqual(t, lastTransactionDate.Format("2006-01-02"), time.Now().Format("2006-01-02"))

	// Add expense
	t.Log("🔄 Adding sample expense using detected columns...")
	err = revenueExpenseExcelRepo.AddExpense(ctx, sheetName, sampleExpense)
	require.Nil(t, err)
	t.Log("✅ Sample expense added successfully")
	numberOfRowsToDelete += 2 // Include date row and expense row

	// Try to read last expense
	t.Log("🔄 Reading last expense...")
	lastExpense, err := revenueExpenseExcelRepo.GetLastExpense(ctx, sheetName)
	require.Nil(t, err)
	t.Logf("✅ Last expense retrieved: %v\n", lastExpense)

	// Compare last expense with expected expense
	t.Log("\n🔍 Comparing last expense with expected expense...")
	compareExpenses(t, sampleExpense, lastExpense)

	// Read last transaction date
	t.Log("🔄 Reading last transaction date...")
	lastTransactionDate2, err := revenueExpenseExcelRepo.GetLastTransactionDate(ctx, sheetName)
	require.Nil(t, err)
	t.Logf("✅ Last transaction date retrieved: %v\n", lastTransactionDate2)

	// Verify that last transaction date is today
	require.Equal(t, lastTransactionDate2.Format("2006-01-02"), time.Now().Format("2006-01-02"))

	sampleExpense2 := map[string]interface{}{}
	for _, header := range schema.Sheets[0].Headers {
		switch header.ColumnName {
		case "STT":
			sampleExpense2[header.ColumnName] = "2"
		case "DIỄN GIẢI":
			sampleExpense2[header.ColumnName] = "Sample Transaction 2"
		case "THU":
			sampleExpense2[header.ColumnName] = "10"
		case "CHI KHÁC":
			sampleExpense2[header.ColumnName] = "500"
		default:
			sampleExpense2[header.ColumnName] = ""
		}
	}

	// Add another expense
	t.Log("🔄 Adding sample expense using detected columns...")
	err = revenueExpenseExcelRepo.AddExpense(ctx, sheetName, sampleExpense2)
	require.Nil(t, err)
	t.Log("✅ Sample expense added successfully")
	numberOfRowsToDelete += 1 // Now only include expense row

	// Try to read last expense
	t.Log("🔄 Reading last expense...")
	lastExpense2, err := revenueExpenseExcelRepo.GetLastExpense(ctx, sheetName)
	require.Nil(t, err)
	t.Logf("✅ Last expense retrieved: %v\n", lastExpense2)

	// Compare last expense with expected expense
	t.Log("\n🔍 Comparing last expense with expected expense...")
	compareExpenses(t, sampleExpense2, lastExpense2)

	t.Log("✅ Thu chi Excel repository test completed successfully")
}

// compareExpenses compares two expense maps and returns true if they match
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

		// Convert both values to strings for comparison (since Excel values are strings)
		expectedStr := strings.ToUpper(fmt.Sprintf("%v", expectedValue))
		actualStr := fmt.Sprintf("%v", actualValue)

		if expectedStr != actualStr {
			require.FailNowf(t, "Value mismatch for key", "expected '%s', got '%s'", expectedStr, actualStr)
		}
	}

	// Check if actual has any extra keys that weren't in expected
	for key := range actual {
		if _, exists := expected[key]; !exists {
			require.FailNowf(t, "Extra key in actual", "key: %s", key)
		}
	}

	return true
}
