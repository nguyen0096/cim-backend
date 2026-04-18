package handlers

import (
	"cim-backend/internal/services"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

type SellingPriceHandler struct {
	sellingPriceService services.SellingPriceService
}

func NewSellingPriceHandler(sellingPriceService services.SellingPriceService) *SellingPriceHandler {
	return &SellingPriceHandler{
		sellingPriceService: sellingPriceService,
	}
}

func (h *SellingPriceHandler) CreateSellingPrice(c echo.Context) error {
	var req dto.CreateSellingPriceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := pkg.Validator.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Validation failed", "details": err.Error()})
	}

	sp, err := h.sellingPriceService.CreateSellingPrice(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create selling price", "details": err.Error()})
	}

	unlinkedCount, _ := h.sellingPriceService.CountUnlinkedPOItems(c.Request().Context(), sp.ID)

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"selling_price":           sp,
		"unlinked_po_items_count": unlinkedCount,
	})
}

func (h *SellingPriceHandler) GetSellingPrice(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	sp, err := h.sellingPriceService.GetSellingPrice(c.Request().Context(), id)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), appErr)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get selling price", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, sp)
}

func (h *SellingPriceHandler) ListByProductID(c echo.Context) error {
	productIDStr := c.QueryParam("product_id")
	if productIDStr == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "product_id query parameter is required"})
	}

	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid product_id format"})
	}

	prices, err := h.sellingPriceService.ListByProductID(c.Request().Context(), uint(productID))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list selling prices", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, prices)
}

func (h *SellingPriceHandler) UpdateSellingPrice(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	var req dto.UpdateSellingPriceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := pkg.Validator.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Validation failed", "details": err.Error()})
	}

	sp, err := h.sellingPriceService.UpdateSellingPrice(c.Request().Context(), id, req)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), appErr)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update selling price", "details": err.Error()})
	}

	unlinkedCount, _ := h.sellingPriceService.CountUnlinkedPOItems(c.Request().Context(), sp.ID)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"selling_price":           sp,
		"unlinked_po_items_count": unlinkedCount,
	})
}

func (h *SellingPriceHandler) DeleteSellingPrice(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	if err := h.sellingPriceService.DeleteSellingPrice(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete selling price", "details": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}

// BackfillPOItems links a selling price to PO items in its effective date range that don't have one yet
func (h *SellingPriceHandler) BackfillPOItems(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	count, err := h.sellingPriceService.BackfillPOItems(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to backfill", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"linked": count,
	})
}

// UpdatePOItemSellingPrice handles PUT /purchase-orders/:id/items/:itemId/selling-price
func (h *SellingPriceHandler) UpdatePOItemSellingPrice(c echo.Context) error {
	poID, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	itemIDStr := c.Param("itemId")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid item ID format"})
	}

	var req dto.UpdatePOItemSellingPriceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if req.SellingPrice.LessThanOrEqual(decimal.Zero) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "selling_price must be greater than 0"})
	}

	result, err := h.sellingPriceService.UpsertPOItemSellingPrice(c.Request().Context(), poID, uint(itemID), req.SellingPrice)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), appErr)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update selling price", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}
