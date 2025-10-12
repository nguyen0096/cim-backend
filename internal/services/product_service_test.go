package services

import (
	"context"
	"errors"
	repositorymocks "import-export-backend/internal/mocks/repositories"
	"import-export-backend/internal/models"
	"import-export-backend/pkg"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestImportProductsFromCSV(t *testing.T) {
	ctx := context.Background()

	t.Run("should successfully import a single product without suppliers", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Suppliers;ContactEmail;ContactPhone;Address
Laptop Dell XPS 13;High-performance laptop;Electronics;;;;`

		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop Dell XPS 13" &&
				p.Description == "High-performance laptop" &&
				p.ProductType == "Electronics" &&
				p.Status == "active"
		})).Return(nil).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		mockProductRepo.AssertExpectations(t)
		mockSupplierRepo.AssertExpectations(t)
	})

	t.Run("should successfully import a product with a single supplier", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Suppliers;ContactEmail;ContactPhone;Address
Laptop;High-performance laptop;Electronics;Tech Inc;tech@email.com;+1-555-0123;123 Tech St`

		createdSupplier := &models.Supplier{
			Base:         models.Base{ID: 1},
			Name:         "Tech Inc",
			ContactEmail: "tech@email.com",
			ContactPhone: "+1-555-0123",
			Address:      "123 Tech St",
			Status:       "active",
		}

		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop" &&
				p.ProductType == "Electronics"
		})).Return(nil).Once()

		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.MatchedBy(func(s *models.Supplier) bool {
			return s.Name == "Tech Inc" &&
				s.ContactEmail == "tech@email.com" &&
				s.ContactPhone == "+1-555-0123" &&
				s.Address == "123 Tech St"
		})).Return(createdSupplier, nil).Once()

		mockProductRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop" &&
				len(p.Suppliers) == 1 &&
				p.Suppliers[0].ID == 1
		})).Return(nil).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		mockProductRepo.AssertExpectations(t)
		mockSupplierRepo.AssertExpectations(t)
	})

	t.Run("should successfully import a product with multiple suppliers", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Suppliers;ContactEmail;ContactPhone;Address
Laptop;High-performance laptop;Electronics;Tech Inc;tech@email.com;+1-555-0123;123 Tech St
;;;Dell;dell@email.com;+1-555-0456;456 Dell Way
;;;HP;hp@email.com;+1-555-0789;789 HP Blvd`

		supplier1 := &models.Supplier{Base: models.Base{ID: 1}, Name: "Tech Inc"}
		supplier2 := &models.Supplier{Base: models.Base{ID: 2}, Name: "Dell"}
		supplier3 := &models.Supplier{Base: models.Base{ID: 3}, Name: "HP"}

		mockProductRepo.On("Create", ctx, mock.AnythingOfType("*models.Product")).Return(nil).Once()

		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.MatchedBy(func(s *models.Supplier) bool {
			return s.Name == "Tech Inc"
		})).Return(supplier1, nil).Once()

		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.MatchedBy(func(s *models.Supplier) bool {
			return s.Name == "Dell"
		})).Return(supplier2, nil).Once()

		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.MatchedBy(func(s *models.Supplier) bool {
			return s.Name == "HP"
		})).Return(supplier3, nil).Once()

		mockProductRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop" && len(p.Suppliers) == 3
		})).Return(nil).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		mockProductRepo.AssertExpectations(t)
		mockSupplierRepo.AssertExpectations(t)
	})

	t.Run("should successfully import multiple products with suppliers", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Suppliers;ContactEmail;ContactPhone;Address
Laptop;High-performance laptop;Electronics;Tech Inc;tech@email.com;;
Tablet;High-performance tablet;Electronics;Tech Inc;tech@email.com;;
Mouse;Wireless mouse;Electronics;;;;`

		supplier1 := &models.Supplier{Base: models.Base{ID: 1}, Name: "Tech Inc"}

		// First product
		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop"
		})).Return(nil).Once()

		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.MatchedBy(func(s *models.Supplier) bool {
			return s.Name == "Tech Inc"
		})).Return(supplier1, nil).Once()

		mockProductRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop"
		})).Return(nil).Once()

		// Second product
		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Tablet"
		})).Return(nil).Once()

		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.MatchedBy(func(s *models.Supplier) bool {
			return s.Name == "Tech Inc"
		})).Return(supplier1, nil).Once()

		mockProductRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Tablet"
		})).Return(nil).Once()

		// Third product (no suppliers)
		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Mouse"
		})).Return(nil).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 3, count)
		mockProductRepo.AssertExpectations(t)
		mockSupplierRepo.AssertExpectations(t)
	})

	t.Run("should return error when CSV header is missing", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := ``

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.Error(t, err)
		assert.Equal(t, 0, count)
		var appErr *pkg.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	})

	t.Run("should return error when required column Name is missing", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Description;ProductType;Suppliers
High-performance laptop;Electronics;Tech Inc`

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.Error(t, err)
		assert.Equal(t, 0, count)
		var appErr *pkg.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	})

	t.Run("should return error when required column ProductType is missing", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;Suppliers
Laptop;High-performance laptop;Tech Inc`

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.Error(t, err)
		assert.Equal(t, 0, count)
		var appErr *pkg.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	})

	t.Run("should return error when product Name is empty", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Suppliers
;High-performance laptop;Electronics;Tech Inc`

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.Error(t, err)
		assert.Equal(t, 0, count)
		var appErr *pkg.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	})

	t.Run("should return error when product ProductType is empty", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Suppliers
Laptop;High-performance laptop;;Tech Inc`

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.Error(t, err)
		assert.Equal(t, 0, count)
		var appErr *pkg.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	})

	t.Run("should return error when supplier row has no preceding product", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Suppliers
;;;Tech Inc`

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.Error(t, err)
		assert.Equal(t, 0, count)
		var appErr *pkg.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	})

	t.Run("should return error when CSV has no valid product data", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Suppliers

`

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.Error(t, err)
		assert.Equal(t, 0, count)
		var appErr *pkg.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	})

	t.Run("should skip empty rows", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Suppliers

Laptop;High-performance laptop;Electronics;

`

		mockProductRepo.On("Create", ctx, mock.AnythingOfType("*models.Product")).Return(nil).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		mockProductRepo.AssertExpectations(t)
	})

	t.Run("should return error when product creation fails", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Suppliers
Laptop;High-performance laptop;Electronics;`

		dbError := errors.New("database connection failed")
		mockProductRepo.On("Create", ctx, mock.AnythingOfType("*models.Product")).Return(dbError).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.Error(t, err)
		assert.Equal(t, 0, count)
		assert.Contains(t, err.Error(), "failed to create product")
		mockProductRepo.AssertExpectations(t)
	})

	t.Run("should return error when supplier creation fails", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Suppliers
Laptop;High-performance laptop;Electronics;Tech Inc`

		mockProductRepo.On("Create", ctx, mock.AnythingOfType("*models.Product")).Return(nil).Once()

		dbError := errors.New("database connection failed")
		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.AnythingOfType("*models.Supplier")).Return(nil, dbError).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.Error(t, err)
		assert.Equal(t, 0, count)
		assert.Contains(t, err.Error(), "failed to create/find supplier")
		mockProductRepo.AssertExpectations(t)
		mockSupplierRepo.AssertExpectations(t)
	})

	t.Run("should return error when supplier association fails", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Suppliers
Laptop;High-performance laptop;Electronics;Tech Inc`

		supplier := &models.Supplier{Base: models.Base{ID: 1}, Name: "Tech Inc"}

		mockProductRepo.On("Create", ctx, mock.AnythingOfType("*models.Product")).Return(nil).Once()
		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.AnythingOfType("*models.Supplier")).Return(supplier, nil).Once()

		dbError := errors.New("database connection failed")
		mockProductRepo.On("Update", ctx, mock.AnythingOfType("*models.Product")).Return(dbError).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.Error(t, err)
		assert.Equal(t, 0, count)
		assert.Contains(t, err.Error(), "failed to associate suppliers")
		mockProductRepo.AssertExpectations(t)
		mockSupplierRepo.AssertExpectations(t)
	})

	t.Run("should handle header case variations", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `name;DESCRIPTION;producttype;suppliers
Laptop;High-performance laptop;Electronics;`

		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop" &&
				p.Description == "High-performance laptop" &&
				p.ProductType == "Electronics"
		})).Return(nil).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		mockProductRepo.AssertExpectations(t)
	})

	t.Run("should trim whitespace from all fields", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Suppliers;ContactEmail;ContactPhone;Address
  Laptop  ;  High-performance laptop  ;  Electronics  ;  Tech Inc  ;  tech@email.com  ;  +1-555-0123  ;  123 Tech St  `

		supplier := &models.Supplier{Base: models.Base{ID: 1}, Name: "Tech Inc"}

		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop" &&
				p.Description == "High-performance laptop" &&
				p.ProductType == "Electronics"
		})).Return(nil).Once()

		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.MatchedBy(func(s *models.Supplier) bool {
			return s.Name == "Tech Inc" &&
				s.ContactEmail == "tech@email.com" &&
				s.ContactPhone == "+1-555-0123" &&
				s.Address == "123 Tech St"
		})).Return(supplier, nil).Once()

		mockProductRepo.On("Update", ctx, mock.AnythingOfType("*models.Product")).Return(nil).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		mockProductRepo.AssertExpectations(t)
		mockSupplierRepo.AssertExpectations(t)
	})

	t.Run("should handle Unicode characters in product and supplier names", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service := NewProductService(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Suppliers;ContactEmail;ContactPhone;Address
Pepsi;;Nước;Công ty TNHH Giải khát Sài Gòn;contact@email.com;0123456789;D5 Khu dân cư Thảo Nguyên`

		supplier := &models.Supplier{
			Base:         models.Base{ID: 1},
			Name:         "Công ty TNHH Giải khát Sài Gòn",
			ContactEmail: "contact@email.com",
			ContactPhone: "0123456789",
			Address:      "D5 Khu dân cư Thảo Nguyên",
		}

		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Pepsi" &&
				p.ProductType == "Nước"
		})).Return(nil).Once()

		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.MatchedBy(func(s *models.Supplier) bool {
			return s.Name == "Công ty TNHH Giải khát Sài Gòn"
		})).Return(supplier, nil).Once()

		mockProductRepo.On("Update", ctx, mock.AnythingOfType("*models.Product")).Return(nil).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		mockProductRepo.AssertExpectations(t)
		mockSupplierRepo.AssertExpectations(t)
	})
}
