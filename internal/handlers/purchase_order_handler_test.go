package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"import-export-backend/internal/handlers/servicemocks"
	"import-export-backend/internal/models"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreatePurchaseOrder(t *testing.T) {
	type testSetup struct {
		mockService *servicemocks.PurchaseOrderService
		handler     *PurchaseOrderHandler
		echo        *echo.Echo
	}

	setupTest := func(t *testing.T) *testSetup {
		mockService := servicemocks.NewPurchaseOrderService(t)
		handler := NewPurchaseOrderHandler(mockService)
		e := echo.New()

		return &testSetup{
			mockService: mockService,
			handler:     handler,
			echo:        e,
		}
	}

	t.Run("should return success case with all fields populated", func(t *testing.T) {
		setup := setupTest(t)

		requestBody := models.PurchaseOrder{
			OrderNumber: "PO-001",
			Status:      "pending",
			TotalAmount: 1500.50,
			Notes:       "Test purchase order with all fields",
			CreatedBy:   "user123",
			Items: []models.PurchaseOrderItem{
				{
					ProductID:  uuid.New(),
					Quantity:   5,
					UnitPrice:  100.50,
					TotalPrice: 502.50,
				},
				{
					ProductID:  uuid.New(),
					Quantity:   10,
					UnitPrice:  250.25,
					TotalPrice: 2502.50,
				},
			},
		}

		// Setup mock
		setup.mockService.On("CreatePurchaseOrder", mock.AnythingOfType("*models.PurchaseOrder")).Return(nil)

		// Create request and context
		req, err := createRequest(http.MethodPost, "/purchase-orders", requestBody)
		require.NoError(t, err)
		rec := httptest.NewRecorder()
		c := setup.echo.NewContext(req, rec)

		// Execute
		err = setup.handler.CreatePurchaseOrder(c)
		require.NoError(t, err)

		// Assertions
		responseOrder := assertSuccessResponse(t, rec, http.StatusCreated)
		assert.Equal(t, requestBody.OrderNumber, responseOrder.OrderNumber)
		assert.Equal(t, requestBody.TotalAmount, responseOrder.TotalAmount)
		assert.Equal(t, requestBody.Notes, responseOrder.Notes)
		assert.Equal(t, requestBody.CreatedBy, responseOrder.CreatedBy)
	})

	t.Run("should return internal server error when service returns error", func(t *testing.T) {
		// Setup
		setup := setupTest(t)

		requestBody := models.PurchaseOrder{
			OrderNumber: "PO-003",
		}

		// Setup mock
		setup.mockService.On("CreatePurchaseOrder", mock.AnythingOfType("*models.PurchaseOrder")).Return(errors.New("database error"))

		// Create request and context
		req, err := createRequest(http.MethodPost, "/purchase-orders", requestBody)
		require.NoError(t, err)
		rec := httptest.NewRecorder()
		c := setup.echo.NewContext(req, rec)

		// Execute
		err = setup.handler.CreatePurchaseOrder(c)
		require.NoError(t, err)

		// Assertions
		assertErrorResponse(t, rec, http.StatusInternalServerError, "Failed to create purchase order")
	})

	t.Run("should return bad request when request body is invalid data types", func(t *testing.T) {
		// Setup
		setup := setupTest(t)

		requestBody := map[string]interface{}{
			"order_number": "PO-004",
			"total_amount": "invalid_amount", // Invalid type
		}

		// Create request and context
		req, err := createRequest(http.MethodPost, "/purchase-orders", requestBody)
		require.NoError(t, err)
		rec := httptest.NewRecorder()
		c := setup.echo.NewContext(req, rec)

		// Execute
		err = setup.handler.CreatePurchaseOrder(c)
		require.NoError(t, err)

		// Assertions
		assertErrorResponse(t, rec, http.StatusBadRequest, "Invalid request body")
	})

	t.Run("should return bad request when request body is invalid JSON", func(t *testing.T) {
		// Setup
		setup := setupTest(t)

		requestBody := `{"invalid": json}` // Invalid JSON

		// Create request and context
		req, err := createRequest(http.MethodPost, "/purchase-orders", requestBody)
		require.NoError(t, err)
		rec := httptest.NewRecorder()
		c := setup.echo.NewContext(req, rec)

		// Execute
		err = setup.handler.CreatePurchaseOrder(c)
		require.NoError(t, err)

		// Assertions
		assertErrorResponse(t, rec, http.StatusBadRequest, "Invalid request body")
	})

	t.Run("should return success when request body is empty", func(t *testing.T) {
		// Setup
		setup := setupTest(t)

		// Setup mock - empty body should still create with default values
		setup.mockService.On("CreatePurchaseOrder", mock.AnythingOfType("*models.PurchaseOrder")).Return(nil)

		// Create request and context
		req, err := createRequest(http.MethodPost, "/purchase-orders", "")
		require.NoError(t, err)
		rec := httptest.NewRecorder()
		c := setup.echo.NewContext(req, rec)

		// Execute
		err = setup.handler.CreatePurchaseOrder(c)
		require.NoError(t, err)

		// Assertions
		assertSuccessResponse(t, rec, http.StatusCreated)
	})
}
