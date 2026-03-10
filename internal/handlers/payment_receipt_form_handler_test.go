package handlers

import (
	"cim-backend/internal/mocks/servicemocks"
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPaymentReceiptFormHandler_CreatePaymentReceiptForm(t *testing.T) {
	e := echo.New()
	mockService := new(servicemocks.PaymentReceiptFormService)
	mockPO := new(servicemocks.PurchaseOrderService)
	handler := NewPaymentReceiptFormHandler(mockService, mockPO)

	t.Run("Success", func(t *testing.T) {
		payload := dto.PaymentReceiptFormPayload{
			PurchaseOrderID: 1,
			FullName:        "John Doe",
			Department:      "Finance",
			Details:         "Office supplies",
			TotalAmount:     1000,
		}
		jsonPayload, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/payment-receipt-forms", strings.NewReader(string(jsonPayload)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockService.On("CreatePaymentReceiptForm", mock.Anything, mock.AnythingOfType("*models.PaymentReceiptForm")).Return(&models.PaymentReceiptForm{
			Base:            models.Base{ID: 1},
			FullName:        "John Doe",
			PurchaseOrderID: 1,
		}, nil).Once()
		mockPO.On("GetPurchaseOrderByID", uint(1)).Return(&models.PurchaseOrder{Base: models.Base{ID: 1}, OrderNumber: "PO-001"}, nil).Once()

		if assert.NoError(t, handler.CreatePaymentReceiptForm(c)) {
			assert.Equal(t, http.StatusCreated, rec.Code)
		}
	})

	t.Run("Invalid Payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/payment-receipt-forms", strings.NewReader("invalid"))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		assert.NoError(t, handler.CreatePaymentReceiptForm(c))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Service Error", func(t *testing.T) {
		payload := dto.PaymentReceiptFormPayload{
			PurchaseOrderID: 1,
			FullName:        "John Doe",
			Details:         "Office supplies",
			TotalAmount:     1000,
		}
		jsonPayload, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/payment-receipt-forms", strings.NewReader(string(jsonPayload)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockService.On("CreatePaymentReceiptForm", mock.Anything, mock.Anything).Return(nil, errors.New("service error")).Once()

		assert.NoError(t, handler.CreatePaymentReceiptForm(c))
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestPaymentReceiptFormHandler_GetPaymentReceiptForm(t *testing.T) {
	e := echo.New()
	mockService := new(servicemocks.PaymentReceiptFormService)
	mockPO := new(servicemocks.PurchaseOrderService)
	handler := NewPaymentReceiptFormHandler(mockService, mockPO)

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/payment-receipt-forms/1", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("1")

		mockService.On("GetPaymentReceiptForm", mock.Anything, uint(1)).Return(&models.PaymentReceiptForm{Base: models.Base{ID: 1}}, nil).Once()

		if assert.NoError(t, handler.GetPaymentReceiptForm(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})

	t.Run("Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/payment-receipt-forms/1", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("1")

		mockService.On("GetPaymentReceiptForm", mock.Anything, uint(1)).Return(nil, nil).Once()

		assert.NoError(t, handler.GetPaymentReceiptForm(c))
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("Invalid ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/payment-receipt-forms/abc", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("abc")

		assert.NoError(t, handler.GetPaymentReceiptForm(c))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestPaymentReceiptFormHandler_ListPaymentReceiptForms(t *testing.T) {
	e := echo.New()
	mockService := new(servicemocks.PaymentReceiptFormService)
	mockPO := new(servicemocks.PurchaseOrderService)
	handler := NewPaymentReceiptFormHandler(mockService, mockPO)

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/payment-receipt-forms?page=1&limit=10", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockService.On("ListPaymentReceiptForms", mock.Anything, mock.Anything).Return([]models.PaymentReceiptForm{{Base: models.Base{ID: 1}}}, int64(1), nil).Once()

		if assert.NoError(t, handler.ListPaymentReceiptForms(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})

	t.Run("Service Error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/payment-receipt-forms", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockService.On("ListPaymentReceiptForms", mock.Anything, mock.Anything).Return(nil, int64(0), errors.New("service error")).Once()

		assert.NoError(t, handler.ListPaymentReceiptForms(c))
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestPaymentReceiptFormHandler_SubmitPaymentReceiptForm(t *testing.T) {
	e := echo.New()
	mockService := new(servicemocks.PaymentReceiptFormService)
	mockPO := new(servicemocks.PurchaseOrderService)
	handler := NewPaymentReceiptFormHandler(mockService, mockPO)

	t.Run("Success", func(t *testing.T) {
		payload := dto.PaymentReceiptFormPayload{
			PurchaseOrderID: 1,
			FullName:        "John Doe",
			Details:         "Office supplies",
			TotalAmount:     1000,
		}
		jsonPayload, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/payment-receipt-forms/1/submit", strings.NewReader(string(jsonPayload)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("1")

		mockService.On("SubmitPaymentReceiptForm", mock.Anything, mock.Anything).Return(nil).Once()

		if assert.NoError(t, handler.SubmitPaymentReceiptForm(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})
}

func TestPaymentReceiptFormHandler_UpdatePaymentReceiptForm(t *testing.T) {
	e := echo.New()
	mockService := new(servicemocks.PaymentReceiptFormService)
	mockPO := new(servicemocks.PurchaseOrderService)
	handler := NewPaymentReceiptFormHandler(mockService, mockPO)

	t.Run("Success", func(t *testing.T) {
		payload := dto.PaymentReceiptFormPayload{
			PurchaseOrderID: 1,
			FullName:        "John Doe",
			Details:         "Office supplies",
			TotalAmount:     1000,
		}
		jsonPayload, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, "/payment-receipt-forms/1", strings.NewReader(string(jsonPayload)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("1")

		mockService.On("UpdatePaymentReceiptForm", mock.Anything, mock.Anything).Return(nil).Once()

		if assert.NoError(t, handler.UpdatePaymentReceiptForm(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})
}

func TestPaymentReceiptFormHandler_ApprovePaymentReceiptForm(t *testing.T) {
	e := echo.New()
	mockService := new(servicemocks.PaymentReceiptFormService)
	mockPO := new(servicemocks.PurchaseOrderService)
	handler := NewPaymentReceiptFormHandler(mockService, mockPO)

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/payment-receipt-forms/1/approve", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("1")

		mockService.On("ApprovePaymentReceiptForm", mock.Anything, uint(1)).Return(nil).Once()

		if assert.NoError(t, handler.ApprovePaymentReceiptForm(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})
}

func TestPaymentReceiptFormHandler_RejectPaymentReceiptForm(t *testing.T) {
	e := echo.New()
	mockService := new(servicemocks.PaymentReceiptFormService)
	mockPO := new(servicemocks.PurchaseOrderService)
	handler := NewPaymentReceiptFormHandler(mockService, mockPO)

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/payment-receipt-forms/1/reject", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("1")

		mockService.On("RejectPaymentReceiptForm", mock.Anything, uint(1)).Return(nil).Once()

		if assert.NoError(t, handler.RejectPaymentReceiptForm(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})
}

func TestPaymentReceiptFormHandler_DeletePaymentReceiptForm(t *testing.T) {
	e := echo.New()
	mockService := new(servicemocks.PaymentReceiptFormService)
	mockPO := new(servicemocks.PurchaseOrderService)
	handler := NewPaymentReceiptFormHandler(mockService, mockPO)

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/payment-receipt-forms/1", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("1")

		mockService.On("DeletePaymentReceiptForm", mock.Anything, uint(1)).Return(nil).Once()

		if assert.NoError(t, handler.DeletePaymentReceiptForm(c)) {
			assert.Equal(t, http.StatusNoContent, rec.Code)
		}
	})
}

func TestPaymentReceiptFormHandler_LatestPendingPaymentReceiptFormStream(t *testing.T) {
	e := echo.New()
	mockService := new(servicemocks.PaymentReceiptFormService)
	mockPO := new(servicemocks.PurchaseOrderService)
	handler := NewPaymentReceiptFormHandler(mockService, mockPO)

	t.Run("Unauthorized - Handled by middleware but handler should handle error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/payment-receipt-forms/pending", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler.LatestPendingPaymentReceiptFormStream(c)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestPaymentReceiptFormHandler_NotifyPaymentReceiptForm(t *testing.T) {
	e := echo.New()
	mockService := new(servicemocks.PaymentReceiptFormService)
	mockPO := new(servicemocks.PurchaseOrderService)
	handler := NewPaymentReceiptFormHandler(mockService, mockPO)

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/payment-receipt-forms/1/notify", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("1")

		mockService.On("GetPaymentReceiptForm", mock.Anything, uint(1)).Return(&models.PaymentReceiptForm{Base: models.Base{ID: 1}}, nil).Once()

		if assert.NoError(t, handler.NotifyPaymentReceiptForm(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})
}
