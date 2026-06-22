package repository

import (
	"cim-backend/internal/models"
	"context"
	"encoding/json"
	"fmt"

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
	*baseRepository
}

// NewSettingsRepository creates a new settings repository
func NewSettingsRepository(base BaseRepository) SettingsRepository {
	return &settingsRepository{baseRepository: asBase(base)}
}

// Get retrieves a setting by key
func (r *settingsRepository) Get(ctx context.Context, key string) (*models.Settings, error) {
	var setting models.Settings
	if err := r.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error; err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return &setting, nil
}

// Set creates or updates a setting using upsert
func (r *settingsRepository) Set(ctx context.Context, key string, value interface{}) error {
	// Convert interface{} to datatypes.JSON
	jsonValue := datatypes.JSON{}
	if value != nil {
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}
		jsonValue = datatypes.JSON(jsonBytes)
	}

	setting := &models.Settings{
		Key:   key,
		Value: jsonValue,
	}

	// Use GORM's Save method which performs upsert (insert if not exists, update if exists)
	return r.db.WithContext(ctx).Save(setting).Error
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
