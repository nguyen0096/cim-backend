package handlers

import (
	"import-export-backend/internal/models"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type PurchaseOrderHandler struct {
	purchaseOrderService PurchaseOrderService
}

//go:generate mockery --name=PurchaseOrderService --structname=PurchaseOrderService --output=./servicemocks --outpkg=servicemocks
type PurchaseOrderService interface {
	CreatePurchaseOrder(purchaseOrder *models.PurchaseOrder) error
	GetPurchaseOrderByID(id uuid.UUID) (*models.PurchaseOrder, error)
	UpdatePurchaseOrder(purchaseOrder *models.PurchaseOrder) error
	DeletePurchaseOrder(id uuid.UUID) error
	ListPurchaseOrders(limit, offset int) ([]models.PurchaseOrder, error)
	UpdatePurchaseOrderStatus(id uuid.UUID, status string) error
	ReceivePurchaseOrder(id uuid.UUID, userID string) error
	CountPurchaseOrders() (int64, error)
}

func NewPurchaseOrderHandler(purchaseOrderService PurchaseOrderService) *PurchaseOrderHandler {
	return &PurchaseOrderHandler{
		purchaseOrderService: purchaseOrderService,
	}
}

func (h *PurchaseOrderHandler) GetPurchaseOrders(c echo.Context) error {
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

	// Get purchase orders and total count
	purchaseOrders, err := h.purchaseOrderService.ListPurchaseOrders(limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch purchase orders"})
	}

	total, err := h.purchaseOrderService.CountPurchaseOrders()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count purchase orders"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Create response
	response := map[string]interface{}{
		"data":       purchaseOrders,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

func (h *PurchaseOrderHandler) CreatePurchaseOrder(c echo.Context) error {
	var purchaseOrder models.PurchaseOrder
	if err := c.Bind(&purchaseOrder); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := h.purchaseOrderService.CreatePurchaseOrder(&purchaseOrder); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create purchase order"})
	}

	return c.JSON(http.StatusCreated, purchaseOrder)
}

func (h *PurchaseOrderHandler) GetPurchaseOrder(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid purchase order ID"})
	}

	purchaseOrder, err := h.purchaseOrderService.GetPurchaseOrderByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Purchase order not found"})
	}

	return c.JSON(http.StatusOK, purchaseOrder)
}

func (h *PurchaseOrderHandler) UpdatePurchaseOrder(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid purchase order ID"})
	}

	var purchaseOrder models.PurchaseOrder
	if err := c.Bind(&purchaseOrder); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	purchaseOrder.ID = id
	if err := h.purchaseOrderService.UpdatePurchaseOrder(&purchaseOrder); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update purchase order"})
	}

	return c.JSON(http.StatusOK, purchaseOrder)
}

func (h *PurchaseOrderHandler) UpdatePurchaseOrderStatus(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid purchase order ID"})
	}

	var req struct {
		Status string `json:"status"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := h.purchaseOrderService.UpdatePurchaseOrderStatus(id, req.Status); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update purchase order status"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Purchase order status updated successfully"})
}

func (h *PurchaseOrderHandler) DeletePurchaseOrder(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid purchase order ID"})
	}

	if err := h.purchaseOrderService.DeletePurchaseOrder(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete purchase order"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Purchase order deleted successfully"})
}

func (h *PurchaseOrderHandler) ReceivePurchaseOrder(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid purchase order ID"})
	}

	userID := c.Get("user_id").(string)
	if userID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}

	if err := h.purchaseOrderService.ReceivePurchaseOrder(id, userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to receive purchase order"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Purchase order received successfully"})
}

func (h *PurchaseOrderHandler) GetPurchaseSummary(c echo.Context) error {
	// Implementation for purchase summary report
	return c.JSON(http.StatusOK, map[string]string{"message": "Purchase summary report"})
}
