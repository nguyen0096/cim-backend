package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"context"
	"fmt"

	"gorm.io/gorm"
)

type MenuItemService interface {
	CreateMenuItem(ctx context.Context, menuItem *models.MenuItem) error
	GetMenuItemByID(ctx context.Context, id uint) (*models.MenuItem, error)
	UpdateMenuItem(ctx context.Context, menuItem *models.MenuItem) error
	DeleteMenuItem(ctx context.Context, id uint) error
	ListMenuItems(ctx context.Context, limit, offset int) ([]models.MenuItem, error)
}

type menuItemService struct {
	menuItemRepo repository.MenuItemRepository
	menuRepo     repository.MenuRepository
	productRepo  repository.ProductRepository
}

func NewMenuItemService(
	menuItemRepo repository.MenuItemRepository,
	menuRepo repository.MenuRepository,
	productRepo repository.ProductRepository,
) MenuItemService {
	return &menuItemService{
		menuItemRepo: menuItemRepo,
		menuRepo:     menuRepo,
		productRepo:  productRepo,
	}
}

func (s *menuItemService) CreateMenuItem(ctx context.Context, menuItem *models.MenuItem) error {
	// Validate menu IDs exist
	if len(menuItem.Menus) > 0 {
		for _, menu := range menuItem.Menus {
			if menu.ID == 0 {
				return fmt.Errorf("invalid menu ID: %d", menu.ID)
			}
			_, err := s.menuRepo.GetByID(ctx, menu.ID)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					return fmt.Errorf("menu with ID %d not found", menu.ID)
				}
				return fmt.Errorf("failed to validate menu: %w", err)
			}
		}
	}

	// Validate product IDs exist
	if len(menuItem.Products) > 0 {
		for _, product := range menuItem.Products {
			if product.ID == 0 {
				return fmt.Errorf("invalid product ID: %d", product.ID)
			}
			_, err := s.productRepo.GetByID(ctx, product.ID)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					return fmt.Errorf("product with ID %d not found", product.ID)
				}
				return fmt.Errorf("failed to validate product: %w", err)
			}
		}
	}

	return s.menuItemRepo.Create(ctx, menuItem)
}

func (s *menuItemService) GetMenuItemByID(ctx context.Context, id uint) (*models.MenuItem, error) {
	return s.menuItemRepo.GetByID(ctx, id)
}

func (s *menuItemService) UpdateMenuItem(ctx context.Context, menuItem *models.MenuItem) error {
	// Check if menu item exists
	existingMenuItem, err := s.menuItemRepo.GetByID(ctx, menuItem.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("menu item with ID %d not found", menuItem.ID)
		}
		return fmt.Errorf("failed to get menu item: %w", err)
	}

	// Validate menu IDs exist
	if len(menuItem.Menus) > 0 {
		for _, menu := range menuItem.Menus {
			if menu.ID == 0 {
				return fmt.Errorf("invalid menu ID: %d", menu.ID)
			}
			_, err := s.menuRepo.GetByID(ctx, menu.ID)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					return fmt.Errorf("menu with ID %d not found", menu.ID)
				}
				return fmt.Errorf("failed to validate menu: %w", err)
			}
		}
	}

	// Validate product IDs exist
	if len(menuItem.Products) > 0 {
		for _, product := range menuItem.Products {
			if product.ID == 0 {
				return fmt.Errorf("invalid product ID: %d", product.ID)
			}
			_, err := s.productRepo.GetByID(ctx, product.ID)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					return fmt.Errorf("product with ID %d not found", product.ID)
				}
				return fmt.Errorf("failed to validate product: %w", err)
			}
		}
	}

	// Preserve created fields
	menuItem.CreatedBy = existingMenuItem.CreatedBy
	menuItem.CreatedAt = existingMenuItem.CreatedAt

	return s.menuItemRepo.Update(ctx, menuItem)
}

func (s *menuItemService) DeleteMenuItem(ctx context.Context, id uint) error {
	return s.menuItemRepo.Delete(ctx, id)
}

func (s *menuItemService) ListMenuItems(ctx context.Context, limit, offset int) ([]models.MenuItem, error) {
	return s.menuItemRepo.List(ctx, limit, offset)
}
