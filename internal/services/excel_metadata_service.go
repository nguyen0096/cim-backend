package services

import (
	"context"
	"fmt"
	"import-export-backend/internal/models"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// ExcelMetadataService manages metadata, parser and writer integration
type ExcelMetadataService struct {
	ctx          context.Context
	fileMetadata *models.FileMetadata
}

// NewExcelMetadataService creates a new ExcelMetadataService
func NewExcelMetadataService(ctx context.Context) *ExcelMetadataService {
	return &ExcelMetadataService{
		ctx: ctx,
	}
}

// InitializeRevenueExpenseMetadata reads from actual Thu chi.xlsx file and extracts metadata
func (s *ExcelMetadataService) InitializeRevenueExpenseMetadata(filePath string) (*models.FileMetadata, error) {
	// Read the Excel file to extract actual metadata
	file, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	// Get sheet names
	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found in file")
	}

	// Process each sheet
	var sheetMetadataList []models.ExcelSheetMetadata
	for _, sheetName := range sheets {
		sheetMetadata, err := s.extractSheetMetadata(file, sheetName)
		if err != nil {
			return nil, fmt.Errorf("failed to extract metadata for sheet %s: %w", sheetName, err)
		}
		sheetMetadataList = append(sheetMetadataList, *sheetMetadata)
	}

	now := time.Now()
	store := &models.FileMetadata{
		FileType:  models.FileTypeRevenueExpense,
		FilePath:  filePath,
		Version:   "1.0",
		Sheets:    sheetMetadataList,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.fileMetadata = store
	return store, nil
}

// extractSheetMetadata analyzes a sheet and extracts its metadata
func (s *ExcelMetadataService) extractSheetMetadata(file *excelize.File, sheetName string) (*models.ExcelSheetMetadata, error) {
	// Get all rows from the sheet
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows: %w", err)
	}

	if len(rows) == 0 {
		return &models.ExcelSheetMetadata{
			SheetName: sheetName,
			Headers:   []models.ExcelColumnMetadata{},
		}, nil
	}

	// Analyze header row to determine columns
	var headers []models.ExcelColumnMetadata

	// Find the header row (contains column names)
	headerRow := s.findHeaderRow(rows)

	if headerRow >= 0 && headerRow < len(rows) {
		headerCells := rows[headerRow]

		for i, cellValue := range headerCells {
			if cellValue != "" && !s.isCommentOrEmpty(cellValue) {
				dataType := s.analyzeColumnType(file, sheetName, i, len(rows))
				header := models.ExcelColumnMetadata{
					ColumnIndex: i,
					ColumnName:  cellValue,
					DataType:    dataType,
					Required:    s.isRequiredColumn(cellValue),
				}
				headers = append(headers, header)
			}
		}
	}

	return &models.ExcelSheetMetadata{
		SheetName: sheetName,
		Headers:   headers,
	}, nil
}

// findHeaderRow finds the row containing column headers
func (s *ExcelMetadataService) findHeaderRow(rows [][]string) int {
	// Look for row with text headers (not purely numeric)
	// Search up to first 10 rows to find headers
	maxSearchRows := len(rows)
	if maxSearchRows > 10 {
		maxSearchRows = 10
	}

	for i := 0; i < maxSearchRows; i++ {
		if len(rows[i]) > 0 {
			// Check if this looks like a header row
			if s.looksLikeHeader(rows[i]) {
				return i
			}
		}
	}
	return 0 // Default to first row
}

// getHeaderKeywords returns the list of keywords that indicate header columns
func (s *ExcelMetadataService) getHeaderKeywords() []string {
	return []string{
		"STT", "DIỄN GIẢI", "THU", "CHI", "NƯỚC", "ĂN", "LƯƠNG",
		"SỐ THỨ TỰ", "MÔ TẢ", "THU NHẬP", "CHI PHÍ", "TÀI SẢN", "ỨNG", "MƯỢN",
		"THU KHÁC", "CHI KHÁC", "LƯƠNG NV", "ĂN NHẸ", "CƠM",
	}
}

// GetHeaderKeywords returns the current header keywords (public method for debugging/testing)
func (s *ExcelMetadataService) GetHeaderKeywords() []string {
	return s.getHeaderKeywords()
}

// GetRequiredFieldKeywords returns the current required field keywords (public method for debugging/testing)
func (s *ExcelMetadataService) GetRequiredFieldKeywords() []string {
	return s.getRequiredFieldKeywords()
}

// looksLikeHeader determines if a row contains headers
func (s *ExcelMetadataService) looksLikeHeader(row []string) bool {
	// Get header keywords from function
	headerKeywords := s.getHeaderKeywords()

	// Count how many cells match header keywords
	matchCount := 0
	totalNonEmptyCells := 0
	hasMultipleHeaders := false

	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			totalNonEmptyCells++
			upperCell := strings.ToUpper(strings.TrimSpace(cell))

			// Check for exact matches or partial matches
			for _, keyword := range headerKeywords {
				if strings.Contains(upperCell, keyword) {
					matchCount++
					break // Count each cell only once
				}
			}
		}
	}

	// Check if we have multiple distinct header-like cells
	if totalNonEmptyCells >= 3 && matchCount >= 3 {
		hasMultipleHeaders = true
	}

	// Consider it a header row if:
	// 1. At least 3 cells match keywords AND we have multiple headers, OR
	// 2. At least 60% of non-empty cells match keywords (for smaller header sets)
	return (matchCount >= 3 && hasMultipleHeaders) ||
		(totalNonEmptyCells > 0 && matchCount >= 2 && float64(matchCount)/float64(totalNonEmptyCells) >= 0.6)
}

// analyzeColumnType determines the data type of a column based on its content
func (s *ExcelMetadataService) analyzeColumnType(file *excelize.File, sheetName string, columnIndex int, totalRows int) string {
	// Look at sample data in this column
	stringCount := 0
	numberCount := 0

	// Check a sample of rows for data type
	sampleSize := totalRows
	if sampleSize > 20 {
		sampleSize = 20
	}

	for rowIndex := 1; rowIndex < sampleSize; rowIndex++ {
		cellName, _ := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+1)
		value, err := file.GetCellValue(sheetName, cellName)
		if err != nil || value == "" {
			continue
		}

		// Check if it's a number
		if _, err := strconv.ParseFloat(value, 64); err == nil {
			numberCount++
		} else {
			stringCount++
		}
	}

	if numberCount > stringCount {
		return "number"
	}
	return "string"
}

// isCommentOrEmpty checks if a cell value is a comment or empty
func (s *ExcelMetadataService) isCommentOrEmpty(value string) bool {
	if value == "" {
		return true
	}

	// Remove common comment markers
	trimmed := strings.TrimSpace(value)
	return len(trimmed) == 0
}

// getRequiredFieldKeywords returns the list of keywords that indicate required columns
func (s *ExcelMetadataService) getRequiredFieldKeywords() []string {
	return []string{
		"STT", "DIỄN GIẢI", "THU", "CHI KHÁC",
		"SỐ THỨ TỰ", "MÔ TẢ", "THU NHẬP",
	}
}

// isRequiredColumn determines if a column is required based on the Vietnamese context
func (s *ExcelMetadataService) isRequiredColumn(columnName string) bool {
	// Get required field keywords from function
	requiredFields := s.getRequiredFieldKeywords()

	upperName := strings.ToUpper(columnName)
	for _, field := range requiredFields {
		if strings.Contains(upperName, field) {
			return true
		}
	}
	return false
}

// ParseExcelFile parses an Excel file using the metadata store
func (s *ExcelMetadataService) ParseExcelFile(filePath string) (models.Parser, error) {
	if s.fileMetadata == nil {
		return nil, fmt.Errorf("metadata store not initialized")
	}

	parser, err := NewExcelParser(s.ctx, s.fileMetadata, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	return parser, nil
}

// WriteToExcel creates a writer for appending data to the Excel file
func (s *ExcelMetadataService) WriteToExcel(filePath string) (models.Writer, error) {
	if s.fileMetadata == nil {
		return nil, fmt.Errorf("metadata store not initialized")
	}

	writer, err := NewExcelWriter(s.ctx, s.fileMetadata, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create writer: %w", err)
	}

	return writer, nil
}

// CreateNewExcelFile creates a new Excel file with headers based on metadata
func (s *ExcelMetadataService) CreateNewExcelFile(filePath string) error {
	if s.fileMetadata == nil {
		return fmt.Errorf("metadata store not initialized")
	}

	file := excelize.NewFile()

	// Create sheets based on metadata
	for i, sheetMetadata := range s.fileMetadata.Sheets {
		if i == 0 {
			// Use the first sheet
			file.SetSheetName("Sheet1", sheetMetadata.SheetName)
		} else {
			file.NewSheet(sheetMetadata.SheetName)
		}

		// Set headers
		for _, header := range sheetMetadata.Headers {
			cellReference, err := excelize.CoordinatesToCellName(header.ColumnIndex+1, 1)
			if err != nil {
				return fmt.Errorf("failed to convert coordinates to cell name: %w", err)
			}

			if err := file.SetCellValue(sheetMetadata.SheetName, cellReference, header.ColumnName); err != nil {
				return fmt.Errorf("failed to set header %s: %w", header.ColumnName, err)
			}
		}
	}

	// Save the file
	if err := file.SaveAs(filePath); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

// GetSchema returns the metadata store schema
func (s *ExcelMetadataService) GetSchema() *models.FileMetadata {
	return s.fileMetadata
}

// DebugMetadata outputs information about detected metadata
func (s *ExcelMetadataService) DebugMetadata() {
	if s.fileMetadata == nil {
		fmt.Println("Metadata store not initialized")
		return
	}

	fmt.Printf("File path: %s\n", s.fileMetadata.FilePath)
	fmt.Printf("File type: %s\n", s.fileMetadata.FileType)
	fmt.Printf("Number of sheets: %d\n", len(s.fileMetadata.Sheets))

	for i, sheet := range s.fileMetadata.Sheets {
		fmt.Printf("Sheet %d: %s\n", i+1, sheet.SheetName)
		fmt.Printf("  Columns: %d\n", len(sheet.Headers))
		for _, header := range sheet.Headers {
			fmt.Printf("    - %s [%s] (Required: %t)\n",
				header.ColumnName,
				header.DataType,
				header.Required)
		}
	}
}

// DebugHeaderDetection provides detailed information about header detection process
func (s *ExcelMetadataService) DebugHeaderDetection(filePath string) error {
	file, err := excelize.OpenFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return fmt.Errorf("no sheets found in file")
	}

	// Debug the first sheet
	sheetName := sheets[0]
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("failed to get rows: %w", err)
	}

	fmt.Printf("\n=== Header Detection Debug for Sheet: %s ===\n", sheetName)
	fmt.Printf("Total rows in sheet: %d\n", len(rows))

	// Show first 10 rows and their header detection results
	maxRows := len(rows)
	if maxRows > 10 {
		maxRows = 10
	}

	for i := 0; i < maxRows; i++ {
		row := rows[i]
		isHeader := s.looksLikeHeader(row)

		fmt.Printf("Row %d: ", i+1)
		if isHeader {
			fmt.Printf("✅ HEADER DETECTED")
		} else {
			fmt.Printf("❌ Not a header")
		}
		fmt.Printf(" - [%s]\n", strings.Join(row, ", "))
	}

	// Find and highlight the detected header row
	headerRow := s.findHeaderRow(rows)
	fmt.Printf("\n🎯 Selected header row: %d\n", headerRow+1)

	if headerRow < len(rows) {
		headerCells := rows[headerRow]
		fmt.Printf("Header cells: %s\n", strings.Join(headerCells, " | "))
	}

	return nil
}

// SetMetadataStore sets an external metadata store
func (s *ExcelMetadataService) SetMetadataStore(store *models.FileMetadata) {
	s.fileMetadata = store
}

// InitializeFromMetadata creates parser and writer from existing metadata
func (s *ExcelMetadataService) InitializeFromMetadata(store *models.FileMetadata) error {
	s.fileMetadata = store
	return nil
}
