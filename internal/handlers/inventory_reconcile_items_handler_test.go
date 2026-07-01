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
		Return(&dto.ReconciliationItemResponse{ID: 777, SubmissionID: pathSubmissionID}, nil).
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

// TestListReconciliationItems_PathScopedSubmissionID asserts the GET list handler
// passes the path submission id to the service and returns the row responses.
func TestListReconciliationItems_PathScopedSubmissionID(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	const pathSubmissionID uint = 50
	mockService.
		On("ListReconciliationItems", mock.Anything, pathSubmissionID).
		Return([]dto.ReconciliationItemResponse{
			{ID: 7, SubmissionID: pathSubmissionID, Label: "Morning", Status: "in_progress"},
		}, nil).
		Once()

	req, err := createRequest(http.MethodGet, "/inventories/submissions/50/reconciliation-items", nil)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/reconciliation-items")
	c.SetParamNames("id")
	c.SetParamValues("50")

	require.NoError(t, handler.ListReconciliationItems(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"label":"Morning"`)
	mockService.AssertExpectations(t)
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
		Return(&dto.ReconciliationItemResponse{ID: pathItemID}, nil).
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

// TestSetReconciliationItemReadiness_PathScopedIDsAndBody asserts the parent and
// child ids come from the path (never the body) and the status comes from the body.
func TestSetReconciliationItemReadiness_PathScopedIDsAndBody(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	const pathSubmissionID uint = 50
	const pathItemID uint = 777

	mockService.
		On("SetReconciliationItemReadiness", mock.Anything, mock.MatchedBy(func(req dto.SetReconciliationItemReadinessRequest) bool {
			return req.SubmissionID == pathSubmissionID && req.ItemID == pathItemID && req.Status == "ready_for_review"
		})).
		Return(&dto.ReconciliationItemResponse{ID: pathItemID, Status: "ready_for_review"}, nil).
		Once()

	// Body tries to retarget ids; only the path ids must be honored.
	body := `{"submission_id":1,"item_id":2,"status":"ready_for_review"}`
	req, _ := createRequest(http.MethodPost, "/inventories/submissions/50/reconciliation-items/777/review-label", body)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/reconciliation-items/:item_id/review-label")
	c.SetParamNames("id", "item_id")
	c.SetParamValues("50", "777")

	require.NoError(t, handler.SetReconciliationItemReadiness(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	mockService.AssertExpectations(t)
}

// TestSetReconciliationItemReadiness_RejectsInvalidStatus asserts an out-of-enum
// status is a validation error before the service is ever called.
func TestSetReconciliationItemReadiness_RejectsInvalidStatus(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	body := `{"status":"approved"}`
	req, _ := createRequest(http.MethodPost, "/inventories/submissions/50/reconciliation-items/777/review-label", body)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/reconciliation-items/:item_id/review-label")
	c.SetParamNames("id", "item_id")
	c.SetParamValues("50", "777")

	require.Error(t, handler.SetReconciliationItemReadiness(c))
	mockService.AssertNotCalled(t, "SetReconciliationItemReadiness", mock.Anything, mock.Anything)
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

// --- Admin reconciliation management handlers (epic #38, Part 6 redesign) ---

func TestCloseReconciliation_PathScopedID(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	mockService.
		On("CloseReconciliation", mock.Anything, uint(50)).
		Return(&dto.CloseReconciliationResult{
			Submission: &models.InventorySubmission{Base: models.Base{ID: 50}, ReconcileStatus: models.ReconcileLifecycleStatusClosed},
		}, nil).
		Once()

	req, _ := createRequest(http.MethodPost, "/inventories/submissions/50/close", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/close")
	c.SetParamNames("id")
	c.SetParamValues("50")

	require.NoError(t, handler.CloseReconciliation(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	mockService.AssertExpectations(t)
}

func TestReopenReconciliation_PathScopedID(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	mockService.
		On("ReopenReconciliation", mock.Anything, uint(50)).
		Return(&models.InventorySubmission{Base: models.Base{ID: 50}, ReconcileStatus: models.ReconcileLifecycleStatusOpen}, nil).
		Once()

	req, _ := createRequest(http.MethodPost, "/inventories/submissions/50/reopen", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/reopen")
	c.SetParamNames("id")
	c.SetParamValues("50")

	require.NoError(t, handler.ReopenReconciliation(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	mockService.AssertExpectations(t)
}

func TestCancelReconciliation_PathScopedID(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	mockService.
		On("CancelReconciliation", mock.Anything, uint(50)).
		Return(&models.InventorySubmission{
			Base:             models.Base{ID: 50},
			ReconcileStatus:  models.ReconcileLifecycleStatusCanceled,
			ProcessingStatus: models.InventorySubmissionStatusCanceled,
		}, nil).
		Once()

	req, _ := createRequest(http.MethodPost, "/inventories/submissions/50/cancel", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/cancel")
	c.SetParamNames("id")
	c.SetParamValues("50")

	require.NoError(t, handler.CancelReconciliation(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	mockService.AssertExpectations(t)
}

// TestStartProcessing_Success returns 200 with the finalized submission.
func TestStartProcessing_Success(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	mockService.
		On("StartProcessing", mock.Anything, uint(50)).
		Return(&dto.StartProcessingResult{Submission: &models.InventorySubmission{Base: models.Base{ID: 50}}}, nil).
		Once()

	req, _ := createRequest(http.MethodPost, "/inventories/submissions/50/start-processing", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/start-processing")
	c.SetParamNames("id")
	c.SetParamValues("50")

	require.NoError(t, handler.StartProcessing(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	mockService.AssertExpectations(t)
}

// TestStartProcessing_Drift returns 409 with the warning-shaped payload.
func TestStartProcessing_Drift(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	mockService.
		On("StartProcessing", mock.Anything, uint(50)).
		Return(&dto.StartProcessingResult{DriftDetected: true, Warnings: []string{"drift"}}, nil).
		Once()

	req, _ := createRequest(http.MethodPost, "/inventories/submissions/50/start-processing", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/start-processing")
	c.SetParamNames("id")
	c.SetParamValues("50")

	require.NoError(t, handler.StartProcessing(c))
	assert.Equal(t, http.StatusConflict, rec.Code)
	mockService.AssertExpectations(t)
}

func TestStartProcessing_InvalidPathID(t *testing.T) {
	handler, mockService, e := newReconItemHandler(t)

	req, _ := createRequest(http.MethodPost, "/inventories/submissions/abc/start-processing", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/inventories/submissions/:id/start-processing")
	c.SetParamNames("id")
	c.SetParamValues("abc")

	require.Error(t, handler.StartProcessing(c))
	mockService.AssertNotCalled(t, "StartProcessing", mock.Anything, mock.Anything)
}
