package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"context"
	"fmt"
)

//go:generate mockery --name=InventoryItemService --structname=InventoryItemService --output=../mocks/servicemocks --outpkg=servicemocks
type InventoryItemService interface {
	CreateInventoryItem(ctx context.Context, item *models.InventoryItem) error
	GetInventoryItemByID(ctx context.Context, id uint) (*models.InventoryItem, error)
	// UpdateInventoryItem applies the metadata update and returns the PERSISTED item
	// (reloaded from the DB). Quantity is immutable on this path, so the returned item
	// reflects the actual stored stock level (any quantity in the request is ignored),
	// not the request-bound value — preventing the API from echoing an ignored quantity.
	UpdateInventoryItem(ctx context.Context, item *models.InventoryItem) (*models.InventoryItem, error)
	DeleteInventoryItem(ctx context.Context, id uint) error
	ListInventoryItems(ctx context.Context, limit, offset int) ([]models.InventoryItem, error)
	GetInventoryItemsByInventoryIDWithFilters(ctx context.Context, inventoryID uint, productType string, params models.ListParams) ([]models.InventoryItem, error)
	GetInventoryItemByProductID(ctx context.Context, productID uint) (*models.InventoryItem, error)
	GetLowStockItems(ctx context.Context, limit, offset int) ([]models.InventoryItem, error)
	CountInventoryItems(ctx context.Context) (int64, error)
	CountInventoryItemsByInventoryIDWithFilters(ctx context.Context, inventoryID uint, productType string, params models.ListParams) (int64, error)
	CountLowStockItems(ctx context.Context) (int64, error)
}

type inventoryItemService struct {
	inventoryItemRepo repository.InventoryItemRepository
	inventoryRepo     repository.InventoryRepository
	productRepo       repository.ProductRepository
}

func NewInventoryItemService(
	inventoryItemRepo repository.InventoryItemRepository,
	inventoryRepo repository.InventoryRepository,
	productRepo repository.ProductRepository,
) InventoryItemService {
	return &inventoryItemService{
		inventoryItemRepo: inventoryItemRepo,
		inventoryRepo:     inventoryRepo,
		productRepo:       productRepo,
	}
}

func (s *inventoryItemService) CreateInventoryItem(ctx context.Context, item *models.InventoryItem) error {
	// Validate that inventory exists
	inventory, err := s.inventoryRepo.GetByID(ctx, item.InventoryID)
	if err != nil {
		return fmt.Errorf("failed to get inventory: %w", err)
	}
	if inventory == nil {
		return pkg.NewAppError(pkg.ErrorCodeNotFound, "inventory not found", nil)
	}

	// Validate that product exists
	product, err := s.productRepo.GetByID(ctx, item.ProductID)
	if err != nil {
		return fmt.Errorf("failed to get product: %w", err)
	}
	if product == nil {
		return pkg.NewAppError(pkg.ErrorCodeNotFound, "product not found", nil)
	}

	// Check if inventory item already exists for this product in this inventory
	existingItem, err := s.inventoryItemRepo.GetByProductID(ctx, item.ProductID)
	if err == nil && existingItem != nil && existingItem.InventoryID == item.InventoryID {
		return pkg.NewAppError(pkg.ErrorCodeValidation, "inventory item already exists for this product in this inventory", nil)
	}

	return s.inventoryItemRepo.Create(ctx, item)
}

func (s *inventoryItemService) GetInventoryItemByID(ctx context.Context, id uint) (*models.InventoryItem, error) {
	item, err := s.inventoryItemRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory item: %w", err)
	}
	if item == nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "inventory item not found", nil)
	}
	return item, nil
}

func (s *inventoryItemService) UpdateInventoryItem(ctx context.Context, item *models.InventoryItem) (*models.InventoryItem, error) {
	// Validate that inventory item exists
	existingItem, err := s.inventoryItemRepo.GetByID(ctx, item.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory item: %w", err)
	}
	if existingItem == nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "inventory item not found", nil)
	}

	// Validate that inventory exists if inventory_id is being changed
	if item.InventoryID != existingItem.InventoryID {
		inventory, err := s.inventoryRepo.GetByID(ctx, item.InventoryID)
		if err != nil {
			return nil, fmt.Errorf("failed to get inventory: %w", err)
		}
		if inventory == nil {
			return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "inventory not found", nil)
		}
	}

	// Validate that product exists if product_id is being changed
	if item.ProductID != existingItem.ProductID {
		product, err := s.productRepo.GetByID(ctx, item.ProductID)
		if err != nil {
			return nil, fmt.Errorf("failed to get product: %w", err)
		}
		if product == nil {
			return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "product not found", nil)
		}

		// Check if another inventory item already exists for this product in this inventory
		duplicateItem, err := s.inventoryItemRepo.GetByProductID(ctx, item.ProductID)
		if err == nil && duplicateItem != nil && duplicateItem.ID != item.ID && duplicateItem.InventoryID == item.InventoryID {
			return nil, pkg.NewAppError(pkg.ErrorCodeValidation, "inventory item already exists for this product in this inventory", nil)
		}

	}

	// Quantity is immutable on this metadata-update path. Stock changes only flow
	// through movement/submission flows (PO receive, dispose/transfer, reconcile
	// apply), each of which records an inventory_submission/transaction so the
	// reconcile drift check can observe it. A direct quantity write here would
	// bypass that drift check (epic #38 redesign dropped the old live-vs-prev
	// optimistic check).
	//
	// We do NOT read-then-rewrite quantity here: the repository Update scopes the
	// UPDATE to the metadata columns only (see inventoryItemMetadataColumns), so the
	// quantity column is never written by this path. That both keeps quantity
	// immutable AND guarantees a concurrent legitimate movement that commits between
	// this read and our write is never clobbered by a stale quantity — any inbound
	// quantity on the request is simply ignored.

	if err := s.inventoryItemRepo.Update(ctx, []*models.InventoryItem{item}, nil); err != nil {
		return nil, err
	}

	// Return the PERSISTED row rather than the request-bound item. The UPDATE is
	// column-scoped and deliberately omits quantity (immutable on this path), so the
	// request-bound struct can carry a quantity the DB never stored. Reloading makes the
	// API response reflect the actual stored quantity together with the applied metadata,
	// so a client can never mistake an ignored quantity for a successful stock change.
	persisted, err := s.inventoryItemRepo.GetByID(ctx, item.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload inventory item: %w", err)
	}
	if persisted == nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "inventory item not found", nil)
	}
	return persisted, nil
}

func (s *inventoryItemService) DeleteInventoryItem(ctx context.Context, id uint) error {
	// Validate that inventory item exists
	item, err := s.inventoryItemRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get inventory item: %w", err)
	}
	if item == nil {
		return pkg.NewAppError(pkg.ErrorCodeNotFound, "inventory item not found", nil)
	}

	return s.inventoryItemRepo.Delete(ctx, id)
}

func (s *inventoryItemService) ListInventoryItems(ctx context.Context, limit, offset int) ([]models.InventoryItem, error) {
	items, err := s.inventoryItemRepo.List(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list inventory items: %w", err)
	}
	return items, nil
}

// mapInventoryItemSortField maps DTO sort field to repository sort field
func mapInventoryItemSortField(sortField string) string {
	switch dto.InventoryItemSortField(sortField) {
	case dto.InventoryItemSortFieldUpdatedAt:
		return string(repository.InventoryItemSortFieldUpdatedAt)
	case dto.InventoryItemSortFieldCreatedAt:
		return string(repository.InventoryItemSortFieldCreatedAt)
	case dto.InventoryItemSortFieldQuantity:
		return string(repository.InventoryItemSortFieldQuantity)
	case dto.InventoryItemSortFieldProductName:
		return string(repository.InventoryItemSortFieldProductName)
	default:
		// Default to updated_at if invalid
		return string(repository.InventoryItemSortFieldUpdatedAt)
	}
}

// GetInventoryItemsByInventoryIDWithFilters retrieves inventory items by inventory ID with filters
func (s *inventoryItemService) GetInventoryItemsByInventoryIDWithFilters(ctx context.Context, inventoryID uint, productType string, params models.ListParams) ([]models.InventoryItem, error) {
	// Validate that inventory exists
	inventory, err := s.inventoryRepo.GetByID(ctx, inventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}
	if inventory == nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "inventory not found", nil)
	}

	// Map DTO sort field to repository sort field
	mappedSortField := mapInventoryItemSortField(params.Sort)

	filters := repository.InventoryItemFilters{
		Status:      params.Status,
		ProductType: productType,
		Search:      params.Search,
		Sort:        mappedSortField,
		Order:       params.Order,
	}

	items, err := s.inventoryItemRepo.GetByInventoryIDWithFilters(ctx, inventoryID, filters, params.Limit, params.GetOffset())
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory items by inventory ID with filters: %w", err)
	}
	return items, nil
}

func (s *inventoryItemService) GetInventoryItemByProductID(ctx context.Context, productID uint) (*models.InventoryItem, error) {
	item, err := s.inventoryItemRepo.GetByProductID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory item by product ID: %w", err)
	}
	if item == nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "inventory item not found for this product", nil)
	}
	return item, nil
}

func (s *inventoryItemService) GetLowStockItems(ctx context.Context, limit, offset int) ([]models.InventoryItem, error) {
	items, err := s.inventoryItemRepo.GetLowStockItems(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get low stock items: %w", err)
	}
	return items, nil
}

func (s *inventoryItemService) CountInventoryItems(ctx context.Context) (int64, error) {
	count, err := s.inventoryItemRepo.Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count inventory items: %w", err)
	}
	return count, nil
}

// CountInventoryItemsByInventoryIDWithFilters counts inventory items by inventory ID with filters
func (s *inventoryItemService) CountInventoryItemsByInventoryIDWithFilters(ctx context.Context, inventoryID uint, productType string, params models.ListParams) (int64, error) {
	filters := repository.InventoryItemFilters{
		Status:      params.Status,
		ProductType: productType,
		Search:      params.Search,
	}

	count, err := s.inventoryItemRepo.CountByInventoryIDWithFilters(ctx, inventoryID, filters)
	if err != nil {
		return 0, fmt.Errorf("failed to count inventory items by inventory ID with filters: %w", err)
	}
	return count, nil
}

func (s *inventoryItemService) CountLowStockItems(ctx context.Context) (int64, error) {
	count, err := s.inventoryItemRepo.CountLowStockItems(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count low stock items: %w", err)
	}
	return count, nil
}
