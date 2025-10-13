package handlers

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type InventoryHandler struct {
	inventoryService services.InventoryService
}

func NewInventoryHandler(inventoryService services.InventoryService) *InventoryHandler {
	return &InventoryHandler{
		inventoryService: inventoryService,
	}
}

// CreateInventory creates a new inventory
// @Summary Create inventory
// @Description Create a new inventory
// @Tags inventories
// @Accept json
// @Produce json
// @Param inventory body models.Inventory true "Inventory data"
// @Success 201 {object} models.Inventory
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventories [post]
func (h *InventoryHandler) CreateInventory(c echo.Context) error {
	var inventory models.Inventory
	if err := c.Bind(&inventory); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := h.inventoryService.CreateInventory(c.Request().Context(), &inventory); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create inventory"})
	}

	return c.JSON(http.StatusCreated, inventory)
}

// GetInventory retrieves an inventory by ID
// @Summary Get inventory
// @Description Get an inventory by ID
// @Tags inventories
// @Accept json
// @Produce json
// @Param id path int true "Inventory ID"
// @Success 200 {object} models.Inventory
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/{id} [get]
func (h *InventoryHandler) GetInventory(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	inventory, err := h.inventoryService.GetInventoryByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get inventory"})
	}

	return c.JSON(http.StatusOK, inventory)
}

// ListInventory lists all inventories with pagination
// @Summary List inventories
// @Description List all inventories with pagination
// @Tags inventories
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventories [get]
func (h *InventoryHandler) ListInventory(c echo.Context) error {
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

	// Get inventory and total count
	inventory, err := h.inventoryService.ListInventory(c.Request().Context(), limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch inventory"})
	}

	return c.JSON(http.StatusOK, inventory)
}

// UpdateInventory updates an inventory
// @Summary Update inventory
// @Description Update an inventory
// @Tags inventories
// @Accept json
// @Produce json
// @Param id path int true "Inventory ID"
// @Param inventory body models.Inventory true "Inventory data"
// @Success 200 {object} models.Inventory
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/{id} [put]
func (h *InventoryHandler) UpdateInventory(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	var inventory models.Inventory
	if err := c.Bind(&inventory); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	inventory.ID = id
	if err := h.inventoryService.UpdateInventory(c.Request().Context(), &inventory); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update inventory"})
	}

	return c.JSON(http.StatusOK, inventory)
}

// DeleteInventory deletes an inventory
// @Summary Delete inventory
// @Description Delete an inventory
// @Tags inventories
// @Accept json
// @Produce json
// @Param id path int true "Inventory ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/{id} [delete]
func (h *InventoryHandler) DeleteInventory(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	if err := h.inventoryService.DeleteInventory(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete inventory"})
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *InventoryHandler) GetInventorySummary(c echo.Context) error {
	// Implementation for inventory summary report
	return c.JSON(http.StatusOK, map[string]string{"message": "Inventory summary report"})
}

// DisposeInventoryItem disposes multiple inventory items
// @Summary Dispose inventory items
// @Description Dispose multiple inventory items by reducing their quantities
// @Tags inventory-items
// @Accept json
// @Produce json
// @Param disposal body dto.DisposeItemsRequest true "Disposal data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/dispose [put]
func (h *InventoryHandler) DisposeInventoryItems(c echo.Context) error {
	var req dto.DisposeItemsRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := pkg.Validator.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	disposedItems, err := h.inventoryService.DisposeItems(c.Request().Context(), req)
	if err != nil {
		return fmt.Errorf("failed to dispose inventory items: %w", err)
	}

	response := map[string]interface{}{
		"message": "Inventory items disposed successfully",
		"items":   disposedItems,
	}

	return c.JSON(http.StatusOK, response)
}

// ReconcileInventory confirms the actual inventory count for multiple items
// @Summary Confirm inventory items
// @Description Confirm the actual inventory count for multiple inventory items
// @Tags inventory-items
// @Accept json
// @Produce json
// @Param confirmation body dto.ConfirmInventoryRequest true "Confirmation data"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/confirm [put]
func (h *InventoryHandler) ReconcileInventory(c echo.Context) error {
	var req dto.ConfirmInventoryRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := pkg.Validator.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	err := h.inventoryService.ReconcileInventory(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to confirm inventory items"})
	}

	return c.JSON(http.StatusNoContent, nil)
}

// GetLastPurchasePrices retrieves the last purchase transaction price
// for each Supplier Product.
// @Summary Get last purchase prices
// @Description Get the last purchase transaction price for each Supplier Product as a nested map (product_id -> supplier_id -> price)
// @Tags inventories
// @Accept json
// @Produce json
// @Success 200 {object} dto.LastPurchasePriceMap
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /inventories/last-purchase-prices [get]
func (h *InventoryHandler) GetLastPurchasePrices(c echo.Context) error {
	prices, err := h.inventoryService.GetLastPurchasePrices(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get last purchase prices"})
	}

	return c.JSON(http.StatusOK, prices)
}
