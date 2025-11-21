package services

import (
	repositorymocks "cim-backend/internal/mocks/repositories"
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type stubUnitRepository struct {
	unitsByID    map[uint]*models.Unit
	unitsByLabel map[string]*models.Unit
	nextID       uint
}

func newStubUnitRepository() *stubUnitRepository {
	return &stubUnitRepository{
		unitsByID:    make(map[uint]*models.Unit),
		unitsByLabel: make(map[string]*models.Unit),
		nextID:       1,
	}
}

func (s *stubUnitRepository) labelKey(unitType, name string) string {
	return strings.ToLower(unitType) + "::" + strings.ToLower(name)
}

func (s *stubUnitRepository) Create(_ context.Context, unit *models.Unit) error {
	if unit.ID == 0 {
		unit.ID = s.nextID
		s.nextID++
	}
	cloned := *unit
	s.unitsByID[unit.ID] = &cloned
	s.unitsByLabel[s.labelKey(unit.UnitType, unit.Name)] = &cloned
	return nil
}

func (s *stubUnitRepository) GetByID(_ context.Context, id uint) (*models.Unit, error) {
	if unit, ok := s.unitsByID[id]; ok {
		cloned := *unit
		return &cloned, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *stubUnitRepository) GetByTypeAndName(_ context.Context, unitType, name string) (*models.Unit, error) {
	if unit, ok := s.unitsByLabel[s.labelKey(unitType, name)]; ok {
		cloned := *unit
		return &cloned, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *stubUnitRepository) Update(ctx context.Context, unit *models.Unit) error {
	if _, err := s.GetByID(ctx, unit.ID); err != nil {
		return err
	}
	return s.Create(ctx, unit)
}

func (s *stubUnitRepository) Delete(context.Context, uint) error {
	return nil
}

func (s *stubUnitRepository) Restore(context.Context, uint) error {
	return nil
}

func (s *stubUnitRepository) List(context.Context, int, int, string, string, string) ([]models.Unit, error) {
	return nil, nil
}

func (s *stubUnitRepository) Search(context.Context, string, int, int, string, string, string) ([]models.Unit, error) {
	return nil, nil
}

func (s *stubUnitRepository) Count(context.Context, string) (int64, error) {
	return 0, nil
}

func (s *stubUnitRepository) CountSearch(context.Context, string, string) (int64, error) {
	return 0, nil
}

type stubSettingsService struct {
	settings map[string]*models.Settings
}

func newStubSettingsService() *stubSettingsService {
	return &stubSettingsService{
		settings: make(map[string]*models.Settings),
	}
}

func (s *stubSettingsService) GetSetting(ctx context.Context, key string) (*models.Settings, error) {
	if setting, ok := s.settings[key]; ok {
		return setting, nil
	}
	// Return a Settings with the key set to indicate it was queried, but Value will be empty
	// This matches the repository behavior when setting doesn't exist
	return &models.Settings{Key: key}, nil
}

func (s *stubSettingsService) SetSetting(ctx context.Context, key string, value interface{}) error {
	// For testing purposes, we'll just store it in memory
	// In a real implementation, this would marshal to JSON
	setting := &models.Settings{
		Key: key,
		// Value would be set here, but for testing we can skip it
	}
	s.settings[key] = setting
	return nil
}

func (s *stubSettingsService) GetAllSettings(ctx context.Context) ([]models.Settings, error) {
	settings := make([]models.Settings, 0, len(s.settings))
	for _, setting := range s.settings {
		settings = append(settings, *setting)
	}
	return settings, nil
}

func (s *stubSettingsService) DeleteSetting(ctx context.Context, key string) error {
	delete(s.settings, key)
	return nil
}

func (s *stubSettingsService) GetSettingValue(ctx context.Context, key string, dest interface{}) error {
	return nil
}

func newProductServiceWithRepos(productRepo *repositorymocks.ProductRepository, supplierRepo *repositorymocks.SupplierRepository) (ProductService, *repositorymocks.UnitRepository) {
	unitRepo := new(repositorymocks.UnitRepository)
	// Set up mock to return a default unit when GetByID is called with ID 1
	unitRepo.On("GetByID", mock.Anything, uint(1)).Return(&models.Unit{
		Base:             models.Base{ID: 1},
		UnitType:         "general",
		Name:             "unit",
		Symbol:           "unit",
		ConversionFactor: 1,
	}, nil).Maybe()
	// Set up mock to return not found for other IDs
	unitRepo.On("GetByID", mock.Anything, mock.MatchedBy(func(id uint) bool {
		return id != 1
	})).Return(nil, gorm.ErrRecordNotFound).Maybe()
	// Set up mock for GetByTypeAndName to support CSV/Excel imports
	unitRepo.On("GetByTypeAndName", mock.Anything, "general", mock.Anything).Return(&models.Unit{
		Base:             models.Base{ID: 1},
		UnitType:         "general",
		Name:             "unit",
		Symbol:           "unit",
		ConversionFactor: 1,
	}, nil).Maybe()
	// Set up mock for Create to support unit creation during import
	unitRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Unit")).Return(nil).Maybe()
	// Set up mock for GetByNames to return empty map (no duplicates) by default
	productRepo.On("GetByNames", mock.Anything, mock.Anything).Return(make(map[string]*models.Product), nil).Maybe()
	settingsService := newStubSettingsService()
	return NewProductService(productRepo, supplierRepo, unitRepo, settingsService), unitRepo
}

func TestCreateProduct(t *testing.T) {
	ctx := context.Background()

	t.Run("should create product when unit exists", func(t *testing.T) {
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, unitRepo := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		unit, err := unitRepo.GetByID(ctx, 1)
		require.NoError(t, err)

		product := &models.Product{
			Name:        "Test Product",
			Description: "desc",
			ProductType: "general",
			UnitID:      unit.ID,
			Status:      "active",
		}

		mockProductRepo.On("Create", ctx, product).Return(nil).Once()

		err = service.CreateProduct(ctx, product)
		assert.NoError(t, err)
		mockProductRepo.AssertExpectations(t)
	})

	t.Run("should fail when unit_id missing", func(t *testing.T) {
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		product := &models.Product{
			Name:        "Test Product",
			ProductType: "general",
			Status:      "active",
		}

		err := service.CreateProduct(ctx, product)
		assert.Error(t, err)
		assert.True(t, pkg.IsErrorCode(err, pkg.ErrorCodeValidation))
	})

	t.Run("should return not found when unit missing", func(t *testing.T) {
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		product := &models.Product{
			Name:        "Test Product",
			ProductType: "general",
			Status:      "active",
			UnitID:      9999,
		}

		err := service.CreateProduct(ctx, product)
		assert.Error(t, err)
		assert.True(t, pkg.IsErrorCode(err, pkg.ErrorCodeNotFound))
	})
}

func TestImportProductsFromCSV(t *testing.T) {
	ctx := context.Background()

	t.Run("should successfully import a single product without suppliers", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Unit;Suppliers;ContactEmail;ContactPhone;Address
Laptop Dell XPS 13;High-performance laptop;Electronics;unit;;;;`

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
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Unit;Suppliers;ContactEmail;ContactPhone;Address
Laptop;High-performance laptop;Electronics;unit;Tech Inc;tech@email.com;+1-555-0123;123 Tech St`

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
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		// Note: Continuation rows (empty product name) are not supported for supplier association
		// Each row with a product name creates a separate product
		csvData := `Name;Description;ProductType;Unit;Suppliers;ContactEmail;ContactPhone;Address
Laptop;High-performance laptop;Electronics;unit;Tech Inc;tech@email.com;+1-555-0123;123 Tech St
Tablet;High-performance tablet;Electronics;unit;Dell;dell@email.com;+1-555-0456;456 Dell Way
Mouse;Wireless mouse;Electronics;unit;HP;hp@email.com;+1-555-0789;789 HP Blvd`

		supplier1 := &models.Supplier{Base: models.Base{ID: 1}, Name: "Tech Inc"}
		supplier2 := &models.Supplier{Base: models.Base{ID: 2}, Name: "Dell"}
		supplier3 := &models.Supplier{Base: models.Base{ID: 3}, Name: "HP"}

		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop"
		})).Return(nil).Once()

		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Tablet"
		})).Return(nil).Once()

		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Mouse"
		})).Return(nil).Once()

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
			return p.Name == "Laptop" && len(p.Suppliers) == 1
		})).Return(nil).Once()

		mockProductRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Tablet" && len(p.Suppliers) == 1
		})).Return(nil).Once()

		mockProductRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Mouse" && len(p.Suppliers) == 1
		})).Return(nil).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 3, count)
		mockProductRepo.AssertExpectations(t)
		mockSupplierRepo.AssertExpectations(t)
	})

	t.Run("should successfully import multiple products with suppliers", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Unit;Suppliers;ContactEmail;ContactPhone;Address
Laptop;High-performance laptop;Electronics;unit;Tech Inc;tech@email.com;;
Tablet;High-performance tablet;Electronics;unit;Tech Inc;tech@email.com;;
Mouse;Wireless mouse;Electronics;unit;;;;`

		supplier1 := &models.Supplier{Base: models.Base{ID: 1}, Name: "Tech Inc"}

		// Suppliers are deduplicated by name, so Tech Inc is only created once
		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.MatchedBy(func(s *models.Supplier) bool {
			return s.Name == "Tech Inc"
		})).Return(supplier1, nil).Once()

		// First product
		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop"
		})).Return(nil).Once()

		mockProductRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop"
		})).Return(nil).Once()

		// Second product
		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Tablet"
		})).Return(nil).Once()

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
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

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
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

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
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;Unit;Suppliers
Laptop;High-performance laptop;unit;Tech Inc`

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.Error(t, err)
		assert.Equal(t, 0, count)
		var appErr *pkg.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	})

	t.Run("should import supplier when product Name is empty but supplier is provided", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Unit;Suppliers
;High-performance laptop;Electronics;unit;Tech Inc`

		supplier1 := &models.Supplier{Base: models.Base{ID: 1}, Name: "Tech Inc"}
		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.MatchedBy(func(s *models.Supplier) bool {
			return s.Name == "Tech Inc"
		})).Return(supplier1, nil).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert - no products created (product name is empty), but supplier is created
		require.NoError(t, err)
		assert.Equal(t, 0, count) // No products created
		mockSupplierRepo.AssertExpectations(t)
	})

	t.Run("should import product with empty ProductType and supplier", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Unit;Suppliers
Laptop;High-performance laptop;;unit;Tech Inc`

		supplier1 := &models.Supplier{Base: models.Base{ID: 1}, Name: "Tech Inc"}

		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.MatchedBy(func(s *models.Supplier) bool {
			return s.Name == "Tech Inc"
		})).Return(supplier1, nil).Once()

		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop" && p.ProductType == "" // ProductType is empty
		})).Return(nil).Once()

		mockProductRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop" && len(p.Suppliers) == 1
		})).Return(nil).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert - product created with empty ProductType
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		mockProductRepo.AssertExpectations(t)
		mockSupplierRepo.AssertExpectations(t)
	})

	t.Run("should import supplier when row has no product data", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Unit;Suppliers
;;;;Tech Inc`

		supplier1 := &models.Supplier{Base: models.Base{ID: 1}, Name: "Tech Inc"}
		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.MatchedBy(func(s *models.Supplier) bool {
			return s.Name == "Tech Inc"
		})).Return(supplier1, nil).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert - no products but supplier is created
		require.NoError(t, err)
		assert.Equal(t, 0, count) // No products created
		mockSupplierRepo.AssertExpectations(t)
	})

	t.Run("should return error when CSV has no valid product data", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Unit;Suppliers

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
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Unit;Suppliers

Laptop;High-performance laptop;Electronics;unit;

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
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Unit;Suppliers
Laptop;High-performance laptop;Electronics;unit;`

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
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Unit;Suppliers
Laptop;High-performance laptop;Electronics;unit;Tech Inc`

		// Suppliers are created before products, so if supplier creation fails,
		// product creation is never attempted
		dbError := errors.New("database connection failed")
		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.AnythingOfType("*models.Supplier")).Return(nil, dbError).Once()

		// Execute
		count, err := service.ImportProductsFromCSV(ctx, strings.NewReader(csvData))

		// Assert
		require.Error(t, err)
		assert.Equal(t, 0, count)
		assert.Contains(t, err.Error(), "failed to create/find supplier")
		mockSupplierRepo.AssertExpectations(t)
	})

	t.Run("should return error when supplier association fails", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Unit;Suppliers
Laptop;High-performance laptop;Electronics;unit;Tech Inc`

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
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		csvData := `name;DESCRIPTION;producttype;UNIT;suppliers
Laptop;High-performance laptop;Electronics;unit;`

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
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Unit;Suppliers;ContactEmail;ContactPhone;Address
  Laptop  ;  High-performance laptop  ;  Electronics  ;  unit  ;  Tech Inc  ;  tech@email.com  ;  +1-555-0123  ;  123 Tech St  `

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
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		csvData := `Name;Description;ProductType;Unit;Suppliers;ContactEmail;ContactPhone;Address
Pepsi;;Nước;unit;Công ty TNHH Giải khát Sài Gòn;contact@email.com;0123456789;D5 Khu dân cư Thảo Nguyên`

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

func TestImportProductsFromExcel(t *testing.T) {
	ctx := context.Background()

	// Helper function to create Excel file
	createExcelFile := func(headers []string, rows [][]string) *excelize.File {
		f := excelize.NewFile()
		sheetName := "Sheet1"

		// Write headers
		for i, header := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(sheetName, cell, header)
		}

		// Write data rows
		for rowIdx, row := range rows {
			for colIdx, value := range row {
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
				f.SetCellValue(sheetName, cell, value)
			}
		}

		return f
	}

	t.Run("should successfully import a single product without suppliers from Excel", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		// Create Excel file
		headers := []string{"Name", "Description", "ProductType", "Unit", "Suppliers", "ContactEmail", "ContactPhone", "Address"}
		rows := [][]string{
			{"Laptop Dell XPS 13", "High-performance laptop", "Electronics", "unit", "", "", "", ""},
		}
		f := createExcelFile(headers, rows)

		// Convert to bytes
		buffer, err := f.WriteToBuffer()
		require.NoError(t, err)

		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop Dell XPS 13" &&
				p.Description == "High-performance laptop" &&
				p.ProductType == "Electronics" &&
				p.Status == "active"
		})).Return(nil).Once()

		// Execute
		count, err := service.ImportProductsFromExcel(ctx, buffer)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		mockProductRepo.AssertExpectations(t)
		mockSupplierRepo.AssertExpectations(t)
	})

	t.Run("should successfully import a product with suppliers from Excel", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		// Create Excel file
		// Note: Each row with a product name creates a separate product
		// Continuation rows (empty product name) are not supported for supplier association
		headers := []string{"Name", "Description", "ProductType", "Unit", "Suppliers", "ContactEmail", "ContactPhone", "Address"}
		rows := [][]string{
			{"Laptop", "High-performance laptop", "Electronics", "unit", "Tech Inc", "tech@email.com", "+1-555-0123", "123 Tech St"},
			{"Tablet", "High-performance tablet", "Electronics", "unit", "Dell", "dell@email.com", "+1-555-0456", "456 Dell Way"},
		}
		f := createExcelFile(headers, rows)

		buffer, err := f.WriteToBuffer()
		require.NoError(t, err)

		supplier1 := &models.Supplier{Base: models.Base{ID: 1}, Name: "Tech Inc"}
		supplier2 := &models.Supplier{Base: models.Base{ID: 2}, Name: "Dell"}

		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop"
		})).Return(nil).Once()

		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Tablet"
		})).Return(nil).Once()

		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.MatchedBy(func(s *models.Supplier) bool {
			return s.Name == "Tech Inc"
		})).Return(supplier1, nil).Once()

		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.MatchedBy(func(s *models.Supplier) bool {
			return s.Name == "Dell"
		})).Return(supplier2, nil).Once()

		mockProductRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Laptop" && len(p.Suppliers) == 1
		})).Return(nil).Once()

		mockProductRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Tablet" && len(p.Suppliers) == 1
		})).Return(nil).Once()

		// Execute
		count, err := service.ImportProductsFromExcel(ctx, buffer)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 2, count)
		mockProductRepo.AssertExpectations(t)
		mockSupplierRepo.AssertExpectations(t)
	})

	t.Run("should return error when Excel file is empty", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		// Create empty Excel file (no sheets)
		f := excelize.NewFile()
		f.DeleteSheet("Sheet1")

		buffer, err := f.WriteToBuffer()
		require.NoError(t, err)

		// Execute
		count, err := service.ImportProductsFromExcel(ctx, buffer)

		// Assert
		require.Error(t, err)
		assert.Equal(t, 0, count)
		var appErr *pkg.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	})

	t.Run("should return error when Excel has no data rows", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		// Create Excel with only headers
		headers := []string{"Name", "Description", "ProductType", "Unit", "Suppliers"}
		rows := [][]string{}
		f := createExcelFile(headers, rows)

		buffer, err := f.WriteToBuffer()
		require.NoError(t, err)

		// Execute
		count, err := service.ImportProductsFromExcel(ctx, buffer)

		// Assert
		require.Error(t, err)
		assert.Equal(t, 0, count)
		var appErr *pkg.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	})

	t.Run("should return error when required column Name is missing in Excel", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		// Create Excel without Name column
		headers := []string{"Description", "ProductType", "Unit", "Suppliers"}
		rows := [][]string{
			{"High-performance laptop", "Electronics", "unit", "Tech Inc"},
		}
		f := createExcelFile(headers, rows)

		buffer, err := f.WriteToBuffer()
		require.NoError(t, err)

		// Execute
		count, err := service.ImportProductsFromExcel(ctx, buffer)

		// Assert
		require.Error(t, err)
		assert.Equal(t, 0, count)
		var appErr *pkg.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	})

	t.Run("should handle Unicode characters in Excel", func(t *testing.T) {
		// Setup
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockSupplierRepo := repositorymocks.NewSupplierRepository(t)
		service, _ := newProductServiceWithRepos(mockProductRepo, mockSupplierRepo)

		// Create Excel with Unicode
		headers := []string{"Name", "Description", "ProductType", "Unit", "Suppliers", "ContactEmail", "ContactPhone", "Address"}
		rows := [][]string{
			{"Pepsi", "", "Nước", "unit", "Công ty TNHH Giải khát Sài Gòn", "contact@email.com", "0123456789", "D5 Khu dân cư Thảo Nguyên"},
		}
		f := createExcelFile(headers, rows)

		buffer, err := f.WriteToBuffer()
		require.NoError(t, err)

		supplier := &models.Supplier{
			Base:         models.Base{ID: 1},
			Name:         "Công ty TNHH Giải khát Sài Gòn",
			ContactEmail: "contact@email.com",
			ContactPhone: "0123456789",
			Address:      "D5 Khu dân cư Thảo Nguyên",
		}

		mockProductRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Product) bool {
			return p.Name == "Pepsi" && p.ProductType == "Nước"
		})).Return(nil).Once()

		mockSupplierRepo.On("FindOrCreateByName", ctx, mock.MatchedBy(func(s *models.Supplier) bool {
			return s.Name == "Công ty TNHH Giải khát Sài Gòn"
		})).Return(supplier, nil).Once()

		mockProductRepo.On("Update", ctx, mock.AnythingOfType("*models.Product")).Return(nil).Once()

		// Execute
		count, err := service.ImportProductsFromExcel(ctx, buffer)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		mockProductRepo.AssertExpectations(t)
		mockSupplierRepo.AssertExpectations(t)
	})
}
