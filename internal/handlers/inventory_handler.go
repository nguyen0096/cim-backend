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
}

func NewInventoryHandler(inventoryService InventoryService) *InventoryHandler {
	return &InventoryHandler{
		inventoryService: inventoryService,
	}
}

func (h *InventoryHandler) GetInventory(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	
	if limit == 0 {
		limit = 20
	}

	inventory, err := h.inventoryService.GetInventory(limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch inventory"})
	}

	return c.JSON(http.StatusOK, inventory)
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
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	productIDStr := c.QueryParam("product_id")
	
	if limit == 0 {
		limit = 20
	}

	var productID uuid.UUID
	if productIDStr != "" {
		var err error
		productID, err = uuid.Parse(productIDStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid product ID"})
		}
	}

	transactions, err := h.inventoryService.GetTransactions(productID, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch transactions"})
	}

	return c.JSON(http.StatusOK, transactions)
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
