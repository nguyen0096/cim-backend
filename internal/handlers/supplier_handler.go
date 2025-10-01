package handlers

import (
	"fmt"
	"import-export-backend/internal/models"
	"import-export-backend/internal/services"
	"import-export-backend/pkg"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type SupplierHandler struct {
	supplierService services.SupplierService
}

func NewSupplierHandler(supplierService services.SupplierService) *SupplierHandler {
	return &SupplierHandler{
		supplierService: supplierService,
	}
}

// validateEmail validates email format
func validateEmail(email string) bool {
	if email == "" {
		return true // Email is optional
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// validatePhone validates phone number format
func validatePhone(phone string) bool {
	if phone == "" {
		return true // Phone is optional
	}

	// Remove all non-digit characters for validation
	cleanPhone := regexp.MustCompile(`\D`).ReplaceAllString(phone, "")

	// Check if phone has 9-15 digits (international format)
	return len(cleanPhone) >= 9 && len(cleanPhone) <= 15
}

func (h *SupplierHandler) GetSuppliers(c echo.Context) error {
	// Parse query parameters
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	page, _ := strconv.Atoi(c.QueryParam("page"))
	sortBy := c.QueryParam("sort")
	sortOrder := c.QueryParam("order")
	status := c.QueryParam("status")

	// Set defaults
	if limit == 0 {
		limit = 20
	}
	if page == 0 {
		page = 1
	}

	if sortBy == "" {
		sortBy = "created_at"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}

	// Calculate offset
	offset := (page - 1) * limit

	// Get suppliers and total count
	suppliers, err := h.supplierService.ListSuppliers(c.Request().Context(), limit, offset, sortBy, sortOrder, status)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch suppliers"})
	}

	total, err := h.supplierService.CountSuppliers(c.Request().Context(), status)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count suppliers"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Create response
	response := map[string]interface{}{
		"data":       suppliers,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

func (h *SupplierHandler) SearchSuppliers(c echo.Context) error {
	query := c.QueryParam("q")
	if query == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Query parameter 'q' is required"})
	}

	// Parse query parameters
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	page, _ := strconv.Atoi(c.QueryParam("page"))
	sortBy := c.QueryParam("sort")
	sortOrder := c.QueryParam("order")
	status := c.QueryParam("status")

	// Set defaults
	if limit == 0 {
		limit = 20
	}
	if page == 0 {
		page = 1
	}

	// Calculate offset
	offset := (page - 1) * limit

	// Get suppliers and total count
	suppliers, err := h.supplierService.SearchSuppliersWithPagination(c.Request().Context(), query, limit, offset, sortBy, sortOrder, status)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to search suppliers"})
	}

	total, err := h.supplierService.CountSearchSuppliers(c.Request().Context(), query, status)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count search results"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Create response
	response := map[string]interface{}{
		"data":       suppliers,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

func (h *SupplierHandler) CreateSupplier(c echo.Context) error {
	var supplier models.Supplier
	if err := c.Bind(&supplier); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	// Validate required fields
	if strings.TrimSpace(supplier.Name) == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Supplier name is required"})
	}

	// Validate email format
	if !validateEmail(supplier.ContactEmail) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid email format"})
	}

	// Validate phone format
	if !validatePhone(supplier.ContactPhone) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid phone number format"})
	}

	if err := h.supplierService.CreateSupplier(c.Request().Context(), &supplier); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create supplier"})
	}

	return c.JSON(http.StatusCreated, supplier)
}

func (h *SupplierHandler) GetSupplier(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	supplier, err := h.supplierService.GetSupplierByID(c.Request().Context(), id)
	if err != nil {
		return fmt.Errorf("failed to get supplier: %w", err)
	}

	return c.JSON(http.StatusOK, supplier)
}

func (h *SupplierHandler) UpdateSupplier(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	var supplier models.Supplier
	if err := c.Bind(&supplier); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	// Validate required fields
	if strings.TrimSpace(supplier.Name) == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Supplier name is required"})
	}

	// Validate email format
	if !validateEmail(supplier.ContactEmail) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid email format"})
	}

	// Validate phone format
	if !validatePhone(supplier.ContactPhone) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid phone number format"})
	}

	supplier.ID = id
	if err := h.supplierService.UpdateSupplier(c.Request().Context(), &supplier); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update supplier"})
	}

	return c.JSON(http.StatusOK, supplier)
}

// UpdateSupplierStatus godoc
// @Summary Update supplier status
// @Description Update the status of a supplier (active, inactive)
// @Tags suppliers
// @Accept json
// @Produce json
// @Param id path int true "Supplier ID"
// @Param status body map[string]string true "Status update request"
// @Success 200 {object} map[string]interface{} "Supplier status updated successfully"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 404 {object} map[string]interface{} "Supplier not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /suppliers/{id}/status [put]
func (h *SupplierHandler) UpdateSupplierStatus(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	var request struct {
		Status string `json:"status" validate:"required,oneof=active inactive"`
	}
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := h.supplierService.UpdateSupplierStatus(c.Request().Context(), id, request.Status); err != nil {
		return fmt.Errorf("failed to update supplier status: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Supplier status updated successfully"})
}

func (h *SupplierHandler) DeleteSupplier(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	if err := h.supplierService.DeleteSupplier(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete supplier"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Supplier deleted successfully"})
}

func (h *SupplierHandler) RestoreSupplier(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	if err := h.supplierService.RestoreSupplier(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid supplier ID"})
	}

	if err := h.supplierService.RestoreSupplier(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to restore supplier"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Supplier restored successfully"})
}
