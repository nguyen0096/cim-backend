package services

import (
	"context"
	"import-export-backend/internal/repository"
	"strconv"

	"github.com/xuri/excelize/v2"
)

type ExcelService interface {
	ExportProducts(ctx context.Context) (*excelize.File, error)
	ExportInventory(ctx context.Context) (*excelize.File, error)
	ImportProducts(ctx context.Context, file *excelize.File) error
	ImportInventory(ctx context.Context, file *excelize.File) error
	GetProductTemplate(ctx context.Context) (*excelize.File, error)
	GetInventoryTemplate(ctx context.Context) (*excelize.File, error)
}

type excelService struct {
	productRepo   repository.ProductRepository
	inventoryRepo repository.InventoryRepository
}

func NewExcelService(productRepo repository.ProductRepository, inventoryRepo repository.InventoryRepository) ExcelService {
	return &excelService{
		productRepo:   productRepo,
		inventoryRepo: inventoryRepo,
	}
}

func (s *excelService) ExportProducts(ctx context.Context) (*excelize.File, error) {
	products, err := s.productRepo.List(ctx, 1000, 0, "created_at", "desc") // Get all products
	if err != nil {
		return nil, err
	}

	file := excelize.NewFile()
	sheetName := "Products"
	file.NewSheet(sheetName)

	// Headers
	headers := []string{"ID", "Name", "Supplier", "Unit Price", "Status", "Stock Quantity", "Reorder Level", "Location", "Last Updated"}
	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		file.SetCellValue(sheetName, cell, header)
	}

	// Data
	for i, product := range products {
		row := i + 2
		file.SetCellValue(sheetName, "A"+strconv.Itoa(row), strconv.Itoa(int(product.ID)))
		file.SetCellValue(sheetName, "B"+strconv.Itoa(row), product.Name)
		file.SetCellValue(sheetName, "C"+strconv.Itoa(row), product.Supplier.Name)
		file.SetCellValue(sheetName, "D"+strconv.Itoa(row), product.UnitPrice)
		file.SetCellValue(sheetName, "E"+strconv.Itoa(row), product.Status)

		if product.Inventory != nil {
			file.SetCellValue(sheetName, "F"+strconv.Itoa(row), product.Inventory.Quantity)
			file.SetCellValue(sheetName, "G"+strconv.Itoa(row), product.Inventory.ReorderLevel)
			file.SetCellValue(sheetName, "H"+strconv.Itoa(row), product.Inventory.Location)
		}
	}

	return file, nil
}

func (s *excelService) ExportInventory(ctx context.Context) (*excelize.File, error) {
	inventory, err := s.inventoryRepo.List(ctx, 1000, 0) // Get all inventory
	if err != nil {
		return nil, err
	}

	file := excelize.NewFile()
	sheetName := "Inventory"
	file.NewSheet(sheetName)

	// Headers
	headers := []string{"Product ID", "Product Name", "Current Stock", "Reorder Level", "Location", "Last Transaction", "Transaction Type", "Quantity Changed", "Date"}
	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		file.SetCellValue(sheetName, cell, header)
	}

	// Data
	for i, inv := range inventory {
		row := i + 2
		file.SetCellValue(sheetName, "A"+strconv.Itoa(row), strconv.Itoa(int(inv.ProductID)))
		file.SetCellValue(sheetName, "B"+strconv.Itoa(row), inv.Product.Name)
		file.SetCellValue(sheetName, "C"+strconv.Itoa(row), inv.Quantity)
		file.SetCellValue(sheetName, "D"+strconv.Itoa(row), inv.ReorderLevel)
		file.SetCellValue(sheetName, "E"+strconv.Itoa(row), inv.Location)
		file.SetCellValue(sheetName, "F"+strconv.Itoa(row), "N/A") // Last transaction - would need to query transactions
		file.SetCellValue(sheetName, "G"+strconv.Itoa(row), "N/A") // Transaction type
		file.SetCellValue(sheetName, "H"+strconv.Itoa(row), "N/A") // Quantity changed
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
	file := excelize.NewFile()
	sheetName := "Products"
	file.NewSheet(sheetName)

	// Headers
	headers := []string{"Name", "Supplier ID", "Unit Price", "Status", "Description"}
	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		file.SetCellValue(sheetName, cell, header)
	}

	// Example data
	file.SetCellValue(sheetName, "A2", "Sample Product")
	file.SetCellValue(sheetName, "B2", "supplier-uuid-here")
	file.SetCellValue(sheetName, "C2", 99.99)
	file.SetCellValue(sheetName, "D2", "active")
	file.SetCellValue(sheetName, "E2", "Sample product description")

	return file, nil
}

func (s *excelService) GetInventoryTemplate(ctx context.Context) (*excelize.File, error) {
	file := excelize.NewFile()
	sheetName := "Inventory"
	file.NewSheet(sheetName)

	// Headers
	headers := []string{"Product ID", "Quantity", "Reorder Level", "Location"}
	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		file.SetCellValue(sheetName, cell, header)
	}

	// Example data
	file.SetCellValue(sheetName, "A2", "product-uuid-here")
	file.SetCellValue(sheetName, "B2", 100)
	file.SetCellValue(sheetName, "C2", 10)
	file.SetCellValue(sheetName, "D2", "A1-B2")

	return file, nil
}
