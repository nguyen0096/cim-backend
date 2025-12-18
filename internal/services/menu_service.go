package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"context"
	"fmt"

	"gorm.io/gorm"
)

type MenuService interface {
	CreateMenu(ctx context.Context, menu *models.Menu) error
	GetMenuByID(ctx context.Context, id uint) (*models.Menu, error)
	UpdateMenu(ctx context.Context, menu *models.Menu) error
	DeleteMenu(ctx context.Context, id uint) error
	ListMenus(ctx context.Context, params models.ListParams) (*models.PaginationResult[models.Menu], error)
}

type menuService struct {
	menuRepo      repository.MenuRepository
	menuItemRepo  repository.MenuItemRepository
	inventoryRepo repository.InventoryRepository
}

func NewMenuService(
	menuRepo repository.MenuRepository,
	menuItemRepo repository.MenuItemRepository,
	inventoryRepo repository.InventoryRepository,
) MenuService {
	return &menuService{
		menuRepo:      menuRepo,
		menuItemRepo:  menuItemRepo,
		inventoryRepo: inventoryRepo,
	}
}

func (s *menuService) CreateMenu(ctx context.Context, menu *models.Menu) error {
	// Validate menu item IDs exist
	if len(menu.MenuItems) > 0 {
		for _, menuItem := range menu.MenuItems {
			if menuItem.ID == 0 {
				return fmt.Errorf("invalid menu item ID: %d", menuItem.ID)
			}
			_, err := s.menuItemRepo.GetByID(ctx, menuItem.ID)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					return fmt.Errorf("menu item with ID %d not found", menuItem.ID)
				}
				return fmt.Errorf("failed to validate menu item: %w", err)
			}
		}
	}

	// Validate inventory IDs exist
	if len(menu.Inventories) > 0 {
		for _, inventory := range menu.Inventories {
			if inventory.ID == 0 {
				return fmt.Errorf("invalid inventory ID: %d", inventory.ID)
			}
			_, err := s.inventoryRepo.GetByID(ctx, inventory.ID)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					return fmt.Errorf("inventory with ID %d not found", inventory.ID)
				}
				return fmt.Errorf("failed to validate inventory: %w", err)
			}
		}
	}

	return s.menuRepo.Create(ctx, menu)
}

func (s *menuService) GetMenuByID(ctx context.Context, id uint) (*models.Menu, error) {
	return s.menuRepo.GetByID(ctx, id)
}

func (s *menuService) UpdateMenu(ctx context.Context, menu *models.Menu) error {
	// Check if menu exists
	existingMenu, err := s.menuRepo.GetByID(ctx, menu.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("menu with ID %d not found", menu.ID)
		}
		return fmt.Errorf("failed to get menu: %w", err)
	}

	// Validate menu item IDs exist
	if len(menu.MenuItems) > 0 {
		for _, menuItem := range menu.MenuItems {
			if menuItem.ID == 0 {
				return fmt.Errorf("invalid menu item ID: %d", menuItem.ID)
			}
			_, err := s.menuItemRepo.GetByID(ctx, menuItem.ID)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					return fmt.Errorf("menu item with ID %d not found", menuItem.ID)
				}
				return fmt.Errorf("failed to validate menu item: %w", err)
			}
		}
	}

	// Validate inventory IDs exist
	if len(menu.Inventories) > 0 {
		for _, inventory := range menu.Inventories {
			if inventory.ID == 0 {
				return fmt.Errorf("invalid inventory ID: %d", inventory.ID)
			}
			_, err := s.inventoryRepo.GetByID(ctx, inventory.ID)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					return fmt.Errorf("inventory with ID %d not found", inventory.ID)
				}
				return fmt.Errorf("failed to validate inventory: %w", err)
			}
		}
	}

	// Preserve created fields
	menu.CreatedBy = existingMenu.CreatedBy
	menu.CreatedAt = existingMenu.CreatedAt

	return s.menuRepo.Update(ctx, menu)
}

func (s *menuService) DeleteMenu(ctx context.Context, id uint) error {
	return s.menuRepo.Delete(ctx, id)
}

func (s *menuService) ListMenus(ctx context.Context, params models.ListParams) (*models.PaginationResult[models.Menu], error) {
	params.ValidateAndSetDefaults()

	menus, total, err := s.menuRepo.List(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list menus: %w", err)
	}

	return models.NewPaginationResult(menus, total, params.Page, params.Limit), nil
}
