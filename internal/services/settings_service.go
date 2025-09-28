package services

import (
	"context"
	"encoding/json"
	"fmt"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
)

// SettingsService defines the interface for settings business logic
type SettingsService interface {
	GetSetting(ctx context.Context, key string) (*models.Settings, error)
	SetSetting(ctx context.Context, key string, value interface{}) error
	GetAllSettings(ctx context.Context) ([]models.Settings, error)
	DeleteSetting(ctx context.Context, key string) error
	GetSettingValue(ctx context.Context, key string, dest interface{}) error
}

// settingsService implements SettingsService
type settingsService struct {
	settingsRepo repository.SettingsRepository
}

// NewSettingsService creates a new settings service
func NewSettingsService(settingsRepo repository.SettingsRepository) SettingsService {
	return &settingsService{
		settingsRepo: settingsRepo,
	}
}

// GetSetting retrieves a setting by key
func (s *settingsService) GetSetting(ctx context.Context, key string) (*models.Settings, error) {
	if key == "" {
		return nil, fmt.Errorf("setting key cannot be empty")
	}

	setting, err := s.settingsRepo.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get setting: %w", err)
	}

	return setting, nil
}

// SetSetting creates or updates a setting
func (s *settingsService) SetSetting(ctx context.Context, key string, value interface{}) error {
	if key == "" {
		return fmt.Errorf("setting key cannot be empty")
	}

	if value == nil {
		return fmt.Errorf("setting value cannot be nil")
	}

	if err := s.settingsRepo.Set(ctx, key, value); err != nil {
		return fmt.Errorf("failed to set setting: %w", err)
	}

	return nil
}

// GetAllSettings retrieves all settings
func (s *settingsService) GetAllSettings(ctx context.Context) ([]models.Settings, error) {
	settings, err := s.settingsRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all settings: %w", err)
	}

	return settings, nil
}

// DeleteSetting removes a setting by key
func (s *settingsService) DeleteSetting(ctx context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("setting key cannot be empty")
	}

	if err := s.settingsRepo.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to delete setting: %w", err)
	}

	return nil
}

// GetSettingValue retrieves a setting and unmarshals its value into dest
func (s *settingsService) GetSettingValue(ctx context.Context, key string, dest interface{}) error {
	setting, err := s.GetSetting(ctx, key)
	if err != nil {
		return err
	}

	if setting == nil {
		return fmt.Errorf("setting with key '%s' not found", key)
	}

	// Convert the JSON value to bytes and unmarshal into dest
	valueBytes, err := json.Marshal(setting.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal setting value: %w", err)
	}

	if err := json.Unmarshal(valueBytes, dest); err != nil {
		return fmt.Errorf("failed to unmarshal setting value: %w", err)
	}

	return nil
}
