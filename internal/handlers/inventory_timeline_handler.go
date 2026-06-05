package handlers

import (
	"cim-backend/internal/services"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type InventoryTimelineHandler struct {
	inventoryTimelineService services.InventoryTimelineService
}

func NewInventoryTimelineHandler(inventoryTimelineService services.InventoryTimelineService) *InventoryTimelineHandler {
	return &InventoryTimelineHandler{
		inventoryTimelineService: inventoryTimelineService,
	}
}

func (h *InventoryTimelineHandler) GetInventoryTimeline(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	startDate := c.QueryParam("start_date")
	endDate := c.QueryParam("end_date")

	if startDate == "" || endDate == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "start_date and end_date query parameters are required"})
	}

	// Parse product_ids if provided
	var productIDs []uint
	productIDsParam := c.QueryParam("product_ids")
	if productIDsParam != "" {
		for _, idStr := range strings.Split(productIDsParam, ",") {
			idStr = strings.TrimSpace(idStr)
			if idStr == "" {
				continue
			}
			pid, err := strconv.Atoi(idStr)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid product_ids format"})
			}
			productIDs = append(productIDs, uint(pid))
		}
	}

	// Pagination + search (defaults/caps enforced in the service).
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	req := dto.InventoryTimelineRequest{
		InventoryID: id,
		StartDate:   startDate,
		EndDate:     endDate,
		ProductIDs:  productIDs,
		Search:      strings.TrimSpace(c.QueryParam("search")),
		Page:        page,
		Limit:       limit,
	}

	result, err := h.inventoryTimelineService.GetInventoryTimeline(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get inventory timeline", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}
