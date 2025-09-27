package services

import (
	"bytes"
	"context"
	"fmt"
	"import-export-backend/internal/models"
	"strconv"

	"github.com/xuri/excelize/v2"
)

// ExcelParser implements the Parser interface to parse Excel files based on metadata
type ExcelParser struct {
	metadataStore *models.FileMetadata
	excelFile     *excelize.File
	ctx           context.Context
}

// NewExcelParser creates a new ExcelParser instance
func NewExcelParser(ctx context.Context, metadataStore *models.FileMetadata, filePath string) (*ExcelParser, error) {
	excelFile, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel file: %w", err)
	}

	return &ExcelParser{
		metadataStore: metadataStore,
		excelFile:     excelFile,
		ctx:           ctx,
	}, nil
}

// NewExcelParserFromBuffer creates a new ExcelParser from a file buffer
func NewExcelParserFromBuffer(ctx context.Context, metadataStore *models.FileMetadata, buffer []byte) (*ExcelParser, error) {
	reader := bytes.NewReader(buffer)
	excelFile, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel file from buffer: %w", err)
	}

	return &ExcelParser{
		metadataStore: metadataStore,
		excelFile:     excelFile,
		ctx:           ctx,
	}, nil
}

// Parse reads all data from the Excel file based on the metadata schema
func (p *ExcelParser) Parse() ([]map[string]interface{}, error) {
	var allData []map[string]interface{}

	// Parse all sheets defined in metadata
	for _, sheet := range p.metadataStore.Sheets {
		sheetData, err := p.ParseSheet(sheet.SheetName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse sheet %s: %w", sheet.SheetName, err)
		}
		// Add sheet name as metadata to each row
		for _, row := range sheetData {
			row["_sheet_name"] = sheet.SheetName
			allData = append(allData, row)
		}
	}

	return allData, nil
}

// ParseSheet reads data from a specific sheet based on metadata schema
func (p *ExcelParser) ParseSheet(sheetName string) ([]map[string]interface{}, error) {
	var result []map[string]interface{}

	// Find metadata for this sheet
	var sheetMetadata *models.ExcelSheetMetadata
	for _, sheet := range p.metadataStore.Sheets {
		if sheet.SheetName == sheetName {
			sheetMetadata = &sheet
			break
		}
	}

	if sheetMetadata == nil {
		return nil, fmt.Errorf("metadata not found for sheet: %s", sheetName)
	}

	// Get all rows from the sheet
	rows, err := p.excelFile.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows from sheet %s: %w", sheetName, err)
	}

	if len(rows) == 0 {
		return result, nil
	}

	// Create a map to lookup column name by index
	columnMap := make(map[int]string)
	for _, header := range sheetMetadata.Headers {
		columnMap[header.ColumnIndex] = header.ColumnName
	}

	// Parse data rows (skip header if present)
	startRow := 0
	if len(rows) > 0 {
		// Check if first row is header by comparing with metadata
		isHeader := false
		for i, header := range sheetMetadata.Headers {
			if i < len(rows[0]) {
				expectedValue := rows[0][i]
				if expectedValue == header.ColumnName {
					isHeader = true
				} else {
					isHeader = false
					break
				}
			}
		}
		if isHeader {
			startRow = 1
		}
	}

	// Parse each row
	for i := startRow; i < len(rows); i++ {
		row := rows[i]
		rowData := make(map[string]interface{})

		// Parse each column based on metadata
		for _, header := range sheetMetadata.Headers {
			rowIndex := header.ColumnIndex
			var cellValue interface{}

			if rowIndex < len(row) {
				cellValue = row[rowIndex]
			} else {
				// Empty cell
				cellValue = ""
			}

			// Convert based on data type
			if header.DataType == "number" && cellValue != "" {
				if val, err := strconv.ParseFloat(fmt.Sprintf("%v", cellValue), 64); err == nil {
					cellValue = val
				}
			}

			rowData[header.ColumnName] = cellValue
		}

		result = append(result, rowData)
	}

	return result, nil
}

// GetSheetData reads raw data from a specific sheet
func (p *ExcelParser) GetSheetData(sheetName string) ([][]string, error) {
	rows, err := p.excelFile.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows from sheet %s: %w", sheetName, err)
	}

	return rows, nil
}

// GetSheetNames returns all sheet names in the Excel file
func (p *ExcelParser) GetSheetNames() []string {
	return p.excelFile.GetSheetList()
}

// Close releases the file resources
func (p *ExcelParser) Close() error {
	if p.excelFile != nil {
		return p.excelFile.Close()
	}
	return nil
}
