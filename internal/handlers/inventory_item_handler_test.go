package handlers

import (
	"cim-backend/internal/middleware"
	"cim-backend/internal/mocks/servicemocks"
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestInventoryItemHandler_GetInventoryItemsByInventoryID(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := servicemocks.NewInventoryItemService(t)
	handler := NewInventoryItemHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/inventories/1/inventory-items?limit=20&page=1&sort=updated_at&order=desc", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/:id/inventory-items")
	c.SetParamNames("id")
	c.SetParamValues("1")

	mockUnit := &models.Unit{
		Base:             models.Base{ID: 1},
		UnitType:         "mass",
		Name:             "Kilogram",
		Symbol:           "kg",
		ConversionFactor: 1,
		Level:            1,
		DecimalPlaces:    2,
	}

	mockProduct := &models.Product{
		Base:   models.Base{ID: 1},
		Name:   "Test Product",
		Status: "active",
		UnitID: 1,
		Unit:   mockUnit,
	}

	mockInventory := &models.Inventory{
		Base:   models.Base{ID: 1},
		Name:   "Test Inventory",
		Status: models.InventoryStatusActive,
	}

	mockItems := []models.InventoryItem{
		{
			Base:        models.Base{ID: 1},
			InventoryID: 1,
			Inventory:   mockInventory,
			ProductID:   1,
			Product:     mockProduct,
			Quantity:    decimal.NewFromInt(10),
			UnitID:      1,
			Unit:        mockUnit,
			Status:      models.InventoryItemStatusActive,
		},
		{
			Base:        models.Base{ID: 2},
			InventoryID: 1,
			Inventory:   mockInventory,
			ProductID:   1,
			Product:     mockProduct,
			Quantity:    decimal.NewFromInt(5),
			UnitID:      1,
			Unit:        mockUnit,
			Status:      models.InventoryItemStatusActive,
		},
	}

	mockService.On("GetInventoryItemsByInventoryIDWithFilters", mock.Anything, uint(1), "", mock.MatchedBy(func(params models.ListParams) bool {
		return params.Limit == 20 && params.Page == 1 && params.Sort == "updated_at" && params.Order == "desc"
	})).Return(mockItems, nil)

	mockService.On("CountInventoryItemsByInventoryIDWithFilters", mock.Anything, uint(1), "", mock.MatchedBy(func(params models.ListParams) bool {
		return params.Status == ""
	})).Return(int64(2), nil)

	err := handler.GetInventoryItemsByInventoryID(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response models.PaginationResult[models.InventoryItem]
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(response.Data))
	assert.Equal(t, int64(2), response.Total)
	assert.Equal(t, 1, response.Page)
	assert.Equal(t, 20, response.Limit)

	// Verify Unit is included in response
	for _, item := range response.Data {
		assert.NotNil(t, item.Unit, "Unit should be preloaded")
		assert.Equal(t, uint(1), item.Unit.ID)
		assert.Equal(t, "Kilogram", item.Unit.Name)
		assert.Equal(t, "kg", item.Unit.Symbol)
	}

	// Verify Inventory and Product are also included
	for _, item := range response.Data {
		assert.NotNil(t, item.Inventory, "Inventory should be preloaded")
		assert.NotNil(t, item.Product, "Product should be preloaded")
	}
}

func TestInventoryItemHandler_GetInventoryItemsByInventoryID_WithStatusFilter(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := servicemocks.NewInventoryItemService(t)
	handler := NewInventoryItemHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/inventories/1/inventory-items?status=active&limit=20&page=1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/:id/inventory-items")
	c.SetParamNames("id")
	c.SetParamValues("1")

	mockUnit := &models.Unit{
		Base:             models.Base{ID: 1},
		Name:             "Kilogram",
		Symbol:           "kg",
		ConversionFactor: 1,
	}

	mockItems := []models.InventoryItem{
		{
			Base:        models.Base{ID: 1},
			InventoryID: 1,
			ProductID:   1,
			Quantity:    decimal.NewFromInt(10),
			UnitID:      1,
			Unit:        mockUnit,
			Status:      models.InventoryItemStatusActive,
		},
	}

	mockService.On("GetInventoryItemsByInventoryIDWithFilters", mock.Anything, uint(1), "", mock.MatchedBy(func(params models.ListParams) bool {
		return params.Status == "active"
	})).Return(mockItems, nil)

	mockService.On("CountInventoryItemsByInventoryIDWithFilters", mock.Anything, uint(1), "", mock.MatchedBy(func(params models.ListParams) bool {
		return params.Status == "active"
	})).Return(int64(1), nil)

	err := handler.GetInventoryItemsByInventoryID(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response models.PaginationResult[models.InventoryItem]
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(response.Data))
	assert.Equal(t, models.InventoryItemStatusActive, response.Data[0].Status)
}

func TestInventoryItemHandler_GetInventoryItemsByInventoryID_WithProductTypeFilter(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := servicemocks.NewInventoryItemService(t)
	handler := NewInventoryItemHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/inventories/1/inventory-items?product_type=material&limit=20&page=1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/:id/inventory-items")
	c.SetParamNames("id")
	c.SetParamValues("1")

	mockUnit := &models.Unit{
		Base:   models.Base{ID: 1},
		Name:   "Kilogram",
		Symbol: "kg",
	}

	mockProduct := &models.Product{
		Base:        models.Base{ID: 1},
		Name:        "Material Product",
		ProductType: "material",
		UnitID:      1,
	}

	mockItems := []models.InventoryItem{
		{
			Base:        models.Base{ID: 1},
			InventoryID: 1,
			ProductID:   1,
			Product:     mockProduct,
			Quantity:    decimal.NewFromInt(10),
			UnitID:      1,
			Unit:        mockUnit,
			Status:      models.InventoryItemStatusActive,
		},
	}

	mockService.On("GetInventoryItemsByInventoryIDWithFilters", mock.Anything, uint(1), "material", mock.Anything).Return(mockItems, nil)
	mockService.On("CountInventoryItemsByInventoryIDWithFilters", mock.Anything, uint(1), "material", mock.Anything).Return(int64(1), nil)

	err := handler.GetInventoryItemsByInventoryID(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response models.PaginationResult[models.InventoryItem]
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(response.Data))
	assert.Equal(t, "material", response.Data[0].Product.ProductType)
}

func TestInventoryItemHandler_GetInventoryItemsByInventoryID_InvalidInventoryID(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := servicemocks.NewInventoryItemService(t)
	handler := NewInventoryItemHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/inventories/invalid/inventory-items", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/:id/inventory-items")
	c.SetParamNames("id")
	c.SetParamValues("invalid")

	err := handler.GetInventoryItemsByInventoryID(c)
	assert.Error(t, err)
	// Error is returned for invalid inventory ID
}

func TestInventoryItemHandler_GetInventoryItemsByInventoryID_ServiceError(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := servicemocks.NewInventoryItemService(t)
	handler := NewInventoryItemHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/inventories/1/inventory-items?limit=20&page=1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/:id/inventory-items")
	c.SetParamNames("id")
	c.SetParamValues("1")

	// Mock service to return error
	mockService.On("GetInventoryItemsByInventoryIDWithFilters", mock.Anything, uint(1), "", mock.Anything).Return([]models.InventoryItem(nil), pkg.NewAppError(pkg.ErrorCodeNotFound, "inventory not found", nil))

	err := handler.GetInventoryItemsByInventoryID(c)
	assert.NoError(t, err) // Handler handles errors internally and returns JSON
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestInventoryItemHandler_GetInventoryItemsByInventoryID_SortingOptions(t *testing.T) {
	tests := []struct {
		name          string
		sortField     string
		sortOrder     string
		expectedSort  string
		expectedOrder string
	}{
		{
			name:          "Sort by updated_at desc",
			sortField:     "updated_at",
			sortOrder:     "desc",
			expectedSort:  "updated_at",
			expectedOrder: "desc",
		},
		{
			name:          "Sort by quantity asc",
			sortField:     "quantity",
			sortOrder:     "asc",
			expectedSort:  "quantity",
			expectedOrder: "asc",
		},
		{
			name:          "Sort by product_name asc",
			sortField:     "product_name",
			sortOrder:     "asc",
			expectedSort:  "product_name",
			expectedOrder: "asc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			e.HTTPErrorHandler = middleware.CustomErrorHandler

			mockService := servicemocks.NewInventoryItemService(t)
			handler := NewInventoryItemHandler(mockService)

			req := httptest.NewRequest(http.MethodGet, "/inventories/1/inventory-items?sort="+tt.sortField+"&order="+tt.sortOrder+"&limit=20&page=1", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/inventories/:id/inventory-items")
			c.SetParamNames("id")
			c.SetParamValues("1")

			mockUnit := &models.Unit{
				Base:   models.Base{ID: 1},
				Name:   "Kilogram",
				Symbol: "kg",
			}

			mockItems := []models.InventoryItem{
				{
					Base:        models.Base{ID: 1},
					InventoryID: 1,
					ProductID:   1,
					Quantity:    decimal.NewFromInt(10),
					UnitID:      1,
					Unit:        mockUnit,
					Status:      models.InventoryItemStatusActive,
				},
			}

			mockService.On("GetInventoryItemsByInventoryIDWithFilters", mock.Anything, uint(1), "", mock.MatchedBy(func(params models.ListParams) bool {
				return params.Sort == tt.expectedSort && params.Order == tt.expectedOrder
			})).Return(mockItems, nil)

			mockService.On("CountInventoryItemsByInventoryIDWithFilters", mock.Anything, uint(1), "", mock.Anything).Return(int64(1), nil)

			err := handler.GetInventoryItemsByInventoryID(c)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}
