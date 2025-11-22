package handlers

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"cim-backend/pkg/log"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type PurchaseOrderHandler struct {
	purchaseOrderRepository   repository.PurchaseOrderRepository
	purchaseOrderService      services.PurchaseOrderService
	paymentReceiptFormService services.PaymentReceiptFormService
}

func NewPurchaseOrderHandler(
	purchaseOrderRepo repository.PurchaseOrderRepository,
	purchaseOrderService services.PurchaseOrderService,
	paymentReceiptFormService services.PaymentReceiptFormService,
) *PurchaseOrderHandler {
	return &PurchaseOrderHandler{
		purchaseOrderRepository:   purchaseOrderRepo,
		purchaseOrderService:      purchaseOrderService,
		paymentReceiptFormService: paymentReceiptFormService,
	}
}

func (h *PurchaseOrderHandler) getRequestLogger(c echo.Context, operation string) *logrus.Entry {
	correlationID := c.Request().Header.Get("X-Correlation-ID")
	if correlationID == "" {
		correlationID = uuid.New().String()
	}

	return log.Logger.WithFields(logrus.Fields{
		"operation":      operation,
		"method":         c.Request().Method,
		"path":           c.Request().URL.Path,
		"correlation_id": correlationID,
	})
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
// @Success 200 {object} map[string]interface{} "Successfully retrieved purchase orders"
// @Failure 400 {object} map[string]string "Invalid request parameters"
// @Failure 500 {object} map[string]string "Failed to fetch purchase orders"
// @Router /api/purchase-orders [get]
// @Security BearerAuth
func (h *PurchaseOrderHandler) ListPurchaseOrders(c echo.Context) error {
	// Parse query parameters into pagination params
	var params models.ListParams
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
	logger := h.getRequestLogger(c, "CreatePurchaseOrder")
	var purchaseOrder models.PurchaseOrder
	if err := c.Bind(&purchaseOrder); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}

	validate := validator.New()
	if err := validate.Struct(purchaseOrder); err != nil {
		logger.WithError(err).Error("Validation failed")
		return pkg.ErrValidation("validation failed", err)
	}

	if err := h.purchaseOrderService.CreatePurchaseOrder(c.Request().Context(), &purchaseOrder); err != nil {
		// Check if error is already an AppError, return it directly
		var appErr *pkg.AppError
		if errors.As(err, &appErr) {
			return err
		}
		return pkg.ErrInternal("Failed to create purchase order", err)
	}

	return c.JSON(http.StatusCreated, purchaseOrder)
}

// UpdatePurchaseOrder godoc
// @Summary Update a purchase order
// @Description Update a purchase order's inventory, notes, and items while preserving received quantities
// @Tags purchase-orders
// @Accept json
// @Produce json
// @Param id path int true "Purchase Order ID"
// @Param purchase_order body dto.UpdatePurchaseOrderRequest true "Purchase order update data"
// @Success 200 {object} models.PurchaseOrder "Successfully updated purchase order"
// @Failure 400 {object} map[string]string "Invalid request body or validation failed"
// @Failure 404 {object} map[string]string "Purchase order not found"
// @Failure 422 {object} map[string]string "Cannot edit completed or cancelled purchase order"
// @Failure 500 {object} map[string]string "Failed to update purchase order"
// @Router /api/purchase-orders/{id} [put]
// @Security BearerAuth
func (h *PurchaseOrderHandler) UpdatePurchaseOrder(c echo.Context) error {
	logger := h.getRequestLogger(c, "UpdatePurchaseOrder")

	// Extract purchase order ID from path
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	// Parse request body
	var req dto.UpdatePurchaseOrderRequest
	if err := c.Bind(&req); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}

	// Validate request
	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		logger.WithError(err).Error("Validation failed")
		return pkg.ErrValidation("validation failed", err)
	}

	// Update purchase order
	po, err := h.purchaseOrderService.UpdatePurchaseOrder(c.Request().Context(), id, req)
	if err != nil {
		// Check if error is already an AppError, return it directly
		var appErr *pkg.AppError
		if errors.As(err, &appErr) {
			return err
		}
		return pkg.ErrInternal("Failed to update purchase order", err)
	}

	return c.JSON(http.StatusOK, po)
}

// GetPurchaseOrder godoc
// @Summary Get purchase order by ID
// @Description Retrieve a purchase order by its ID with all related items and relationships
// @Tags purchase-orders
// @Accept json
// @Produce json
// @Param id path int true "Purchase Order ID"
// @Success 200 {object} models.PurchaseOrder "Successfully retrieved purchase order"
// @Failure 400 {object} map[string]string "Invalid purchase order ID"
// @Failure 404 {object} map[string]string "Purchase order not found"
// @Failure 500 {object} map[string]string "Failed to get purchase order"
// @Router /api/purchase-orders/{id} [get]
// @Security BearerAuth
func (h *PurchaseOrderHandler) GetPurchaseOrder(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	purchaseOrder, err := h.purchaseOrderService.GetPurchaseOrderByID(id)
	if err != nil {
		// Check if error is already an AppError, return it directly
		var appErr *pkg.AppError
		if errors.As(err, &appErr) {
			return err
		}
		return pkg.ErrInternal("Failed to get purchase order", err)
	}

	return c.JSON(http.StatusOK, purchaseOrder)
}

func (h *PurchaseOrderHandler) UpdatePurchaseOrderStatus(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	var req struct {
		Status string `json:"status" validate:"required,oneof=cancelled completed"`
	}

	if err := c.Bind(&req); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}

	if err := pkg.Validator.Struct(req); err != nil {
		return pkg.ErrValidation("validation failed", err)
	}

	reqCtx := c.Request().Context()
	userRole, _ := reqCtx.Value(pkg.AuthContextKeyUserRole).(string)

	statusToActionsMap := map[string][]string{
		"cancelled": []string{"update", "cancel"},
		"completed": []string{"update", "confirm"},
	}
	allowedActions := statusToActionsMap[req.Status]
	isAllowed := false
	for _, action := range allowedActions {
		if pkg.HasPermission(reqCtx, "purchase-orders", action) {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": fmt.Sprintf("Access denied: %s role cannot change purchase order status to %s", userRole, req.Status),
		})
	}

	if err := h.purchaseOrderService.UpdatePurchaseOrderStatus(reqCtx, id, models.PurchaseOrderStatus(req.Status)); err != nil {
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

func (h *PurchaseOrderHandler) GetPurchaseSummary(c echo.Context) error {
	// Implementation for purchase summary report
	return c.JSON(http.StatusOK, map[string]string{"message": "Purchase summary report"})
}

// UpdatePurchaseOrderItemStatus godoc
// @Summary Update purchase order item status
// @Description Update the status of a specific item in a purchase order
// @Tags purchase-orders
// @Accept json
// @Produce json
// @Param id path int true "Purchase Order ID"
// @Param item_id path int true "Purchase Order Item ID"
// @Param status body object{status=string} true "Status update request"
// @Success 200 {object} dto.UpdatePurchaseOrderItemStatusResponse "Successfully updated purchase order item status"
// @Failure 400 {object} map[string]string "Invalid request parameters"
// @Failure 404 {object} map[string]string "Purchase order or item not found"
// @Failure 500 {object} map[string]string "Failed to update purchase order item status"
// @Router /api/purchase-orders/{id}/items/{item_id}/status [put]
// @Security BearerAuth
func (h *PurchaseOrderHandler) UpdatePurchaseOrderItemStatus(c echo.Context) error {
	purchaseOrderID, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	itemID, err := pkg.ExtractIDParamFromPath(c, "item_id")
	if err != nil {
		return err
	}

	var req struct {
		Status string `json:"status" validate:"required,oneof=delivering delivered"`
	}

	if err := c.Bind(&req); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}

	if err := pkg.Validator.Struct(req); err != nil {
		return pkg.ErrValidation("validation failed", err)
	}

	status := models.PurchaseOrderItemStatus(req.Status)
	response, err := h.purchaseOrderService.UpdatePurchaseOrderItemStatus(c.Request().Context(), purchaseOrderID, itemID, status)
	if err != nil {
		return pkg.ErrInternal("Failed to update purchase order item status", err)
	}

	return c.JSON(http.StatusOK, response)
}

// ReceiveInventory godoc
// @Summary Update purchase order delivery status
// @Description Update the delivery status of purchase order items by recording received quantities
// @Tags purchase-orders
// @Accept json
// @Produce json
// @Param request body dto.UpdatePurchaseOrderDeliveryStatusRequest true "Delivery status update request"
// @Success 204 "Successfully updated purchase order delivery status"
// @Failure 400 {object} map[string]string "Invalid request body or validation failed"
// @Failure 404 {object} map[string]string "Purchase order not found"
// @Failure 500 {object} map[string]string "Failed to update purchase order delivery status"
// @Router /api/purchase-orders/delivery-status [put]
// @Security BearerAuth
func (h *PurchaseOrderHandler) ReceiveInventory(c echo.Context) error {
	var req dto.UpdatePurchaseOrderDeliveryStatusRequest
	if err := c.Bind(&req); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		return pkg.ErrValidation("validation failed", err)
	}

	reqCtx := c.Request().Context()
	userRole, _ := reqCtx.Value(pkg.AuthContextKeyUserRole).(string)
	if !pkg.HasPermission(reqCtx, "purchase-orders", "confirm") && !pkg.HasPermission(reqCtx, "purchase-orders", "update") {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": fmt.Sprintf("Access denied: %s role cannot confirm or update purchase order delivery status", userRole),
		})
	}

	po, err := h.purchaseOrderService.ReceiveInventory(reqCtx, req)
	if err != nil {
		return fmt.Errorf("failed to update purchase order delivery status: %w", err)
	}

	return c.JSON(http.StatusOK, po)
}

// RetryQueueRevenueExpenseRequest godoc
// @Summary Retry queue revenue expense request
// @Description Retry queueing a revenue expense request for a payment receipt form
// @Tags purchase-orders
// @Accept json
// @Produce json
// @Param payment_receipt_form_id path int true "Payment Receipt Form ID"
// @Success 200 {object} map[string]string "Successfully queued revenue expense request"
// @Failure 400 {object} map[string]string "Invalid payment receipt form ID"
// @Failure 404 {object} map[string]string "Payment receipt form not found"
// @Failure 500 {object} map[string]string "Failed to queue revenue expense request"
// @Router /api/v1/purchase-orders/revenue-expense/{payment_receipt_form_id}/retry [post]
// @Security BearerAuth
func (h *PurchaseOrderHandler) RetryQueueRevenueExpenseRequest(c echo.Context) error {
	purchaseOrderID, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	forms, err := h.paymentReceiptFormService.GetLatestPaymentReceiptForms(c.Request().Context(), purchaseOrderID, models.PaymentReceiptFormStatusApproved, 0)
	if err != nil {
		log.WithFields(logrus.Fields{
			"operation":         "UpdatePurchaseOrderStatus",
			"purchase_order_id": purchaseOrderID,
			"error":             err,
		}).Error("Failed to get approved payment receipt forms")
		return fmt.Errorf("failed to get approved payment receipt forms: %w", err)
	}

	if len(forms) == 0 {
		log.WithFields(logrus.Fields{
			"operation":         "UpdatePurchaseOrderStatus",
			"purchase_order_id": purchaseOrderID,
		}).Warn("Cannot complete purchase order: no approved payment receipt form found")
		return pkg.ErrNoApprovedPaymentReceiptForm()
	}

	// Extract form IDs for queueing
	formIDs := make([]uint, len(forms))
	for i, form := range forms {
		formIDs[i] = form.ID
	}

	if err := h.purchaseOrderService.RetryQueueRevenueExpenseRequest(c.Request().Context(), formIDs); err != nil {
		var appErr *pkg.AppError
		if errors.As(err, &appErr) {
			return err
		}
		return pkg.ErrInternal("Failed to retry queue revenue expense request", err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Revenue expense request queued successfully. Please wait for minutes to complete the request.",
	})
}
