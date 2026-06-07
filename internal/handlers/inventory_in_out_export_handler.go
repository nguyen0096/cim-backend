package handlers

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"cim-backend/internal/services"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
)

// InventoryInOutExportHandler handles the inventory in/out Excel export
// endpoint. It validates inputs, delegates to the service, and translates
// service errors into HTTP responses.
type InventoryInOutExportHandler struct {
	exportService services.InventoryInOutExportService
}

// NewInventoryInOutExportHandler constructs the handler.
func NewInventoryInOutExportHandler(exportService services.InventoryInOutExportService) *InventoryInOutExportHandler {
	return &InventoryInOutExportHandler{exportService: exportService}
}

// ExportInventoryInOut handles GET /api/v1/inventories/:id/export/inventory-in-out
//
// Query params:
//   - start_date  (YYYY-MM-DD, required)
//   - end_date    (YYYY-MM-DD, required)
//   - ignore_missing_selling_price (optional; "true" bypasses the
//     missing-selling-price warning)
//
// Returns 200 with { download_url, filename } on success.
// Returns 400 with structured AppError on the missing-selling-price precondition
// (when ignore_missing_selling_price is not set), where the body includes a
// "missing_selling_prices" array of { po_id, po_number }. The client warns the
// user, then re-requests with ignore_missing_selling_price=true to export
// anyway — uncomputable values render as "N/A".
func (h *InventoryInOutExportHandler) ExportInventoryInOut(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	startDate := c.QueryParam("start_date")
	endDate := c.QueryParam("end_date")
	if startDate == "" || endDate == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "start_date and end_date query parameters are required",
		})
	}

	req := dto.InventoryInOutExportRequest{
		InventoryID:               id,
		StartDate:                 startDate,
		EndDate:                   endDate,
		IgnoreMissingSellingPrice: c.QueryParam("ignore_missing_selling_price") == "true",
	}

	result, err := h.exportService.Export(c.Request().Context(), req)
	if err != nil {
		var appErr *pkg.AppError
		if errors.As(err, &appErr) {
			return c.JSON(appErr.HTTPStatus(), err)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Failed to generate inventory in/out export",
			"details": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, result)
}
