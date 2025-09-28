package handlers

import (
	"context"
	"import-export-backend/internal/models"
	"import-export-backend/pkg"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type InventoryHandler struct {
	inventoryService InventoryService
}

type InventoryService interface {
	GetInventory(ctx context.Context, limit, offset int) ([]models.Inventory, error)
	GetInventoryByID(ctx context.Context, id uint) (*models.Inventory, error)
	UpdateInventory(ctx context.Context, inventory *models.Inventory) error
	AdjustInventory(ctx context.Context, productID uint, quantity int, notes string) error
	GetLowStock(ctx context.Context) ([]models.Inventory, error)
	GetTransactions(ctx context.Context, productID uint, limit, offset int) ([]models.InventoryTransaction, error)
	CountInventory(ctx context.Context) (int64, error)
	CountTransactions(ctx context.Context, productID uint) (int64, error)
}

func NewInventoryHandler(inventoryService InventoryService) *InventoryHandler {
	return &InventoryHandler{
		inventoryService: inventoryService,
	}
}

func (h *InventoryHandler) GetInventory(c echo.Context) error {
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
	inventory, err := h.inventoryService.GetInventory(c.Request().Context(), limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch inventory"})
	}

	total, err := h.inventoryService.CountInventory(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count inventory"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Create response
	response := map[string]interface{}{
		"data":       inventory,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

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

func (h *InventoryHandler) AdjustInventory(c echo.Context) error {
	var req struct {
		ProductID uint   `json:"product_id"`
		Quantity  int    `json:"quantity"`
		Notes     string `json:"notes"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := h.inventoryService.AdjustInventory(c.Request().Context(), req.ProductID, req.Quantity, req.Notes); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to adjust inventory"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Inventory adjusted successfully"})
}

func (h *InventoryHandler) GetTransactions(c echo.Context) error {
	// Parse query parameters
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	page, _ := strconv.Atoi(c.QueryParam("page"))
	productIDStr := c.QueryParam("product_id")

	// Set defaults
	if limit == 0 {
		limit = 20
	}
	if page == 0 {
		page = 1
	}

	// Calculate offset
	offset := (page - 1) * limit

	var productID uint
	if productIDStr != "" {
		var err error
		var parsedID int
		parsedID, err = strconv.Atoi(productIDStr)
		productID = uint(parsedID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid product ID"})
		}
	}

	// Get transactions and total count
	transactions, err := h.inventoryService.GetTransactions(c.Request().Context(), productID, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch transactions"})
	}

	total, err := h.inventoryService.CountTransactions(c.Request().Context(), productID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count transactions"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Create response
	response := map[string]interface{}{
		"data":       transactions,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

func (h *InventoryHandler) GetLowStock(c echo.Context) error {
	inventory, err := h.inventoryService.GetLowStock(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch low stock items"})
	}

	return c.JSON(http.StatusOK, inventory)
}

func (h *InventoryHandler) GetInventorySummary(c echo.Context) error {
	// Implementation for inventory summary report
	return c.JSON(http.StatusOK, map[string]string{"message": "Inventory summary report"})
}
