package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cim-backend/internal/middleware"
	"cim-backend/internal/mocks/servicemocks"
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestInventoryHandler_ListReconciliations_Success drives the happy path: the
// handler validates pagination, calls the service, and wraps the result in a
// PaginationResult. With no reconcile_status query param the service receives a
// nil status filter (default open+closed set).
func TestInventoryHandler_ListReconciliations_Success(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler
	mockService := servicemocks.NewInventoryService(t)
	handler := NewInventoryHandler(mockService)

	rows := []dto.SubmissionResponse{{
		ID:              7,
		InventoryID:     10,
		SubmissionType:  models.InventorySubmissionTypeReconcile,
		Status:          models.InventorySubmissionStatusPending,
		ReconcileStatus: models.ReconcileLifecycleStatusOpen,
		Items:           []dto.QuantityItem{},
	}}
	mockService.
		On("ListActiveReconciliations", mock.Anything, mock.Anything, []string(nil)).
		Return(rows, int64(1), nil).
		Once()

	req, err := createRequest(http.MethodGet, "/inventories/reconciliations", nil)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/reconciliations")

	require.NoError(t, handler.ListReconciliations(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var body models.PaginationResult[dto.SubmissionResponse]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, int64(1), body.Total)
	require.Len(t, body.Data, 1)
	assert.Equal(t, uint(7), body.Data[0].ID)
	mockService.AssertExpectations(t)
}

// TestInventoryHandler_ListReconciliations_StatusFilterPassesThrough confirms a
// valid reconcile_status=closed is forwarded to the service as the narrowed
// status set.
func TestInventoryHandler_ListReconciliations_StatusFilterPassesThrough(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler
	mockService := servicemocks.NewInventoryService(t)
	handler := NewInventoryHandler(mockService)

	mockService.
		On("ListActiveReconciliations", mock.Anything, mock.Anything, []string{"closed"}).
		Return([]dto.SubmissionResponse{}, int64(0), nil).
		Once()

	req, err := createRequest(http.MethodGet, "/inventories/reconciliations?reconcile_status=closed", nil)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/reconciliations")

	require.NoError(t, handler.ListReconciliations(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	mockService.AssertExpectations(t)
}

// TestInventoryHandler_ListReconciliations_InvalidStatus rejects an out-of-set
// reconcile_status with 400 and never calls the service.
func TestInventoryHandler_ListReconciliations_InvalidStatus(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler
	mockService := servicemocks.NewInventoryService(t)
	handler := NewInventoryHandler(mockService)

	req, err := createRequest(http.MethodGet, "/inventories/reconciliations?reconcile_status=processing", nil)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/reconciliations")

	require.NoError(t, handler.ListReconciliations(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockService.AssertNotCalled(t, "ListActiveReconciliations", mock.Anything, mock.Anything, mock.Anything)
}

// TestInventoryHandler_ListReconciliations_ForbiddenPropagates verifies a service-
// layer recon_manage denial (ErrForbidden) surfaces as a 403 through the custom
// error handler — auth is enforced in the service, not via route middleware.
func TestInventoryHandler_ListReconciliations_ForbiddenPropagates(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler
	mockService := servicemocks.NewInventoryService(t)
	handler := NewInventoryHandler(mockService)

	mockService.
		On("ListActiveReconciliations", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, int64(0), pkg.ErrForbidden("user does not have permission to manage reconciliations", nil)).
		Once()

	req, err := createRequest(http.MethodGet, "/inventories/reconciliations", nil)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/reconciliations")

	// The handler returns the error; the echo CustomErrorHandler renders it.
	if hErr := handler.ListReconciliations(c); hErr != nil {
		e.HTTPErrorHandler(hErr, c)
	}
	assert.Equal(t, http.StatusForbidden, rec.Code)
	mockService.AssertExpectations(t)
}

// TestInventoryHandler_ListReconciliations_RoutePrecedence is the belt-and-
// suspenders guard that the static /reconciliations segment is routed to
// ListReconciliations and is NOT swallowed by the /:id param route (echo gives
// static segments precedence over :id; precedented by /last-purchase-prices). It
// registers both routes on a real echo router and asserts the static path
// dispatches to the queue handler.
func TestInventoryHandler_ListReconciliations_RoutePrecedence(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler
	mockService := servicemocks.NewInventoryService(t)
	handler := NewInventoryHandler(mockService)

	mockService.
		On("ListActiveReconciliations", mock.Anything, mock.Anything, mock.Anything).
		Return([]dto.SubmissionResponse{}, int64(0), nil).
		Once()
	// GetInventory (the /:id route) must NOT be hit for the static path.
	mockService.On("GetInventoryByID", mock.Anything, mock.Anything).
		Return(&models.Inventory{}, nil).Maybe()

	inventories := e.Group("/inventories")
	inventories.GET("/:id", handler.GetInventory)
	inventories.GET("/reconciliations", handler.ListReconciliations)

	req := httptest.NewRequest(http.MethodGet, "/inventories/reconciliations", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	mockService.AssertCalled(t, "ListActiveReconciliations", mock.Anything, mock.Anything, mock.Anything)
	mockService.AssertNotCalled(t, "GetInventoryByID", mock.Anything, mock.Anything)
}
