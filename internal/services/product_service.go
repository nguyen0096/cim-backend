package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/pkg"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
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
	productRepo     repository.ProductRepository
	supplierRepo    repository.SupplierRepository
	unitRepo        repository.UnitRepository
	settingsService SettingsService
}

func NewProductService(productRepo repository.ProductRepository, supplierRepo repository.SupplierRepository, unitRepo repository.UnitRepository, settingsService SettingsService) ProductService {
	return &productService{
		productRepo:     productRepo,
		supplierRepo:    supplierRepo,
		unitRepo:        unitRepo,
		settingsService: settingsService,
	}
}

func (s *productService) CreateProduct(ctx context.Context, product *models.Product) error {
	if product.UnitID == 0 {
		return pkg.ErrValidation("unit_id is required", nil)
	}

	unit, err := s.unitRepo.GetByID(ctx, product.UnitID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return pkg.ErrNotFound(fmt.Sprintf("unit %d not found", product.UnitID), err)
		}
		return fmt.Errorf("failed to validate unit for product: %w", err)
	}

	product.Unit = unit

	return s.productRepo.Create(ctx, product)
}

func (s *productService) GetProductByID(ctx context.Context, id uint) (*models.Product, error) {
	return s.productRepo.GetByID(ctx, id)
}

func (s *productService) UpdateProduct(ctx context.Context, product *models.Product) error {
	if product.UnitID == 0 {
		return pkg.ErrValidation("unit_id is required", nil)
	}

	unit, err := s.unitRepo.GetByID(ctx, product.UnitID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return pkg.ErrNotFound(fmt.Sprintf("unit %d not found", product.UnitID), err)
		}
		return fmt.Errorf("failed to validate unit for product %d: %w", product.ID, err)
	}

	product.Unit = unit

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

func (s *productService) ensureUnit(ctx context.Context, label string) (*models.Unit, error) {
	unitLabel := strings.TrimSpace(label)
	if unitLabel == "" {
		return nil, pkg.ErrValidation("unit label cannot be empty", nil)
	}

	unit, err := s.unitRepo.GetByTypeAndName(ctx, "general", unitLabel)
	if err == nil {
		return unit, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to look up unit '%s': %w", unitLabel, err)
	}

	newUnit := &models.Unit{
		UnitType:         "general",
		Name:             unitLabel,
		Symbol:           unitLabel,
		ConversionFactor: 1,
	}
	if createErr := s.unitRepo.Create(ctx, newUnit); createErr != nil {
		return nil, fmt.Errorf("failed to create unit '%s': %w", unitLabel, createErr)
	}

	return newUnit, nil
}

// updateProductTypesInSettings updates the product_types setting with new product types from import
func (s *productService) updateProductTypesInSettings(ctx context.Context, newProductTypes []string) error {
	if len(newProductTypes) == 0 {
		return nil
	}

	// Get existing product types from settings
	var existingTypes []string
	setting, err := s.settingsService.GetSetting(ctx, "product_types")
	if err == nil && setting != nil && setting.Key == "product_types" && len(setting.Value) > 0 {
		_ = json.Unmarshal(setting.Value, &existingTypes)
	}

	// Create a map for efficient lookup (case-insensitive)
	typeMap := make(map[string]bool)
	for _, t := range existingTypes {
		if strings.TrimSpace(t) != "" {
			typeMap[strings.ToLower(strings.TrimSpace(t))] = true
		}
	}

	// Add new types to the map (case-insensitive)
	for _, t := range newProductTypes {
		trimmed := strings.TrimSpace(t)
		if trimmed != "" {
			typeMap[strings.ToLower(trimmed)] = true
		}
	}

	// Convert map back to slice, preserving original case from existing types or using new types
	// We'll use the first occurrence we encounter
	resultMap := make(map[string]string)
	for _, t := range existingTypes {
		if strings.TrimSpace(t) != "" {
			key := strings.ToLower(strings.TrimSpace(t))
			if _, exists := resultMap[key]; !exists {
				resultMap[key] = strings.TrimSpace(t)
			}
		}
	}
	for _, t := range newProductTypes {
		trimmed := strings.TrimSpace(t)
		if trimmed != "" {
			key := strings.ToLower(trimmed)
			if _, exists := resultMap[key]; !exists {
				resultMap[key] = trimmed
			}
		}
	}

	// Convert to slice
	allTypes := make([]string, 0, len(resultMap))
	for _, v := range resultMap {
		allTypes = append(allTypes, v)
	}
	sort.Strings(allTypes)

	// Save to settings
	if err := s.settingsService.SetSetting(ctx, "product_types", allTypes); err != nil {
		return fmt.Errorf("failed to update product types in settings: %w", err)
	}

	return nil
}

// ImportProductsFromCSV imports products with suppliers from a CSV file
// CSV format: Name;Description;ProductType;Unit;Suppliers;ContactEmail;ContactPhone;Address
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
	unitIdx := -1
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
		case "unit":
			unitIdx = i
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
	if unitIdx == -1 {
		return 0, pkg.ErrValidation("CSV header missing required column: 'Unit'", nil)
	}
	if supplierNameIdx == -1 {
		return 0, pkg.ErrValidation("CSV header missing required column: 'Suppliers'", nil)
	}

	// Maps to store unique products and suppliers by name (case-insensitive key)
	productsMap := make(map[string]*models.Product)        // product name (lowercase) -> product
	productUnitMap := make(map[string]string)              // product name (lowercase) -> unit label
	suppliersMap := make(map[string]*models.Supplier)      // supplier name (lowercase) -> supplier
	productSupplierMap := make(map[string]map[string]bool) // product name (lowercase) -> set of supplier names (lowercase)
	productTypesMap := make(map[string]string)             // product type (lowercase) -> original case

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

		// Parse supplier data
		supplierName := ""
		if len(record) > supplierNameIdx {
			supplierName = strings.TrimSpace(record[supplierNameIdx])
		}

		// Skip rows where both product name and supplier name are missing
		if productName == "" && supplierName == "" {
			continue
		}

		// Process product if product name is provided
		if productName != "" {
			productType := ""
			if len(record) > typeIdx {
				productType = strings.TrimSpace(record[typeIdx])
			}
			// Only process product if ProductType is provided
			productDescription := ""
			if descIdx != -1 && len(record) > descIdx {
				productDescription = strings.TrimSpace(record[descIdx])
			}

			// Collect product type (case-insensitive, preserve original case)
			if productType != "" {
				productTypeKey := strings.ToLower(productType)
				if _, exists := productTypesMap[productTypeKey]; !exists {
					productTypesMap[productTypeKey] = productType
				}
			}

			// Check for duplicate product names (case-insensitive)
			productKey := strings.ToLower(productName)
			if _, exists := productsMap[productKey]; !exists {
				// Create new product entry
				productsMap[productKey] = &models.Product{
					Name:        productName,
					Description: productDescription,
					ProductType: productType,
					Status:      "active",
				}
				productSupplierMap[productKey] = make(map[string]bool)
			}

			if unitIdx != -1 && len(record) > unitIdx {
				productUnitMap[productKey] = strings.ToLower(strings.TrimSpace(record[unitIdx]))
			}

			// If supplier name is also provided, associate supplier with product
			if supplierName != "" {
				supplierKey := strings.ToLower(supplierName)
				productSupplierMap[productKey][supplierKey] = true
			}
		}

		// Process supplier if supplier name is provided
		if supplierName != "" {
			supplierKey := strings.ToLower(supplierName)
			if _, exists := suppliersMap[supplierKey]; !exists {
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
		}
	}

	// Check if we have any products or suppliers to import
	if len(productsMap) == 0 && len(suppliersMap) == 0 {
		return 0, pkg.ErrValidation("CSV file contains no valid product or supplier data", nil)
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

	// Batch check for existing products
	productNames := make([]string, 0, len(productsMap))
	for _, product := range productsMap {
		productNames = append(productNames, product.Name)
	}
	existingProducts, err := s.productRepo.GetByNames(ctx, productNames)
	if err != nil {
		return 0, fmt.Errorf("failed to check for existing products: %w", err)
	}

	// Third pass: create all unique products and associate with suppliers
	createdCount := 0
	for productKey, product := range productsMap {
		// Check if product already exists in database (case-insensitive lookup)
		productKeyLower := strings.ToLower(product.Name)
		if _, exists := existingProducts[productKeyLower]; exists {
			// Product already exists, skip it
			continue
		}

		unitLabel, ok := productUnitMap[productKey]
		if !ok || strings.TrimSpace(unitLabel) == "" {
			return 0, pkg.ErrValidation(fmt.Sprintf("unit is required for product '%s'", product.Name), nil)
		}

		unit, err := s.ensureUnit(ctx, unitLabel)
		if err != nil {
			return 0, fmt.Errorf("failed to resolve unit for product '%s': %w", product.Name, err)
		}
		product.UnitID = unit.ID
		product.Unit = unit

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

	// Update product types in settings
	productTypes := make([]string, 0, len(productTypesMap))
	for _, productType := range productTypesMap {
		productTypes = append(productTypes, productType)
	}
	if err := s.updateProductTypesInSettings(ctx, productTypes); err != nil {
		// Log error but don't fail the import
		// The import was successful, just the settings update failed
		return createdCount, fmt.Errorf("import completed but failed to update product types in settings: %w", err)
	}

	return createdCount, nil
}

// ImportProductsFromExcel imports products with suppliers from an Excel file
// Excel format: Name;Description;ProductType;Unit;Suppliers;ContactEmail;ContactPhone;Address
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
	unitIdx := -1
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
		case "unit":
			unitIdx = i
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
	if unitIdx == -1 {
		return 0, pkg.ErrValidation("Excel header missing required column: 'Unit'", nil)
	}
	if supplierNameIdx == -1 {
		return 0, pkg.ErrValidation("Excel header missing required column: 'Suppliers'", nil)
	}

	// Maps to store unique products and suppliers by name (case-insensitive key)
	productsMap := make(map[string]*models.Product)        // product name (lowercase) -> product
	productUnitMap := make(map[string]string)              // product name (lowercase) -> unit label
	suppliersMap := make(map[string]*models.Supplier)      // supplier name (lowercase) -> supplier
	productSupplierMap := make(map[string]map[string]bool) // product name (lowercase) -> set of supplier names (lowercase)
	productTypesMap := make(map[string]string)             // product type (lowercase) -> original case

	// Process data rows (skip header)
	for _, row := range rows[headerInx+1:] {
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

		// Parse supplier data
		supplierName := ""
		if len(row) > supplierNameIdx {
			supplierName = strings.TrimSpace(row[supplierNameIdx])
		}

		// Skip rows where both product name and supplier name are missing
		if productName == "" && supplierName == "" {
			continue
		}

		// Process product if product name is provided
		if productName != "" {
			productType := ""
			if len(row) > typeIdx {
				productType = strings.TrimSpace(row[typeIdx])
			}
			// Only process product if ProductType is provided
			productDescription := ""
			if descIdx != -1 && len(row) > descIdx {
				productDescription = strings.TrimSpace(row[descIdx])
			}

			productUnit := ""
			if unitIdx != -1 && len(row) > unitIdx {
				productUnit = strings.ToLower(strings.TrimSpace(row[unitIdx]))
			}

			// Collect product type (case-insensitive, preserve original case)
			if productType != "" {
				productTypeKey := strings.ToLower(productType)
				if _, exists := productTypesMap[productTypeKey]; !exists {
					productTypesMap[productTypeKey] = productType
				}
			}

			// Check for duplicate product names (case-insensitive)
			productKey := strings.ToLower(productName)
			if _, exists := productsMap[productKey]; !exists {
				// Create new product entry
				productsMap[productKey] = &models.Product{
					Name:        productName,
					Description: productDescription,
					ProductType: productType,
					Status:      "active",
				}
				productSupplierMap[productKey] = make(map[string]bool)
			}

			if unitIdx != -1 && len(row) > unitIdx {
				productUnitMap[productKey] = productUnit
			}

			// If supplier name is also provided, associate supplier with product
			if supplierName != "" {
				supplierKey := strings.ToLower(supplierName)
				productSupplierMap[productKey][supplierKey] = true
			}
		}

		// Process supplier if supplier name is provided
		if supplierName != "" {
			supplierKey := strings.ToLower(supplierName)
			if _, exists := suppliersMap[supplierKey]; !exists {
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
		}
	}

	// Check if we have any products or suppliers to import
	if len(productsMap) == 0 && len(suppliersMap) == 0 {
		return 0, pkg.ErrValidation("Excel file contains no valid product or supplier data", nil)
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

	// Batch check for existing products
	productNames := make([]string, 0, len(productsMap))
	for _, product := range productsMap {
		productNames = append(productNames, product.Name)
	}
	existingProducts, err := s.productRepo.GetByNames(ctx, productNames)
	if err != nil {
		return 0, fmt.Errorf("failed to check for existing products: %w", err)
	}

	// Third pass: create all unique products and associate with suppliers
	createdCount := 0
	for productKey, product := range productsMap {
		// Check if product already exists in database (case-insensitive lookup)
		productKeyLower := strings.ToLower(product.Name)
		if _, exists := existingProducts[productKeyLower]; exists {
			// Product already exists, skip it
			continue
		}

		unitLabel, ok := productUnitMap[productKey]
		if !ok || strings.TrimSpace(unitLabel) == "" {
			return 0, pkg.ErrValidation(fmt.Sprintf("unit is required for product '%s'", product.Name), nil)
		}

		unit, err := s.ensureUnit(ctx, unitLabel)
		if err != nil {
			return 0, fmt.Errorf("failed to resolve unit for product '%s': %w", product.Name, err)
		}
		product.UnitID = unit.ID
		product.Unit = unit

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

	// Update product types in settings
	productTypes := make([]string, 0, len(productTypesMap))
	for _, productType := range productTypesMap {
		productTypes = append(productTypes, productType)
	}
	if err := s.updateProductTypesInSettings(ctx, productTypes); err != nil {
		// Log error but don't fail the import
		// The import was successful, just the settings update failed
		return createdCount, fmt.Errorf("import completed but failed to update product types in settings: %w", err)
	}

	return createdCount, nil
}
