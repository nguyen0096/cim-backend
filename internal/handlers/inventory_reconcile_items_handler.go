package handlers

import (
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Staff reconciliation child-item endpoints (epic #38, Part 4). All routes are
// nested under the parent submission (/inventories/submissions/:id/...), so the
// parent id is taken from the `:id` path param and the child id from `:item_id`;
// a request body can never retarget either (the ids are not JSON-bindable on the
// DTOs). Errors are returned to the central CustomErrorHandler, which localizes
// the domain errors (EN/VI) and maps them to the right HTTP status.

// CreateReconciliationItem files a new staff child item under a parent reconcile.
// @Summary Create reconciliation item
// @Description Staff submits counted quantities as a new in_progress child item under an initiated reconcile submission. Counts must not exceed the per-item snapshot baseline.
// @Tags inventories
// @Accept json
// @Produce json
// @Param id path int true "Parent submission ID"
// @Param request body dto.CreateReconciliationItemRequest true "Counted items"
// @Success 201 {object} models.ReconciliationRequestItem
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/submissions/{id}/reconciliation-items [post]
func (h *InventoryHandler) CreateReconciliationItem(c echo.Context) error {
	submissionID, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	var req dto.CreateReconciliationItemRequest
	if err := c.Bind(&req); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}
	req.SubmissionID = submissionID

	if err := pkg.Validator.Struct(req); err != nil {
		return pkg.ErrValidation(err.Error(), err)
	}

	item, err := h.inventoryService.CreateReconciliationItem(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, item)
}

// UpdateReconciliationItem replaces the counted payload of an owned child item.
// @Summary Update reconciliation item
// @Description Staff replaces the counted quantities of their own child item. Editing a ready/approved row resets it to in_progress.
// @Tags inventories
// @Accept json
// @Produce json
// @Param id path int true "Parent submission ID"
// @Param item_id path int true "Child item ID"
// @Param request body dto.UpdateReconciliationItemRequest true "Counted items"
// @Success 200 {object} models.ReconciliationRequestItem
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/submissions/{id}/reconciliation-items/{item_id} [put]
func (h *InventoryHandler) UpdateReconciliationItem(c echo.Context) error {
	submissionID, itemID, err := extractSubmissionAndItemID(c)
	if err != nil {
		return err
	}

	var req dto.UpdateReconciliationItemRequest
	if err := c.Bind(&req); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}
	req.SubmissionID = submissionID
	req.ItemID = itemID

	if err := pkg.Validator.Struct(req); err != nil {
		return pkg.ErrValidation(err.Error(), err)
	}

	item, err := h.inventoryService.UpdateReconciliationItem(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, item)
}

// MarkReconciliationItemReady transitions an owned child item to ready.
// @Summary Mark reconciliation item ready
// @Description Staff marks their own in_progress child item as ready for review.
// @Tags inventories
// @Produce json
// @Param id path int true "Parent submission ID"
// @Param item_id path int true "Child item ID"
// @Success 200 {object} models.ReconciliationRequestItem
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/submissions/{id}/reconciliation-items/{item_id}/ready [post]
func (h *InventoryHandler) MarkReconciliationItemReady(c echo.Context) error {
	return h.setReconciliationItemReady(c, true)
}

// MarkReconciliationItemNotReady transitions an owned child item back to in_progress.
// @Summary Mark reconciliation item not ready
// @Description Staff moves their own ready child item back to in_progress.
// @Tags inventories
// @Produce json
// @Param id path int true "Parent submission ID"
// @Param item_id path int true "Child item ID"
// @Success 200 {object} models.ReconciliationRequestItem
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/submissions/{id}/reconciliation-items/{item_id}/not-ready [post]
func (h *InventoryHandler) MarkReconciliationItemNotReady(c echo.Context) error {
	return h.setReconciliationItemReady(c, false)
}

func (h *InventoryHandler) setReconciliationItemReady(c echo.Context, ready bool) error {
	submissionID, itemID, err := extractSubmissionAndItemID(c)
	if err != nil {
		return err
	}

	req := dto.SetReconciliationItemReadyRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Ready:        ready,
	}
	if err := pkg.Validator.Struct(req); err != nil {
		return pkg.ErrValidation(err.Error(), err)
	}

	item, err := h.inventoryService.SetReconciliationItemReady(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, item)
}

// DeleteReconciliationItem soft-deletes an owned in_progress/ready child item.
// @Summary Delete reconciliation item
// @Description Staff soft-deletes their own in_progress or ready child item. Approved/applied items cannot be deleted.
// @Tags inventories
// @Produce json
// @Param id path int true "Parent submission ID"
// @Param item_id path int true "Child item ID"
// @Success 204 "No Content"
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/submissions/{id}/reconciliation-items/{item_id} [delete]
func (h *InventoryHandler) DeleteReconciliationItem(c echo.Context) error {
	submissionID, itemID, err := extractSubmissionAndItemID(c)
	if err != nil {
		return err
	}

	req := dto.DeleteReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
	}
	if err := pkg.Validator.Struct(req); err != nil {
		return pkg.ErrValidation(err.Error(), err)
	}

	if err := h.inventoryService.DeleteReconciliationItem(c.Request().Context(), req); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// extractSubmissionAndItemID reads the path-scoped parent submission id (`:id`)
// and child item id (`:item_id`). Both must be valid uints; a bad value is a
// validation error before the service is ever called.
func extractSubmissionAndItemID(c echo.Context) (uint, uint, error) {
	submissionID, err := pkg.ExtractIDParam(c)
	if err != nil {
		return 0, 0, err
	}
	itemID, err := pkg.ExtractIDParamFromPath(c, "item_id")
	if err != nil {
		return 0, 0, err
	}
	return submissionID, itemID, nil
}
