package handlers

import (
	"context"
	"net/http"

	"import-export-backend/internal/models"

	"github.com/labstack/echo/v4"
)

// SettingsHandler handles HTTP requests for settings
type SettingsHandler struct {
	settingsService SettingsService
}

// SettingsService interface for settings business logic
type SettingsService interface {
	GetSetting(ctx context.Context, key string) (*models.Settings, error)
	SetSetting(ctx context.Context, key string, value interface{}) error
	GetAllSettings(ctx context.Context) ([]models.Settings, error)
	DeleteSetting(ctx context.Context, key string) error
}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler(settingsService SettingsService) *SettingsHandler {
	return &SettingsHandler{
		settingsService: settingsService,
	}
}

// GetSetting retrieves a setting by key
// @Summary Get setting by key
// @Description Retrieves a specific setting by its key
// @Tags Settings
// @Accept json
// @Produce json
// @Param key path string true "Setting key"
// @Success 200 {object} models.Settings
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /settings/{key} [get]
func (h *SettingsHandler) GetSetting(c echo.Context) error {
	key := c.Param("key")
	if key == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Setting key is required",
		})
	}

	setting, err := h.settingsService.GetSetting(c.Request().Context(), key)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to get setting",
		})
	}

	if setting == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Setting not found",
		})
	}

	return c.JSON(http.StatusOK, setting.Value)
}

// GetAllSettings retrieves all settings
// @Summary Get all settings
// @Description Retrieves all settings in the system
// @Tags Settings
// @Accept json
// @Produce json
// @Success 200 {object} models.GetSettingsResponse
// @Failure 500 {object} map[string]string
// @Router /settings [get]
func (h *SettingsHandler) GetAllSettings(c echo.Context) error {
	settings, err := h.settingsService.GetAllSettings(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to get settings",
		})
	}

	response := models.GetSettingsResponse{
		Data: settings,
	}

	return c.JSON(http.StatusOK, response)
}

// SetSetting creates or updates a setting (upsert operation)
// @Summary Set setting
// @Description Creates or updates a setting with the specified key and value
// @Tags Settings
// @Accept json
// @Produce json
// @Param key path string true "Setting key"
// @Param request body models.SetSettingValueRequest true "Set setting value request"
// @Success 200 {object} models.SetSettingResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /settings/{key} [post]
func (h *SettingsHandler) SetSetting(c echo.Context) error {
	key := c.Param("key")
	if key == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Setting key is required",
		})
	}

	var req interface{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request format",
		})
	}

	// Validate required fields
	if req == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Payload is required",
		})
	}

	// Set the setting (upsert operation)
	if err := h.settingsService.SetSetting(c.Request().Context(), key, req); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to set setting",
		})
	}

	response := models.SetSettingResponse{
		Message: "Setting saved successfully",
	}

	return c.JSON(http.StatusOK, response)
}

// DeleteSetting removes a setting by key
// @Summary Delete setting
// @Description Deletes a setting by its key
// @Tags Settings
// @Accept json
// @Produce json
// @Param key path string true "Setting key"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /settings/{key} [delete]
func (h *SettingsHandler) DeleteSetting(c echo.Context) error {
	key := c.Param("key")
	if key == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Setting key is required",
		})
	}

	if err := h.settingsService.DeleteSetting(c.Request().Context(), key); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to delete setting",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Setting deleted successfully",
	})
}
