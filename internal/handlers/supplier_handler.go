package handlers

import (
	"import-export-backend/internal/models"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type SupplierHandler struct {
	supplierService SupplierService
}

type SupplierService interface {
	CreateSupplier(supplier *models.Supplier) error
	GetSupplierByID(id uuid.UUID) (*models.Supplier, error)
	UpdateSupplier(supplier *models.Supplier) error
	DeleteSupplier(id uuid.UUID) error
	RestoreSupplier(id uuid.UUID) error
	ListSuppliers(limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error)
	SearchSuppliers(query string, sortBy, sortOrder string) ([]models.Supplier, error)
	SearchSuppliersWithPagination(query string, limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error)
	CountSuppliers() (int64, error)
	CountSearchSuppliers(query string) (int64, error)
}

func NewSupplierHandler(supplierService SupplierService) *SupplierHandler {
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
	suppliers, err := h.supplierService.ListSuppliers(limit, offset, sortBy, sortOrder)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch suppliers"})
	}

	total, err := h.supplierService.CountSuppliers()
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
	suppliers, err := h.supplierService.SearchSuppliersWithPagination(query, limit, offset, sortBy, sortOrder)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to search suppliers"})
	}

	total, err := h.supplierService.CountSearchSuppliers(query)
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

	if err := h.supplierService.CreateSupplier(&supplier); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create supplier"})
	}

	return c.JSON(http.StatusCreated, supplier)
}

func (h *SupplierHandler) GetSupplier(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid supplier ID"})
	}

	supplier, err := h.supplierService.GetSupplierByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Supplier not found"})
	}

	return c.JSON(http.StatusOK, supplier)
}

func (h *SupplierHandler) UpdateSupplier(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid supplier ID"})
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

	supplier.ID = &id
	if err := h.supplierService.UpdateSupplier(&supplier); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update supplier"})
	}

	return c.JSON(http.StatusOK, supplier)
}

func (h *SupplierHandler) DeleteSupplier(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid supplier ID"})
	}

	if err := h.supplierService.DeleteSupplier(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete supplier"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Supplier deleted successfully"})
}

func (h *SupplierHandler) RestoreSupplier(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid supplier ID"})
	}

	if err := h.supplierService.RestoreSupplier(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to restore supplier"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Supplier restored successfully"})
}
