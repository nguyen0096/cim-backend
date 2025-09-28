package handlers

import (
	"fmt"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
	"import-export-backend/internal/services"
	"import-export-backend/pkg"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type PurchaseOrderHandler struct {
	purchaseOrderRepository repository.PurchaseOrderRepository
	purchaseOrderService    services.PurchaseOrderService
}

func NewPurchaseOrderHandler(
	purchaseOrderRepo repository.PurchaseOrderRepository,
	purchaseOrderService services.PurchaseOrderService,
) *PurchaseOrderHandler {
	return &PurchaseOrderHandler{
		purchaseOrderRepository: purchaseOrderRepo,
		purchaseOrderService:    purchaseOrderService,
	}
}

// ListPurchaseOrders godoc
// @Summary List purchase orders
// @Description Retrieve a paginated list of purchase orders with optional search, sorting, and date range filtering
// @Tags purchase-orders
// @Accept json
// @Produce json
// @Param page query int false "Page number (default: 1, minimum: 1)"
// @Param limit query int false "Number of items per page (default: 20, minimum: 1, maximum: 100)"
// @Param q query string false "Search term for order number or notes"
// @Param sort query string false "Sort field (order_number, status, total_amount, created_at, updated_at)"
// @Param order query string false "Sort direction (asc, desc, default: asc)"
// @Param start_date query string false "Start date for filtering (format: YYYY-MM-DD)"
// @Param end_date query string false "End date for filtering (format: YYYY-MM-DD)"
// @Success 200 {object} models.PaginationResult[models.PurchaseOrder] "Successfully retrieved purchase orders"
// @Failure 400 {object} map[string]string "Invalid request parameters"
// @Failure 500 {object} map[string]string "Failed to fetch purchase orders"
// @Router /api/purchase-orders [get]
// @Security BearerAuth
func (h *PurchaseOrderHandler) ListPurchaseOrders(c echo.Context) error {
	// Parse query parameters into pagination params
	var params models.PaginationParams
	if err := c.Bind(&params); err != nil {
		return pkg.ErrValidation("Invalid query parameters", err)
	}

	// Get paginated purchase orders
	result, err := h.purchaseOrderService.ListPurchaseOrders(c.Request().Context(), params)
	if err != nil {
		return pkg.ErrInternal("Failed to fetch purchase orders", err)
	}

	return c.JSON(http.StatusOK, result)
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

	if err := h.purchaseOrderService.CreatePurchaseOrder(c.Request().Context(), &purchaseOrder); err != nil {
		return pkg.ErrInternal("Failed to create purchase order", err)
	}

	return c.JSON(http.StatusCreated, purchaseOrder)
}

func (h *PurchaseOrderHandler) GetPurchaseOrder(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	purchaseOrder, err := h.purchaseOrderService.GetPurchaseOrderByID(id)
	if err != nil {
		return fmt.Errorf("failed to get purchase order: %w", err)
	}

	return c.JSON(http.StatusOK, purchaseOrder)
}

func (h *PurchaseOrderHandler) UpdatePurchaseOrder(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	var purchaseOrder models.PurchaseOrder
	if err := c.Bind(&purchaseOrder); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}

	purchaseOrder.ID = id
	if err := h.purchaseOrderService.UpdatePurchaseOrder(c.Request().Context(), &purchaseOrder); err != nil {
		return fmt.Errorf("failed to update purchase order: %w", err)
	}

	return c.JSON(http.StatusOK, purchaseOrder)
}

func (h *PurchaseOrderHandler) UpdatePurchaseOrderStatus(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	var req struct {
		Status string `json:"status"`
	}

	if err := c.Bind(&req); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}

	if err := h.purchaseOrderService.UpdatePurchaseOrderStatus(c.Request().Context(), id, req.Status); err != nil {
		return fmt.Errorf("failed to update purchase order status: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Purchase order status updated successfully"})
}

func (h *PurchaseOrderHandler) DeletePurchaseOrder(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	if err := h.purchaseOrderService.DeletePurchaseOrder(id); err != nil {
		return fmt.Errorf("failed to delete purchase order: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Purchase order deleted successfully"})
}

func (h *PurchaseOrderHandler) ReceivePurchaseOrder(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	if err := h.purchaseOrderService.ReceivePurchaseOrder(c.Request().Context(), id); err != nil {
		return fmt.Errorf("failed to receive purchase order: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Purchase order received successfully"})
}

func (h *PurchaseOrderHandler) GetPurchaseSummary(c echo.Context) error {
	// Implementation for purchase summary report
	return c.JSON(http.StatusOK, map[string]string{"message": "Purchase summary report"})
}
