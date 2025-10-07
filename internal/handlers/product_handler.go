package handlers

import (
	"import-export-backend/internal/models"
	"import-export-backend/internal/services"
	"import-export-backend/pkg"
	"net/http"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type ProductHandler struct {
	productService services.ProductService
	logger         *logrus.Logger
}

func NewProductHandler(productService services.ProductService, logger *logrus.Logger) *ProductHandler {
	return &ProductHandler{
		productService: productService,
		logger:         logger,
	}
}

// getRequestLogger creates a structured logger with request context
func (h *ProductHandler) getRequestLogger(c echo.Context, operation string) *logrus.Entry {
	correlationID := c.Request().Header.Get("X-Correlation-ID")
	if correlationID == "" {
		correlationID = uuid.New().String()
	}

	return h.logger.WithFields(logrus.Fields{
		"operation":      operation,
		"method":         c.Request().Method,
		"path":           c.Request().URL.Path,
		"correlation_id": correlationID,
		"user_agent":     c.Request().UserAgent(),
		"remote_addr":    c.Request().RemoteAddr,
	})
}

// GetProducts godoc
// @Summary Get all products
// @Description Get a list of all products with pagination and sorting
// @Tags products
// @Accept json
// @Produce json
// @Param limit query int false "Number of items per page" default(20)
// @Param page query int false "Page number" default(1)
// @Param sort query string false "Sort field" default("created_at")
// @Param order query string false "Sort order (asc/desc)" default("desc")
// @Param status query string false "Filter by status (active/inactive)" default("active")
// @Success 200 {object} map[string]interface{} "List of products with pagination info"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /products [get]
func (h *ProductHandler) GetProducts(c echo.Context) error {
	startTime := time.Now()
	logger := h.getRequestLogger(c, "GetProducts")

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

	logger.WithFields(logrus.Fields{
		"limit":      limit,
		"page":       page,
		"offset":     offset,
		"sort_by":    sortBy,
		"sort_order": sortOrder,
	}).Info("Getting products with pagination")

	// Get products and total count
	products, err := h.productService.ListProducts(c.Request().Context(), limit, offset, sortBy, sortOrder, status)
	if err != nil {
		logger.WithError(err).Error("Failed to fetch products from service")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch products"})
	}

	total, err := h.productService.CountProducts(c.Request().Context(), status)
	if err != nil {
		logger.WithError(err).Error("Failed to count products")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count products"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"products_count": len(products),
		"total_products": total,
		"total_pages":    totalPages,
		"duration_ms":    duration.Milliseconds(),
	}).Info("Successfully retrieved products")

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

// SearchProducts godoc
// @Summary Search products
// @Description Search products with pagination and sorting
// @Tags products
// @Accept json
// @Produce json
// @Param q query string true "Search query"
// @Param limit query int false "Number of items per page" default(20)
// @Param page query int false "Page number" default(1)
// @Param sort query string false "Sort field" default("created_at")
// @Param order query string false "Sort order (asc/desc)" default("desc")
// @Param status query string false "Filter by status (active/inactive)" default("active")
// @Success 200 {object} map[string]interface{} "Search results with pagination info"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /products/search [get]
func (h *ProductHandler) SearchProducts(c echo.Context) error {
	startTime := time.Now()
	logger := h.getRequestLogger(c, "SearchProducts")

	query := c.QueryParam("q")
	if query == "" {
		logger.Warn("Search request without query parameter")
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

	logger.WithFields(logrus.Fields{
		"query":      query,
		"limit":      limit,
		"page":       page,
		"offset":     offset,
		"sort_by":    sortBy,
		"sort_order": sortOrder,
		"status":     status,
	}).Info("Searching products")

	// Get products and total count
	products, err := h.productService.SearchProductsWithPagination(c.Request().Context(), query, limit, offset, sortBy, sortOrder, status)
	if err != nil {
		logger.WithError(err).WithField("query", query).Error("Failed to search products")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to search products"})
	}

	total, err := h.productService.CountSearchProducts(c.Request().Context(), query, status)
	if err != nil {
		logger.WithError(err).WithField("query", query).Error("Failed to count search results")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count search results"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"query":         query,
		"results_count": len(products),
		"total_results": total,
		"total_pages":   totalPages,
		"duration_ms":   duration.Milliseconds(),
	}).Info("Search completed successfully")

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

// CreateProduct godoc
// @Summary Create a new product
// @Description Create a new product with the provided information
// @Tags products
// @Accept json
// @Produce json
// @Param product body models.Product true "Product information"
// @Success 201 {object} models.Product "Created product"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /products [post]
func (h *ProductHandler) CreateProduct(c echo.Context) error {
	startTime := time.Now()
	logger := h.getRequestLogger(c, "CreateProduct")

	var product models.Product
	if err := c.Bind(&product); err != nil {
		logger.WithError(err).Error("Failed to bind request body")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body", "details": err.Error()})
	}

	logger.WithFields(logrus.Fields{
		"product_name": product.Name,
		"product_type": product.ProductType,
	}).Info("Creating new product")

	// Validate ProductType if provided
	if product.ProductType == "" {
		logger.Warn("Product creation attempted with empty product type")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Product type is required. Please specify a product type like 'electronics', 'clothing', 'food', etc."})
	}

	if err := h.productService.CreateProduct(c.Request().Context(), &product); err != nil {
		logger.WithError(err).WithField("product_name", product.Name).Error("Failed to create product in service")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create product"})
	}

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"product_id":   product.ID,
		"product_name": product.Name,
		"duration_ms":  duration.Milliseconds(),
	}).Info("Product created successfully")

	return c.JSON(http.StatusCreated, product)
}

func (h *ProductHandler) GetProduct(c echo.Context) error {
	startTime := time.Now()
	logger := h.getRequestLogger(c, "GetProduct")

	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		logger.WithError(err).Error("Invalid ID format")
		return err
	}

	logger.WithField("product_id", id).Info("Getting product by ID")

	product, err := h.productService.GetProductByID(c.Request().Context(), id)
	if err != nil {
		logger.WithError(err).WithField("product_id", id).Error("Product not found")
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Product not found"})
	}

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"product_id":   id,
		"product_name": product.Name,
		"duration_ms":  duration.Milliseconds(),
	}).Info("Product retrieved successfully")

	return c.JSON(http.StatusOK, product)
}
func (h *ProductHandler) UpdateProduct(c echo.Context) error {
	startTime := time.Now()
	logger := h.getRequestLogger(c, "UpdateProduct")

	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		logger.WithError(err).Error("Invalid ID format")
		return err
	}

	var request struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		ProductType string   `json:"product_type"`
		Status      string   `json:"status"`
		SupplierIDs []uint  `json:"supplier_ids"`
	}

	if err := c.Bind(&request); err != nil {
		logger.WithError(err).WithField("product_id", id).Error("Failed to bind request body")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	product := models.Product{
		Base: models.Base{ID: id},
		Name: request.Name,
		Description: request.Description,
		ProductType: request.ProductType,
		Status: request.Status,
	}

	// Add suppliers if IDs provided
	if len(request.SupplierIDs) > 0 {
		suppliers := make([]*models.Supplier, len(request.SupplierIDs))
		for i, sid := range request.SupplierIDs {
			suppliers[i] = &models.Supplier{Base: models.Base{ID: sid}}
		}
		product.Suppliers = suppliers
	}

	logger.WithFields(logrus.Fields{
		"product_id":    id,
		"product_name":  product.Name,
		"product_type":  product.ProductType,
		"supplier_ids":  request.SupplierIDs,
	}).Info("Updating product")

	if err := h.productService.UpdateProduct(c.Request().Context(), &product); err != nil {
		logger.WithError(err).WithField("product_id", id).Error("Failed to update product in service")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update product"})
	}

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"product_id":   id,
		"product_name": product.Name,
		"duration_ms":  duration.Milliseconds(),
	}).Info("Product updated successfully")

	return c.JSON(http.StatusOK, product)
}

// UpdateProductStatus godoc
// @Summary Update product status
// @Description Update the status of a product (active, inactive)
// @Tags products
// @Accept json
// @Produce json
// @Param id path int true "Product ID"
// @Param status body map[string]string true "Status update request"
// @Success 200 {object} map[string]interface{} "Product status updated successfully"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 404 {object} map[string]interface{} "Product not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /products/{id}/status [put]
func (h *ProductHandler) UpdateProductStatus(c echo.Context) error {
	startTime := time.Now()
	logger := h.getRequestLogger(c, "UpdateProductStatus")

	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		logger.WithError(err).Error("Invalid ID format")
		return err
	}

	var request struct {
		Status string `json:"status" validate:"required,oneof=active inactive"`
	}
	if err := c.Bind(&request); err != nil {
		logger.WithError(err).WithField("product_id", id).Error("Failed to bind request body")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	validate := validator.New()
	if err := validate.Struct(request); err != nil {
		return pkg.ErrValidation("validation failed", err)
	}

	logger.WithFields(logrus.Fields{
		"product_id": id,
		"status":     request.Status,
	}).Info("Updating product status")

	if err := h.productService.UpdateProductStatus(c.Request().Context(), id, request.Status); err != nil {
		logger.WithError(err).WithFields(logrus.Fields{
			"product_id": id,
			"status":     request.Status,
		}).Error("Failed to update product status in service")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update product status"})
	}

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"product_id":  id,
		"status":      request.Status,
		"duration_ms": duration.Milliseconds(),
	}).Info("Product status updated successfully")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":    "Product status updated successfully",
		"product_id": id,
		"status":     request.Status,
	})
}

func (h *ProductHandler) DeleteProduct(c echo.Context) error {
	startTime := time.Now()
	logger := h.getRequestLogger(c, "DeleteProduct")

	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		logger.WithError(err).Error("Invalid ID format")
		return err
	}

	logger.WithField("product_id", id).Info("Deleting product")

	if err := h.productService.DeleteProduct(c.Request().Context(), id); err != nil {
		logger.WithError(err).WithField("product_id", id).Error("Failed to delete product in service")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete product"})
	}

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"product_id":  id,
		"duration_ms": duration.Milliseconds(),
	}).Info("Product deleted successfully")

	return c.JSON(http.StatusOK, map[string]string{"message": "Product deleted successfully"})
}

func (h *ProductHandler) RestoreProduct(c echo.Context) error {
	startTime := time.Now()
	logger := h.getRequestLogger(c, "RestoreProduct")

	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		logger.WithError(err).Error("Invalid ID format")
		return err
	}

	logger.WithField("product_id", id).Info("Restoring product")

	if err := h.productService.RestoreProduct(c.Request().Context(), id); err != nil {
		logger.WithError(err).WithField("product_id", id).Error("Failed to restore product in service")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to restore product"})
	}

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"product_id":  id,
		"duration_ms": duration.Milliseconds(),
	}).Info("Product restored successfully")

	return c.JSON(http.StatusOK, map[string]string{"message": "Product restored successfully"})
}

func (h *ProductHandler) GetProductInventory(c echo.Context) error {
	startTime := time.Now()
	logger := h.getRequestLogger(c, "GetProductInventory")

	logger.Info("Getting product inventory - endpoint not yet implemented")

	duration := time.Since(startTime)
	logger.WithField("duration_ms", duration.Milliseconds()).Info("Product inventory endpoint accessed")

	return c.JSON(http.StatusOK, map[string]string{"message": "Product inventory endpoint"})
}
