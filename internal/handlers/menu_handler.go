package handlers

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type MenuHandler struct {
	menuService services.MenuService
}

func NewMenuHandler(menuService services.MenuService) *MenuHandler {
	return &MenuHandler{
		menuService: menuService,
	}
}

// CreateMenuRequest represents the request body for creating a menu
type CreateMenuRequest struct {
	Name         string `json:"name" validate:"required"`
	MenuItemIDs  []uint `json:"menu_item_ids,omitempty"`
	InventoryIDs []uint `json:"inventory_ids,omitempty"`
}

// CreateMenu creates a new menu
// @Summary Create menu
// @Description Create a new menu
// @Tags menus
// @Accept json
// @Produce json
// @Param menu body CreateMenuRequest true "Menu data"
// @Success 201 {object} models.Menu
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /menus [post]
func (h *MenuHandler) CreateMenu(c echo.Context) error {
	var req CreateMenuRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := pkg.Validator.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Validation failed", "details": err.Error()})
	}

	menu := models.Menu{
		Name: req.Name,
	}

	// Set menu items
	if len(req.MenuItemIDs) > 0 {
		menu.MenuItems = make([]*models.MenuItem, len(req.MenuItemIDs))
		for i, id := range req.MenuItemIDs {
			menu.MenuItems[i] = &models.MenuItem{Base: models.Base{ID: id}}
		}
	}

	// Set inventories
	if len(req.InventoryIDs) > 0 {
		menu.Inventories = make([]*models.Inventory, len(req.InventoryIDs))
		for i, id := range req.InventoryIDs {
			menu.Inventories[i] = &models.Inventory{Base: models.Base{ID: id}}
		}
	}

	if err := h.menuService.CreateMenu(c.Request().Context(), &menu); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create menu", "details": err.Error()})
	}

	return c.JSON(http.StatusCreated, menu)
}

// GetMenu retrieves a menu by ID
// @Summary Get menu
// @Description Get a menu by ID
// @Tags menus
// @Accept json
// @Produce json
// @Param id path int true "Menu ID"
// @Success 200 {object} models.Menu
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /menus/{id} [get]
func (h *MenuHandler) GetMenu(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	menu, err := h.menuService.GetMenuByID(c.Request().Context(), id)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get menu", "details": err.Error()})
	}

	if menu == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Menu not found"})
	}

	return c.JSON(http.StatusOK, menu)
}

// ListMenus lists all menus with pagination
// @Summary List menus
// @Description List all menus with pagination
// @Tags menus
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /menus [get]
func (h *MenuHandler) ListMenus(c echo.Context) error {
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

	// Get menus
	menus, err := h.menuService.ListMenus(c.Request().Context(), limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch menus", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, menus)
}

// UpdateMenuRequest represents the request body for updating a menu
type UpdateMenuRequest struct {
	Name         string `json:"name" validate:"required"`
	MenuItemIDs  []uint `json:"menu_item_ids,omitempty"`
	InventoryIDs []uint `json:"inventory_ids,omitempty"`
}

// UpdateMenu updates a menu
// @Summary Update menu
// @Description Update a menu
// @Tags menus
// @Accept json
// @Produce json
// @Param id path int true "Menu ID"
// @Param menu body UpdateMenuRequest true "Menu data"
// @Success 200 {object} models.Menu
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /menus/{id} [put]
func (h *MenuHandler) UpdateMenu(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	var req UpdateMenuRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := pkg.Validator.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Validation failed", "details": err.Error()})
	}

	menu := models.Menu{
		Base: models.Base{ID: id},
		Name: req.Name,
	}

	// Set menu items
	if len(req.MenuItemIDs) > 0 {
		menu.MenuItems = make([]*models.MenuItem, len(req.MenuItemIDs))
		for i, menuItemID := range req.MenuItemIDs {
			menu.MenuItems[i] = &models.MenuItem{Base: models.Base{ID: menuItemID}}
		}
	} else {
		// Empty slice means clear all menu items
		menu.MenuItems = []*models.MenuItem{}
	}

	// Set inventories
	if len(req.InventoryIDs) > 0 {
		menu.Inventories = make([]*models.Inventory, len(req.InventoryIDs))
		for i, inventoryID := range req.InventoryIDs {
			menu.Inventories[i] = &models.Inventory{Base: models.Base{ID: inventoryID}}
		}
	} else {
		// Empty slice means clear all inventories
		menu.Inventories = []*models.Inventory{}
	}

	if err := h.menuService.UpdateMenu(c.Request().Context(), &menu); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update menu", "details": err.Error()})
	}

	// Fetch updated menu to return with relationships
	updatedMenu, err := h.menuService.GetMenuByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch updated menu", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, updatedMenu)
}

// DeleteMenu deletes a menu
// @Summary Delete menu
// @Description Delete a menu
// @Tags menus
// @Accept json
// @Produce json
// @Param id path int true "Menu ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /menus/{id} [delete]
func (h *MenuHandler) DeleteMenu(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	if err := h.menuService.DeleteMenu(c.Request().Context(), id); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete menu", "details": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}
