package handlers

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type InventoryItemHandler struct {
	inventoryItemService services.InventoryItemService
}

func NewInventoryItemHandler(inventoryItemService services.InventoryItemService) *InventoryItemHandler {
	return &InventoryItemHandler{
		inventoryItemService: inventoryItemService,
	}
}

// CreateInventoryItem creates a new inventory item
// @Summary Create inventory item
// @Description Create a new inventory item
// @Tags inventory-items
// @Accept json
// @Produce json
// @Param id path int true "Inventory ID"
// @Param inventoryItem body models.InventoryItem true "Inventory item data"
// @Success 201 {object} models.InventoryItem
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/{id}/inventory-items [post]
func (h *InventoryItemHandler) CreateInventoryItem(c echo.Context) error {
	inventoryID, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	var item models.InventoryItem
	if err := c.Bind(&item); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	// Set the inventory ID from the URL parameter
	item.InventoryID = inventoryID

	if err := h.inventoryItemService.CreateInventoryItem(c.Request().Context(), &item); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create inventory item"})
	}

	return c.JSON(http.StatusCreated, item)
}

// GetInventoryItem retrieves an inventory item by ID
// @Summary Get inventory item
// @Description Get an inventory item by ID
// @Tags inventory-items
// @Accept json
// @Produce json
// @Param id path int true "Inventory ID"
// @Param item_id path int true "Inventory item ID"
// @Success 200 {object} models.InventoryItem
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/{id}/inventory-items/{item_id} [get]
func (h *InventoryItemHandler) GetInventoryItem(c echo.Context) error {
	itemIDStr := c.Param("item_id")
	itemID, err := strconv.ParseUint(itemIDStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid item ID"})
	}

	item, err := h.inventoryItemService.GetInventoryItemByID(c.Request().Context(), uint(itemID))
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get inventory item"})
	}

	return c.JSON(http.StatusOK, item)
}

// UpdateInventoryItem updates an inventory item
// @Summary Update inventory item
// @Description Update an inventory item
// @Tags inventory-items
// @Accept json
// @Produce json
// @Param id path int true "Inventory ID"
// @Param item_id path int true "Inventory item ID"
// @Param inventoryItem body models.InventoryItem true "Inventory item data"
// @Success 200 {object} models.InventoryItem
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/{id}/inventory-items/{item_id} [put]
func (h *InventoryItemHandler) UpdateInventoryItem(c echo.Context) error {
	inventoryID, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	itemIDStr := c.Param("item_id")
	itemID, err := strconv.ParseUint(itemIDStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid item ID"})
	}

	var item models.InventoryItem
	if err := c.Bind(&item); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	// Set IDs from URL parameters
	item.ID = uint(itemID)
	item.InventoryID = inventoryID

	if err := h.inventoryItemService.UpdateInventoryItem(c.Request().Context(), &item); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update inventory item"})
	}

	return c.JSON(http.StatusOK, item)
}

// DeleteInventoryItem deletes an inventory item
// @Summary Delete inventory item
// @Description Delete an inventory item
// @Tags inventory-items
// @Accept json
// @Produce json
// @Param id path int true "Inventory ID"
// @Param item_id path int true "Inventory item ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/{id}/inventory-items/{item_id} [delete]
func (h *InventoryItemHandler) DeleteInventoryItem(c echo.Context) error {
	itemIDStr := c.Param("item_id")
	itemID, err := strconv.ParseUint(itemIDStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid item ID"})
	}

	if err := h.inventoryItemService.DeleteInventoryItem(c.Request().Context(), uint(itemID)); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete inventory item"})
	}

	return c.NoContent(http.StatusNoContent)
}

// ListInventoryItems lists all inventory items with pagination
// @Summary List inventory items
// @Description List all inventory items with pagination
// @Tags inventory-items
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventory-items [get]
func (h *InventoryItemHandler) ListInventoryItems(c echo.Context) error {
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

	// Get inventory items and total count
	items, err := h.inventoryItemService.ListInventoryItems(c.Request().Context(), limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch inventory items"})
	}

	total, err := h.inventoryItemService.CountInventoryItems(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count inventory items"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Create response
	response := map[string]interface{}{
		"data":       items,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

// GetInventoryItemsByInventoryID lists inventory items for a specific inventory
// @Summary Get inventory items by inventory ID
// @Description List inventory items for a specific inventory with pagination
// @Tags inventory-items
// @Accept json
// @Produce json
// @Param id path int true "Inventory ID"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Param status query string false "Filter by status (active/inactive)"
// @Param product_type query string false "Filter by product type"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/{id}/inventory-items [get]
func (h *InventoryItemHandler) GetInventoryItemsByInventoryID(c echo.Context) error {
	inventoryID, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	// Parse query parameters
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	page, _ := strconv.Atoi(c.QueryParam("page"))
	status := c.QueryParam("status")
	productType := c.QueryParam("product_type")
	fmt.Println("productType", productType)

	// Set defaults
	if limit == 0 {
		limit = 20
	}
	if page == 0 {
		page = 1
	}

	// Calculate offset
	offset := (page - 1) * limit

	// Get inventory items with filters (empty filters will return all results)
	items, err := h.inventoryItemService.GetInventoryItemsByInventoryIDWithFilters(c.Request().Context(), inventoryID, status, productType, limit, offset)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch inventory items"})
	}

	total, err := h.inventoryItemService.CountInventoryItemsByInventoryIDWithFilters(c.Request().Context(), inventoryID, status, productType)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count inventory items"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Create response
	response := map[string]interface{}{
		"data":       items,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

// GetInventoryItemByProductID retrieves an inventory item by product ID
// @Summary Get inventory item by product ID
// @Description Get an inventory item by product ID
// @Tags inventory-items
// @Accept json
// @Produce json
// @Param product_id path int true "Product ID"
// @Success 200 {object} models.InventoryItem
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventory-items/product/{product_id} [get]
func (h *InventoryItemHandler) GetInventoryItemByProductID(c echo.Context) error {
	productID, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	item, err := h.inventoryItemService.GetInventoryItemByProductID(c.Request().Context(), productID)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get inventory item"})
	}

	return c.JSON(http.StatusOK, item)
}

// GetLowStockItems lists inventory items that are below reorder level
// @Summary Get low stock items
// @Description List inventory items that are below reorder level
// @Tags inventory-items
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventory-items/low-stock [get]
func (h *InventoryItemHandler) GetLowStockItems(c echo.Context) error {
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

	// Get low stock items and total count
	items, err := h.inventoryItemService.GetLowStockItems(c.Request().Context(), limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch low stock items"})
	}

	total, err := h.inventoryItemService.CountLowStockItems(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count low stock items"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Create response
	response := map[string]interface{}{
		"data":       items,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

// AdjustInventoryItemQuantity adjusts the quantity of an inventory item
// @Summary Adjust inventory item quantity
// @Description Adjust the quantity of an inventory item
// @Tags inventory-items
// @Accept json
// @Produce json
// @Param id path int true "Inventory ID"
// @Param item_id path int true "Inventory item ID"
// @Param adjustment body map[string]interface{} true "Adjustment data"
// @Success 200 {object} models.InventoryItem
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/{id}/inventory-items/{item_id}/adjust [put]
func (h *InventoryItemHandler) AdjustInventoryItemQuantity(c echo.Context) error {
	itemIDStr := c.Param("item_id")
	itemID, err := strconv.ParseUint(itemIDStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid item ID"})
	}

	var req struct {
		Quantity int    `json:"quantity"`
		Notes    string `json:"notes"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := h.inventoryItemService.AdjustInventoryItemQuantity(c.Request().Context(), uint(itemID), req.Quantity, req.Notes); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to adjust inventory item quantity"})
	}

	// Get the updated item
	item, err := h.inventoryItemService.GetInventoryItemByID(c.Request().Context(), uint(itemID))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get updated inventory item"})
	}

	return c.JSON(http.StatusOK, item)
}
