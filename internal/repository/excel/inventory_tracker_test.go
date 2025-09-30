package excel

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInventoryTrackerExcelRepository(t *testing.T) {
	t.Log("=== Excel test: handling with Inventory Tracker Excel ===")
	ctx := context.Background()
	inventoryRepo := NewInventoryTrackerExcelRepository()
	numberOfRowsToDelete := 0

	// Create temporary copy of the original file
	originalFile := TestInventoryTrackerExcelFile
	tempFile := filepath.Join(t.TempDir(), "Inventory_Tracker_test.xlsx")

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
		inventoryRepo.Close()

		// Delete temporary test file
		if err := os.Remove(tempFile); err != nil && !os.IsNotExist(err) {
			t.Logf("Warning: failed to delete temp file %s: %v", tempFile, err)
		} else {
			t.Log("✅ Temporary test file deleted successfully")
		}
	})

	// Initialize with the temporary file
	err = inventoryRepo.InitializeWithFile(ctx, tempFile)
	require.Nil(t, err)

	// Force cache refresh to ensure we read fresh data
	inventoryRepo.ForceCacheRefresh()

	t.Log("✅ Repository initialized successfully")

	// Get and display schema
	schema := inventoryRepo.GetSchema(ctx)
	require.NotNil(t, schema)
	require.Greater(t, len(schema.Sheets), 0)

	t.Logf("\n📄 File path: %s\n", schema.FilePath)
	t.Logf("📋 File type: %s\n", schema.FileType)
	t.Logf("📊 Number of sheets: %d\n", len(schema.Sheets))

	// Use the first sheet for testing
	firstSheet := schema.Sheets[0]
	sheetName := firstSheet.SheetName

	t.Logf("   First sheet: %s with %d columns\n", sheetName, len(firstSheet.Headers))
	t.Logf("   Columns: ")
	for i, header := range firstSheet.Headers {
		t.Logf("%s", header.ColumnName)
		if i < len(firstSheet.Headers)-1 {
			t.Logf(", ")
		}
	}

	t.Log("\n=== Test Methods Functionality ===")

	// Display detected columns to show what the Excel file actually contains
	t.Log("\n📋 Available columns for writing data:")
	t.Logf("   Total columns detected: %d\n", len(firstSheet.Headers))
	for i, header := range firstSheet.Headers {
		requiredText := ""
		if header.Required {
			requiredText = " (REQUIRED)"
		}
		t.Logf("   %d. %s [%s]%s\n", i+1, header.ColumnName, header.DataType, requiredText)
	}

	// Show column mapping for easier understanding
	t.Log("\n🔍 Column mapping for data entry:")
	for _, header := range firstSheet.Headers {
		switch strings.ToUpper(header.ColumnName) {
		case "STT", "NO", "NUMBER":
			t.Logf("   %s → Serial number/row index\n", header.ColumnName)
		case "PRODUCT", "PRODUCT_NAME", "TÊN SẢN PHẨM", "SẢN PHẨM":
			t.Logf("   %s → Product name\n", header.ColumnName)
		case "QUANTITY", "QTY", "SỐ LƯỢNG":
			t.Logf("   %s → Quantity\n", header.ColumnName)
		case "UNIT", "ĐƠN VỊ":
			t.Logf("   %s → Unit of measurement\n", header.ColumnName)
		case "PRICE", "GIÁ", "UNIT_PRICE":
			t.Logf("   %s → Unit price\n", header.ColumnName)
		case "TOTAL", "TỔNG", "TOTAL_AMOUNT":
			t.Logf("   %s → Total amount\n", header.ColumnName)
		case "NOTES", "GHI CHÚ", "DESCRIPTION":
			t.Logf("   %s → Notes/Description\n", header.ColumnName)
		default:
			t.Logf("   %s → %s field\n", header.ColumnName, header.ColumnName)
		}
	}

	// Create sample inventory data using detected columns
	sampleInventory := map[string]interface{}{}
	for i, header := range firstSheet.Headers {
		switch strings.ToUpper(header.ColumnName) {
		case "STT", "NO", "NUMBER":
			sampleInventory[header.ColumnName] = "1"
		case "PRODUCT", "PRODUCT_NAME", "TÊN SẢN PHẨM", "SẢN PHẨM":
			sampleInventory[header.ColumnName] = "Test Product"
		case "QUANTITY", "QTY", "SỐ LƯỢNG":
			sampleInventory[header.ColumnName] = "10"
		case "UNIT", "ĐƠN VỊ":
			sampleInventory[header.ColumnName] = "PCS"
		case "PRICE", "GIÁ", "UNIT_PRICE":
			sampleInventory[header.ColumnName] = "50000"
		case "TOTAL", "TỔNG", "TOTAL_AMOUNT":
			sampleInventory[header.ColumnName] = "500000"
		case "NOTES", "GHI CHÚ", "DESCRIPTION":
			sampleInventory[header.ColumnName] = "Test inventory entry"
		default:
			// For Vietnamese columns, provide meaningful test data
			if strings.Contains(strings.ToUpper(header.ColumnName), "DIỄN GIẢI") {
				sampleInventory[header.ColumnName] = "Test Product Description"
			} else if strings.Contains(strings.ToUpper(header.ColumnName), "ĐVT") {
				sampleInventory[header.ColumnName] = "KG"
			} else if strings.Contains(strings.ToUpper(header.ColumnName), "TỒN") {
				sampleInventory[header.ColumnName] = "100"
			} else if strings.Contains(strings.ToUpper(header.ColumnName), "NHẬP") {
				sampleInventory[header.ColumnName] = "50"
			} else if strings.Contains(strings.ToUpper(header.ColumnName), "XUẤT") {
				sampleInventory[header.ColumnName] = "20"
			} else if strings.Contains(strings.ToUpper(header.ColumnName), "GIÁ") {
				sampleInventory[header.ColumnName] = "25000"
			} else if i == 0 {
				// Ensure at least the first column has data
				sampleInventory[header.ColumnName] = "Test Data"
			} else {
				sampleInventory[header.ColumnName] = ""
			}
		}
	}

	// Try to read last transaction date (may not exist in this file format)
	t.Log("🔄 Reading last transaction date...")
	lastTransactionDate, err := inventoryRepo.GetLastTransactionDate(ctx, sheetName)
	if err != nil {
		t.Logf("⚠️ No transaction date found (expected for this file format): %v", err)
		// This is expected for inventory files that don't have dates in first column
	} else {
		t.Logf("✅ Last transaction date retrieved: %v\n", lastTransactionDate)
		// Just log the date - don't make assumptions about what it should be
		t.Logf("📅 Initial transaction date: %s", lastTransactionDate.Format("2006-01-02"))
	}

	// Add inventory entry
	t.Log("🔄 Adding sample inventory entry using detected columns...")
	err = inventoryRepo.AddInventoryEntry(ctx, sheetName, sampleInventory)
	require.Nil(t, err)
	t.Log("✅ Sample inventory entry added successfully")
	numberOfRowsToDelete += 2 // Include date row and inventory row

	// Try to read last inventory entry
	t.Log("🔄 Reading last inventory entry...")
	lastInventory, err := inventoryRepo.GetLastInventoryEntry(ctx, sheetName)
	require.Nil(t, err)
	t.Logf("✅ Last inventory entry retrieved: %v\n", lastInventory)

	// Compare last inventory with expected inventory
	t.Log("\n🔍 Comparing last inventory with expected inventory...")
	compareInventoryEntries(t, sampleInventory, lastInventory)

	// Try to read last transaction date again
	t.Log("🔄 Reading last transaction date...")
	lastTransactionDate2, err := inventoryRepo.GetLastTransactionDate(ctx, sheetName)
	if err != nil {
		t.Logf("⚠️ No transaction date found (expected for this file format): %v", err)
	} else {
		t.Logf("✅ Last transaction date retrieved: %v\n", lastTransactionDate2)
		// Just log the final date
		t.Logf("📅 Final transaction date: %s", lastTransactionDate2.Format("2006-01-02"))
	}

	// Create second sample inventory entry
	sampleInventory2 := map[string]interface{}{}
	for i, header := range firstSheet.Headers {
		switch strings.ToUpper(header.ColumnName) {
		case "STT", "NO", "NUMBER":
			sampleInventory2[header.ColumnName] = "2"
		case "PRODUCT", "PRODUCT_NAME", "TÊN SẢN PHẨM", "SẢN PHẨM":
			sampleInventory2[header.ColumnName] = "Test Product 2"
		case "QUANTITY", "QTY", "SỐ LƯỢNG":
			sampleInventory2[header.ColumnName] = "5"
		case "UNIT", "ĐƠN VỊ":
			sampleInventory2[header.ColumnName] = "BOX"
		case "PRICE", "GIÁ", "UNIT_PRICE":
			sampleInventory2[header.ColumnName] = "100000"
		case "TOTAL", "TỔNG", "TOTAL_AMOUNT":
			sampleInventory2[header.ColumnName] = "500000"
		case "NOTES", "GHI CHÚ", "DESCRIPTION":
			sampleInventory2[header.ColumnName] = "Second test inventory entry"
		default:
			// For Vietnamese columns, provide meaningful test data
			if strings.Contains(strings.ToUpper(header.ColumnName), "DIỄN GIẢI") {
				sampleInventory2[header.ColumnName] = "Second Test Product Description"
			} else if strings.Contains(strings.ToUpper(header.ColumnName), "ĐVT") {
				sampleInventory2[header.ColumnName] = "THÙNG"
			} else if strings.Contains(strings.ToUpper(header.ColumnName), "TỒN") {
				sampleInventory2[header.ColumnName] = "200"
			} else if strings.Contains(strings.ToUpper(header.ColumnName), "NHẬP") {
				sampleInventory2[header.ColumnName] = "75"
			} else if strings.Contains(strings.ToUpper(header.ColumnName), "XUẤT") {
				sampleInventory2[header.ColumnName] = "30"
			} else if strings.Contains(strings.ToUpper(header.ColumnName), "GIÁ") {
				sampleInventory2[header.ColumnName] = "35000"
			} else if i == 0 {
				// Ensure at least the first column has data
				sampleInventory2[header.ColumnName] = "Second Test Data"
			} else {
				sampleInventory2[header.ColumnName] = ""
			}
		}
	}

	// Add another inventory entry
	t.Log("🔄 Adding second sample inventory entry using detected columns...")
	err = inventoryRepo.AddInventoryEntry(ctx, sheetName, sampleInventory2)
	require.Nil(t, err)
	t.Log("✅ Second sample inventory entry added successfully")
	numberOfRowsToDelete += 1 // Now only include inventory row

	// Try to read last inventory entry
	t.Log("🔄 Reading last inventory entry...")
	lastInventory2, err := inventoryRepo.GetLastInventoryEntry(ctx, sheetName)
	require.Nil(t, err)
	t.Logf("✅ Last inventory entry retrieved: %v\n", lastInventory2)

	// Compare last inventory with expected inventory
	t.Log("\n🔍 Comparing last inventory with expected inventory...")
	compareInventoryEntries(t, sampleInventory2, lastInventory2)

	t.Log("✅ Inventory Tracker Excel repository test completed successfully")
}

// TestInventoryTrackerExcelRepository_EdgeCases tests edge cases and error conditions
func TestInventoryTrackerExcelRepository_EdgeCases(t *testing.T) {
	ctx := context.Background()
	inventoryRepo := NewInventoryTrackerExcelRepository()

	t.Run("should return error when not initialized", func(t *testing.T) {
		_, err := inventoryRepo.GetLastInventoryEntry(ctx, "Sheet1")
		require.Error(t, err)
		require.Contains(t, err.Error(), "repository not initialized")
	})

	t.Run("should return error for invalid file path", func(t *testing.T) {
		err := inventoryRepo.InitializeWithFile(ctx, "nonexistent/file.xlsx")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to open file")
	})

	// Initialize with valid file for remaining tests
	// Create temporary copy of the original file for edge case tests
	originalFile := TestInventoryTrackerExcelFile
	tempFile := filepath.Join(t.TempDir(), "Inventory_Tracker_edge_test.xlsx")

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

	err = inventoryRepo.InitializeWithFile(ctx, tempFile)
	require.Nil(t, err)

	// Cleanup temp file after tests
	t.Cleanup(func() {
		inventoryRepo.Close()
		if err := os.Remove(tempFile); err != nil && !os.IsNotExist(err) {
			t.Logf("Warning: failed to delete temp file %s: %v", tempFile, err)
		}
	})

	schema := inventoryRepo.GetSchema(ctx)
	require.NotNil(t, schema)
	require.Greater(t, len(schema.Sheets), 0)
	sheetName := schema.Sheets[0].SheetName

	t.Run("should return error for nil inventory data", func(t *testing.T) {
		err := inventoryRepo.AddInventoryEntry(ctx, sheetName, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "data cannot be nil")
	})

	t.Run("should return error for empty inventory data", func(t *testing.T) {
		err := inventoryRepo.AddInventoryEntry(ctx, sheetName, map[string]interface{}{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "data cannot be empty")
	})

	t.Run("should return error for inventory data with only empty values", func(t *testing.T) {
		emptyData := map[string]interface{}{
			"field1": "",
			"field2": "   ",
			"field3": nil,
		}
		err := inventoryRepo.AddInventoryEntry(ctx, sheetName, emptyData)
		require.Error(t, err)
		require.Contains(t, err.Error(), "data must contain at least one non-empty value")
	})

	t.Run("should return error for nonexistent sheet", func(t *testing.T) {
		validData := map[string]interface{}{
			"field1": "value1",
		}
		err := inventoryRepo.AddInventoryEntry(ctx, "NonexistentSheet", validData)
		require.Error(t, err)
		require.Contains(t, err.Error(), "sheet NonexistentSheet not found in metadata")
	})

	t.Run("should handle cache refresh correctly", func(t *testing.T) {
		// This should not cause any errors
		inventoryRepo.ForceCacheRefresh()

		// Should still be able to get schema after cache refresh
		schema := inventoryRepo.GetSchema(ctx)
		require.NotNil(t, schema)
	})

	t.Run("should close repository without errors", func(t *testing.T) {
		err := inventoryRepo.Close()
		require.Nil(t, err)
	})
}

// compareInventoryEntries compares two inventory entry maps and returns true if they match
func compareInventoryEntries(t *testing.T, expected, actual map[string]interface{}) bool {
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
			require.FailNowf(t, "Value mismatch for key", "key: %s, expected '%s', got '%s'", key, expectedStr, actualStr)
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

// TestInventoryTrackerExcelRepository_Performance tests performance aspects
func TestInventoryTrackerExcelRepository_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()
	inventoryRepo := NewInventoryTrackerExcelRepository()

	// Create temporary copy of the original file for performance tests
	originalFile := TestInventoryTrackerExcelFile
	tempFile := filepath.Join(t.TempDir(), "Inventory_Tracker_perf_test.xlsx")

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

	err = inventoryRepo.InitializeWithFile(ctx, tempFile)
	require.Nil(t, err)

	// Cleanup temp file after tests
	t.Cleanup(func() {
		inventoryRepo.Close()
		if err := os.Remove(tempFile); err != nil && !os.IsNotExist(err) {
			t.Logf("Warning: failed to delete temp file %s: %v", tempFile, err)
		}
	})

	schema := inventoryRepo.GetSchema(ctx)
	require.NotNil(t, schema)
	require.Greater(t, len(schema.Sheets), 0)
	sheetName := schema.Sheets[0].SheetName

	t.Run("should handle multiple rapid operations", func(t *testing.T) {
		start := time.Now()

		// Perform multiple schema retrievals (should use cache)
		for i := 0; i < 10; i++ {
			schema := inventoryRepo.GetSchema(ctx)
			require.NotNil(t, schema)
		}

		// Perform multiple last entry retrievals (should use cache)
		for i := 0; i < 5; i++ {
			_, err := inventoryRepo.GetLastInventoryEntry(ctx, sheetName)
			require.Nil(t, err)
		}

		duration := time.Since(start)
		t.Logf("Multiple operations completed in %v", duration)

		// Should complete reasonably quickly due to caching
		require.Less(t, duration, 5*time.Second)
	})

	t.Run("should handle cache invalidation correctly", func(t *testing.T) {
		// Get initial data
		_, err := inventoryRepo.GetLastInventoryEntry(ctx, sheetName)
		require.Nil(t, err)

		// Force cache refresh
		inventoryRepo.ForceCacheRefresh()

		// Should still work after cache refresh
		_, err = inventoryRepo.GetLastInventoryEntry(ctx, sheetName)
		require.Nil(t, err)
	})
}
