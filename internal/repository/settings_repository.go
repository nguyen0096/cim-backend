package repository

import (
	"context"
	"encoding/json"
	"import-export-backend/internal/models"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// SettingsRepository defines the interface for settings data access
type SettingsRepository interface {
	Get(ctx context.Context, key string) (*models.Settings, error)
	Set(ctx context.Context, key string, value interface{}) error
	GetAll(ctx context.Context) ([]models.Settings, error)
	Delete(ctx context.Context, key string) error
}

// settingsRepository implements SettingsRepository
type settingsRepository struct {
	db *gorm.DB
}

// NewSettingsRepository creates a new settings repository
func NewSettingsRepository(db *gorm.DB) SettingsRepository {
	return &settingsRepository{
		db: db,
	}
}

// Get retrieves a setting by key
func (r *settingsRepository) Get(ctx context.Context, key string) (*models.Settings, error) {
	var setting models.Settings
	if err := r.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &setting, nil
}

// Set creates or updates a setting
func (r *settingsRepository) Set(ctx context.Context, key string, value interface{}) error {
	setting := &models.Settings{
		Key: key,
	}

	// Convert interface{} to datatypes.JSON
	jsonValue := datatypes.JSON{}
	if value != nil {
		if jsonBytes, err := json.Marshal(value); err != nil {
			return err
		} else {
			jsonValue = datatypes.JSON(jsonBytes)
		}
	}

	// Try to find existing setting
	if err := r.db.WithContext(ctx).Where("key = ?", key).First(setting).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new setting
			setting.Value = jsonValue
			return r.db.WithContext(ctx).Create(setting).Error
		}
		return err
	}

	// Update existing setting
	return r.db.WithContext(ctx).Model(setting).Update("value", jsonValue).Error
}

// GetAll retrieves all settings
func (r *settingsRepository) GetAll(ctx context.Context) ([]models.Settings, error) {
	var settings []models.Settings
	if err := r.db.WithContext(ctx).Find(&settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

// Delete removes a setting by key
func (r *settingsRepository) Delete(ctx context.Context, key string) error {
	return r.db.WithContext(ctx).Where("key = ?", key).Delete(&models.Settings{}).Error
}
