package services

import (
	"context"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
	"import-export-backend/internal/repository/excel"
	"strconv"

	"github.com/xuri/excelize/v2"
)

// FontConfig represents font styling configuration for Excel cells
type FontConfig struct {
	Family    string  // Font family (e.g., "Arial", "Calibri", "Times New Roman")
	Size      float64 // Font size
	Bold      bool    // Bold text
	Italic    bool    // Italic text
	Underline string  // Underline style ("single", "double", "none")
	Color     string  // Font color (hex format like "FF0000" for red)
}

// CellStyle represents complete cell styling including font
type CellStyle struct {
	Font      FontConfig
	Fill      string // Background color (hex format)
	Alignment string // Text alignment ("left", "center", "right")
	Border    bool   // Whether to add borders
}

// DefaultFontConfigs provides predefined font configurations
var (
	DefaultHeaderFont = FontConfig{
		Family:    "Times New Roman",
		Size:      12,
		Bold:      true,
		Italic:    false,
		Underline: "none",
		Color:     "000000", // Black
	}

	DefaultDataFont = FontConfig{
		Family:    "Times New Roman",
		Size:      11,
		Bold:      false,
		Italic:    false,
		Underline: "none",
		Color:     "000000", // Black
	}

	DefaultHeaderStyle = CellStyle{
		Font:      DefaultHeaderFont,
		Fill:      "E7E6E6", // Light gray background
		Alignment: "center",
		Border:    true,
	}

	DefaultDataStyle = CellStyle{
		Font:      DefaultDataFont,
		Fill:      "FFFFFF", // White background
		Alignment: "left",
		Border:    true,
	}
)

// Predefined style configurations for common use cases
var (
	// Professional style with blue headers
	ProfessionalHeaderStyle = CellStyle{
		Font: FontConfig{
			Family:    "Times New Roman",
			Size:      12,
			Bold:      true,
			Italic:    false,
			Underline: "none",
			Color:     "FFFFFF", // White text
		},
		Fill:      "366092", // Dark blue background
		Alignment: "center",
		Border:    true,
	}

	ProfessionalDataStyle = CellStyle{
		Font:      DefaultDataFont,
		Fill:      "FFFFFF", // White background
		Alignment: "left",
		Border:    true,
	}

	// Corporate style with green headers
	CorporateHeaderStyle = CellStyle{
		Font: FontConfig{
			Family:    "Arial",
			Size:      11,
			Bold:      true,
			Italic:    false,
			Underline: "none",
			Color:     "FFFFFF", // White text
		},
		Fill:      "70AD47", // Green background
		Alignment: "center",
		Border:    true,
	}

	CorporateDataStyle = CellStyle{
		Font: FontConfig{
			Family:    "Arial",
			Size:      10,
			Bold:      false,
			Italic:    false,
			Underline: "none",
			Color:     "000000", // Black text
		},
		Fill:      "FFFFFF", // White background
		Alignment: "left",
		Border:    true,
	}

	// Minimal style with no borders
	MinimalHeaderStyle = CellStyle{
		Font: FontConfig{
			Family:    "Times New Roman",
			Size:      11,
			Bold:      true,
			Italic:    false,
			Underline: "none",
			Color:     "000000", // Black text
		},
		Fill:      "F2F2F2", // Light gray background
		Alignment: "left",
		Border:    false,
	}

	MinimalDataStyle = CellStyle{
		Font:      DefaultDataFont,
		Fill:      "FFFFFF", // White background
		Alignment: "left",
		Border:    false,
	}
)

type ExcelService interface {
	ExportProducts(ctx context.Context) (*excelize.File, error)
	ExportProductsWithStyle(ctx context.Context, headerStyle, dataStyle *CellStyle) (*excelize.File, error)
	ExportInventory(ctx context.Context) (*excelize.File, error)
	ExportInventoryWithStyle(ctx context.Context, headerStyle, dataStyle *CellStyle) (*excelize.File, error)
	ImportProducts(ctx context.Context, file *excelize.File) error
	ImportInventory(ctx context.Context, file *excelize.File) error
	GetProductTemplate(ctx context.Context) (*excelize.File, error)
	GetProductTemplateWithStyle(ctx context.Context, headerStyle, dataStyle *CellStyle) (*excelize.File, error)
	GetInventoryTemplate(ctx context.Context) (*excelize.File, error)
	GetInventoryTemplateWithStyle(ctx context.Context, headerStyle, dataStyle *CellStyle) (*excelize.File, error)
	// Revenue/Expense Excel operations
	InitializeRevenueExpenseFile(ctx context.Context, filePath string) error
	AddExpense(ctx context.Context, sheetName string, expenseData map[string]interface{}) error
	GetRevenueExpenseSchema(ctx context.Context) *models.FileMetadata
}

type excelService struct {
	productRepo             repository.ProductRepository
	inventoryRepo           repository.InventoryRepository
	revenueExpenseExcelRepo excel.RevenueExpenseExcelRepository
}

func NewExcelService(productRepo repository.ProductRepository, inventoryRepo repository.InventoryRepository) ExcelService {
	return &excelService{
		productRepo:             productRepo,
		inventoryRepo:           inventoryRepo,
		revenueExpenseExcelRepo: excel.NewRevenueExpenseExcelRepository(),
	}
}

// createFontStyle creates an Excel font style ID from FontConfig
func (s *excelService) createFontStyle(file *excelize.File, fontConfig FontConfig) (int, error) {
	style := &excelize.Style{
		Font: &excelize.Font{
			Family:    fontConfig.Family,
			Size:      fontConfig.Size,
			Bold:      fontConfig.Bold,
			Italic:    fontConfig.Italic,
			Underline: fontConfig.Underline,
			Color:     fontConfig.Color,
		},
	}
	return file.NewStyle(style)
}

// createCellStyle creates a complete Excel style ID from CellStyle
func (s *excelService) createCellStyle(file *excelize.File, cellStyle CellStyle) (int, error) {
	style := &excelize.Style{
		Font: &excelize.Font{
			Family:    cellStyle.Font.Family,
			Size:      cellStyle.Font.Size,
			Bold:      cellStyle.Font.Bold,
			Italic:    cellStyle.Font.Italic,
			Underline: cellStyle.Font.Underline,
			Color:     cellStyle.Font.Color,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{cellStyle.Fill},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: cellStyle.Alignment,
		},
	}

	if cellStyle.Border {
		style.Border = []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		}
	}

	return file.NewStyle(style)
}

// applyStyleToCell applies a style to a specific cell
func (s *excelService) applyStyleToCell(file *excelize.File, sheetName, cell string, styleID int) error {
	return file.SetCellStyle(sheetName, cell, cell, styleID)
}

// applyStyleToRange applies a style to a range of cells
func (s *excelService) applyStyleToRange(file *excelize.File, sheetName, startCell, endCell string, styleID int) error {
	return file.SetCellStyle(sheetName, startCell, endCell, styleID)
}

// setCellValueWithStyle sets cell value and applies style in one operation
func (s *excelService) setCellValueWithStyle(file *excelize.File, sheetName, cell string, value interface{}, styleID int) error {
	if err := file.SetCellValue(sheetName, cell, value); err != nil {
		return err
	}
	return file.SetCellStyle(sheetName, cell, cell, styleID)
}

func (s *excelService) ExportProducts(ctx context.Context) (*excelize.File, error) {
	// Use default styling
	return s.ExportProductsWithStyle(ctx, &DefaultHeaderStyle, &DefaultDataStyle)
}

func (s *excelService) ExportProductsWithStyle(ctx context.Context, headerStyle, dataStyle *CellStyle) (*excelize.File, error) {
	products, err := s.productRepo.List(ctx, 1000, 0, "created_at", "desc") // Get all products
	if err != nil {
		return nil, err
	}

	file := excelize.NewFile()
	sheetName := "Products"
	file.NewSheet(sheetName)

	// Create styles
	headerStyleID, err := s.createCellStyle(file, *headerStyle)
	if err != nil {
		return nil, err
	}

	dataStyleID, err := s.createCellStyle(file, *dataStyle)
	if err != nil {
		return nil, err
	}

	// Headers with styling
	headers := []string{"ID", "Name", "Supplier", "Unit Price", "Status", "Stock Quantity", "Reorder Level", "Location", "Last Updated"}
	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		if err := s.setCellValueWithStyle(file, sheetName, cell, header, headerStyleID); err != nil {
			return nil, err
		}
	}

	// Data with styling
	for i, product := range products {
		row := i + 2
		rowStr := strconv.Itoa(row)

		if err := s.setCellValueWithStyle(file, sheetName, "A"+rowStr, strconv.Itoa(int(product.ID)), dataStyleID); err != nil {
			return nil, err
		}
		if err := s.setCellValueWithStyle(file, sheetName, "B"+rowStr, product.Name, dataStyleID); err != nil {
			return nil, err
		}
		if err := s.setCellValueWithStyle(file, sheetName, "C"+rowStr, product.Supplier.Name, dataStyleID); err != nil {
			return nil, err
		}
		if err := s.setCellValueWithStyle(file, sheetName, "D"+rowStr, product.UnitPrice, dataStyleID); err != nil {
			return nil, err
		}
		if err := s.setCellValueWithStyle(file, sheetName, "E"+rowStr, product.Status, dataStyleID); err != nil {
			return nil, err
		}

		if product.Inventory != nil {
			if err := s.setCellValueWithStyle(file, sheetName, "F"+rowStr, product.Inventory.Quantity, dataStyleID); err != nil {
				return nil, err
			}
			if err := s.setCellValueWithStyle(file, sheetName, "G"+rowStr, product.Inventory.ReorderLevel, dataStyleID); err != nil {
				return nil, err
			}
			if err := s.setCellValueWithStyle(file, sheetName, "H"+rowStr, product.Inventory.Location, dataStyleID); err != nil {
				return nil, err
			}
		}
	}

	// Auto-fit columns
	for i := range headers {
		col := string(rune('A' + i))
		if err := file.SetColWidth(sheetName, col, col, 15); err != nil {
			return nil, err
		}
	}

	return file, nil
}

func (s *excelService) ExportInventory(ctx context.Context) (*excelize.File, error) {
	// Use default styling
	return s.ExportInventoryWithStyle(ctx, &DefaultHeaderStyle, &DefaultDataStyle)
}

func (s *excelService) ExportInventoryWithStyle(ctx context.Context, headerStyle, dataStyle *CellStyle) (*excelize.File, error) {
	inventory, err := s.inventoryRepo.List(ctx, 1000, 0) // Get all inventory
	if err != nil {
		return nil, err
	}

	file := excelize.NewFile()
	sheetName := "Inventory"
	file.NewSheet(sheetName)

	// Create styles
	headerStyleID, err := s.createCellStyle(file, *headerStyle)
	if err != nil {
		return nil, err
	}

	dataStyleID, err := s.createCellStyle(file, *dataStyle)
	if err != nil {
		return nil, err
	}

	// Headers with styling
	headers := []string{"Product ID", "Product Name", "Current Stock", "Reorder Level", "Location", "Last Transaction", "Transaction Type", "Quantity Changed", "Date"}
	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		if err := s.setCellValueWithStyle(file, sheetName, cell, header, headerStyleID); err != nil {
			return nil, err
		}
	}

	// Data with styling
	for i, inv := range inventory {
		row := i + 2
		rowStr := strconv.Itoa(row)

		if err := s.setCellValueWithStyle(file, sheetName, "A"+rowStr, strconv.Itoa(int(inv.ProductID)), dataStyleID); err != nil {
			return nil, err
		}
		if err := s.setCellValueWithStyle(file, sheetName, "B"+rowStr, inv.Product.Name, dataStyleID); err != nil {
			return nil, err
		}
		if err := s.setCellValueWithStyle(file, sheetName, "C"+rowStr, inv.Quantity, dataStyleID); err != nil {
			return nil, err
		}
		if err := s.setCellValueWithStyle(file, sheetName, "D"+rowStr, inv.ReorderLevel, dataStyleID); err != nil {
			return nil, err
		}
		if err := s.setCellValueWithStyle(file, sheetName, "E"+rowStr, inv.Location, dataStyleID); err != nil {
			return nil, err
		}
		if err := s.setCellValueWithStyle(file, sheetName, "F"+rowStr, "N/A", dataStyleID); err != nil { // Last transaction - would need to query transactions
			return nil, err
		}
		if err := s.setCellValueWithStyle(file, sheetName, "G"+rowStr, "N/A", dataStyleID); err != nil { // Transaction type
			return nil, err
		}
		if err := s.setCellValueWithStyle(file, sheetName, "H"+rowStr, "N/A", dataStyleID); err != nil { // Quantity changed
			return nil, err
		}
	}

	// Auto-fit columns
	for i := range headers {
		col := string(rune('A' + i))
		if err := file.SetColWidth(sheetName, col, col, 15); err != nil {
			return nil, err
		}
	}

	return file, nil
}

func (s *excelService) ImportProducts(ctx context.Context, file *excelize.File) error {
	// Implementation for importing products from Excel
	// This would read the Excel file and create/update products
	return nil
}

func (s *excelService) ImportInventory(ctx context.Context, file *excelize.File) error {
	// Implementation for importing inventory from Excel
	// This would read the Excel file and update inventory quantities
	return nil
}

func (s *excelService) GetProductTemplate(ctx context.Context) (*excelize.File, error) {
	// Use default styling
	return s.GetProductTemplateWithStyle(ctx, &DefaultHeaderStyle, &DefaultDataStyle)
}

func (s *excelService) GetProductTemplateWithStyle(ctx context.Context, headerStyle, dataStyle *CellStyle) (*excelize.File, error) {
	file := excelize.NewFile()
	sheetName := "Products"
	file.NewSheet(sheetName)

	// Create styles
	headerStyleID, err := s.createCellStyle(file, *headerStyle)
	if err != nil {
		return nil, err
	}

	dataStyleID, err := s.createCellStyle(file, *dataStyle)
	if err != nil {
		return nil, err
	}

	// Headers with styling
	headers := []string{"Name", "Supplier ID", "Unit Price", "Status", "Description"}
	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		if err := s.setCellValueWithStyle(file, sheetName, cell, header, headerStyleID); err != nil {
			return nil, err
		}
	}

	// Example data with styling
	if err := s.setCellValueWithStyle(file, sheetName, "A2", "Sample Product", dataStyleID); err != nil {
		return nil, err
	}
	if err := s.setCellValueWithStyle(file, sheetName, "B2", "supplier-uuid-here", dataStyleID); err != nil {
		return nil, err
	}
	if err := s.setCellValueWithStyle(file, sheetName, "C2", 99.99, dataStyleID); err != nil {
		return nil, err
	}
	if err := s.setCellValueWithStyle(file, sheetName, "D2", "active", dataStyleID); err != nil {
		return nil, err
	}
	if err := s.setCellValueWithStyle(file, sheetName, "E2", "Sample product description", dataStyleID); err != nil {
		return nil, err
	}

	// Auto-fit columns
	for i := range headers {
		col := string(rune('A' + i))
		if err := file.SetColWidth(sheetName, col, col, 15); err != nil {
			return nil, err
		}
	}

	return file, nil
}

func (s *excelService) GetInventoryTemplate(ctx context.Context) (*excelize.File, error) {
	// Use default styling
	return s.GetInventoryTemplateWithStyle(ctx, &DefaultHeaderStyle, &DefaultDataStyle)
}

func (s *excelService) GetInventoryTemplateWithStyle(ctx context.Context, headerStyle, dataStyle *CellStyle) (*excelize.File, error) {
	file := excelize.NewFile()
	sheetName := "Inventory"
	file.NewSheet(sheetName)

	// Create styles
	headerStyleID, err := s.createCellStyle(file, *headerStyle)
	if err != nil {
		return nil, err
	}

	dataStyleID, err := s.createCellStyle(file, *dataStyle)
	if err != nil {
		return nil, err
	}

	// Headers with styling
	headers := []string{"Product ID", "Quantity", "Reorder Level", "Location"}
	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		if err := s.setCellValueWithStyle(file, sheetName, cell, header, headerStyleID); err != nil {
			return nil, err
		}
	}

	// Example data with styling
	if err := s.setCellValueWithStyle(file, sheetName, "A2", "product-uuid-here", dataStyleID); err != nil {
		return nil, err
	}
	if err := s.setCellValueWithStyle(file, sheetName, "B2", 100, dataStyleID); err != nil {
		return nil, err
	}
	if err := s.setCellValueWithStyle(file, sheetName, "C2", 10, dataStyleID); err != nil {
		return nil, err
	}
	if err := s.setCellValueWithStyle(file, sheetName, "D2", "A1-B2", dataStyleID); err != nil {
		return nil, err
	}

	// Auto-fit columns
	for i := range headers {
		col := string(rune('A' + i))
		if err := file.SetColWidth(sheetName, col, col, 15); err != nil {
			return nil, err
		}
	}

	return file, nil
}

// InitializeRevenueExpenseFile initializes the revenue/expense Excel file
func (s *excelService) InitializeRevenueExpenseFile(ctx context.Context, filePath string) error {
	return s.revenueExpenseExcelRepo.InitializeWithFile(ctx, filePath)
}

// AddExpense adds a new expense entry to the Excel file
func (s *excelService) AddExpense(ctx context.Context, sheetName string, expenseData map[string]interface{}) error {
	return s.revenueExpenseExcelRepo.AddExpense(ctx, sheetName, expenseData)
}

// GetRevenueExpenseSchema returns the Excel file schema
func (s *excelService) GetRevenueExpenseSchema(ctx context.Context) *models.FileMetadata {
	return s.revenueExpenseExcelRepo.GetSchema(ctx)
}
