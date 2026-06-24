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

// These tests cover the staff reconciliation child-item handlers (epic #38,
// Part 4): path-scoping (the parent submission id + child item id come from the
// path, never the body), id validation before the service is called, and the
// success status codes (201 create / 204 delete).

func newReconItemHandler(t *testing.T) (*InventoryHandler, *servicemocks.InventoryService, *echo.Echo) {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomErrorHandler
	mockService := servicemocks.NewInventoryService(t)
	return NewInventoryHandler(mockService), mockService, e
}

// TestCreateReconciliationItem_PathScopedSubmissionID asserts the parent
// submission id always comes from the `:id` path param: the DTO is not
// JSON-bindable for it, so a body cannot retarget the parent.
func TestCreateReconciliationItem_PathScopedSubmissionID(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	const pathSubmissionID uint = 50

	mockService.
		On("CreateReconciliationItem", mock.Anything, mock.MatchedBy(func(req dto.CreateReconciliationItemRequest) bool {
			return req.SubmissionID == pathSubmissionID && len(req.Items) == 1 && req.Items[0].InventoryItemID == 10
		})).
		Return(&models.ReconciliationRequestItem{Base: models.Base{ID: 777}, SubmissionID: pathSubmissionID}, nil).
		Once()

	// Body carries a stray submission_id which must be ignored (not bindable).
	body := `{"submission_id":999,"items":[{"inventory_item_id":10,"quantity":"5"}]}`
	req, err := createRequest(http.MethodPost, "/inventories/submissions/50/reconciliation-items", body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/reconciliation-items")
	c.SetParamNames("id")
	c.SetParamValues("50")

	require.NoError(t, handler.CreateReconciliationItem(c))
	assert.Equal(t, http.StatusCreated, rec.Code)
	mockService.AssertExpectations(t)
}

func TestCreateReconciliationItem_InvalidPathID(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	body := `{"items":[{"inventory_item_id":10,"quantity":"5"}]}`
	req, _ := createRequest(http.MethodPost, "/inventories/submissions/abc/reconciliation-items", body)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/reconciliation-items")
	c.SetParamNames("id")
	c.SetParamValues("abc")

	require.Error(t, handler.CreateReconciliationItem(c))
	mockService.AssertNotCalled(t, "CreateReconciliationItem", mock.Anything, mock.Anything)
}

// TestUpdateReconciliationItem_PathScopedIDs asserts both parent and child ids
// come from the path, never the body.
func TestUpdateReconciliationItem_PathScopedIDs(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	const pathSubmissionID uint = 50
	const pathItemID uint = 777

	mockService.
		On("UpdateReconciliationItem", mock.Anything, mock.MatchedBy(func(req dto.UpdateReconciliationItemRequest) bool {
			return req.SubmissionID == pathSubmissionID && req.ItemID == pathItemID
		})).
		Return(&models.ReconciliationRequestItem{Base: models.Base{ID: pathItemID}}, nil).
		Once()

	body := `{"submission_id":1,"item_id":2,"items":[{"inventory_item_id":10,"quantity":"3"}]}`
	req, _ := createRequest(http.MethodPut, "/inventories/submissions/50/reconciliation-items/777", body)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/reconciliation-items/:item_id")
	c.SetParamNames("id", "item_id")
	c.SetParamValues("50", "777")

	require.NoError(t, handler.UpdateReconciliationItem(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	mockService.AssertExpectations(t)
}

func TestMarkReady_PassesReadyTrue(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	mockService.
		On("SetReconciliationItemReady", mock.Anything, mock.MatchedBy(func(req dto.SetReconciliationItemReadyRequest) bool {
			return req.SubmissionID == 50 && req.ItemID == 777 && req.Ready
		})).
		Return(&models.ReconciliationRequestItem{Base: models.Base{ID: 777}, Status: models.ReconciliationRequestItemStatusReady}, nil).
		Once()

	req, _ := createRequest(http.MethodPost, "/inventories/submissions/50/reconciliation-items/777/ready", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/reconciliation-items/:item_id/ready")
	c.SetParamNames("id", "item_id")
	c.SetParamValues("50", "777")

	require.NoError(t, handler.MarkReconciliationItemReady(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	mockService.AssertExpectations(t)
}

func TestMarkNotReady_PassesReadyFalse(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	mockService.
		On("SetReconciliationItemReady", mock.Anything, mock.MatchedBy(func(req dto.SetReconciliationItemReadyRequest) bool {
			return req.SubmissionID == 50 && req.ItemID == 777 && !req.Ready
		})).
		Return(&models.ReconciliationRequestItem{Base: models.Base{ID: 777}, Status: models.ReconciliationRequestItemStatusInProgress}, nil).
		Once()

	req, _ := createRequest(http.MethodPost, "/inventories/submissions/50/reconciliation-items/777/not-ready", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/reconciliation-items/:item_id/not-ready")
	c.SetParamNames("id", "item_id")
	c.SetParamValues("50", "777")

	require.NoError(t, handler.MarkReconciliationItemNotReady(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	mockService.AssertExpectations(t)
}

func TestDeleteReconciliationItem_NoContent(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	mockService.
		On("DeleteReconciliationItem", mock.Anything, mock.MatchedBy(func(req dto.DeleteReconciliationItemRequest) bool {
			return req.SubmissionID == 50 && req.ItemID == 777
		})).
		Return(nil).
		Once()

	req, _ := createRequest(http.MethodDelete, "/inventories/submissions/50/reconciliation-items/777", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/reconciliation-items/:item_id")
	c.SetParamNames("id", "item_id")
	c.SetParamValues("50", "777")

	require.NoError(t, handler.DeleteReconciliationItem(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
	mockService.AssertExpectations(t)
}

func TestDeleteReconciliationItem_InvalidItemID(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	req, _ := createRequest(http.MethodDelete, "/inventories/submissions/50/reconciliation-items/xyz", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/reconciliation-items/:item_id")
	c.SetParamNames("id", "item_id")
	c.SetParamValues("50", "xyz")

	require.Error(t, handler.DeleteReconciliationItem(c))
	mockService.AssertNotCalled(t, "DeleteReconciliationItem", mock.Anything, mock.Anything)
}
