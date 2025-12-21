package handlers

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type MenuItemHandler struct {
	menuItemService services.MenuItemService
}

func NewMenuItemHandler(menuItemService services.MenuItemService) *MenuItemHandler {
	return &MenuItemHandler{
		menuItemService: menuItemService,
	}
}

// CreateMenuItemRequest represents the request body for creating a menu item
type CreateMenuItemRequest struct {
	Name       string   `json:"name" validate:"required"`
	Tags       []string `json:"tags,omitempty"`
	ProductIDs []uint   `json:"product_ids,omitempty"`
	MenuIDs    []uint   `json:"menu_ids,omitempty"`
}

// CreateMenuItem creates a new menu item
// @Summary Create menu item
// @Description Create a new menu item
// @Tags menu-items
// @Accept json
// @Produce json
// @Param menuItem body CreateMenuItemRequest true "Menu item data"
// @Success 201 {object} models.MenuItem
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /menu-items [post]
func (h *MenuItemHandler) CreateMenuItem(c echo.Context) error {
	var req CreateMenuItemRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := pkg.Validator.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Validation failed", "details": err.Error()})
	}

	menuItem := models.MenuItem{
		Name: req.Name,
	}
	if len(req.Tags) > 0 {
		menuItem.Tags = req.Tags
	}

	// Set products
	if len(req.ProductIDs) > 0 {
		menuItem.Products = make([]*models.Product, len(req.ProductIDs))
		for i, id := range req.ProductIDs {
			menuItem.Products[i] = &models.Product{Base: models.Base{ID: id}}
		}
	}

	// Set menus
	if len(req.MenuIDs) > 0 {
		menuItem.Menus = make([]*models.Menu, len(req.MenuIDs))
		for i, id := range req.MenuIDs {
			menuItem.Menus[i] = &models.Menu{Base: models.Base{ID: id}}
		}
	}

	if err := h.menuItemService.CreateMenuItem(c.Request().Context(), &menuItem); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create menu item", "details": err.Error()})
	}

	return c.JSON(http.StatusCreated, menuItem)
}

// GetMenuItem retrieves a menu item by ID
// @Summary Get menu item
// @Description Get a menu item by ID
// @Tags menu-items
// @Accept json
// @Produce json
// @Param id path int true "Menu item ID"
// @Success 200 {object} models.MenuItem
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /menu-items/{id} [get]
func (h *MenuItemHandler) GetMenuItem(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	menuItem, err := h.menuItemService.GetMenuItemByID(c.Request().Context(), id)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get menu item", "details": err.Error()})
	}

	if menuItem == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Menu item not found"})
	}

	return c.JSON(http.StatusOK, menuItem)
}

// ListMenuItems lists all menu items with pagination
// @Summary List menu items
// @Description List all menu items with pagination, search, and tag filtering
// @Tags menu-items
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Param q query string false "Search query (searches by name)"
// @Param tags query string false "Comma-separated list of tags to filter by (e.g., appetizer,mains,desserts)"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /menu-items [get]
func (h *MenuItemHandler) ListMenuItems(c echo.Context) error {
	// Parse query parameters
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	page, _ := strconv.Atoi(c.QueryParam("page"))
	search := c.QueryParam("q")
	tagsParam := c.QueryParam("tags")

	// Set defaults
	if limit == 0 {
		limit = 20
	}
	if page == 0 {
		page = 1
	}

	// Calculate offset
	offset := (page - 1) * limit

	// Parse tags from comma-separated string
	var tags []string
	if tagsParam != "" {
		tagList := strings.Split(tagsParam, ",")
		for _, tag := range tagList {
			trimmedTag := strings.TrimSpace(tag)
			if trimmedTag != "" {
				tags = append(tags, trimmedTag)
			}
		}
	}

	// Get menu items
	menuItems, err := h.menuItemService.ListMenuItems(c.Request().Context(), limit, offset, search, tags)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch menu items", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, menuItems)
}

// UpdateMenuItemRequest represents the request body for updating a menu item
type UpdateMenuItemRequest struct {
	Name       string   `json:"name" validate:"required"`
	Tags       []string `json:"tags,omitempty"`
	ProductIDs []uint   `json:"product_ids,omitempty"`
	MenuIDs    []uint   `json:"menu_ids,omitempty"`
}

// UpdateMenuItem updates a menu item
// @Summary Update menu item
// @Description Update a menu item
// @Tags menu-items
// @Accept json
// @Produce json
// @Param id path int true "Menu item ID"
// @Param menuItem body UpdateMenuItemRequest true "Menu item data"
// @Success 200 {object} models.MenuItem
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /menu-items/{id} [put]
func (h *MenuItemHandler) UpdateMenuItem(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	var req UpdateMenuItemRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := pkg.Validator.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Validation failed", "details": err.Error()})
	}

	menuItem := models.MenuItem{
		Base: models.Base{ID: id},
		Name: req.Name,
	}
	if req.Tags != nil {
		menuItem.Tags = req.Tags
	}

	// Set products
	if len(req.ProductIDs) > 0 {
		menuItem.Products = make([]*models.Product, len(req.ProductIDs))
		for i, productID := range req.ProductIDs {
			menuItem.Products[i] = &models.Product{Base: models.Base{ID: productID}}
		}
	} else {
		// Empty slice means clear all products
		menuItem.Products = []*models.Product{}
	}

	// Set menus
	if len(req.MenuIDs) > 0 {
		menuItem.Menus = make([]*models.Menu, len(req.MenuIDs))
		for i, menuID := range req.MenuIDs {
			menuItem.Menus[i] = &models.Menu{Base: models.Base{ID: menuID}}
		}
	} else {
		// Empty slice means clear all menus
		menuItem.Menus = []*models.Menu{}
	}

	if err := h.menuItemService.UpdateMenuItem(c.Request().Context(), &menuItem); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update menu item", "details": err.Error()})
	}

	// Fetch updated menu item to return with relationships
	updatedMenuItem, err := h.menuItemService.GetMenuItemByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch updated menu item", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, updatedMenuItem)
}

// DeleteMenuItem deletes a menu item
// @Summary Delete menu item
// @Description Delete a menu item
// @Tags menu-items
// @Accept json
// @Produce json
// @Param id path int true "Menu item ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /menu-items/{id} [delete]
func (h *MenuItemHandler) DeleteMenuItem(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	if err := h.menuItemService.DeleteMenuItem(c.Request().Context(), id); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
				"error": appErr.Message,
				"code":  appErr.Code.String(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete menu item", "details": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}
