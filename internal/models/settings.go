package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Settings struct {
	Key   string         `json:"key" gorm:"primaryKey" example:"app_name"`
	Value datatypes.JSON `json:"value" swaggertype:"object"`
}

// BeforeCreate overrides the Base BeforeCreate to handle migration context
func (s *Settings) BeforeCreate(tx *gorm.DB) error {
	// Skip user email validation for settings to allow system-level settings
	return nil
}

// SetSettingRequest represents the request to set a setting
// @Description Request payload for setting a configuration value
type SetSettingRequest struct {
	Key   string      `json:"key" validate:"required" example:"app_name"`
	Value interface{} `json:"value" validate:"required"`
}

// SetSettingResponse represents the response after setting a value
// @Description Response after successfully setting a configuration value
type SetSettingResponse struct {
	Message string `json:"message" example:"Setting updated successfully"`
}

// GetSettingsResponse represents the response for getting all settings
// @Description Response containing all application settings
type GetSettingsResponse struct {
	Data []Settings `json:"data"`
}
