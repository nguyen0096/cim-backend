package repository

import (
	"cim-backend/internal/models"
	"context"
	"fmt"

	"gorm.io/gorm"
)

type MenuRepository interface {
	Create(ctx context.Context, menu *models.Menu) error
	GetByID(ctx context.Context, id uint) (*models.Menu, error)
	Update(ctx context.Context, menu *models.Menu) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, limit, offset int) ([]models.Menu, error)
}

type menuRepository struct {
	db *gorm.DB
}

func NewMenuRepository(db *gorm.DB) MenuRepository {
	return &menuRepository{db: db}
}

func (r *menuRepository) Create(ctx context.Context, menu *models.Menu) error {
	return r.db.WithContext(ctx).Create(menu).Error
}

func (r *menuRepository) GetByID(ctx context.Context, id uint) (*models.Menu, error) {
	var menu models.Menu
	err := r.db.WithContext(ctx).
		Preload("MenuItems").
		Preload("MenuItems.Products").
		Preload("Inventories").
		First(&menu, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &menu, nil
}

func (r *menuRepository) Update(ctx context.Context, menu *models.Menu) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get existing menu
		var existing models.Menu
		if err := tx.First(&existing, menu.ID).Error; err != nil {
			return fmt.Errorf("failed to get existing menu: %w", err)
		}

		// Update base fields
		existing.Name = menu.Name
		if err := tx.Save(&existing).Error; err != nil {
			return fmt.Errorf("failed to update menu: %w", err)
		}

		// Handle many-to-many associations separately
		if menu.MenuItems != nil {
			// Build list of MenuItem structs with only ID set for Replace
			// GORM's Replace only needs the IDs to work correctly
			menuItems := make([]*models.MenuItem, len(menu.MenuItems))
			for i, mi := range menu.MenuItems {
				menuItems[i] = &models.MenuItem{Base: models.Base{ID: mi.ID}}
			}
			if err := tx.Model(&existing).Association("MenuItems").Replace(menuItems); err != nil {
				return fmt.Errorf("failed to update menu items association: %w", err)
			}
		}

		if menu.Inventories != nil {
			// Build list of Inventory structs with only ID set for Replace
			inventories := make([]*models.Inventory, len(menu.Inventories))
			for i, inv := range menu.Inventories {
				inventories[i] = &models.Inventory{Base: models.Base{ID: inv.ID}}
			}
			if err := tx.Model(&existing).Association("Inventories").Replace(inventories); err != nil {
				return fmt.Errorf("failed to update inventories association: %w", err)
			}
		}

		return nil
	})
}

func (r *menuRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Menu{}, "id = ?", id).Error
}

func (r *menuRepository) List(ctx context.Context, limit, offset int) ([]models.Menu, error) {
	var menus []models.Menu
	err := r.db.WithContext(ctx).
		Preload("MenuItems").
		Preload("MenuItems.Products").
		Preload("Inventories").
		Limit(limit).
		Offset(offset).
		Find(&menus).Error
	return menus, err
}
