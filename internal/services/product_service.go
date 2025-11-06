package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/pkg"
	"context"
	"encoding/csv"
	"fmt"
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
	SearchProductsWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, status, productType string, supplierID uint) ([]models.Product, error)
	CountProducts(ctx context.Context, status, productType string, supplierID uint) (int64, error)
	CountSearchProducts(ctx context.Context, query string, status, productType string, supplierID uint) (int64, error)

	// v1
	ListProducts(ctx context.Context, limit, offset int, sortBy, sortOrder, status, productType string, supplierID uint) ([]models.Product, error)
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

func (s *productService) ListProducts(ctx context.Context, limit, offset int, sortBy, sortOrder, status, productType string, supplierID uint) ([]models.Product, error) {
	return s.productRepo.List(ctx, limit, offset, sortBy, sortOrder, status, productType, supplierID)
}

func (s *productService) GetProductsBySupplier(ctx context.Context, supplierID uint) ([]models.Product, error) {
	return s.productRepo.GetBySupplier(ctx, supplierID)
}

func (s *productService) SearchProducts(ctx context.Context, query string, sortBy, sortOrder string) ([]models.Product, error) {
	return s.productRepo.Search(ctx, query, sortBy, sortOrder)
}

func (s *productService) SearchProductsWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, status, productType string, supplierID uint) ([]models.Product, error) {
	return s.productRepo.SearchWithPagination(ctx, query, limit, offset, sortBy, sortOrder, status, productType, supplierID)
}

func (s *productService) CountProducts(ctx context.Context, status, productType string, supplierID uint) (int64, error) {
	return s.productRepo.Count(ctx, status, productType, supplierID)
}

func (s *productService) CountSearchProducts(ctx context.Context, query string, status, productType string, supplierID uint) (int64, error) {
	return s.productRepo.CountSearch(ctx, query, status, productType, supplierID)
}

// ImportProductsFromCSV imports products with suppliers from a CSV file
// CSV format: Name;Description;ProductType;Suppliers;ContactEmail;ContactPhone;Address
// Product names and supplier names must be unique within the file (n-n relationship)
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
	if supplierNameIdx == -1 {
		return 0, pkg.ErrValidation("CSV header missing required column: 'Suppliers'", nil)
	}

	// Maps to store unique products and suppliers by name (case-insensitive key)
	productsMap := make(map[string]*models.Product)        // product name (lowercase) -> product
	suppliersMap := make(map[string]*models.Supplier)      // supplier name (lowercase) -> supplier
	productSupplierMap := make(map[string]map[string]bool) // product name (lowercase) -> set of supplier names (lowercase)

	lineNumber := 1 // Start at 1 since we already read the header

	// First pass: collect all products and suppliers, validate uniqueness
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

		// Parse product data
		productName := ""
		if len(record) > nameIdx {
			productName = strings.TrimSpace(record[nameIdx])
		}
		// Skip rows with missing product name
		if productName == "" {
			continue
		}

		productType := ""
		if len(record) > typeIdx {
			productType = strings.TrimSpace(record[typeIdx])
		}
		if productType == "" {
			return 0, pkg.ErrValidation(fmt.Sprintf("line %d: product 'ProductType' is required", lineNumber+1), nil)
		}

		productDescription := ""
		if descIdx != -1 && len(record) > descIdx {
			productDescription = strings.TrimSpace(record[descIdx])
		}

		// Parse supplier data
		supplierName := ""
		if len(record) > supplierNameIdx {
			supplierName = strings.TrimSpace(record[supplierNameIdx])
		}
		// Skip rows with missing supplier name
		if supplierName == "" {
			continue
		}

		// Check for duplicate product names (case-insensitive)
		productKey := strings.ToLower(productName)
		if existingProduct, exists := productsMap[productKey]; exists {
			// Validate that product data is consistent
			if existingProduct.ProductType != productType {
				return 0, pkg.ErrValidation(fmt.Sprintf("line %d: duplicate product name '%s' with different ProductType", lineNumber+1, productName), nil)
			}
			// Use the first description if current is empty, otherwise keep existing
			if productDescription != "" && existingProduct.Description == "" {
				existingProduct.Description = productDescription
			}
		} else {
			// Create new product entry
			productsMap[productKey] = &models.Product{
				Name:        productName,
				Description: productDescription,
				ProductType: productType,
				Status:      "active",
			}
			productSupplierMap[productKey] = make(map[string]bool)
		}

		// Check for duplicate supplier names (case-insensitive)
		supplierKey := strings.ToLower(supplierName)
		if existingSupplier, exists := suppliersMap[supplierKey]; exists {
			// Validate that supplier data is consistent
			contactEmail := ""
			if contactEmailIdx != -1 && len(record) > contactEmailIdx {
				contactEmail = strings.TrimSpace(record[contactEmailIdx])
			}
			contactPhone := ""
			if contactPhoneIdx != -1 && len(record) > contactPhoneIdx {
				contactPhone = strings.TrimSpace(record[contactPhoneIdx])
			}
			address := ""
			if addressIdx != -1 && len(record) > addressIdx {
				address = strings.TrimSpace(record[addressIdx])
			}

			// Update supplier fields if provided and existing is empty
			if contactEmail != "" && existingSupplier.ContactEmail == "" {
				existingSupplier.ContactEmail = contactEmail
			}
			if contactPhone != "" && existingSupplier.ContactPhone == "" {
				existingSupplier.ContactPhone = contactPhone
			}
			if address != "" && existingSupplier.Address == "" {
				existingSupplier.Address = address
			}
		} else {
			// Create new supplier entry
			supplier := &models.Supplier{
				Name:   supplierName,
				Status: "active",
			}

			if contactEmailIdx != -1 && len(record) > contactEmailIdx {
				supplier.ContactEmail = strings.TrimSpace(record[contactEmailIdx])
			}
			if contactPhoneIdx != -1 && len(record) > contactPhoneIdx {
				supplier.ContactPhone = strings.TrimSpace(record[contactPhoneIdx])
			}
			if addressIdx != -1 && len(record) > addressIdx {
				supplier.Address = strings.TrimSpace(record[addressIdx])
			}

			suppliersMap[supplierKey] = supplier
		}

		// Add supplier to product's supplier set (n-n relationship)
		productSupplierMap[productKey][supplierKey] = true
	}

	// Check if we have any products to import
	if len(productsMap) == 0 {
		return 0, pkg.ErrValidation("CSV file contains no valid product data", nil)
	}

	// Second pass: create all unique suppliers first
	supplierIDMap := make(map[string]*models.Supplier) // supplier key -> created supplier
	for supplierKey, supplier := range suppliersMap {
		createdSupplier, err := s.supplierRepo.FindOrCreateByName(ctx, supplier)
		if err != nil {
			return 0, fmt.Errorf("failed to create/find supplier '%s': %w", supplier.Name, err)
		}
		supplierIDMap[supplierKey] = createdSupplier
	}

	// Third pass: create all unique products and associate with suppliers
	createdCount := 0
	for productKey, product := range productsMap {
		// Create product
		if err := s.productRepo.Create(ctx, product); err != nil {
			return 0, fmt.Errorf("failed to create product '%s': %w", product.Name, err)
		}

		// Associate suppliers with product
		supplierSet := productSupplierMap[productKey]
		if len(supplierSet) > 0 {
			var supplierPointers []*models.Supplier
			for supplierKey := range supplierSet {
				supplierPointers = append(supplierPointers, supplierIDMap[supplierKey])
			}

			product.Suppliers = supplierPointers
			if err := s.productRepo.Update(ctx, product); err != nil {
				return 0, fmt.Errorf("failed to associate suppliers with product '%s': %w", product.Name, err)
			}
		}

		createdCount++
	}

	return createdCount, nil
}

// ImportProductsFromExcel imports products with suppliers from an Excel file
// Excel format: Name;Description;ProductType;Suppliers;ContactEmail;ContactPhone;Address
// Product names and supplier names must be unique within the file (n-n relationship)
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
	headerInx := 0
	for headerInx < len(rows) && len(rows[headerInx]) < 6 {
		headerInx++
	}

	// Process header
	header := rows[headerInx]
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
	if supplierNameIdx == -1 {
		return 0, pkg.ErrValidation("Excel header missing required column: 'Suppliers'", nil)
	}

	// Maps to store unique products and suppliers by name (case-insensitive key)
	productsMap := make(map[string]*models.Product)        // product name (lowercase) -> product
	suppliersMap := make(map[string]*models.Supplier)      // supplier name (lowercase) -> supplier
	productSupplierMap := make(map[string]map[string]bool) // product name (lowercase) -> set of supplier names (lowercase)

	// Process data rows (skip header)
	for lineNumber, row := range rows[headerInx+1:] {
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

		// Parse product data
		productName := ""
		if len(row) > nameIdx {
			productName = strings.TrimSpace(row[nameIdx])
		}
		// Skip rows with missing product name
		if productName == "" {
			continue
		}

		productType := ""
		if len(row) > typeIdx {
			productType = strings.TrimSpace(row[typeIdx])
		}
		if productType == "" {
			return 0, pkg.ErrValidation(fmt.Sprintf("line %d: product 'ProductType' is required", lineNumber+2), nil)
		}

		productDescription := ""
		if descIdx != -1 && len(row) > descIdx {
			productDescription = strings.TrimSpace(row[descIdx])
		}

		// Parse supplier data
		supplierName := ""
		if len(row) > supplierNameIdx {
			supplierName = strings.TrimSpace(row[supplierNameIdx])
		}
		// Skip rows with missing supplier name
		if supplierName == "" {
			continue
		}

		// Check for duplicate product names (case-insensitive)
		productKey := strings.ToLower(productName)
		if existingProduct, exists := productsMap[productKey]; exists {
			// Validate that product data is consistent
			if existingProduct.ProductType != productType {
				return 0, pkg.ErrValidation(fmt.Sprintf("line %d: duplicate product name '%s' with different ProductType", lineNumber+2, productName), nil)
			}
			// Use the first description if current is empty, otherwise keep existing
			if productDescription != "" && existingProduct.Description == "" {
				existingProduct.Description = productDescription
			}
		} else {
			// Create new product entry
			productsMap[productKey] = &models.Product{
				Name:        productName,
				Description: productDescription,
				ProductType: productType,
				Status:      "active",
			}
			productSupplierMap[productKey] = make(map[string]bool)
		}

		// Check for duplicate supplier names (case-insensitive)
		supplierKey := strings.ToLower(supplierName)
		if existingSupplier, exists := suppliersMap[supplierKey]; exists {
			// Validate that supplier data is consistent
			contactEmail := ""
			if contactEmailIdx != -1 && len(row) > contactEmailIdx {
				contactEmail = strings.TrimSpace(row[contactEmailIdx])
			}
			contactPhone := ""
			if contactPhoneIdx != -1 && len(row) > contactPhoneIdx {
				contactPhone = strings.TrimSpace(row[contactPhoneIdx])
			}
			address := ""
			if addressIdx != -1 && len(row) > addressIdx {
				address = strings.TrimSpace(row[addressIdx])
			}

			// Update supplier fields if provided and existing is empty
			if contactEmail != "" && existingSupplier.ContactEmail == "" {
				existingSupplier.ContactEmail = contactEmail
			}
			if contactPhone != "" && existingSupplier.ContactPhone == "" {
				existingSupplier.ContactPhone = contactPhone
			}
			if address != "" && existingSupplier.Address == "" {
				existingSupplier.Address = address
			}
		} else {
			// Create new supplier entry
			supplier := &models.Supplier{
				Name:   supplierName,
				Status: "active",
			}

			if contactEmailIdx != -1 && len(row) > contactEmailIdx {
				supplier.ContactEmail = strings.TrimSpace(row[contactEmailIdx])
			}
			if contactPhoneIdx != -1 && len(row) > contactPhoneIdx {
				supplier.ContactPhone = strings.TrimSpace(row[contactPhoneIdx])
			}
			if addressIdx != -1 && len(row) > addressIdx {
				supplier.Address = strings.TrimSpace(row[addressIdx])
			}

			suppliersMap[supplierKey] = supplier
		}

		// Add supplier to product's supplier set (n-n relationship)
		productSupplierMap[productKey][supplierKey] = true
	}

	// Check if we have any products to import
	if len(productsMap) == 0 {
		return 0, pkg.ErrValidation("Excel file contains no valid product data", nil)
	}

	// Second pass: create all unique suppliers first
	supplierIDMap := make(map[string]*models.Supplier) // supplier key -> created supplier
	for supplierKey, supplier := range suppliersMap {
		createdSupplier, err := s.supplierRepo.FindOrCreateByName(ctx, supplier)
		if err != nil {
			return 0, fmt.Errorf("failed to create/find supplier '%s': %w", supplier.Name, err)
		}
		supplierIDMap[supplierKey] = createdSupplier
	}

	// Third pass: create all unique products and associate with suppliers
	createdCount := 0
	for productKey, product := range productsMap {
		// Create product
		if err := s.productRepo.Create(ctx, product); err != nil {
			return 0, fmt.Errorf("failed to create product '%s': %w", product.Name, err)
		}

		// Associate suppliers with product
		supplierSet := productSupplierMap[productKey]
		if len(supplierSet) > 0 {
			var supplierPointers []*models.Supplier
			for supplierKey := range supplierSet {
				supplierPointers = append(supplierPointers, supplierIDMap[supplierKey])
			}

			product.Suppliers = supplierPointers
			if err := s.productRepo.Update(ctx, product); err != nil {
				return 0, fmt.Errorf("failed to associate suppliers with product '%s': %w", product.Name, err)
			}
		}

		createdCount++
	}

	return createdCount, nil
}
