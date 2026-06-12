package handlers

import (
	"cim-backend/internal/mocks/servicemocks"
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
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

// newUpdatePOItemSellingPriceCtx builds an echo context for
// PUT /purchase-orders/:id/items/:itemId/selling-price with the given JSON body.
func newUpdatePOItemSellingPriceCtx(e *echo.Echo, body string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodPut, "/purchase-orders/1/items/2/selling-price", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id", "itemId")
	c.SetParamValues("1", "2")
	return c, rec
}

// A selling price of 0 is a valid value (only NEGATIVE is rejected): the handler
// must accept it and call the service.
func TestUpdatePOItemSellingPrice_Zero_Accepted(t *testing.T) {
	e := echo.New()
	mockService := servicemocks.NewSellingPriceService(t)
	handler := NewSellingPriceHandler(mockService)

	c, rec := newUpdatePOItemSellingPriceCtx(e, `{"selling_price": 0}`)

	mockService.On("UpsertPOItemSellingPrice", mock.Anything, uint(1), uint(2), mock.MatchedBy(func(d decimal.Decimal) bool {
		return d.Equal(decimal.Zero)
	})).Return(&models.POItemSellingPrice{}, nil)

	err := handler.UpdatePOItemSellingPrice(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUpdatePOItemSellingPrice_Positive_Accepted(t *testing.T) {
	e := echo.New()
	mockService := servicemocks.NewSellingPriceService(t)
	handler := NewSellingPriceHandler(mockService)

	c, rec := newUpdatePOItemSellingPriceCtx(e, `{"selling_price": 150}`)

	mockService.On("UpsertPOItemSellingPrice", mock.Anything, uint(1), uint(2), mock.MatchedBy(func(d decimal.Decimal) bool {
		return d.Equal(decimal.NewFromInt(150))
	})).Return(&models.POItemSellingPrice{}, nil)

	err := handler.UpdatePOItemSellingPrice(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUpdatePOItemSellingPrice_Negative_Rejected(t *testing.T) {
	e := echo.New()
	mockService := servicemocks.NewSellingPriceService(t)
	handler := NewSellingPriceHandler(mockService)

	c, rec := newUpdatePOItemSellingPriceCtx(e, `{"selling_price": -1}`)

	// Service must NOT be called: validation fails before reaching it.
	err := handler.UpdatePOItemSellingPrice(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "must not be negative")
	mockService.AssertNotCalled(t, "UpsertPOItemSellingPrice", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// An OMITTED selling_price binds to a nil pointer and must be rejected: now that
// the guard only rejects negatives, a missing field would otherwise bind to 0
// and silently create a real zero-price override. It must be explicit.
func TestUpdatePOItemSellingPrice_Omitted_Rejected(t *testing.T) {
	e := echo.New()
	mockService := servicemocks.NewSellingPriceService(t)
	handler := NewSellingPriceHandler(mockService)

	c, rec := newUpdatePOItemSellingPriceCtx(e, `{}`)

	// Service must NOT be called: the field is required and absent.
	err := handler.UpdatePOItemSellingPrice(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "selling_price is required")
	mockService.AssertNotCalled(t, "UpsertPOItemSellingPrice", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The backfill boundary assertion surfaces as a conflict AppError; the handler
// must map it to 409 (not 500) so the FE can prompt a re-preview.
func TestBackfillPOItems_BoundaryConflict_Returns409(t *testing.T) {
	e := echo.New()
	mockService := servicemocks.NewSellingPriceService(t)
	handler := NewSellingPriceHandler(mockService)

	req := httptest.NewRequest(http.MethodPost, "/selling-prices/1/backfill", strings.NewReader(`{"end_effective_from": "2026-05-01"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	endDate := "2026-05-01"
	mockService.On("ApplyMassiveLinks", mock.Anything, uint(1), &endDate).
		Return(int64(0), pkg.NewAppError(pkg.ErrorCodeConflict, "selling price range boundary changed since preview", nil))

	err := handler.BackfillPOItems(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
}
