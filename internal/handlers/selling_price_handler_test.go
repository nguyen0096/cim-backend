package handlers

import (
	"cim-backend/internal/mocks/servicemocks"
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetSellingPrice_NotFound_Returns404(t *testing.T) {
	e := echo.New()
	mockService := servicemocks.NewSellingPriceService(t)
	handler := NewSellingPriceHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/selling-prices/999", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("999")

	mockService.On("GetSellingPrice", mock.Anything, uint(999)).
		Return((*models.SellingPrice)(nil), pkg.ErrNotFound("selling price with ID 999 not found", errors.New("not found")))

	err := handler.GetSellingPrice(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetSellingPrice_InternalError_Returns500(t *testing.T) {
	e := echo.New()
	mockService := servicemocks.NewSellingPriceService(t)
	handler := NewSellingPriceHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/selling-prices/1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	mockService.On("GetSellingPrice", mock.Anything, uint(1)).
		Return((*models.SellingPrice)(nil), errors.New("database connection failed"))

	err := handler.GetSellingPrice(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
