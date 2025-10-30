package excel

import (
	"cim-backend/pkg"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	// TestRevenueExpenseExcelFile is the path to the revenue expense test Excel file
	TestRevenueExpenseExcelFile = "../../../test/data/excel/revenue_expense_sample.xlsx"
)

func TestRevenueExpenseExcelRepository(t *testing.T) {
	t.Log("=== Excel test: handling with THU CHI Excel ===")
	ctx := context.Background()
	sheetName := "TIỀN MẶT"
	revenueExpenseExcelRepo := NewRevenueExpenseExcelRepository()

	// Create temporary copy of the original file
	originalFile := TestRevenueExpenseExcelFile
	tempFile := filepath.Join("../../../test/data/excel/Thu_chi_test.xlsx")

	// Copy original file to temp location
	src, err := os.Open(originalFile)
	require.Nil(t, err)
	defer src.Close()

	dst, err := os.Create(tempFile)
	require.Nil(t, err)
	defer dst.Close()

	_, err = io.Copy(dst, src)
	require.Nil(t, err)
	dst.Close() // Close before using in test

	t.Cleanup(func() {
		t.Log("Cleaning up...")

		// Close repository to release file handles
		revenueExpenseExcelRepo.Close()

		// Delete temporary test file
		if err := os.Remove(tempFile); err != nil && !os.IsNotExist(err) {
			t.Logf("Warning: failed to delete temp file %s: %v", tempFile, err)
		} else {
			t.Log("✅ Temporary test file deleted successfully")
		}
	})

	// Initialize with the temporary file
	err = revenueExpenseExcelRepo.InitializeWithFile(ctx, tempFile)
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

	t.Log("\n=== Test Add First Expense ===")

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
	err = revenueExpenseExcelRepo.AddExpenses(ctx, sheetName, []map[string]interface{}{sampleExpense}, []string{pkg.RevenueExpenseColumnSnackAndRiceColor, pkg.RevenueExpenseColumnWaterColor})
	require.Nil(t, err)

	// Try to read last expense
	lastExpense, err := revenueExpenseExcelRepo.GetLastExpense(ctx, sheetName)
	require.Nil(t, err)

	// Compare last expense with expected expense
	expectedExpense := map[string]interface{}{
		pkg.RevenueExpenseColumnName:          "Test expense",
		pkg.RevenueExpenseColumnWater:         15000,
		pkg.RevenueExpenseColumnOrdinalNumber: 1,
	}
	compareExpenses(t, expectedExpense, lastExpense)

	sampleExpense2 := map[string]interface{}{
		pkg.RevenueExpenseColumnName:          "Test expense 2",
		pkg.RevenueExpenseColumnSnackAndRice:  1000000,
		pkg.RevenueExpenseColumnOrdinalNumber: 2,
	}

	// Add another expense
	err = revenueExpenseExcelRepo.AddExpenses(ctx, sheetName,
		[]map[string]interface{}{sampleExpense2},
		[]string{pkg.RevenueExpenseColumnSnackAndRiceColor})
	require.Nil(t, err)

	// Try to read last expense
	lastExpense2, err := revenueExpenseExcelRepo.GetLastExpense(ctx, sheetName)
	require.Nil(t, err)

	// Compare last expense with expected expense
	expectedExpense2 := map[string]interface{}{
		pkg.RevenueExpenseColumnName:          "Test expense 2",
		pkg.RevenueExpenseColumnSnackAndRice:  1000000,
		pkg.RevenueExpenseColumnOrdinalNumber: 2,
	}
	compareExpenses(t, expectedExpense2, lastExpense2)

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
