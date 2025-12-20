package handlers

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type SaleOrderHandler struct {
	saleOrderService services.SaleOrderService
}

func NewSaleOrderHandler(saleOrderService services.SaleOrderService) *SaleOrderHandler {
	return &SaleOrderHandler{
		saleOrderService: saleOrderService,
	}
}

// CreateSaleOrder creates a new sale order
// @Summary Create sale order
// @Description Create a new sale order with previousOrderId null and isLatest true by default
// @Tags sale-orders
// @Accept json
// @Produce json
// @Param sale_order body models.SaleOrder true "Sale order data"
// @Success 201 {object} models.SaleOrder
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /sale-orders [post]
func (h *SaleOrderHandler) CreateSaleOrder(c echo.Context) error {
	var saleOrder models.SaleOrder
	if err := c.Bind(&saleOrder); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}

	if err := h.saleOrderService.CreateSaleOrder(c.Request().Context(), &saleOrder); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create sale order", "details": err.Error()})
	}

	return c.JSON(http.StatusCreated, saleOrder)
}

// UpdateSaleOrder updates a sale order by creating a new version
// @Summary Update sale order
// @Description Updates a sale order by creating a new version with previousOrderId pointing to existing order
// @Tags sale-orders
// @Accept json
// @Produce json
// @Param id path int true "Sale Order ID"
// @Param sale_order body models.SaleOrder true "Sale order update data"
// @Success 200 {object} models.SaleOrder
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /sale-orders/{id} [put]
func (h *SaleOrderHandler) UpdateSaleOrder(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	var saleOrder models.SaleOrder
	if err := c.Bind(&saleOrder); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}

	updated, err := h.saleOrderService.UpdateSaleOrder(c.Request().Context(), id, &saleOrder)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update sale order", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, updated)
}

// UpdateSaleOrderStatus updates only the status of a sale order
// @Summary Update sale order status
// @Description Update only the status of a sale order without creating a new record
// @Tags sale-orders
// @Accept json
// @Produce json
// @Param id path int true "Sale Order ID"
// @Param status body object true "Status update" SchemaExample({"status": "served"})
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /sale-orders/{id}/status [put]
func (h *SaleOrderHandler) UpdateSaleOrderStatus(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	var req struct {
		Status string `json:"status" validate:"required,oneof=ordered served completed cancelled"`
	}

	if err := c.Bind(&req); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}

	if err := pkg.Validator.Struct(req); err != nil {
		return pkg.ErrValidation("validation failed", err)
	}

	if err := h.saleOrderService.UpdateSaleOrderStatus(c.Request().Context(), id, models.SaleOrderStatus(req.Status)); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update sale order status", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Sale order status updated successfully"})
}

// ListSaleOrders lists all sale orders with pagination
// @Summary List sale orders
// @Description List all sale orders with pagination and optional tag filtering
// @Tags sale-orders
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Param q query string false "Search term for order number or notes"
// @Param sort query string false "Sort field (order_number, status, created_at, updated_at, tag)"
// @Param order query string false "Sort direction (asc, desc)" default(desc)
// @Param status query string false "Filter by status (comma-separated)"
// @Param tag query int false "Filter by tag"
// @Param start_date query string false "Start date for filtering (format: YYYY-MM-DD)"
// @Param end_date query string false "End date for filtering (format: YYYY-MM-DD)"
// @Success 200 {object} models.PaginationResult[models.SaleOrder]
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /sale-orders [get]
func (h *SaleOrderHandler) ListSaleOrders(c echo.Context) error {
	var params models.ListParams
	if err := c.Bind(&params); err != nil {
		return pkg.ErrValidation("Invalid query parameters", err)
	}

	// Parse tag parameter
	var tag *int
	if tagStr := c.QueryParam("tag"); tagStr != "" {
		tagVal, err := strconv.Atoi(tagStr)
		if err != nil {
			return pkg.ErrValidation("Invalid tag parameter", err)
		}
		tag = &tagVal
	}

	result, err := h.saleOrderService.ListSaleOrders(c.Request().Context(), params, tag)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch sale orders", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// GetSaleOrder retrieves a sale order by ID
// @Summary Get sale order
// @Description Get a sale order by ID
// @Tags sale-orders
// @Accept json
// @Produce json
// @Param id path int true "Sale Order ID"
// @Success 200 {object} models.SaleOrder
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /sale-orders/{id} [get]
func (h *SaleOrderHandler) GetSaleOrder(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	saleOrder, err := h.saleOrderService.GetSaleOrderByID(c.Request().Context(), id)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get sale order", "details": err.Error()})
	}

	if saleOrder == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Sale order not found"})
	}

	return c.JSON(http.StatusOK, saleOrder)
}
