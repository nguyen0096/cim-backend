package repository

import (
	"cim-backend/internal/models"
	"context"
	"fmt"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

type MenuItemRepository interface {
	Create(ctx context.Context, menuItem *models.MenuItem) error
	GetByID(ctx context.Context, id uint) (*models.MenuItem, error)
	Update(ctx context.Context, menuItem *models.MenuItem) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, limit, offset int, search string, tags []string) ([]models.MenuItem, error)
}

type menuItemRepository struct {
	db *gorm.DB
}

func NewMenuItemRepository(db *gorm.DB) MenuItemRepository {
	return &menuItemRepository{db: db}
}

func (r *menuItemRepository) Create(ctx context.Context, menuItem *models.MenuItem) error {
	return r.db.WithContext(ctx).Create(menuItem).Error
}

func (r *menuItemRepository) GetByID(ctx context.Context, id uint) (*models.MenuItem, error) {
	var menuItem models.MenuItem
	err := r.db.WithContext(ctx).
		Preload("Menus").
		Preload("Products").
		First(&menuItem, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &menuItem, nil
}

func (r *menuItemRepository) Update(ctx context.Context, menuItem *models.MenuItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get existing menu item
		var existing models.MenuItem
		if err := tx.First(&existing, menuItem.ID).Error; err != nil {
			return fmt.Errorf("failed to get existing menu item: %w", err)
		}

		// Update base fields
		existing.Name = menuItem.Name
		if menuItem.Tags != nil {
			existing.Tags = menuItem.Tags
		}
		if err := tx.Save(&existing).Error; err != nil {
			return fmt.Errorf("failed to update menu item: %w", err)
		}

		// Handle many-to-many associations separately
		if menuItem.Products != nil {
			// Build list of Product structs with only ID set for Replace
			// GORM's Replace only needs the IDs to work correctly
			products := make([]*models.Product, len(menuItem.Products))
			for i, p := range menuItem.Products {
				products[i] = &models.Product{Base: models.Base{ID: p.ID}}
			}
			if err := tx.Model(&existing).Association("Products").Replace(products); err != nil {
				return fmt.Errorf("failed to update products association: %w", err)
			}
		}

		if menuItem.Menus != nil {
			// Build list of Menu structs with only ID set for Replace
			menus := make([]*models.Menu, len(menuItem.Menus))
			for i, m := range menuItem.Menus {
				menus[i] = &models.Menu{Base: models.Base{ID: m.ID}}
			}
			if err := tx.Model(&existing).Association("Menus").Replace(menus); err != nil {
				return fmt.Errorf("failed to update menus association: %w", err)
			}
		}

		return nil
	})
}

func (r *menuItemRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.MenuItem{}, "id = ?", id).Error
}

func (r *menuItemRepository) List(ctx context.Context, limit, offset int, search string, tags []string) ([]models.MenuItem, error) {
	var menuItems []models.MenuItem
	query := r.db.WithContext(ctx).
		Preload("Menus").
		Preload("Products")

	// Apply search filter if provided
	if search != "" {
		searchPattern := "%" + search + "%"
		// Use unaccent for Vietnamese fuzzy search (handles accents)
		// This allows searching "pho" to match "Phở", "phở", "PHỞ", etc.
		// Note: Requires PostgreSQL unaccent extension to be installed
		// The OR clause provides fallback matching even if unaccent normalization differs
		query = query.Where(
			"unaccent(LOWER(name)) ILIKE unaccent(LOWER(?)) OR name ILIKE ?",
			searchPattern, searchPattern,
		)
	}

	// Apply tag filter if provided
	if len(tags) > 0 {
		// Use PostgreSQL array overlap operator (&&) to find items with any matching tag
		// pq.Array converts Go slice to PostgreSQL array format
		query = query.Where("tags && ?", pq.Array(tags))
	}

	err := query.
		Limit(limit).
		Offset(offset).
		Find(&menuItems).Error
	return menuItems, err
}
