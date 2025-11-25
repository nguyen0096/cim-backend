package handlers

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"cim-backend/pkg/log"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

// NotificationMessage represents a message to be sent to connected clients
type NotificationMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// ClientConnection represents a connected SSE client
type ClientConnection struct {
	ID       string
	Context  echo.Context
	SendChan chan NotificationMessage
}

func (c *ClientConnection) Close() {
	_ = c.Context.NoContent(http.StatusGone)
}

type PaymentReceiptFormHandler struct {
	paymentReceiptFormService services.PaymentReceiptFormService
	settingsService           services.SettingsService
	purchaseOrderService      services.PurchaseOrderService
	// Notification system
	clients   sync.Map
	broadcast chan NotificationMessage
}

// NewPaymentReceiptFormHandler creates a new payment receipt form handler
func NewPaymentReceiptFormHandler(paymentReceiptFormService services.PaymentReceiptFormService, purchaseOrderService services.PurchaseOrderService, settingsService services.SettingsService) *PaymentReceiptFormHandler {
	handler := &PaymentReceiptFormHandler{
		paymentReceiptFormService: paymentReceiptFormService,
		purchaseOrderService:      purchaseOrderService,
		settingsService:           settingsService,
		broadcast:                 make(chan NotificationMessage),
	}

	// Start the notification hub
	go handler.runNotificationHub()

	return handler
}

func (h *PaymentReceiptFormHandler) registerClient(client *ClientConnection) {
	h.clients.Store(client.ID, client)
	log.WithFields(logrus.Fields{
		"client_id":         client.ID,
		"total_connections": h.getConnectionCount(),
	}).Info("Client connected")
}

func (h *PaymentReceiptFormHandler) unregisterClient(clientID string) {
	if client, ok := h.clients.LoadAndDelete(clientID); ok {
		client.(*ClientConnection).Close()
	}

	log.WithFields(logrus.Fields{
		"client_id":         clientID,
		"total_connections": h.getConnectionCount(),
	}).Info("Client disconnected")
}

func (h *PaymentReceiptFormHandler) getClientConnection(clientID string) *ClientConnection {
	client, ok := h.clients.Load(clientID)
	if !ok {
		return nil
	}
	return client.(*ClientConnection)
}

// getConnectionCount returns the number of active connections
func (h *PaymentReceiptFormHandler) getConnectionCount() int {
	count := 0
	h.clients.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// runNotificationHub manages client connections and broadcasts messages
func (h *PaymentReceiptFormHandler) runNotificationHub() {
	for message := range h.broadcast {
		connectionCount := h.getConnectionCount()
		log.WithFields(logrus.Fields{
			"message_type":      message.Type,
			"total_connections": connectionCount,
		}).Info("Broadcasting message to connected clients")

		h.clients.Range(func(key, value interface{}) bool {
			client := value.(*ClientConnection)
			select {
			case client.SendChan <- message:
			default:
				// Client channel is full, skip this client
				log.WithField("client_id", client.ID).Warn("Client channel full, skipping message")
			}
			return true
		})
	}
}

// CreatePaymentReceiptForm creates a new payment receipt form
// @Summary Create payment receipt form
// @Description Create a new payment receipt form with auto-generated form number if not provided
// @Tags payment-receipt-forms
// @Accept json
// @Produce json
// @Param paymentReceiptForm body dto.PaymentReceiptFormPayload true "Payment receipt form data"
// @Success 201 {object} models.PaymentReceiptForm
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /payment-receipt-forms [post]
func (h *PaymentReceiptFormHandler) CreatePaymentReceiptForm(c echo.Context) error {
	var payload dto.PaymentReceiptFormPayload

	// Do not validate the request body here, it will be validated later
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body", "details": err.Error()})
	}

	// Check if there's already a pending form
	// pendingForm, err := h.paymentReceiptFormService.LatestPendingPaymentReceiptFormStream(c.Request().Context(), payload.PurchaseOrderID)
	// if err != nil {
	// 	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to check for pending forms"})
	// }

	// if pendingForm != nil {
	// 	return c.JSON(http.StatusConflict, map[string]string{"error": "There is already a pending payment receipt form. Please complete or cancel the existing form before creating a new one."})
	// }

	form, err := payload.ToPaymentReceiptForm()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body", "details": err.Error()})
	}

	form.Date = pkg.GetTodayDate()

	// if err := h.settingsService.GetSettingValue(c.Request().Context(), config.LastFinalizedDateSettingsKey, &form.Date); err != nil {
	// 	h.logger.WithFields(logrus.Fields{
	// 		"error":   err.Error(),
	// 		"details": "Failed to get last finalized date",
	// 	}).Error("Failed to get last finalized date")
	// 	// Continue with the current date
	// }

	form, err = h.paymentReceiptFormService.CreatePaymentReceiptForm(c.Request().Context(), form)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create payment receipt form"})
	}

	// Get the purchase order (to retrieve its order number)
	po, err := h.purchaseOrderService.GetPurchaseOrderByID(form.PurchaseOrderID)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to retrieve purchase order"})
	}
	if po != nil {
		form.PurchaseOrder = po
	}

	// Send notification to all connected clients
	notification := NotificationMessage{
		Type:      "pending_form_update",
		Data:      form,
		Timestamp: time.Now().UTC(),
	}

	// Broadcast to all connected clients
	select {
	case h.broadcast <- notification:
		log.WithFields(logrus.Fields{
			"form_id":    form.ID,
			"full_name":  form.FullName,
			"department": form.Department,
		}).Info("Form created and notification queued for broadcast")
	default:
		// Broadcast channel is full, continue without blocking
		log.Warn("Broadcast channel full, notification not sent")
	}

	return c.JSON(http.StatusCreated, form)
}

// GetPaymentReceiptForm retrieves a payment receipt form by ID
// @Summary Get payment receipt form
// @Description Get a payment receipt form by ID
// @Tags payment-receipt-forms
// @Accept json
// @Produce json
// @Param id path int true "Payment receipt form ID"
// @Success 200 {object} models.PaymentReceiptForm
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /payment-receipt-forms/{id} [get]
func (h *PaymentReceiptFormHandler) GetPaymentReceiptForm(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID format"})
	}

	form, err := h.paymentReceiptFormService.GetPaymentReceiptForm(c.Request().Context(), uint(id))
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get payment receipt form"})
	}

	if form == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Payment receipt form not found"})
	}

	return c.JSON(http.StatusOK, form)
}

// ListPaymentReceiptForms retrieves a paginated list of payment receipt forms
// @Summary List payment receipt forms
// @Description Retrieve a paginated list of payment receipt forms with optional search, sorting, and filtering
// @Tags payment-receipt-forms
// @Accept json
// @Produce json
// @Param page query int false "Page number (default: 1, minimum: 1)"
// @Param limit query int false "Number of items per page (default: 20, minimum: 1, maximum: 100)"
// @Param q query string false "Search term for full name, department, or details"
// @Param purchase_order_id query int false "Filter by purchase order ID"
// @Param inventory_id query int false "Filter by inventory ID"
// @Param statuses query []string false "Filter by statuses (pending, submitted, approved, rejected)"
// @Param sort query string false "Sort field (full_name, department, total_amount, date, created_at, updated_at)"
// @Param order query string false "Sort direction (asc, desc, default: asc)"
// @Success 200 {object} models.PaginatedResponse[models.PaymentReceiptForm]
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /payment-receipt-forms [get]
func (h *PaymentReceiptFormHandler) ListPaymentReceiptForms(c echo.Context) error {
	// Parse pagination parameters
	var req dto.PaymentReceiptFormListRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid query parameters", "details": err.Error()})
	}

	// Validate and set defaults
	req.ValidateAndSetDefaults()

	// Get search query
	query := c.QueryParam("q")

	var forms []models.PaymentReceiptForm
	var total int64
	var err error

	if query != "" {
		forms, total, err = h.paymentReceiptFormService.SearchPaymentReceiptForms(c.Request().Context(), query, &req)
	} else {
		forms, total, err = h.paymentReceiptFormService.ListPaymentReceiptForms(c.Request().Context(), &req)
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to retrieve payment receipt forms"})
	}

	// Create paginated response
	response := models.NewPaginationResult(forms, total, req.Page, req.Limit)

	return c.JSON(http.StatusOK, response)
}

// SubmitPaymentReceiptForm updates a payment receipt form (used by bot_form role)
// @Summary Submit payment receipt form
// @Description Submit a payment receipt form by ID (used by bot_form role)
// @Tags payment-receipt-forms
// @Accept json
// @Produce json
// @Param id path int true "Payment receipt form ID"
// @Param paymentReceiptForm body dto.PaymentReceiptFormPayload true "Payment receipt form data"
// @Success 200 {object} models.PaymentReceiptForm
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /payment-receipt-forms/{id}/submit [post]
func (h *PaymentReceiptFormHandler) SubmitPaymentReceiptForm(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID format"})
	}

	var req dto.PaymentReceiptFormPayload

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body", "details": err.Error()})
	}

	if err := pkg.Validator.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Request body validation failed", "details": err.Error()})
	}

	form, err := req.ToPaymentReceiptForm()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body", "details": err.Error()})
	}

	form.ID = uint(id)

	if err := h.paymentReceiptFormService.SubmitPaymentReceiptForm(c.Request().Context(), form); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		log.WithFields(logrus.Fields{
			"error":   err.Error(),
			"details": "Failed to submit payment receipt form",
		}).Error("Failed to submit payment receipt form")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to submit payment receipt form"})
	}

	return c.JSON(http.StatusOK, form)
}

// UpdatePaymentReceiptForm updates a payment receipt form (used by admin/accountant)
// @Summary Update payment receipt form
// @Description Update a payment receipt form by ID (used by admin/accountant, form still needs approval after update)
// @Tags payment-receipt-forms
// @Accept json
// @Produce json
// @Param id path int true "Payment receipt form ID"
// @Param paymentReceiptForm body dto.PaymentReceiptFormPayload true "Payment receipt form data"
// @Success 200 {object} models.PaymentReceiptForm
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /payment-receipt-forms/{id} [put]
func (h *PaymentReceiptFormHandler) UpdatePaymentReceiptForm(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID format"})
	}

	var req dto.PaymentReceiptFormPayload

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body", "details": err.Error()})
	}

	if err := pkg.Validator.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Request body validation failed", "details": err.Error()})
	}

	form, err := req.ToPaymentReceiptForm()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body", "details": err.Error()})
	}

	form.ID = uint(id)

	if err := h.paymentReceiptFormService.UpdatePaymentReceiptForm(c.Request().Context(), form); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		log.WithFields(logrus.Fields{
			"error":   err.Error(),
			"details": "Failed to update payment receipt form",
		}).Error("Failed to update payment receipt form")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update payment receipt form"})
	}

	return c.JSON(http.StatusOK, form)
}

// ApprovePaymentReceiptForm approves a payment receipt form
// @Summary Approve payment receipt form
// @Description Approve a payment receipt form by ID
// @Tags payment-receipt-forms
// @Accept json
// @Produce json
// @Param id path int true "Payment receipt form ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /payment-receipt-forms/{id}/approve [put]
func (h *PaymentReceiptFormHandler) ApprovePaymentReceiptForm(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID format"})
	}

	if err := h.paymentReceiptFormService.ApprovePaymentReceiptForm(c.Request().Context(), uint(id)); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message, "details": appErr.Cause.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to approve payment receipt form", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Payment receipt form approved successfully"})
}

// RejectPaymentReceiptForm rejects a payment receipt form
// @Summary Reject payment receipt form
// @Description Reject a payment receipt form by ID
// @Tags payment-receipt-forms
// @Accept json
// @Produce json
// @Param id path int true "Payment receipt form ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /payment-receipt-forms/{id}/reject [put]
func (h *PaymentReceiptFormHandler) RejectPaymentReceiptForm(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID format"})
	}

	if err := h.paymentReceiptFormService.RejectPaymentReceiptForm(c.Request().Context(), uint(id)); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to reject payment receipt form"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Payment receipt form rejected successfully"})
}

// DeletePaymentReceiptForm deletes a payment receipt form
// @Summary Delete payment receipt form
// @Description Delete a payment receipt form by ID
// @Tags payment-receipt-forms
// @Accept json
// @Produce json
// @Param id path int true "Payment receipt form ID"
// @Success 204 "No Content"
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /payment-receipt-forms/{id} [delete]
func (h *PaymentReceiptFormHandler) DeletePaymentReceiptForm(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID format"})
	}

	if err := h.paymentReceiptFormService.DeletePaymentReceiptForm(c.Request().Context(), uint(id)); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete payment receipt form"})
	}

	// Send notification to SSE clients if the deleted form was pending
	notification := NotificationMessage{
		Type:      "form_deleted",
		Data:      models.Base{ID: uint(id)},
		Timestamp: time.Now().UTC(),
	}

	// Broadcast to all connected clients
	select {
	case h.broadcast <- notification:
		log.WithFields(logrus.Fields{
			"form_id": id,
		}).Info("Form deleted and notification queued for broadcast")
	default:
		// Broadcast channel is full, continue without blocking
		log.Warn("Broadcast channel full, notification not sent")
	}

	return c.NoContent(http.StatusNoContent)
}

// LatestPendingPaymentReceiptFormStream streams the latest payment receipt form in pending status using Server-Sent Events
// @Summary Get latest pending payment receipt form (SSE)
// @Description Stream the latest payment receipt form in pending status using Server-Sent Events. The connection will remain open and send updates when the pending form changes. Events are sent every 5 seconds with keep-alive every 30 seconds.
// @Tags payment-receipt-forms
// @Accept json
// @Produce text/event-stream
// @Success 200 {string} string "Server-Sent Events stream"
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /payment-receipt-forms/pending [get]
//
// Example SSE Events:
//
//	event: pending_form_update
//	data: {"status":"pending_form","form_id":1,"timestamp":"2024-01-15T10:30:00Z"}
//
//	event: pending_form_update
//	data: {"status":"no_pending_form","message":"No pending payment receipt form found","timestamp":"2024-01-15T10:30:00Z"}
//
//	event: form_deleted
//	data: {"id":1,"timestamp":"2024-01-15T10:30:00Z"}
//
//	event: keep_alive
//	data: {"status":"keep_alive","timestamp":"2024-01-15T10:30:00Z"}
//
//	event: error
//	data: {"error":"Error messzage","status":"error","timestamp":"2024-01-15T10:30:00Z"}
func (h *PaymentReceiptFormHandler) LatestPendingPaymentReceiptFormStream(c echo.Context) error {
	// Set SSE headers
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	// Create a unique client ID
	clientID, err := pkg.GetUserIDFromContext(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get user ID"})
	}

	client := h.getClientConnection(clientID)
	if client != nil {
		log.WithField("client_id", clientID).Info("Another connection has been made, closing old connection")
		h.unregisterClient(clientID)
	}

	// Create client connection
	client = &ClientConnection{
		ID:       clientID,
		Context:  c,
		SendChan: make(chan NotificationMessage, 10),
	}

	h.registerClient(client)
	defer h.unregisterClient(clientID)

	// Create tickers for different purposes
	keepAliveTicker := time.NewTicker(30 * time.Second)
	defer keepAliveTicker.Stop()

	// Send initial pending form if exists
	if err := h.sendInitialPendingForm(c); err != nil {
		log.WithFields(logrus.Fields{
			"client_id": clientID,
			"error":     err.Error(),
		}).Error("Failed to send initial pending form")
		return fmt.Errorf("failed to send initial pending form: %w", err)
	}

	// Keep connection alive and send updates
	for {
		select {
		case <-c.Request().Context().Done():
			// Client disconnected
			log.WithField("client_id", clientID).Info("SSE client disconnected")
			return nil
		case notification := <-client.SendChan:
			// Send notification to client
			if err := h.sendSSEEvent(c, notification.Type, notification.Data); err != nil {
				return fmt.Errorf("failed to send notification: %w", err)
			}
		case <-keepAliveTicker.C:
			// Send keep-alive to maintain connection
			if err := h.SendKeepAlive(c); err != nil {
				return fmt.Errorf("failed to send keep-alive: %w", err)
			}
		}
	}
}

// sendSSEEvent sends a Server-Sent Event
func (h *PaymentReceiptFormHandler) sendSSEEvent(c echo.Context, eventType string, data interface{}) error {
	// Convert data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal SSE data: %w", err)
	}

	// Format as SSE event
	event := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(jsonData))

	// Send the event
	if _, err := c.Response().Write([]byte(event)); err != nil {
		return fmt.Errorf("failed to write SSE event: %w", err)
	}

	// Flush the response to ensure immediate delivery
	c.Response().Flush()

	return nil
}

// SendKeepAlive sends a keep-alive event to maintain the SSE connection
func (h *PaymentReceiptFormHandler) SendKeepAlive(c echo.Context) error {
	keepAliveData := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	return h.sendSSEEvent(c, "keep_alive", keepAliveData)
}

// sendInitialPendingForm sends the current pending form(s) if they exist
func (h *PaymentReceiptFormHandler) sendInitialPendingForm(c echo.Context) error {
	forms, err := h.paymentReceiptFormService.GetLatestPaymentReceiptForms(c.Request().Context(), 0, models.PaymentReceiptFormStatusPending, 0)
	if err != nil {
		// Send error event
		errorData := map[string]interface{}{
			"message": "Failed to get latest pending form",
			"error":   err.Error(),
		}
		return h.sendSSEEvent(c, "error", errorData)
	}

	if len(forms) == 0 {
		eventData := map[string]interface{}{
			"message": "No pending payment receipt form found",
		}
		return h.sendSSEEvent(c, "pending_form_update", eventData)
	}

	for _, form := range forms {
		if err := h.sendSSEEvent(c, "pending_form_update", form); err != nil {
			return fmt.Errorf("failed to send pending form: %w", err)
		}
	}

	return nil
}

// NotifyPaymentReceiptForm resends the initial pending form event to all connected SSE clients
// @Summary Notify payment receipt form
// @Description Resend the initial pending form event to all connected SSE clients
// @Tags payment-receipt-forms
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /payment-receipt-forms/{id}/notify [post]
func (h *PaymentReceiptFormHandler) NotifyPaymentReceiptForm(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID format"})
	}

	// Get all pending forms
	form, err := h.paymentReceiptFormService.GetPaymentReceiptForm(c.Request().Context(), uint(id))
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get payment receipt form"})
	}

	if form == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Payment receipt form not found"})
	}

	notification := NotificationMessage{
		Type:      "pending_form_update",
		Data:      form,
		Timestamp: time.Now().UTC(),
	}

	select {
	case h.broadcast <- notification:
	default:
		// Client channel is full, skip this client
		log.Warn("Broadcast channel full, notification not sent")
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Notification sent successfully"})
}
