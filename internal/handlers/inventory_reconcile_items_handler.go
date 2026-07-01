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

// CreateReconciliationItem files a new staff count-session row under a reconcile.
// @Summary Create reconciliation item
// @Description Staff submits a count session (optional row label + counted quantities, each with an optional count label) as a new in_progress row under an initiated reconcile submission. Counts must not exceed the per-item snapshot baseline.
// @Tags inventories
// @Accept json
// @Produce json
// @Param id path int true "Parent submission ID"
// @Param request body dto.CreateReconciliationItemRequest true "Row label + counted items"
// @Success 201 {object} dto.ReconciliationItemResponse
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

// ListReconciliationItems lists the live count-session rows under a reconcile.
// @Summary List reconciliation items
// @Description Returns the live count-session rows of an initiated reconcile, each with its row label and flattened count lines (inventory_item_id, quantity, count label). Staff see only their own rows; admin/accountant see all rows. Ordered by id ascending.
// @Tags inventories
// @Produce json
// @Param id path int true "Parent submission ID"
// @Success 200 {array} dto.ReconciliationItemResponse
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/submissions/{id}/reconciliation-items [get]
func (h *InventoryHandler) ListReconciliationItems(c echo.Context) error {
	submissionID, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	items, err := h.inventoryService.ListReconciliationItems(c.Request().Context(), submissionID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}

// UpdateReconciliationItem replaces the counted payload of an owned child item.
// @Summary Update reconciliation item
// @Description Staff replaces their own count session in full — the row label and the entire counted-quantities payload are overwritten.
// @Tags inventories
// @Accept json
// @Produce json
// @Param id path int true "Parent submission ID"
// @Param item_id path int true "Child item ID"
// @Param request body dto.UpdateReconciliationItemRequest true "Row label + counted items"
// @Success 200 {object} dto.ReconciliationItemResponse
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

// SetReconciliationItemReadiness toggles a staff count session's readiness.
// @Summary Set reconciliation item readiness
// @Description Staff toggles their OWN count session between in_progress and ready_for_review to signal they've finished entering counts. Staff-only and self-scoped (no admin bypass); allowed only while the parent reconciliation is open. The submission-level review_label aggregates from these per-session statuses.
// @Tags inventories
// @Accept json
// @Produce json
// @Param id path int true "Parent submission ID"
// @Param item_id path int true "Child item ID"
// @Param request body dto.SetReconciliationItemReadinessRequest true "Target session readiness"
// @Success 200 {object} dto.ReconciliationItemResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/submissions/{id}/reconciliation-items/{item_id}/review-label [post]
func (h *InventoryHandler) SetReconciliationItemReadiness(c echo.Context) error {
	submissionID, itemID, err := extractSubmissionAndItemID(c)
	if err != nil {
		return err
	}

	var req dto.SetReconciliationItemReadinessRequest
	if err := c.Bind(&req); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}
	req.SubmissionID = submissionID
	req.ItemID = itemID

	if err := pkg.Validator.Struct(req); err != nil {
		return pkg.ErrValidation(err.Error(), err)
	}

	item, err := h.inventoryService.SetReconciliationItemReadiness(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, item)
}

// DeleteReconciliationItem soft-deletes a child item.
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

// CloseReconciliation locks staff out of a reconciliation (admin/accountant).
// @Summary Close reconciliation submission
// @Description Admin/accountant closes an open reconciliation (open -> closed). Staff can no longer edit child items; admin/accountant may still edit, then start processing or reopen. The close always succeeds; if any count session was still in_progress (not marked ready for review), the response carries an advisory, non-blocking warnings list (HTTP 200).
// @Tags inventories
// @Produce json
// @Param id path int true "Submission ID"
// @Success 200 {object} dto.CloseReconciliationResult
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/submissions/{id}/close [post]
func (h *InventoryHandler) CloseReconciliation(c echo.Context) error {
	submissionID, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}
	result, err := h.inventoryService.CloseReconciliation(c.Request().Context(), submissionID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// ReopenReconciliation re-opens a closed reconciliation (admin/accountant).
// @Summary Reopen reconciliation submission
// @Description Admin/accountant reopens a closed reconciliation (closed -> open) so staff can edit child items again.
// @Tags inventories
// @Produce json
// @Param id path int true "Submission ID"
// @Success 200 {object} models.InventorySubmission
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/submissions/{id}/reopen [post]
func (h *InventoryHandler) ReopenReconciliation(c echo.Context) error {
	submissionID, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}
	submission, err := h.inventoryService.ReopenReconciliation(c.Request().Context(), submissionID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, submission)
}

// StartProcessing applies a closed reconciliation (admin/accountant). One atomic,
// advisory-locked transaction: event-based drift re-check then snapshot-aware
// apply. On drift, returns HTTP 409 with the warning-shaped payload and applies
// nothing.
// @Summary Start processing a reconciliation
// @Description Admin/accountant applies a closed reconciliation in one atomic transaction. Re-checks for a consuming submission processed during the reconciliation window; on drift it rolls back and returns a warning payload (HTTP 409). Otherwise it creates the consuming transactions (snapshot - counted) and finalizes the submission.
// @Tags inventories
// @Produce json
// @Param id path int true "Submission ID"
// @Success 200 {object} dto.StartProcessingResult
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} dto.StartProcessingResult
// @Security BearerAuth
// @Router /inventories/submissions/{id}/start-processing [post]
func (h *InventoryHandler) StartProcessing(c echo.Context) error {
	submissionID, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}
	result, err := h.inventoryService.StartProcessing(c.Request().Context(), submissionID)
	if err != nil {
		return err
	}
	// Drift: nothing was applied (rolled back). Surface the warning-shaped payload
	// with a 409 so the client distinguishes it from a successful apply.
	if result.DriftDetected {
		return c.JSON(http.StatusConflict, result)
	}
	return c.JSON(http.StatusOK, result)
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
