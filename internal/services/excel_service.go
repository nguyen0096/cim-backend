package services

import (
	"import-export-backend/internal/repository"
	"strconv"

	"github.com/xuri/excelize/v2"
)

type ExcelService interface {
	ExportProducts() (*excelize.File, error)
	ExportInventory() (*excelize.File, error)
	ImportProducts(file *excelize.File) error
	ImportInventory(file *excelize.File) error
	GetProductTemplate() (*excelize.File, error)
	GetInventoryTemplate() (*excelize.File, error)
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

func (s *excelService) ExportProducts() (*excelize.File, error) {
	products, err := s.productRepo.List(1000, 0, "created_at", "desc") // Get all products
	if err != nil {
		return nil, err
	}

	file := excelize.NewFile()
	sheetName := "Products"
	file.NewSheet(sheetName)

	// Headers
	headers := []string{"ID", "Name", "SKU", "Supplier", "Unit Price", "Status", "Stock Quantity", "Reorder Level", "Location", "Last Updated"}
	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		file.SetCellValue(sheetName, cell, header)
	}

	// Data
	for i, product := range products {
		row := i + 2
		file.SetCellValue(sheetName, "A"+strconv.Itoa(row), product.ID.String())
		file.SetCellValue(sheetName, "B"+strconv.Itoa(row), product.Name)
		file.SetCellValue(sheetName, "C"+strconv.Itoa(row), product.SKU)
		file.SetCellValue(sheetName, "D"+strconv.Itoa(row), product.Supplier.Name)
		file.SetCellValue(sheetName, "E"+strconv.Itoa(row), product.UnitPrice)
		file.SetCellValue(sheetName, "F"+strconv.Itoa(row), product.Status)

		if product.Inventory != nil {
			file.SetCellValue(sheetName, "G"+strconv.Itoa(row), product.Inventory.Quantity)
			file.SetCellValue(sheetName, "H"+strconv.Itoa(row), product.Inventory.ReorderLevel)
			file.SetCellValue(sheetName, "I"+strconv.Itoa(row), product.Inventory.Location)
			file.SetCellValue(sheetName, "J"+strconv.Itoa(row), product.Inventory.LastUpdated.Format("2006-01-02 15:04:05"))
		}
	}

	return file, nil
}

func (s *excelService) ExportInventory() (*excelize.File, error) {
	inventory, err := s.inventoryRepo.List(1000, 0) // Get all inventory
	if err != nil {
		return nil, err
	}

	file := excelize.NewFile()
	sheetName := "Inventory"
	file.NewSheet(sheetName)

	// Headers
	headers := []string{"Product ID", "Product Name", "SKU", "Current Stock", "Reorder Level", "Location", "Last Transaction", "Transaction Type", "Quantity Changed", "Date"}
	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		file.SetCellValue(sheetName, cell, header)
	}

	// Data
	for i, inv := range inventory {
		row := i + 2
		file.SetCellValue(sheetName, "A"+strconv.Itoa(row), inv.ProductID.String())
		file.SetCellValue(sheetName, "B"+strconv.Itoa(row), inv.Product.Name)
		file.SetCellValue(sheetName, "C"+strconv.Itoa(row), inv.Product.SKU)
		file.SetCellValue(sheetName, "D"+strconv.Itoa(row), inv.Quantity)
		file.SetCellValue(sheetName, "E"+strconv.Itoa(row), inv.ReorderLevel)
		file.SetCellValue(sheetName, "F"+strconv.Itoa(row), inv.Location)
		file.SetCellValue(sheetName, "G"+strconv.Itoa(row), "N/A") // Last transaction - would need to query transactions
		file.SetCellValue(sheetName, "H"+strconv.Itoa(row), "N/A") // Transaction type
		file.SetCellValue(sheetName, "I"+strconv.Itoa(row), "N/A") // Quantity changed
		file.SetCellValue(sheetName, "J"+strconv.Itoa(row), inv.LastUpdated.Format("2006-01-02 15:04:05"))
	}

	return file, nil
}

func (s *excelService) ImportProducts(file *excelize.File) error {
	// Implementation for importing products from Excel
	// This would read the Excel file and create/update products
	return nil
}

func (s *excelService) ImportInventory(file *excelize.File) error {
	// Implementation for importing inventory from Excel
	// This would read the Excel file and update inventory quantities
	return nil
}

func (s *excelService) GetProductTemplate() (*excelize.File, error) {
	file := excelize.NewFile()
	sheetName := "Products"
	file.NewSheet(sheetName)

	// Headers
	headers := []string{"Name", "SKU", "Supplier ID", "Unit Price", "Status", "Description"}
	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		file.SetCellValue(sheetName, cell, header)
	}

	// Example data
	file.SetCellValue(sheetName, "A2", "Sample Product")
	file.SetCellValue(sheetName, "B2", "SKU-001")
	file.SetCellValue(sheetName, "C2", "supplier-uuid-here")
	file.SetCellValue(sheetName, "D2", 99.99)
	file.SetCellValue(sheetName, "E2", "active")
	file.SetCellValue(sheetName, "F2", "Sample product description")

	return file, nil
}

func (s *excelService) GetInventoryTemplate() (*excelize.File, error) {
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
