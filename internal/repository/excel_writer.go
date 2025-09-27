package repository

import (
	"bytes"
	"context"
	"fmt"
	"import-export-backend/internal/models"

	"github.com/xuri/excelize/v2"
)

// ExcelWriterRepository defines the interface for Excel writing operations
type ExcelWriterRepository interface {
	NewWriter(ctx context.Context, metadataStore *models.FileMetadata, filePath string) (ExcelWriterInterface, error)
	NewWriterFromBuffer(ctx context.Context, metadataStore *models.FileMetadata, buffer []byte, filePath string) (ExcelWriterInterface, error)
}

// ExcelWriterInterface defines the interface for writing to Excel files
type ExcelWriterInterface interface {
	AppendRow(rowData map[string]interface{}) error
	AppendRowToSheet(sheetName string, rowData map[string]interface{}) error
	AppendRowWithValidation(sheetName string, rowData map[string]interface{}) error
	AppendRows(sheetName string, rows []map[string]interface{}) error
	GetCellValue(sheetName, cellReference string) (interface{}, error)
	SetCellValue(sheetName, cellReference string, value interface{}) error
	Save() error
	SaveToPath(filePath string) error
	Close() error
}

// excelWriterRepository implements ExcelWriterRepository
type excelWriterRepository struct{}

// NewExcelWriterRepository creates a new ExcelWriterRepository
func NewExcelWriterRepository() ExcelWriterRepository {
	return &excelWriterRepository{}
}

// NewWriter creates a new ExcelWriter instance for an existing file
func (r *excelWriterRepository) NewWriter(ctx context.Context, metadataStore *models.FileMetadata, filePath string) (ExcelWriterInterface, error) {
	return NewExcelWriter(ctx, metadataStore, filePath)
}

// NewWriterFromBuffer creates a new ExcelWriter from a buffer
func (r *excelWriterRepository) NewWriterFromBuffer(ctx context.Context, metadataStore *models.FileMetadata, buffer []byte, filePath string) (ExcelWriterInterface, error) {
	return NewExcelWriterFromBuffer(ctx, metadataStore, buffer, filePath)
}

// ExcelWriter implements the Writer interface to append rows to Excel files
type ExcelWriter struct {
	metadataStore *models.FileMetadata
	excelFile     *excelize.File
	ctx           context.Context
	filePath      string
}

// NewExcelWriter creates a new ExcelWriter instance for an existing file
func NewExcelWriter(ctx context.Context, metadataStore *models.FileMetadata, filePath string) (*ExcelWriter, error) {
	excelFile, err := excelize.OpenFile(filePath)
	if err != nil {
		// Try to create a new file if it doesn't exist
		excelFile = excelize.NewFile()
	}

	return &ExcelWriter{
		metadataStore: metadataStore,
		excelFile:     excelFile,
		ctx:           ctx,
		filePath:      filePath,
	}, nil
}

// NewExcelWriterFromBuffer creates a new ExcelWriter from a buffer
func NewExcelWriterFromBuffer(ctx context.Context, metadataStore *models.FileMetadata, buffer []byte, filePath string) (*ExcelWriter, error) {
	reader := bytes.NewReader(buffer)
	excelFile, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel file from buffer: %w", err)
	}

	return &ExcelWriter{
		metadataStore: metadataStore,
		excelFile:     excelFile,
		ctx:           ctx,
		filePath:      filePath,
	}, nil
}

// AppendRow appends a new row to the default sheet (first sheet in metadata)
func (w *ExcelWriter) AppendRow(rowData map[string]interface{}) error {
	if len(w.metadataStore.Sheets) == 0 {
		return fmt.Errorf("no sheets defined in metadata store")
	}

	// Use the first sheet as default
	defaultSheet := w.metadataStore.Sheets[0]
	return w.AppendRowToSheet(defaultSheet.SheetName, rowData)
}

// AppendRowToSheet appends a new row to a specific sheet
func (w *ExcelWriter) AppendRowToSheet(sheetName string, rowData map[string]interface{}) error {
	// Find sheet metadata
	var sheetMetadata *models.ExcelSheetMetadata
	for _, sheet := range w.metadataStore.Sheets {
		if sheet.SheetName == sheetName {
			sheetMetadata = &sheet
			break
		}
	}

	if sheetMetadata == nil {
		return fmt.Errorf("sheet metadata not found: %s", sheetName)
	}

	// Check if sheet exists, create if not
	if !w.sheetExists(sheetName) {
		if err := w.createSheetFromMetadata(sheetMetadata); err != nil {
			return fmt.Errorf("failed to create sheet %s: %w", sheetName, err)
		}
	}

	// Find the next empty row
	nextRow, err := w.getNextEmptyRow(sheetName)
	if err != nil {
		return fmt.Errorf("failed to find next empty row: %w", err)
	}

	// Write row data according to schema
	for _, header := range sheetMetadata.Headers {
		cellReference, err := excelize.CoordinatesToCellName(header.ColumnIndex+1, nextRow)
		if err != nil {
			return fmt.Errorf("failed to convert coordinates to cell name: %w", err)
		}

		var value interface{}
		if dataValue, exists := rowData[header.ColumnName]; exists {
			value = dataValue
		} else {
			value = ""
		}

		if err := w.excelFile.SetCellValue(sheetName, cellReference, value); err != nil {
			return fmt.Errorf("failed to set cell value %s: %w", cellReference, err)
		}
	}

	return nil
}

// AppendRowWithValidation appends a row after validating data against schema
func (w *ExcelWriter) AppendRowWithValidation(sheetName string, rowData map[string]interface{}) error {
	// Find sheet metadata
	var sheetMetadata *models.ExcelSheetMetadata
	for _, sheet := range w.metadataStore.Sheets {
		if sheet.SheetName == sheetName {
			sheetMetadata = &sheet
			break
		}
	}

	if sheetMetadata == nil {
		return fmt.Errorf("sheet metadata not found: %s", sheetName)
	}

	// Validate required fields
	for _, header := range sheetMetadata.Headers {
		if header.Required {
			if value, exists := rowData[header.ColumnName]; !exists || value == nil || value == "" {
				return fmt.Errorf("required field '%s' is empty", header.ColumnName)
			}
		}
	}

	return w.AppendRowToSheet(sheetName, rowData)
}

// getNextEmptyRow finds the next empty row in the specified sheet
func (w *ExcelWriter) getNextEmptyRow(sheetName string) (int, error) {
	rows, err := w.excelFile.GetRows(sheetName)
	if err != nil {
		return 1, nil // Start from row 1 if sheet doesn't exist
	}

	return len(rows) + 1, nil
}

// sheetExists checks if a sheet exists in the Excel file
func (w *ExcelWriter) sheetExists(sheetName string) bool {
	for _, sheet := range w.excelFile.GetSheetList() {
		if sheet == sheetName {
			return true
		}
	}
	return false
}

// createSheetFromMetadata creates a new sheet with headers based on metadata
func (w *ExcelWriter) createSheetFromMetadata(sheetMetadata *models.ExcelSheetMetadata) error {
	// Create the sheet
	index, err := w.excelFile.NewSheet(sheetMetadata.SheetName)
	if err != nil {
		return fmt.Errorf("failed to create new sheet: %w", err)
	}

	// Set headers
	for _, header := range sheetMetadata.Headers {
		cellReference, err := excelize.CoordinatesToCellName(header.ColumnIndex+1, 1)
		if err != nil {
			return fmt.Errorf("failed to convert coordinates to cell name: %w", err)
		}

		if err := w.excelFile.SetCellValue(sheetMetadata.SheetName, cellReference, header.ColumnName); err != nil {
			return fmt.Errorf("failed to set header %s: %w", header.ColumnName, err)
		}
	}

	// Set as active sheet if it's the first sheet
	if index == 1 {
		w.excelFile.SetActiveSheet(index)
	}

	return nil
}

// Save saves the Excel file to disk
func (w *ExcelWriter) Save() error {
	if err := w.excelFile.SaveAs(w.filePath); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}
	return nil
}

// SaveToPath saves the Excel file to a different path
func (w *ExcelWriter) SaveToPath(filePath string) error {
	if err := w.excelFile.SaveAs(filePath); err != nil {
		return fmt.Errorf("failed to save file to %s: %w", filePath, err)
	}
	return nil
}

// AppendRows appends multiple rows at once
func (w *ExcelWriter) AppendRows(sheetName string, rows []map[string]interface{}) error {
	for _, row := range rows {
		if err := w.AppendRowToSheet(sheetName, row); err != nil {
			return fmt.Errorf("failed to append row: %w", err)
		}
	}
	return nil
}

// GetCellValue retrieves a value from a specific cell
func (w *ExcelWriter) GetCellValue(sheetName, cellReference string) (interface{}, error) {
	value, err := w.excelFile.GetCellValue(sheetName, cellReference)
	if err != nil {
		return nil, fmt.Errorf("failed to get cell value: %w", err)
	}
	return value, nil
}

// SetCellValue sets a value to a specific cell
func (w *ExcelWriter) SetCellValue(sheetName, cellReference string, value interface{}) error {
	return w.excelFile.SetCellValue(sheetName, cellReference, value)
}

// Close closes the writer (performed automatically by Save)
func (w *ExcelWriter) Close() error {
	if w.excelFile != nil {
		return w.excelFile.Close()
	}
	return nil
}
