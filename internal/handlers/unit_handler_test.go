package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"cim-backend/internal/middleware"
	"cim-backend/internal/mocks/servicemocks"
	"cim-backend/internal/models"
	"cim-backend/pkg"
)

func TestUnitHandler_ListUnits(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := servicemocks.NewUnitService(t)
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
}

func TestUnitHandler_ListUnits_WithSearch(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := servicemocks.NewUnitService(t)
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
}

func TestUnitHandler_CreateUnit(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := servicemocks.NewUnitService(t)
	handler := NewUnitHandler(mockService)

	requestBody := createUnitRequest{
		UnitType:         "volume",
		Name:             "liter",
		Symbol:           "L",
		ConversionFactor: 1,
	}

	createdUnit := &models.Unit{
		Base:             models.Base{ID: 1},
		UnitType:         "volume",
		Name:             "LITER",
		Symbol:           "L",
		ConversionFactor: 1,
	}

	mockService.
		On("GetOrCreateUnit", mock.Anything, mock.MatchedBy(func(unit *models.Unit) bool {
			return unit.UnitType == "volume" && unit.Name == "liter" && unit.Symbol == "L"
		})).
		Return(createdUnit, nil)

	req, err := createRequest(http.MethodPost, "/units", requestBody)
	assert.NoError(t, err)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.CreateUnit(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestUnitHandler_CreateUnit_DerivedUnit(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := servicemocks.NewUnitService(t)
	handler := NewUnitHandler(mockService)

	baseUnitID := uint(10)
	requestBody := createUnitRequest{
		UnitType:         "mass",
		Name:             "gram",
		Symbol:           "g",
		BaseUnitID:       &baseUnitID,
		ConversionFactor: 0.001,
	}

	createdUnit := &models.Unit{
		Base:             models.Base{ID: 1},
		UnitType:         "mass",
		Name:             "GRAM",
		Symbol:           "g",
		BaseUnitID:       &baseUnitID,
		ConversionFactor: 0.001,
	}

	mockService.
		On("GetOrCreateUnit", mock.Anything, mock.MatchedBy(func(unit *models.Unit) bool {
			return unit.BaseUnitID != nil && *unit.BaseUnitID == baseUnitID && unit.ConversionFactor == 0.001
		})).
		Return(createdUnit, nil)

	req, err := createRequest(http.MethodPost, "/units", requestBody)
	assert.NoError(t, err)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.CreateUnit(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestUnitHandler_CreateUnit_ValidationError(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := servicemocks.NewUnitService(t)
	handler := NewUnitHandler(mockService)

	requestBody := createUnitRequest{}

	req, err := createRequest(http.MethodPost, "/units", requestBody)
	assert.NoError(t, err)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.CreateUnit(c)
	assert.Error(t, err)
	e.HTTPErrorHandler(err, c)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUnitHandler_GetUnit_NotFound(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := servicemocks.NewUnitService(t)
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
}

func TestUnitHandler_DeleteUnit(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := servicemocks.NewUnitService(t)
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
}
