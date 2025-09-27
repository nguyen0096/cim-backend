package handlers

import (
	"context"
	"import-export-backend/internal/models"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type OrderHandler struct {
	orderService OrderService
}

type OrderService interface {
	CreateOrder(order *models.Order) error
	GetOrderByID(id uint) (*models.Order, error)
	UpdateOrder(order *models.Order) error
	DeleteOrder(id uint) error
	ListOrders(limit, offset int) ([]models.Order, error)
	UpdateOrderStatus(id uint, status string) error
	CompleteOrder(ctx context.Context, id uint) error
	CountOrders() (int64, error)
}

func NewOrderHandler(orderService OrderService) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
	}
}

func (h *OrderHandler) GetOrders(c echo.Context) error {
	// Parse query parameters
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	page, _ := strconv.Atoi(c.QueryParam("page"))
	
	// Set defaults
	if limit == 0 {
		limit = 20
	}
	if page == 0 {
		page = 1
	}
	
	// Calculate offset
	offset := (page - 1) * limit

	// Get orders and total count
	orders, err := h.orderService.ListOrders(limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch orders"})
	}

	total, err := h.orderService.CountOrders()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count orders"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Create response
	response := map[string]interface{}{
		"data":       orders,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

func (h *OrderHandler) CreateOrder(c echo.Context) error {
	var order models.Order
	if err := c.Bind(&order); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := h.orderService.CreateOrder(&order); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create order"})
	}

	return c.JSON(http.StatusCreated, order)
}

func (h *OrderHandler) GetOrder(c echo.Context) error {
	idStr := c.Param("id")
	idInt, err := strconv.Atoi(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID format"})
	}
	id := uint(idInt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid order ID"})
	}

	order, err := h.orderService.GetOrderByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Order not found"})
	}

	return c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) UpdateOrder(c echo.Context) error {
	idStr := c.Param("id")
	idInt, err := strconv.Atoi(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID format"})
	}
	id := uint(idInt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid order ID"})
	}

	var order models.Order
	if err := c.Bind(&order); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	order.ID = id
	if err := h.orderService.UpdateOrder(&order); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update order"})
	}

	return c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) UpdateOrderStatus(c echo.Context) error {
	idStr := c.Param("id")
	idInt, err := strconv.Atoi(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID format"})
	}
	id := uint(idInt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid order ID"})
	}

	var req struct {
		Status string `json:"status"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := h.orderService.UpdateOrderStatus(id, req.Status); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update order status"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Order status updated successfully"})
}

func (h *OrderHandler) DeleteOrder(c echo.Context) error {
	idStr := c.Param("id")
	idInt, err := strconv.Atoi(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID format"})
	}
	id := uint(idInt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid order ID"})
	}

	if err := h.orderService.DeleteOrder(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete order"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Order deleted successfully"})
}

func (h *OrderHandler) CompleteOrder(c echo.Context) error {
	idStr := c.Param("id")
	idInt, err := strconv.Atoi(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID format"})
	}
	id := uint(idInt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid order ID"})
	}

	if err := h.orderService.CompleteOrder(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to complete order"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Order completed successfully"})
}

func (h *OrderHandler) GetOrderSummary(c echo.Context) error {
	// Implementation for order summary report
	return c.JSON(http.StatusOK, map[string]string{"message": "Order summary report"})
}
