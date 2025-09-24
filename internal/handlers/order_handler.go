package handlers

import (
	"import-export-backend/internal/models"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type OrderHandler struct {
	orderService OrderService
}

type OrderService interface {
	CreateOrder(order *models.Order) error
	GetOrderByID(id uuid.UUID) (*models.Order, error)
	UpdateOrder(order *models.Order) error
	DeleteOrder(id uuid.UUID) error
	ListOrders(limit, offset int) ([]models.Order, error)
	UpdateOrderStatus(id uuid.UUID, status string) error
	CompleteOrder(id uuid.UUID, userID uuid.UUID) error
}

func NewOrderHandler(orderService OrderService) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
	}
}

func (h *OrderHandler) GetOrders(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	
	if limit == 0 {
		limit = 20
	}

	orders, err := h.orderService.ListOrders(limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch orders"})
	}

	return c.JSON(http.StatusOK, orders)
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
	id, err := uuid.Parse(c.Param("id"))
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
	id, err := uuid.Parse(c.Param("id"))
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
	id, err := uuid.Parse(c.Param("id"))
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
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid order ID"})
	}

	if err := h.orderService.DeleteOrder(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete order"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Order deleted successfully"})
}

func (h *OrderHandler) CompleteOrder(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid order ID"})
	}

	userID := c.Get("user_id").(string)
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}

	if err := h.orderService.CompleteOrder(id, userUUID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to complete order"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Order completed successfully"})
}

func (h *OrderHandler) GetOrderSummary(c echo.Context) error {
	// Implementation for order summary report
	return c.JSON(http.StatusOK, map[string]string{"message": "Order summary report"})
}
