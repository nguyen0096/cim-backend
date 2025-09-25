package handlers

import (
	"import-export-backend/internal/models"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type ProductHandler struct {
	productService ProductService
}

type ProductService interface {
	CreateProduct(product *models.Product) error
	GetProductByID(id uuid.UUID) (*models.Product, error)
	UpdateProduct(product *models.Product) error
	DeleteProduct(id uuid.UUID) error
	RestoreProduct(id uuid.UUID) error
	ListProducts(limit, offset int, sortBy, sortOrder string) ([]models.Product, error)
	GetProductsBySupplier(supplierID uuid.UUID) ([]models.Product, error)
	SearchProducts(query string, sortBy, sortOrder string) ([]models.Product, error)
	SearchProductsWithPagination(query string, limit, offset int, sortBy, sortOrder string) ([]models.Product, error)
	CountProducts() (int64, error)
	CountSearchProducts(query string) (int64, error)
}

func NewProductHandler(productService ProductService) *ProductHandler {
	return &ProductHandler{
		productService: productService,
	}
}

func (h *ProductHandler) GetProducts(c echo.Context) error {
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

	// Get products and total count
	products, err := h.productService.ListProducts(limit, offset, sortBy, sortOrder)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch products"})
	}

	total, err := h.productService.CountProducts()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count products"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Create response
	response := map[string]interface{}{
		"data":       products,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

func (h *ProductHandler) SearchProducts(c echo.Context) error {
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

	// Get products and total count
	products, err := h.productService.SearchProductsWithPagination(query, limit, offset, sortBy, sortOrder)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to search products"})
	}

	total, err := h.productService.CountSearchProducts(query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count search results"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Create response
	response := map[string]interface{}{
		"data":       products,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

func (h *ProductHandler) CreateProduct(c echo.Context) error {
	var product models.Product
	if err := c.Bind(&product); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := h.productService.CreateProduct(&product); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create product"})
	}

	return c.JSON(http.StatusCreated, product)
}

func (h *ProductHandler) GetProduct(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid product ID"})
	}

	product, err := h.productService.GetProductByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Product not found"})
	}

	return c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) UpdateProduct(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid product ID"})
	}

	var product models.Product
	if err := c.Bind(&product); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	product.ID = id
	if err := h.productService.UpdateProduct(&product); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update product"})
	}

	return c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) DeleteProduct(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid product ID"})
	}

	if err := h.productService.DeleteProduct(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete product"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Product deleted successfully"})
}

func (h *ProductHandler) RestoreProduct(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid product ID"})
	}

	if err := h.productService.RestoreProduct(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to restore product"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Product restored successfully"})
}

func (h *ProductHandler) GetProductInventory(c echo.Context) error {
	// Implementation for getting product inventory
	return c.JSON(http.StatusOK, map[string]string{"message": "Product inventory endpoint"})
}
