package handlers

import (
	"cim-backend/internal/middleware"
	"cim-backend/internal/models"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockUnitService struct {
	mock.Mock
}

func (m *mockUnitService) CreateUnit(ctx context.Context, unit *models.Unit) error {
	args := m.Called(ctx, unit)
	return args.Error(0)
}

func (m *mockUnitService) GetUnitByID(ctx context.Context, id uint) (*models.Unit, error) {
	args := m.Called(ctx, id)
	unit, _ := args.Get(0).(*models.Unit)
	return unit, args.Error(1)
}

func (m *mockUnitService) UpdateUnit(ctx context.Context, unit *models.Unit) error {
	args := m.Called(ctx, unit)
	return args.Error(0)
}

func (m *mockUnitService) DeleteUnit(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUnitService) ListUnits(ctx context.Context, limit, offset int, sortBy, sortOrder, unitType string, baseOnly bool) ([]models.Unit, error) {
	args := m.Called(ctx, limit, offset, sortBy, sortOrder, unitType, baseOnly)
	units, _ := args.Get(0).([]models.Unit)
	return units, args.Error(1)
}

func (m *mockUnitService) SearchUnits(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, unitType string, baseOnly bool) ([]models.Unit, error) {
	args := m.Called(ctx, query, limit, offset, sortBy, sortOrder, unitType, baseOnly)
	units, _ := args.Get(0).([]models.Unit)
	return units, args.Error(1)
}

func (m *mockUnitService) CountUnits(ctx context.Context, unitType string, baseOnly bool) (int64, error) {
	args := m.Called(ctx, unitType, baseOnly)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockUnitService) CountSearchUnits(ctx context.Context, query string, unitType string, baseOnly bool) (int64, error) {
	args := m.Called(ctx, query, unitType, baseOnly)
	return args.Get(0).(int64), args.Error(1)
}

func TestUnitHandler_ListUnits(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := new(mockUnitService)
	handler := NewUnitHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/units?limit=10&page=2&sort=name&order=asc", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockUnits := []models.Unit{
		{
			Base:             models.Base{ID: 1},
			UnitType:         "mass",
			Name:             "kilogram",
			Symbol:           "kg",
			ConversionFactor: 1,
		},
	}

	mockService.On("ListUnits", mock.Anything, 10, 10, "name", "asc", "", false).Return(mockUnits, nil)
	mockService.On("CountUnits", mock.Anything, "", false).Return(int64(1), nil)

	err := handler.ListUnits(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	mockService.AssertExpectations(t)
}

func TestUnitHandler_ListUnits_WithSearch(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := new(mockUnitService)
	handler := NewUnitHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/units?q=kg&limit=10&page=1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockUnits := []models.Unit{
		{
			Base:             models.Base{ID: 1},
			UnitType:         "mass",
			Name:             "kilogram",
			Symbol:           "kg",
			ConversionFactor: 1,
		},
	}

	mockService.On("SearchUnits", mock.Anything, "kg", 10, 0, "created_at", "desc", "", false).Return(mockUnits, nil)
	mockService.On("CountSearchUnits", mock.Anything, "kg", "", false).Return(int64(1), nil)

	err := handler.ListUnits(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	mockService.AssertExpectations(t)
}

func TestUnitHandler_CreateUnit(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := new(mockUnitService)
	handler := NewUnitHandler(mockService)

	requestBody := createUnitRequest{
		UnitType:         "volume",
		Name:             "liter",
		Symbol:           "L",
		ConversionFactor: 1,
	}

	mockService.
		On("CreateUnit", mock.Anything, mock.MatchedBy(func(unit *models.Unit) bool {
			return unit.UnitType == "volume" && unit.Name == "liter" && unit.Symbol == "L"
		})).
		Return(nil)

	req, err := createRequest(http.MethodPost, "/units", requestBody)
	assert.NoError(t, err)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.CreateUnit(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	mockService.AssertExpectations(t)
}

func TestUnitHandler_CreateUnit_DerivedUnit(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := new(mockUnitService)
	handler := NewUnitHandler(mockService)

	baseUnitID := uint(10)
	requestBody := createUnitRequest{
		UnitType:         "mass",
		Name:             "gram",
		Symbol:           "g",
		BaseUnitID:       &baseUnitID,
		ConversionFactor: 0.001,
	}

	mockService.
		On("CreateUnit", mock.Anything, mock.MatchedBy(func(unit *models.Unit) bool {
			return unit.BaseUnitID != nil && *unit.BaseUnitID == baseUnitID && unit.ConversionFactor == 0.001
		})).
		Return(nil)

	req, err := createRequest(http.MethodPost, "/units", requestBody)
	assert.NoError(t, err)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.CreateUnit(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	mockService.AssertExpectations(t)
}

func TestUnitHandler_CreateUnit_ValidationError(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := new(mockUnitService)
	handler := NewUnitHandler(mockService)

	requestBody := createUnitRequest{}

	mockService.
		On("CreateUnit", mock.Anything, mock.AnythingOfType("*models.Unit")).
		Return(pkg.ErrValidation("unit_type is required", nil))

	req, err := createRequest(http.MethodPost, "/units", requestBody)
	assert.NoError(t, err)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.CreateUnit(c)
	assert.Error(t, err)
	e.HTTPErrorHandler(err, c)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockService.AssertExpectations(t)
}

func TestUnitHandler_GetUnit_NotFound(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := new(mockUnitService)
	handler := NewUnitHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/units/999", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("999")

	mockService.On("GetUnitByID", mock.Anything, uint(999)).Return((*models.Unit)(nil), pkg.ErrNotFound("unit not found", errors.New("not found")))

	err := handler.GetUnit(c)
	assert.Error(t, err)
	e.HTTPErrorHandler(err, c)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	mockService.AssertExpectations(t)
}

func TestUnitHandler_DeleteUnit(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := new(mockUnitService)
	handler := NewUnitHandler(mockService)

	req := httptest.NewRequest(http.MethodDelete, "/units/1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	mockService.On("DeleteUnit", mock.Anything, uint(1)).Return(nil)

	err := handler.DeleteUnit(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	mockService.AssertExpectations(t)
}

var _ services.UnitService = (*mockUnitService)(nil)
