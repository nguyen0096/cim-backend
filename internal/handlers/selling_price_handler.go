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

	sp, applying, err := h.sellingPriceService.CreateSellingPriceWithApplying(c.Request().Context(), req)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), appErr)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create selling price", "details": err.Error()})
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"selling_price":    sp,
		"massive_applying": applying,
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

	sp, applying, err := h.sellingPriceService.UpdateSellingPriceWithApplying(c.Request().Context(), id, req)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), appErr)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update selling price", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"selling_price":    sp,
		"massive_applying": applying,
	})
}

func (h *SellingPriceHandler) DeleteSellingPrice(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	applying, err := h.sellingPriceService.DeleteSellingPriceWithApplying(c.Request().Context(), id)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), appErr)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete selling price", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"massive_applying": applying,
	})
}

// BackfillPOItems re-points PO items to the start selling price (:id) across its
// server-resolved effective range. The body's end_effective_from asserts the
// previewed exclusive end date (null = open-ended); a mismatch returns 409. Runs
// in a single transaction.
// @Summary Apply a selling price to PO items in its effective range
// @Description Re-point PO items to the start selling price (:id) across its server-resolved effective range. end_effective_from ("YYYY-MM-DD") must match the previewed boundary date (null = open-ended) or a 409 is returned.
// @Tags selling-prices
// @Accept json
// @Produce json
// @Param id path int true "Start selling price ID"
// @Param body body dto.BackfillSellingPriceRequest false "Previewed end-of-range boundary date (optimistic-concurrency assertion)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /selling-prices/{id}/backfill [post]
func (h *SellingPriceHandler) BackfillPOItems(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	var req dto.BackfillSellingPriceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	count, err := h.sellingPriceService.ApplyMassiveLinks(c.Request().Context(), id, req.EndEffectiveFrom)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), appErr)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to backfill", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"applied": count,
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

	if req.SellingPrice == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "selling_price is required"})
	}

	if req.SellingPrice.LessThan(decimal.Zero) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "selling_price must not be negative"})
	}

	result, err := h.sellingPriceService.UpsertPOItemSellingPrice(c.Request().Context(), poID, uint(itemID), *req.SellingPrice)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), appErr)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update selling price", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}
