package handlers

import (
	"cim-backend/internal/middleware"
	"cim-backend/internal/mocks/servicemocks"
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestInventoryHandler_InitiateReconcile_BodyCannotOverridePathID is the
// regression test for the Codex P2 scope-escape finding: a JSON body carrying
// `inventory_id` must NOT be able to change which inventory is reconciled. The
// endpoint is path-scoped, so the service must always receive the `:id` from the
// path, regardless of any inventory_id supplied in the body.
func TestInventoryHandler_InitiateReconcile_BodyCannotOverridePathID(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := servicemocks.NewInventoryService(t)
	handler := NewInventoryHandler(mockService)

	const pathInventoryID uint = 1
	const attackerInventoryID uint = 2

	// The mock asserts the service is invoked with the PATH inventory id (1),
	// never the body's inventory_id (2). If the handler let the body override the
	// path, this expectation would fail because the arg would be 2.
	mockService.
		On("InitiateReconcile", mock.Anything, mock.MatchedBy(func(req dto.InitiateReconcileRequest) bool {
			return req.InventoryID == pathInventoryID
		})).
		Return(&models.InventorySubmission{Base: models.Base{ID: 99}, InventoryID: pathInventoryID}, nil).
		Once()

	// Malicious body attempts to retarget the reconcile to inventory 2.
	body := `{"inventory_id":2}`
	req, err := createRequest(http.MethodPost, "/inventories/1/reconcile/initiate", body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/:id/reconcile/initiate")
	c.SetParamNames("id")
	c.SetParamValues("1")

	err = handler.InitiateReconcile(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Explicit guard: the attacker id must never reach the service.
	mockService.AssertNotCalled(t, "InitiateReconcile", mock.Anything, mock.MatchedBy(func(req dto.InitiateReconcileRequest) bool {
		return req.InventoryID == attackerInventoryID
	}))
	mockService.AssertExpectations(t)
}

// TestInventoryHandler_InitiateReconcile_UsesPathID confirms the normal path:
// with no body, the service receives the inventory id from the `:id` path param.
func TestInventoryHandler_InitiateReconcile_UsesPathID(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := servicemocks.NewInventoryService(t)
	handler := NewInventoryHandler(mockService)

	const pathInventoryID uint = 7

	mockService.
		On("InitiateReconcile", mock.Anything, mock.MatchedBy(func(req dto.InitiateReconcileRequest) bool {
			return req.InventoryID == pathInventoryID
		})).
		Return(&models.InventorySubmission{Base: models.Base{ID: 1}, InventoryID: pathInventoryID}, nil).
		Once()

	req, err := createRequest(http.MethodPost, "/inventories/7/reconcile/initiate", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/:id/reconcile/initiate")
	c.SetParamNames("id")
	c.SetParamValues("7")

	err = handler.InitiateReconcile(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
	mockService.AssertExpectations(t)
}

// TestInventoryHandler_InitiateReconcile_InvalidPathID confirms a non-numeric
// path id is rejected before the service is ever called.
func TestInventoryHandler_InitiateReconcile_InvalidPathID(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	mockService := servicemocks.NewInventoryService(t)
	handler := NewInventoryHandler(mockService)

	req, err := createRequest(http.MethodPost, "/inventories/abc/reconcile/initiate", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/:id/reconcile/initiate")
	c.SetParamNames("id")
	c.SetParamValues("abc")

	err = handler.InitiateReconcile(c)
	require.Error(t, err)

	mockService.AssertNotCalled(t, "InitiateReconcile", mock.Anything, mock.Anything)
}
