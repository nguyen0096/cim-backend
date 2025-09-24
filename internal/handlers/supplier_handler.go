package handlers

import (
	"import-export-backend/internal/models"
	"net/http"
	"strconv"

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
	ListSuppliers(limit, offset int) ([]models.Supplier, error)
	SearchSuppliers(query string) ([]models.Supplier, error)
	SearchSuppliersWithPagination(query string, limit, offset int) ([]models.Supplier, error)
	CountSuppliers() (int64, error)
	CountSearchSuppliers(query string) (int64, error)
}

func NewSupplierHandler(supplierService SupplierService) *SupplierHandler {
	return &SupplierHandler{
		supplierService: supplierService,
	}
}

func (h *SupplierHandler) GetSuppliers(c echo.Context) error {
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

	// Get suppliers and total count
	suppliers, err := h.supplierService.ListSuppliers(limit, offset)
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
	suppliers, err := h.supplierService.SearchSuppliersWithPagination(query, limit, offset)
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

	supplier.ID = id
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
