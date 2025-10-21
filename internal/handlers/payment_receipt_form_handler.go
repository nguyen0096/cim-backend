package handlers

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

type PaymentReceiptFormHandler struct {
	paymentReceiptFormService services.PaymentReceiptFormService
	formID                    chan uint
}

// NewPaymentReceiptFormHandler creates a new payment receipt form handler
func NewPaymentReceiptFormHandler(paymentReceiptFormService services.PaymentReceiptFormService) *PaymentReceiptFormHandler {
	return &PaymentReceiptFormHandler{
		paymentReceiptFormService: paymentReceiptFormService,
		formID:                    make(chan uint, 1),
	}
}

// CreatePaymentReceiptForm creates a new payment receipt form
// @Summary Create payment receipt form
// @Description Create a new payment receipt form
// @Tags payment-receipt-forms
// @Accept json
// @Produce json
// @Param paymentReceiptForm body dto.PaymentReceiptFormPayload true "Payment receipt form data"
// @Success 201 {object} models.PaymentReceiptForm
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /payment-receipt-forms [post]
func (h *PaymentReceiptFormHandler) CreatePaymentReceiptForm(c echo.Context) error {
	var payload dto.PaymentReceiptFormPayload
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if len(h.formID) >= 1 {
		return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "Too many requests"})
	}

	form, err := h.paymentReceiptFormService.CreatePaymentReceiptForm(c.Request().Context(), &payload)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create payment receipt form"})
	}

	h.formID <- form.ID

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
// @Param q query string false "Search term for full name, department, details, or location"
// @Param sort query string false "Sort field (full_name, department, total_amount, date, created_at, updated_at)"
// @Param order query string false "Sort direction (asc, desc, default: asc)"
// @Success 200 {object} models.PaginatedResponse[models.PaymentReceiptForm]
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /payment-receipt-forms [get]
func (h *PaymentReceiptFormHandler) ListPaymentReceiptForms(c echo.Context) error {
	// Parse pagination parameters
	var params models.ListParams
	if err := c.Bind(&params); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid query parameters"})
	}

	// Validate and set defaults
	params.ValidateAndSetDefaults()

	// Get search query
	query := c.QueryParam("q")

	var forms []models.PaymentReceiptForm
	var total int64
	var err error

	if query != "" {
		forms, total, err = h.paymentReceiptFormService.SearchPaymentReceiptForms(c.Request().Context(), query, params)
	} else {
		forms, total, err = h.paymentReceiptFormService.ListPaymentReceiptForms(c.Request().Context(), params)
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to retrieve payment receipt forms"})
	}

	// Create paginated response
	response := models.NewPaginationResult(forms, total, params.Page, params.Limit)

	return c.JSON(http.StatusOK, response)
}

// UpdatePaymentReceiptForm updates a payment receipt form
// @Summary Update payment receipt form
// @Description Update a payment receipt form by ID
// @Tags payment-receipt-forms
// @Accept json
// @Produce json
// @Param id path int true "Payment receipt form ID"
// @Param paymentReceiptForm body models.PaymentReceiptForm true "Payment receipt form data"
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

	var form models.PaymentReceiptForm
	if err := c.Bind(&form); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	form.ID = uint(id)

	if err := h.paymentReceiptFormService.UpdatePaymentReceiptForm(c.Request().Context(), &form); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update payment receipt form"})
	}

	return c.JSON(http.StatusOK, form)
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

	return c.NoContent(http.StatusNoContent)
}

// GetLatestPendingPaymentReceiptForm streams the latest payment receipt form in pending status using Server-Sent Events
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
//	data: {"status":"pending_form","data":{"id":1,"full_name":"John Doe"},"timestamp":"2024-01-15T10:30:00Z"}
//
//	event: pending_form_update
//	data: {"status":"no_pending_form","message":"No pending payment receipt form found","timestamp":"2024-01-15T10:30:00Z"}
//
//	event: keep_alive
//	data: {"status":"keep_alive","timestamp":"2024-01-15T10:30:00Z"}
//
//	event: error
//	data: {"error":"Error messzage","status":"error","timestamp":"2024-01-15T10:30:00Z"}
func (h *PaymentReceiptFormHandler) GetLatestPendingPaymentReceiptForm(c echo.Context) error {
	// Set SSE headers
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	c.Response().Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	// Create tickers for different purposes
	keepAliveTicker := time.NewTicker(30 * time.Second)
	defer keepAliveTicker.Stop()

	// Create a channel to handle client disconnect
	clientDisconnect := c.Request().Context().Done()

	// Keep connection alive and send updates
	for {
		select {
		case <-clientDisconnect:
			// Client disconnected
			return nil
		case formID := <-h.formID:
			// Send periodic updates
			if err := h.sendPendingFormUpdate(c, formID); err != nil {
				return fmt.Errorf("failed to send pending form update: %w", err)
			}
		case <-keepAliveTicker.C:
			// Send keep-alive to maintain connection
			if err := h.SendKeepAlive(c); err != nil {
				return fmt.Errorf("failed to send keep-alive: %w", err)
			}
		}
	}
}

// sendPendingFormUpdate sends the current pending form status via SSE
func (h *PaymentReceiptFormHandler) sendPendingFormUpdate(c echo.Context, formID uint) error {
	// Send form data
	eventData := map[string]interface{}{
		"status":    "pending_form",
		"form_id":   formID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	return h.sendSSEEvent(c, "pending_form_update", eventData)
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
		"status":    "keep_alive",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	return h.sendSSEEvent(c, "keep_alive", keepAliveData)
}
