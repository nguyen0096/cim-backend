package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
	"import-export-backend/pkg"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

type ProductService interface {
	CreateProduct(ctx context.Context, product *models.Product) error
	GetProductByID(ctx context.Context, id uint) (*models.Product, error)
	UpdateProduct(ctx context.Context, product *models.Product) error
	UpdateProductStatus(ctx context.Context, id uint, status string) error
	DeleteProduct(ctx context.Context, id uint) error
	RestoreProduct(ctx context.Context, id uint) error
	GetProductsBySupplier(ctx context.Context, supplierID uint) ([]models.Product, error)
	SearchProducts(ctx context.Context, query string, sortBy, sortOrder string) ([]models.Product, error)
	SearchProductsWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, status string) ([]models.Product, error)
	CountProducts(ctx context.Context, status string) (int64, error)
	CountSearchProducts(ctx context.Context, query string, status string) (int64, error)

	// v1
	ListProducts(ctx context.Context, limit, offset int, sortBy, sortOrder, status string) ([]models.Product, error)
	ImportProductsFromCSV(ctx context.Context, csvReader io.Reader) (int, error)
	ImportProductsFromExcel(ctx context.Context, excelReader io.Reader) (int, error)
}

type productService struct {
	productRepo  repository.ProductRepository
	supplierRepo repository.SupplierRepository
}

func NewProductService(productRepo repository.ProductRepository, supplierRepo repository.SupplierRepository) ProductService {
	return &productService{
		productRepo:  productRepo,
		supplierRepo: supplierRepo,
	}
}

func (s *productService) CreateProduct(ctx context.Context, product *models.Product) error {
	return s.productRepo.Create(ctx, product)
}

func (s *productService) GetProductByID(ctx context.Context, id uint) (*models.Product, error) {
	return s.productRepo.GetByID(ctx, id)
}

func (s *productService) UpdateProduct(ctx context.Context, product *models.Product) error {
	return s.productRepo.Update(ctx, product)
}

// UpdateProductStatus updates the status of a product
func (s *productService) UpdateProductStatus(ctx context.Context, id uint, status string) error {
	return s.productRepo.UpdateStatus(ctx, id, status)
}

func (s *productService) DeleteProduct(ctx context.Context, id uint) error {
	return s.productRepo.Delete(ctx, id)
}

func (s *productService) RestoreProduct(ctx context.Context, id uint) error {
	return s.productRepo.Restore(ctx, id)
}

func (s *productService) ListProducts(ctx context.Context, limit, offset int, sortBy, sortOrder, status string) ([]models.Product, error) {
	return s.productRepo.List(ctx, limit, offset, sortBy, sortOrder, status)
}

func (s *productService) GetProductsBySupplier(ctx context.Context, supplierID uint) ([]models.Product, error) {
	return s.productRepo.GetBySupplier(ctx, supplierID)
}

func (s *productService) SearchProducts(ctx context.Context, query string, sortBy, sortOrder string) ([]models.Product, error) {
	return s.productRepo.Search(ctx, query, sortBy, sortOrder)
}

func (s *productService) SearchProductsWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, status string) ([]models.Product, error) {
	return s.productRepo.SearchWithPagination(ctx, query, limit, offset, sortBy, sortOrder, status)
}

func (s *productService) CountProducts(ctx context.Context, status string) (int64, error) {
	return s.productRepo.Count(ctx, status)
}

func (s *productService) CountSearchProducts(ctx context.Context, query string, status string) (int64, error) {
	return s.productRepo.CountSearch(ctx, query, status)
}

// ImportProductsFromCSV imports products with suppliers from a CSV file
// CSV format: Name;Description;ProductType;Suppliers;ContactEmail;ContactPhone;Address
// Products can have multiple suppliers by repeating rows with empty product fields
func (s *productService) ImportProductsFromCSV(ctx context.Context, csvReader io.Reader) (int, error) {
	reader := csv.NewReader(csvReader)
	reader.Comma = ';' // Use semicolon as delimiter
	reader.TrimLeadingSpace = true

	// Read header
	header, err := reader.Read()
	if err != nil {
		return 0, pkg.ErrValidation("failed to read CSV header", err)
	}

	// Normalize headers (trim spaces and convert to lowercase)
	for i := range header {
		header[i] = strings.ToLower(strings.TrimSpace(header[i]))
	}

	// Find column indices
	nameIdx := -1
	descIdx := -1
	typeIdx := -1
	supplierNameIdx := -1
	contactEmailIdx := -1
	contactPhoneIdx := -1
	addressIdx := -1

	for i, h := range header {
		switch h {
		case "name":
			nameIdx = i
		case "description":
			descIdx = i
		case "producttype", "product_type", "type":
			typeIdx = i
		case "suppliers", "supplier", "suppliername":
			supplierNameIdx = i
		case "contactemail", "contact_email", "email":
			contactEmailIdx = i
		case "contactphone", "contact_phone", "phone":
			contactPhoneIdx = i
		case "address":
			addressIdx = i
		}
	}

	// Validate required columns
	if nameIdx == -1 {
		return 0, pkg.ErrValidation("CSV header missing required column: 'Name'", nil)
	}
	if typeIdx == -1 {
		return 0, pkg.ErrValidation("CSV header missing required column: 'ProductType'", nil)
	}

	// Structure to hold products with their suppliers
	type productWithSuppliers struct {
		product   models.Product
		suppliers []models.Supplier
	}

	var productsData []productWithSuppliers
	var currentProduct *productWithSuppliers
	lineNumber := 1 // Start at 1 since we already read the header

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("failed to read CSV line %d: %w", lineNumber+1, err)
		}
		lineNumber++

		// Skip completely empty rows
		allEmpty := true
		for _, field := range record {
			if strings.TrimSpace(field) != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			continue
		}

		// Check if this is a product row or a supplier row
		hasProductData := false
		if len(record) > nameIdx && strings.TrimSpace(record[nameIdx]) != "" {
			hasProductData = true
		}

		if hasProductData {
			// This is a new product row
			productName := strings.TrimSpace(record[nameIdx])
			productType := ""
			if len(record) > typeIdx {
				productType = strings.TrimSpace(record[typeIdx])
			}
			productDescription := ""
			if len(record) > descIdx {
				productDescription = strings.TrimSpace(record[descIdx])
			}

			if productName == "" {
				return 0, pkg.ErrValidation(fmt.Sprintf("line %d: product 'Name' is required", lineNumber+1), nil)
			}
			if productType == "" {
				return 0, pkg.ErrValidation(fmt.Sprintf("line %d: product 'ProductType' is required", lineNumber+1), nil)
			}

			// Create new product
			product := models.Product{
				Name:        productName,
				Description: productDescription,
				ProductType: productType,
				Status:      "active",
			}

			// Optional description
			if descIdx != -1 && len(record) > descIdx {
				product.Description = strings.TrimSpace(record[descIdx])
			}

			currentProduct = &productWithSuppliers{
				product:   product,
				suppliers: []models.Supplier{},
			}
			productsData = append(productsData, *currentProduct)
		}

		// Parse supplier information if available
		if supplierNameIdx != -1 && len(record) > supplierNameIdx {
			supplierName := strings.TrimSpace(record[supplierNameIdx])
			if supplierName != "" {
				if currentProduct == nil {
					return 0, pkg.ErrValidation(fmt.Sprintf("line %d: supplier data found without a product", lineNumber+1), nil)
				}

				supplier := models.Supplier{
					Name:   supplierName,
					Status: "active",
				}

				// Optional supplier fields
				if contactEmailIdx != -1 && len(record) > contactEmailIdx {
					supplier.ContactEmail = strings.TrimSpace(record[contactEmailIdx])
				}
				if contactPhoneIdx != -1 && len(record) > contactPhoneIdx {
					supplier.ContactPhone = strings.TrimSpace(record[contactPhoneIdx])
				}
				if addressIdx != -1 && len(record) > addressIdx {
					supplier.Address = strings.TrimSpace(record[addressIdx])
				}

				// Add supplier to current product
				productsData[len(productsData)-1].suppliers = append(productsData[len(productsData)-1].suppliers, supplier)
			}
		}
	}

	// Check if we have any products to import
	if len(productsData) == 0 {
		return 0, pkg.ErrValidation("CSV file contains no valid product data", nil)
	}

	// Import products with their suppliers
	for i := range productsData {
		// Create product
		if err := s.productRepo.Create(ctx, &productsData[i].product); err != nil {
			return 0, fmt.Errorf("failed to create product '%s': %w", productsData[i].product.Name, err)
		}

		// Create or find suppliers and associate with product
		if len(productsData[i].suppliers) > 0 {
			var supplierPointers []*models.Supplier
			for j := range productsData[i].suppliers {
				supplier, err := s.supplierRepo.FindOrCreateByName(ctx, &productsData[i].suppliers[j])
				if err != nil {
					return 0, fmt.Errorf("failed to create/find supplier '%s': %w", productsData[i].suppliers[j].Name, err)
				}
				supplierPointers = append(supplierPointers, supplier)
			}

			// Associate suppliers with product
			productsData[i].product.Suppliers = supplierPointers
			if err := s.productRepo.Update(ctx, &productsData[i].product); err != nil {
				return 0, fmt.Errorf("failed to associate suppliers with product '%s': %w", productsData[i].product.Name, err)
			}
		}
	}

	return len(productsData), nil
}

// ImportProductsFromExcel imports products with suppliers from an Excel file
// Excel format: Name;Description;ProductType;Suppliers;ContactEmail;ContactPhone;Address
// Products can have multiple suppliers by repeating rows with empty product fields
func (s *productService) ImportProductsFromExcel(ctx context.Context, excelReader io.Reader) (int, error) {
	// Open Excel file
	f, err := excelize.OpenReader(excelReader)
	if err != nil {
		return 0, pkg.ErrValidation("failed to open Excel file", err)
	}
	defer f.Close()

	// Get the first sheet name
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return 0, pkg.ErrValidation("Excel file has no sheets", nil)
	}
	sheetName := sheets[0]

	// Read all rows from the sheet
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return 0, pkg.ErrValidation("failed to read Excel rows", err)
	}

	if len(rows) == 0 {
		return 0, pkg.ErrValidation("Excel file is empty", nil)
	}

	// Process header
	header := rows[0]

	// Normalize headers (trim spaces and convert to lowercase)
	for i := range header {
		header[i] = strings.ToLower(strings.TrimSpace(header[i]))
	}

	// Find column indices
	nameIdx := -1
	descIdx := -1
	typeIdx := -1
	supplierNameIdx := -1
	contactEmailIdx := -1
	contactPhoneIdx := -1
	addressIdx := -1

	for i, h := range header {
		switch h {
		case "name":
			nameIdx = i
		case "description":
			descIdx = i
		case "producttype", "product_type", "type":
			typeIdx = i
		case "suppliers", "supplier", "suppliername":
			supplierNameIdx = i
		case "contactemail", "contact_email", "email":
			contactEmailIdx = i
		case "contactphone", "contact_phone", "phone":
			contactPhoneIdx = i
		case "address":
			addressIdx = i
		}
	}

	// Validate required columns
	if nameIdx == -1 {
		return 0, pkg.ErrValidation("Excel header missing required column: 'Name'", nil)
	}
	if typeIdx == -1 {
		return 0, pkg.ErrValidation("Excel header missing required column: 'ProductType'", nil)
	}

	// Structure to hold products with their suppliers
	type productWithSuppliers struct {
		product   models.Product
		suppliers []models.Supplier
	}

	var productsData []productWithSuppliers
	var currentProduct *productWithSuppliers

	// Process data rows (skip header)
	for lineNumber, row := range rows[1:] {
		// Skip completely empty rows
		allEmpty := true
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			continue
		}

		// Check if this is a product row or a supplier row
		hasProductData := false
		if len(row) > nameIdx && strings.TrimSpace(row[nameIdx]) != "" {
			hasProductData = true
		}

		if hasProductData {
			// This is a new product row
			productName := strings.TrimSpace(row[nameIdx])
			productType := ""
			if len(row) > typeIdx {
				productType = strings.TrimSpace(row[typeIdx])
			}
			productDescription := ""
			if descIdx != -1 && len(row) > descIdx {
				productDescription = strings.TrimSpace(row[descIdx])
			}

			if productName == "" {
				return 0, pkg.ErrValidation(fmt.Sprintf("line %d: product 'Name' is required", lineNumber+2), nil)
			}
			if productType == "" {
				return 0, pkg.ErrValidation(fmt.Sprintf("line %d: product 'ProductType' is required", lineNumber+2), nil)
			}

			// Create new product
			product := models.Product{
				Name:        productName,
				Description: productDescription,
				ProductType: productType,
				Status:      "active",
			}

			currentProduct = &productWithSuppliers{
				product:   product,
				suppliers: []models.Supplier{},
			}
			productsData = append(productsData, *currentProduct)
		}

		// Parse supplier information if available
		if supplierNameIdx != -1 && len(row) > supplierNameIdx {
			supplierName := strings.TrimSpace(row[supplierNameIdx])
			if supplierName != "" {
				if currentProduct == nil {
					return 0, pkg.ErrValidation(fmt.Sprintf("line %d: supplier data found without a product", lineNumber+2), nil)
				}

				supplier := models.Supplier{
					Name:   supplierName,
					Status: "active",
				}

				// Optional supplier fields
				if contactEmailIdx != -1 && len(row) > contactEmailIdx {
					supplier.ContactEmail = strings.TrimSpace(row[contactEmailIdx])
				}
				if contactPhoneIdx != -1 && len(row) > contactPhoneIdx {
					supplier.ContactPhone = strings.TrimSpace(row[contactPhoneIdx])
				}
				if addressIdx != -1 && len(row) > addressIdx {
					supplier.Address = strings.TrimSpace(row[addressIdx])
				}

				// Add supplier to current product
				productsData[len(productsData)-1].suppliers = append(productsData[len(productsData)-1].suppliers, supplier)
			}
		}
	}

	// Check if we have any products to import
	if len(productsData) == 0 {
		return 0, pkg.ErrValidation("Excel file contains no valid product data", nil)
	}

	// Import products with their suppliers
	for i := range productsData {
		// Create product
		if err := s.productRepo.Create(ctx, &productsData[i].product); err != nil {
			return 0, fmt.Errorf("failed to create product '%s': %w", productsData[i].product.Name, err)
		}

		// Create or find suppliers and associate with product
		if len(productsData[i].suppliers) > 0 {
			var supplierPointers []*models.Supplier
			for j := range productsData[i].suppliers {
				supplier, err := s.supplierRepo.FindOrCreateByName(ctx, &productsData[i].suppliers[j])
				if err != nil {
					return 0, fmt.Errorf("failed to create/find supplier '%s': %w", productsData[i].suppliers[j].Name, err)
				}
				supplierPointers = append(supplierPointers, supplier)
			}

			// Associate suppliers with product
			productsData[i].product.Suppliers = supplierPointers
			if err := s.productRepo.Update(ctx, &productsData[i].product); err != nil {
				return 0, fmt.Errorf("failed to associate suppliers with product '%s': %w", productsData[i].product.Name, err)
			}
		}
	}

	return len(productsData), nil
}
