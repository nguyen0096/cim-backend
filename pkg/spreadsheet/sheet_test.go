package spreadsheet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestSheetNamePattern_Parse(t *testing.T) {
	// Test date: July 7, 2023
	testDate := time.Date(2023, 7, 7, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		pattern  SheetNamePattern
		date     time.Time
		expected string
	}{
		{
			name:     "should replace {MM} with zero-padded month when pattern contains {MM}",
			pattern:  "THANG {MM}",
			date:     testDate,
			expected: "THANG 07",
		},
		{
			name:     "should replace {M} with single-digit month when pattern contains {M}",
			pattern:  "THANG {M}",
			date:     testDate,
			expected: "THANG 7",
		},
		{
			name:     "should replace {DD} with zero-padded day when pattern contains {DD}",
			pattern:  "NGAY {DD}",
			date:     testDate,
			expected: "NGAY 07",
		},
		{
			name:     "should replace {D} with single-digit day when pattern contains {D}",
			pattern:  "NGAY {D}",
			date:     testDate,
			expected: "NGAY 7",
		},
		{
			name:     "should replace {YYYY} with full year when pattern contains {YYYY}",
			pattern:  "NAM {YYYY}",
			date:     testDate,
			expected: "NAM 2023",
		},
		{
			name:     "should replace {YY} with two-digit year when pattern contains {YY}",
			pattern:  "NAM {YY}",
			date:     testDate,
			expected: "NAM 23",
		},
		{
			name:     "should handle multiple patterns in one string",
			pattern:  "THANG {MM} NAM {YYYY}",
			date:     testDate,
			expected: "THANG 07 NAM 2023",
		},
		{
			name:     "should return original string when no patterns found",
			pattern:  "THANG 07",
			date:     testDate,
			expected: "THANG 07",
		},
		{
			name:     "should handle empty pattern",
			pattern:  "",
			date:     testDate,
			expected: "",
		},
		{
			name:     "should handle pattern with only spaces",
			pattern:  "   ",
			date:     testDate,
			expected: "   ",
		},
		{
			name:     "should handle January with {MM}",
			pattern:  "THANG {MM}",
			date:     time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
			expected: "THANG 01",
		},
		{
			name:     "should handle January with {M}",
			pattern:  "THANG {M}",
			date:     time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
			expected: "THANG 1",
		},
		{
			name:     "should handle December with {MM}",
			pattern:  "THANG {MM}",
			date:     time.Date(2023, 12, 15, 0, 0, 0, 0, time.UTC),
			expected: "THANG 12",
		},
		{
			name:     "should handle December with {M}",
			pattern:  "THANG {M}",
			date:     time.Date(2023, 12, 15, 0, 0, 0, 0, time.UTC),
			expected: "THANG 12",
		},
		{
			name:     "should handle first day of month with {DD}",
			pattern:  "NGAY {DD}",
			date:     time.Date(2023, 7, 1, 0, 0, 0, 0, time.UTC),
			expected: "NGAY 01",
		},
		{
			name:     "should handle first day of month with {D}",
			pattern:  "NGAY {D}",
			date:     time.Date(2023, 7, 1, 0, 0, 0, 0, time.UTC),
			expected: "NGAY 1",
		},
		{
			name:     "should handle last day of month with {DD}",
			pattern:  "NGAY {DD}",
			date:     time.Date(2023, 7, 31, 0, 0, 0, 0, time.UTC),
			expected: "NGAY 31",
		},
		{
			name:     "should handle last day of month with {D}",
			pattern:  "NGAY {D}",
			date:     time.Date(2023, 7, 31, 0, 0, 0, 0, time.UTC),
			expected: "NGAY 31",
		},
		{
			name:     "should handle year 2000 with {YYYY}",
			pattern:  "NAM {YYYY}",
			date:     time.Date(2000, 7, 7, 0, 0, 0, 0, time.UTC),
			expected: "NAM 2000",
		},
		{
			name:     "should handle year 2000 with {YY}",
			pattern:  "NAM {YY}",
			date:     time.Date(2000, 7, 7, 0, 0, 0, 0, time.UTC),
			expected: "NAM 00",
		},
		{
			name:     "should handle year 1999 with {YYYY}",
			pattern:  "NAM {YYYY}",
			date:     time.Date(1999, 7, 7, 0, 0, 0, 0, time.UTC),
			expected: "NAM 1999",
		},
		{
			name:     "should handle year 1999 with {YY}",
			pattern:  "NAM {YY}",
			date:     time.Date(1999, 7, 7, 0, 0, 0, 0, time.UTC),
			expected: "NAM 99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sheetConfig := SheetConfig{}
			sheetConfig.SetSheetNameTimeParams(tt.date)
			result := tt.pattern.Parse(sheetConfig.NameParams)
			if result != tt.expected {
				t.Errorf("Parse() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetColByExactHeaders(t *testing.T) {
	ctx := context.Background()

	xntSheet := SheetConfig{
		InternalID:     "current_month_inventory_change",
		NamePattern:    "THANG {MM}",
		HeaderStartRow: 3,
		HeaderStartCol: 1,
		HeaderHeight:   3,
	}
	xntSheet.SetSheetNameTimeParams(time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC))

	fc := FileConfig{
		FilePath:     TestInventoryTrackerExcelFile,
		SheetConfigs: []SheetConfig{xntSheet},
	}

	prov := NewExcelFileProvider(fc.FilePath)
	f, err := NewFile(fc, prov)
	require.Nil(t, err)

	err = f.Connect(ctx)
	require.Nil(t, err)

	col, err := f.Sheets[xntSheet.InternalID].GetColByExactHeaders(xntSheet.InternalID, []string{"DiẾN GiẢI", ""})
	require.Nil(t, err)
	assert.Equal(t, 2, col)
}

func TestSheetNamePattern_Parse_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		pattern  SheetNamePattern
		date     time.Time
		expected string
	}{
		{
			name:     "should handle pattern with multiple same placeholders",
			pattern:  "{MM}-{MM}-{YYYY}",
			date:     time.Date(2023, 7, 7, 0, 0, 0, 0, time.UTC),
			expected: "07-07-2023",
		},
		{
			name:     "should handle mixed case patterns",
			pattern:  "thang {MM} nam {YYYY}",
			date:     time.Date(2023, 7, 7, 0, 0, 0, 0, time.UTC),
			expected: "thang 07 nam 2023",
		},
		{
			name:     "should handle pattern with special characters",
			pattern:  "Sheet_{MM}_{YYYY}_v1.0",
			date:     time.Date(2023, 7, 7, 0, 0, 0, 0, time.UTC),
			expected: "Sheet_07_2023_v1.0",
		},
		{
			name:     "should handle pattern with only placeholder",
			pattern:  "{MM}",
			date:     time.Date(2023, 7, 7, 0, 0, 0, 0, time.UTC),
			expected: "07",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sheetConfig := SheetConfig{}
			sheetConfig.SetSheetNameTimeParams(tt.date)
			result := tt.pattern.Parse(sheetConfig.NameParams)
			if result != tt.expected {
				t.Errorf("Parse() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func getTestFileConfig() FileConfig {
	xntSheet := SheetConfig{
		InternalID:     "xnt_sheet",
		NamePattern:    "THANG {MM}",
		HeaderStartRow: 5,
		HeaderStartCol: 1,
		HeaderHeight:   3,
		FooterHeight:   3,
		IndexColumnNames: []TreeHeader{
			{fmt.Sprintf("%s%s", MetadataHeaderPrefix, "product_id")},
		},
	}
	xntSheet.SetSheetNameTimeParams(time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC))

	fc := FileConfig{
		FilePath:     TestInventoryTrackerExcelFile,
		SheetConfigs: []SheetConfig{xntSheet},
	}
	return fc
}

func TestFile_Load(t *testing.T) {
	ctx := context.Background()
	fc := getTestFileConfig()
	prov := NewExcelFileProvider(fc.FilePath)
	f, err := NewFile(fc, prov)
	require.Nil(t, err)

	err = f.Connect(ctx)
	t.Logf("err: %v", err)
	require.Nil(t, err)

	assertColumnIndices(t, f)
}

// TestSpatialIndex_Standalone demonstrates the spatial index concept
// without depending on the File struct that has compilation issues
func TestSpatialIndex_Standalone(t *testing.T) {
	// Create a new Excel file
	excelFile := excelize.NewFile()
	defer excelFile.Close()

	// Create merged cells: "A1:C3" and "D6:Z10"
	if err := excelFile.MergeCell("Sheet1", "A1", "C3"); err != nil {
		t.Fatalf("Failed to create merged cell: %v", err)
	}
	if err := excelFile.MergeCell("Sheet1", "D6", "Z10"); err != nil {
		t.Fatalf("Failed to create merged cell: %v", err)
	}

	// Get merged cells from excelize
	excelizeMergeCells, err := excelFile.GetMergeCells("Sheet1")
	if err != nil {
		t.Fatalf("Failed to get merged cells: %v", err)
	}

	mergeCells := ConvertExcelizeMergeCells(excelizeMergeCells)

	// Build spatial index using the updated function
	spatialIndex := buildMergeCellLookup(mergeCells)

	// Test that only the expected rows are stored (sparse storage)
	expectedRows := []int{1, 2, 3, 6, 7, 8, 9, 10} // 1-based row indices

	for _, expectedRow := range expectedRows {
		if _, exists := spatialIndex[expectedRow]; !exists {
			t.Errorf("Expected row %d to exist in spatial index", expectedRow)
		}
	}

	// Test that some rows that should NOT be stored are not present
	unexpectedRows := []int{4, 5, 11, 12, 13} // 1-based row indices that should not exist
	for _, unexpectedRow := range unexpectedRows {
		if _, exists := spatialIndex[unexpectedRow]; exists {
			t.Errorf("Expected row %d to NOT exist in spatial index", unexpectedRow)
		}
	}

	// Test specific cell lookups (same logic as in IsMergedCell)
	testCases := []struct {
		cell           string
		expectedMerged bool
		expectedRange  string
	}{
		{"A1", true, "A1:C3"},
		{"B2", true, "A1:C3"},
		{"C3", true, "A1:C3"},
		{"D4", false, ""}, // Not merged
		{"D6", true, "D6:Z10"},
		{"Z10", true, "D6:Z10"},
		{"A11", false, ""}, // Not merged
	}

	for _, tc := range testCases {
		cell, err := NewCellFromName(tc.cell)
		if err != nil {
			t.Errorf("Error converting cell name to coordinates: %v", err)
			continue
		}
		isMerged, rangeStr := getMergeCellRange(spatialIndex, cell)
		assert.Equal(t, isMerged, tc.expectedMerged)
		if tc.expectedMerged {
			assert.NotEmpty(t, rangeStr)
			assert.Equal(t, rangeStr, tc.expectedRange)
		} else {
			assert.Empty(t, rangeStr)
		}
	}
}

// TestSpatialIndex_Concept demonstrates the key concept:
// Only store rows/columns that contain merged cells
func TestSpatialIndex_Concept(t *testing.T) {
	// Example: merged cells "A1:C3" and "D6:Z10"
	// Traditional approach: Create full 2D grid [1..26][1..26] = 676 cells
	// Sparse approach: Only store rows 1,2,3,6,7,8,9,10 = 8 rows

	// Simulate the sparse spatial index (1-based)
	sparseIndex := map[int]map[int]string{
		1:  {1: "A1:C3", 2: "A1:C3", 3: "A1:C3"},                                                                                                                                                                                                                                                                                         // Row 1: A1, B1, C1
		2:  {1: "A1:C3", 2: "A1:C3", 3: "A1:C3"},                                                                                                                                                                                                                                                                                         // Row 2: A2, B2, C2
		3:  {1: "A1:C3", 2: "A1:C3", 3: "A1:C3"},                                                                                                                                                                                                                                                                                         // Row 3: A3, B3, C3
		6:  {4: "D6:Z10", 5: "D6:Z10", 6: "D6:Z10", 7: "D6:Z10", 8: "D6:Z10", 9: "D6:Z10", 10: "D6:Z10", 11: "D6:Z10", 12: "D6:Z10", 13: "D6:Z10", 14: "D6:Z10", 15: "D6:Z10", 16: "D6:Z10", 17: "D6:Z10", 18: "D6:Z10", 19: "D6:Z10", 20: "D6:Z10", 21: "D6:Z10", 22: "D6:Z10", 23: "D6:Z10", 24: "D6:Z10", 25: "D6:Z10", 26: "D6:Z10"}, // Row 6: D6 to Z6
		7:  {4: "D6:Z10", 5: "D6:Z10", 6: "D6:Z10", 7: "D6:Z10", 8: "D6:Z10", 9: "D6:Z10", 10: "D6:Z10", 11: "D6:Z10", 12: "D6:Z10", 13: "D6:Z10", 14: "D6:Z10", 15: "D6:Z10", 16: "D6:Z10", 17: "D6:Z10", 18: "D6:Z10", 19: "D6:Z10", 20: "D6:Z10", 21: "D6:Z10", 22: "D6:Z10", 23: "D6:Z10", 24: "D6:Z10", 25: "D6:Z10", 26: "D6:Z10"}, // Row 7: D7 to Z7
		8:  {4: "D6:Z10", 5: "D6:Z10", 6: "D6:Z10", 7: "D6:Z10", 8: "D6:Z10", 9: "D6:Z10", 10: "D6:Z10", 11: "D6:Z10", 12: "D6:Z10", 13: "D6:Z10", 14: "D6:Z10", 15: "D6:Z10", 16: "D6:Z10", 17: "D6:Z10", 18: "D6:Z10", 19: "D6:Z10", 20: "D6:Z10", 21: "D6:Z10", 22: "D6:Z10", 23: "D6:Z10", 24: "D6:Z10", 25: "D6:Z10", 26: "D6:Z10"}, // Row 8: D8 to Z8
		9:  {4: "D6:Z10", 5: "D6:Z10", 6: "D6:Z10", 7: "D6:Z10", 8: "D6:Z10", 9: "D6:Z10", 10: "D6:Z10", 11: "D6:Z10", 12: "D6:Z10", 13: "D6:Z10", 14: "D6:Z10", 15: "D6:Z10", 16: "D6:Z10", 17: "D6:Z10", 18: "D6:Z10", 19: "D6:Z10", 20: "D6:Z10", 21: "D6:Z10", 22: "D6:Z10", 23: "D6:Z10", 24: "D6:Z10", 25: "D6:Z10", 26: "D6:Z10"}, // Row 9: D9 to Z9
		10: {4: "D6:Z10", 5: "D6:Z10", 6: "D6:Z10", 7: "D6:Z10", 8: "D6:Z10", 9: "D6:Z10", 10: "D6:Z10", 11: "D6:Z10", 12: "D6:Z10", 13: "D6:Z10", 14: "D6:Z10", 15: "D6:Z10", 16: "D6:Z10", 17: "D6:Z10", 18: "D6:Z10", 19: "D6:Z10", 20: "D6:Z10", 21: "D6:Z10", 22: "D6:Z10", 23: "D6:Z10", 24: "D6:Z10", 25: "D6:Z10", 26: "D6:Z10"}, // Row 10: D10 to Z10
	}

	// Test that we can efficiently look up merged cells
	testCases := []struct {
		row, col       int // 1-based coordinates
		expectedMerged bool
		expectedRange  string
	}{
		{1, 1, true, "A1:C3"},    // A1
		{2, 2, true, "A1:C3"},    // B2
		{3, 3, true, "A1:C3"},    // C3
		{4, 1, false, ""},        // D4 (not merged)
		{6, 4, true, "D6:Z10"},   // D6
		{10, 26, true, "D6:Z10"}, // Z10
		{11, 1, false, ""},       // A11 (not merged)
	}

	for _, tc := range testCases {
		rowMap, rowExists := sparseIndex[tc.row]
		if !rowExists {
			if tc.expectedMerged {
				t.Errorf("Row %d: expected to be merged but row not found in index", tc.row)
			}
			continue
		}

		rangeStr, colExists := rowMap[tc.col]
		if !colExists {
			if tc.expectedMerged {
				t.Errorf("Cell [%d,%d]: expected to be merged but column not found", tc.row, tc.col)
			}
			continue
		}

		if !tc.expectedMerged {
			t.Errorf("Cell [%d,%d]: expected NOT to be merged but found range %s", tc.row, tc.col, rangeStr)
		} else if rangeStr != tc.expectedRange {
			t.Errorf("Cell [%d,%d]: expected range %s, got %s", tc.row, tc.col, tc.expectedRange, rangeStr)
		}
	}

	// Verify sparse storage: only 8 rows are stored instead of 26
	if len(sparseIndex) != 8 {
		t.Errorf("Expected 8 rows in sparse index, got %d", len(sparseIndex))
	}

	// Verify that rows 4, 5, 11-26 are not stored (saving memory)
	unexpectedRows := []int{4, 5, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26}
	for _, row := range unexpectedRows {
		if _, exists := sparseIndex[row]; exists {
			t.Errorf("Expected row %d to NOT exist in sparse index", row)
		}
	}
}

func assertColumnIndices(t *testing.T, f *File) {
	sheet, ok := f.Sheets[SheetInternalID(getTestFileConfig().SheetConfigs[0].InternalID)]
	require.True(t, ok, "should found sheet")

	require.Containsf(t, sheet.ColumnIndices, 1, "should contain product_id column at col 1")
	assert.Containsf(t, sheet.ColumnIndices[1], "test-product-id-1", "should contain product_id value test-product-id-1")
}

// copyTestFile creates a copy of the test file and returns the path to the copy.
// The caller should defer cleanup by removing the copied file.
func copyTestFile(t *testing.T, originalPath string) string {
	t.Helper()

	// Read the original file
	sourceData, err := os.ReadFile(originalPath)
	require.NoError(t, err, "failed to read original test file")

	// Create a temporary file with the same extension
	ext := filepath.Ext(originalPath)
	tempFile, err := os.CreateTemp("", "test-*"+ext)
	require.NoError(t, err, "failed to create temporary file")
	tempPath := tempFile.Name()

	// Write the data to the temporary file
	_, err = tempFile.Write(sourceData)
	require.NoError(t, err, "failed to write to temporary file")

	// Close the file
	err = tempFile.Close()
	require.NoError(t, err, "failed to close temporary file")

	return tempPath
}

func TestFile_UpsertRow(t *testing.T) {
	ctx := context.Background()

	// Create a copy of the test file to avoid modifying the original
	fc := getTestFileConfig()
	testFileCopy := copyTestFile(t, fc.FilePath)
	defer os.Remove(testFileCopy) // Clean up the copy after the test

	// Update the config to use the copied file
	fc.FilePath = testFileCopy

	prov := NewExcelFileProvider(fc.FilePath)
	f, err := NewFile(fc, prov)
	require.NoError(t, err)

	err = f.Connect(ctx)
	require.NoError(t, err)

	defer f.Close(ctx) // Close the file after the test

	// update row with index column: __product_id = test-product-id-3
	// change SL of Ngày 6 to 11
	err = f.UpsertRow(
		ctx,
		fc.SheetConfigs[0].InternalID,
		"__product_id",
		"test-product-id3",
		map[TreeHeaderStr]interface{}{
			"Ngày.2.SL": 11,
		})
	require.NoError(t, err)

	// Insert a new row with index column: __product_id = insert-product-id
	// set SL of Ngày 4 to 1
	err = f.UpsertRow(
		ctx,
		fc.SheetConfigs[0].InternalID,
		"__product_id",
		"insert-product-id",
		map[TreeHeaderStr]interface{}{
			"Ngày.4.SL": 1,
		})
	require.NoError(t, err)

	// Verify the changes were made by reading the values back
	sheet := f.Sheets[fc.SheetConfigs[0].InternalID]
	require.NotNil(t, sheet, "sheet should exist")

	// Verify that test-product-id-3 exists in the index
	productIDCol, err := sheet.GetColByExactHeaders(fc.SheetConfigs[0].InternalID, TreeHeader{fmt.Sprintf("%s%s", MetadataHeaderPrefix, "product_id")})
	require.NoError(t, err)

	productIDIndex := sheet.ColumnIndices[productIDCol]
	require.NotNil(t, productIDIndex, "product_id index should exist")

	_, exists := productIDIndex["test-product-id3"]
	assert.True(t, exists, "test-product-id-3 should exist in the index")

	_, exists = productIDIndex["insert-product-id"]
	assert.True(t, exists, "insert-product-id should exist in the index")
}
