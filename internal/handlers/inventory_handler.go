package handlers

import (
	"import-export-backend/internal/models"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type InventoryHandler struct {
	inventoryService InventoryService
}

type InventoryService interface {
	GetInventory(limit, offset int) ([]models.Inventory, error)
	GetInventoryByID(id uuid.UUID) (*models.Inventory, error)
	UpdateInventory(inventory *models.Inventory) error
	AdjustInventory(productID uuid.UUID, quantity int, notes string, userID string) error
	GetLowStock() ([]models.Inventory, error)
	GetTransactions(productID uuid.UUID, limit, offset int) ([]models.InventoryTransaction, error)
	CountInventory() (int64, error)
	CountTransactions(productID uuid.UUID) (int64, error)
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
	inventory, err := h.inventoryService.GetInventory(limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch inventory"})
	}

	total, err := h.inventoryService.CountInventory()
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
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid inventory ID"})
	}

	var inventory models.Inventory
	if err := c.Bind(&inventory); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	inventory.ID = id
	if err := h.inventoryService.UpdateInventory(&inventory); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update inventory"})
	}

	return c.JSON(http.StatusOK, inventory)
}

func (h *InventoryHandler) AdjustInventory(c echo.Context) error {
	var req struct {
		ProductID uuid.UUID `json:"product_id"`
		Quantity  int       `json:"quantity"`
		Notes     string    `json:"notes"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	userID := c.Get("user_id").(string)
	if userID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}

	if err := h.inventoryService.AdjustInventory(req.ProductID, req.Quantity, req.Notes, userID); err != nil {
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

	var productID uuid.UUID
	if productIDStr != "" {
		var err error
		productID, err = uuid.Parse(productIDStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid product ID"})
		}
	}

	// Get transactions and total count
	transactions, err := h.inventoryService.GetTransactions(productID, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch transactions"})
	}

	total, err := h.inventoryService.CountTransactions(productID)
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
	inventory, err := h.inventoryService.GetLowStock()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch low stock items"})
	}

	return c.JSON(http.StatusOK, inventory)
}

func (h *InventoryHandler) GetInventorySummary(c echo.Context) error {
	// Implementation for inventory summary report
	return c.JSON(http.StatusOK, map[string]string{"message": "Inventory summary report"})
}
