package handlers

import (
	"context"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
	"import-export-backend/pkg"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type PurchaseOrderHandler struct {
	purchaseOrderRepository repository.PurchaseOrderRepository
	purchaseOrderService    PurchaseOrderService
}

//go:generate mockery --name=PurchaseOrderService --structname=PurchaseOrderService --output=./servicemocks --outpkg=servicemocks
type PurchaseOrderService interface {
	CreatePurchaseOrder(ctx context.Context, purchaseOrder *models.PurchaseOrder) error
	GetPurchaseOrderByID(id uuid.UUID) (*models.PurchaseOrder, error)
	UpdatePurchaseOrder(purchaseOrder *models.PurchaseOrder) error
	DeletePurchaseOrder(id uuid.UUID) error
	ListPurchaseOrders(limit, offset int) ([]models.PurchaseOrder, error)
	UpdatePurchaseOrderStatus(id uuid.UUID, status string) error
	ReceivePurchaseOrder(id uuid.UUID, userID string) error
	CountPurchaseOrders() (int64, error)
}

func NewPurchaseOrderHandler(
	purchaseOrderRepo repository.PurchaseOrderRepository,
	purchaseOrderService PurchaseOrderService,
) *PurchaseOrderHandler {
	return &PurchaseOrderHandler{
		purchaseOrderRepository: purchaseOrderRepo,
		purchaseOrderService:    purchaseOrderService,
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
	purchaseOrders, err := h.purchaseOrderRepository.List(limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch purchase orders"})
	}

	total, err := h.purchaseOrderRepository.Count()
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

// CreatePurchaseOrder godoc
// @Summary Create a new purchase order
// @Description Create a new purchase order with the provided details
// @Tags purchase-orders
// @Accept json
// @Produce json
// @Param purchase_order body models.PurchaseOrder true "Purchase order data"
// @Success 201 {object} models.PurchaseOrder "Successfully created purchase order"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 500 {object} map[string]string "Failed to create purchase order"
// @Router /api/purchase-orders [post]
// @Security BearerAuth
func (h *PurchaseOrderHandler) CreatePurchaseOrder(c echo.Context) error {
	var purchaseOrder models.PurchaseOrder
	if err := c.Bind(&purchaseOrder); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}

	validate := validator.New()
	if err := validate.Struct(purchaseOrder); err != nil {
		return pkg.ErrValidation("validation failed", err)
	}

	if err := h.purchaseOrderRepository.Create(c.Request().Context(), &purchaseOrder); err != nil {
		return pkg.ErrInternal("Failed to create purchase order", err)
	}

	return c.JSON(http.StatusCreated, purchaseOrder)
}

func (h *PurchaseOrderHandler) GetPurchaseOrder(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid purchase order ID"})
	}

	purchaseOrder, err := h.purchaseOrderRepository.GetByID(id)
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

	purchaseOrder.ID = &id
	if err := h.purchaseOrderRepository.Update(&purchaseOrder); err != nil {
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
