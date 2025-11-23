package handlers

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"cim-backend/pkg/log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

// ProductListRequest represents the query parameters for listing products
type ProductListRequest struct {
	Limit       int    `query:"limit"`
	Page        int    `query:"page"`
	Sort        string `query:"sort"`
	Order       string `query:"order"`
	Status      string `query:"status"`
	ProductType string `query:"product_type"`
	SupplierID  uint   `query:"supplier_id"`
}

// ProductSearchRequest represents the query parameters for searching products
type ProductSearchRequest struct {
	ProductListRequest
	Query string `query:"q" validate:"required"`
}

// SetDefaults sets default values for the request
func (r *ProductListRequest) SetDefaults() {
	if r.Limit == 0 {
		r.Limit = 20
	}
	if r.Page == 0 {
		r.Page = 1
	}
	if r.Sort == "" {
		r.Sort = "created_at"
	}
	if r.Order == "" {
		r.Order = "desc"
	}
}

// SetDefaults sets default values for the request
func (r *ProductSearchRequest) SetDefaults() {
	if r.Limit == 0 {
		r.Limit = 20
	}
	if r.Page == 0 {
		r.Page = 1
	}
	if r.Sort == "" {
		r.Sort = "created_at"
	}
	if r.Order == "" {
		r.Order = "desc"
	}
}

type ProductHandler struct {
	productService services.ProductService
}

func NewProductHandler(productService services.ProductService) *ProductHandler {
	return &ProductHandler{
		productService: productService,
	}
}

// getRequestLogger creates a structured logger with request context
func (h *ProductHandler) getRequestLogger(c echo.Context, operation string) *logrus.Entry {
	correlationID := c.Request().Header.Get("X-Correlation-ID")
	if correlationID == "" {
		correlationID = uuid.New().String()
	}

	return log.Logger.WithFields(logrus.Fields{
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
// @Param product_type query string false "Filter by product type (electronics, clothing, food, etc.)"
// @Param supplier_id query int false "Filter by supplier ID"
// @Success 200 {object} map[string]interface{} "List of products with pagination info"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /products [get]
func (h *ProductHandler) GetProducts(c echo.Context) error {
	startTime := time.Now()
	logger := h.getRequestLogger(c, "GetProducts")

	// Parse query parameters into request struct
	var request ProductListRequest
	if err := c.Bind(&request); err != nil {
		logger.WithError(err).Error("Failed to bind query parameters")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid query parameters"})
	}

	// Set default values
	request.SetDefaults()

	// Calculate offset
	offset := (request.Page - 1) * request.Limit

	logger.WithFields(logrus.Fields{
		"limit":        request.Limit,
		"page":         request.Page,
		"offset":       offset,
		"sort_by":      request.Sort,
		"sort_order":   request.Order,
		"status":       request.Status,
		"product_type": request.ProductType,
		"supplier_id":  request.SupplierID,
	}).Info("Getting products with pagination")

	// Get products and total count
	products, err := h.productService.ListProducts(c.Request().Context(), request.Limit, offset, request.Sort, request.Order, request.Status, request.ProductType, request.SupplierID)
	if err != nil {
		logger.WithError(err).Error("Failed to fetch products from service")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch products"})
	}

	total, err := h.productService.CountProducts(c.Request().Context(), request.Status, request.ProductType, request.SupplierID)
	if err != nil {
		logger.WithError(err).Error("Failed to count products")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count products"})
	}

	// Calculate total pages
	totalPages := int((total + int64(request.Limit) - 1) / int64(request.Limit))

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
		"page":       request.Page,
		"limit":      request.Limit,
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
// @Param product_type query string false "Filter by product type (electronics, clothing, food, etc.)"
// @Param supplier_id query int false "Filter by supplier ID"
// @Success 200 {object} map[string]interface{} "Search results with pagination info"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /products/search [get]
func (h *ProductHandler) SearchProducts(c echo.Context) error {
	startTime := time.Now()
	logger := h.getRequestLogger(c, "SearchProducts")

	// Parse query parameters into request struct
	var request ProductSearchRequest
	if err := c.Bind(&request); err != nil {
		logger.WithError(err).Error("Failed to bind query parameters")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid query parameters"})
	}

	// Validate required query parameter
	if request.Query == "" {
		logger.Warn("Search request without query parameter")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Query parameter 'q' is required"})
	}

	// Set default values
	request.SetDefaults()

	// Calculate offset
	offset := (request.Page - 1) * request.Limit

	logger.WithFields(logrus.Fields{
		"query":        request.Query,
		"limit":        request.Limit,
		"page":         request.Page,
		"offset":       offset,
		"sort_by":      request.Sort,
		"sort_order":   request.Order,
		"status":       request.Status,
		"product_type": request.ProductType,
		"supplier_id":  request.SupplierID,
	}).Info("Searching products")

	// Get products and total count
	products, err := h.productService.SearchProductsWithPagination(c.Request().Context(), request.Query, request.Limit, offset, request.Sort, request.Order, request.Status, request.ProductType, request.SupplierID)
	if err != nil {
		logger.WithError(err).WithField("query", request.Query).Error("Failed to search products")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to search products"})
	}

	total, err := h.productService.CountSearchProducts(c.Request().Context(), request.Query, request.Status, request.ProductType, request.SupplierID)
	if err != nil {
		logger.WithError(err).WithField("query", request.Query).Error("Failed to count search results")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count search results"})
	}

	// Calculate total pages
	totalPages := int((total + int64(request.Limit) - 1) / int64(request.Limit))

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"query":         request.Query,
		"results_count": len(products),
		"total_results": total,
		"total_pages":   totalPages,
		"duration_ms":   duration.Milliseconds(),
	}).Info("Search completed successfully")

	// Create response
	response := map[string]interface{}{
		"data":       products,
		"total":      total,
		"page":       request.Page,
		"limit":      request.Limit,
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

	var request struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ProductType string `json:"product_type"`
		UnitID      uint   `json:"unit_id"`
		Status      string `json:"status"`
		SupplierIDs []uint `json:"supplier_ids"`
	}

	if err := c.Bind(&request); err != nil {
		logger.WithError(err).Error("Failed to bind request body")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body", "details": err.Error()})
	}

	logger.WithFields(logrus.Fields{
		"product_name": request.Name,
		"product_type": request.ProductType,
		"unit_id":      request.UnitID,
		"supplier_ids": request.SupplierIDs,
	}).Info("Creating new product")

	// Validate ProductType if provided
	if request.ProductType == "" {
		logger.Warn("Product creation attempted with empty product type")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Product type is required. Please specify a product type like 'electronics', 'clothing', 'food', etc."})
	}
	if request.UnitID == 0 {
		return pkg.ErrValidation("unit_id is required", nil)
	}

	product := models.Product{
		Name:        request.Name,
		Description: request.Description,
		ProductType: request.ProductType,
		UnitID:      request.UnitID,
		Status:      request.Status,
	}

	if request.UnitID == 0 {
		return pkg.ErrValidation("unit_id is required", nil)
	}

	// Add suppliers if IDs provided
	if len(request.SupplierIDs) > 0 {
		suppliers := make([]*models.Supplier, len(request.SupplierIDs))
		for i, sid := range request.SupplierIDs {
			suppliers[i] = &models.Supplier{Base: models.Base{ID: sid}}
		}
		product.Suppliers = suppliers
	}

	if err := h.productService.CreateProduct(c.Request().Context(), &product); err != nil {
		logger.WithError(err).WithField("product_name", request.Name).Error("Failed to create product in service")
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
		Name        string `json:"name" validate:"required"`
		Description string `json:"description"`
		ProductType string `json:"product_type" validate:"required"`
		UnitID      uint   `json:"unit_id" validate:"required"`
		Status      string `json:"status" validate:"required,oneof=active inactive"`
		SupplierIDs []uint `json:"supplier_ids"`
	}

	if err := c.Bind(&request); err != nil {
		logger.WithError(err).WithField("product_id", id).Error("Failed to bind request body")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := pkg.Validator.Struct(request); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Validation failed", "details": err.Error()})
	}

	product := models.Product{
		Base:        models.Base{ID: id},
		Name:        request.Name,
		Description: request.Description,
		ProductType: request.ProductType,
		UnitID:      request.UnitID,
		Status:      request.Status,
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
		"product_id":   id,
		"product_name": product.Name,
		"product_type": product.ProductType,
		"supplier_ids": request.SupplierIDs,
	}).Info("Updating product")

	if err := h.productService.UpdateProduct(c.Request().Context(), &product); err != nil {
		logger.WithError(err).WithField("product_id", id).Error("Failed to update product in service")
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
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

// ImportProductsCSV godoc
// @Summary Import products with suppliers from CSV or Excel
// @Description Import products and their suppliers from a CSV or Excel file upload. Files must have headers: Name, Description, ProductType, Unit, Suppliers, ContactEmail, ContactPhone, Address. CSV uses semicolon (;) as delimiter. Products can have multiple suppliers by repeating rows with empty product fields.
// @Tags products
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "CSV or Excel file to upload"
// @Success 200 {object} map[string]interface{} "Import successful"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /products/import-csv [post]
func (h *ProductHandler) ImportProductsCSV(c echo.Context) error {
	startTime := time.Now()
	logger := h.getRequestLogger(c, "ImportProductsCSV")

	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		logger.WithError(err).Error("No file uploaded")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No file uploaded"})
	}

	logger.WithFields(logrus.Fields{
		"filename": file.Filename,
		"size":     file.Size,
	}).Info("Processing file upload")

	// Validate file type
	if !pkg.IsAllowedFileTypes(file.Filename) {
		logger.WithField("filename", file.Filename).Warn("Invalid file type")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "File must be a CSV or Excel file (.csv, .xlsx, .xls)"})
	}

	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		logger.WithError(err).Error("Failed to open uploaded file")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to open file"})
	}
	defer src.Close()

	// Import products based on file type
	var count int
	if pkg.IsCSVFile(file.Filename) {
		logger.Info("Importing from CSV file")
		count, err = h.productService.ImportProductsFromCSV(c.Request().Context(), src)
	} else if pkg.IsExcelFile(file.Filename) {
		logger.Info("Importing from Excel file")
		count, err = h.productService.ImportProductsFromExcel(c.Request().Context(), src)
	} else {
		logger.WithField("filename", file.Filename).Error("Unexpected file type passed validation")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Unsupported file format"})
	}

	if err != nil {
		logger.WithError(err).Error("Failed to import products")
		// Check if it's an AppError
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to import products"})
	}

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"imported_count": count,
		"file_type":      filepath.Ext(file.Filename),
		"duration_ms":    duration.Milliseconds(),
	}).Info("Products imported successfully")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Products imported successfully",
		"count":   count,
	})
}

// ExportProductsCSV godoc
// @Summary Export products to CSV
// @Description Export products to CSV file with the same format as the import template. CSV uses semicolon (;) as delimiter. Products with multiple suppliers will have one row per supplier.
// @Tags products
// @Accept json
// @Produce text/csv
// @Param status query string false "Filter by status (active/inactive)"
// @Param product_type query string false "Filter by product type"
// @Param supplier_id query int false "Filter by supplier ID"
// @Success 200 {file} file "CSV file download"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /products/export-csv [get]
func (h *ProductHandler) ExportProductsCSV(c echo.Context) error {
	startTime := time.Now()
	logger := h.getRequestLogger(c, "ExportProductsCSV")

	// Parse query parameters
	var request ProductListRequest
	if err := c.Bind(&request); err != nil {
		logger.WithError(err).Error("Failed to bind query parameters")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid query parameters"})
	}

	logger.WithFields(logrus.Fields{
		"status":       request.Status,
		"product_type": request.ProductType,
		"supplier_id":  request.SupplierID,
	}).Info("Exporting products to CSV")

	// Set response headers for CSV download
	c.Response().Header().Set("Content-Type", "text/csv; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=products_export.csv")

	// Export products to CSV
	if err := h.productService.ExportProductsToCSV(c.Request().Context(), c.Response().Writer, request.Status, request.ProductType, request.SupplierID); err != nil {
		logger.WithError(err).Error("Failed to export products to CSV")
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to export products"})
	}

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"duration_ms": duration.Milliseconds(),
	}).Info("Products exported successfully")

	return nil
}

// ExportProductsExcel godoc
// @Summary Export products to Excel
// @Description Export products to Excel file with the same format as the import template. Products with multiple suppliers will have one row per supplier.
// @Tags products
// @Accept json
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param status query string false "Filter by status (active/inactive)"
// @Param product_type query string false "Filter by product type"
// @Param supplier_id query int false "Filter by supplier ID"
// @Success 200 {file} file "Excel file download"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /products/export-excel [get]
func (h *ProductHandler) ExportProductsExcel(c echo.Context) error {
	startTime := time.Now()
	logger := h.getRequestLogger(c, "ExportProductsExcel")

	// Parse query parameters
	var request ProductListRequest
	if err := c.Bind(&request); err != nil {
		logger.WithError(err).Error("Failed to bind query parameters")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid query parameters"})
	}

	logger.WithFields(logrus.Fields{
		"status":       request.Status,
		"product_type": request.ProductType,
		"supplier_id":  request.SupplierID,
	}).Info("Exporting products to Excel")

	// Set response headers for Excel download
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=products_export.xlsx")

	// Export products to Excel
	if err := h.productService.ExportProductsToExcel(c.Request().Context(), c.Response().Writer, request.Status, request.ProductType, request.SupplierID); err != nil {
		logger.WithError(err).Error("Failed to export products to Excel")
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to export products"})
	}

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"duration_ms": duration.Milliseconds(),
	}).Info("Products exported successfully")

	return nil
}
