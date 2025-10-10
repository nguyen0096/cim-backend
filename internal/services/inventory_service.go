package services

import (
	"context"
	"fmt"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
	"import-export-backend/internal/services/dto"
	"import-export-backend/pkg"
)

//go:generate mockery --name=InventoryService --structname=InventoryService --output=./servicemocks --outpkg=servicemocks
type InventoryService interface {
	CreateInventory(ctx context.Context, inventory *models.Inventory) error
	GetInventoryByID(ctx context.Context, id uint) (*models.Inventory, error)
	UpdateInventory(ctx context.Context, inventory *models.Inventory) error
	DeleteInventory(ctx context.Context, id uint) error
	ListInventory(ctx context.Context, limit, offset int) ([]models.Inventory, error)
	AddInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType, notes string) error
	RemoveInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType, notes string) error

	// v1
	ConfirmInventory(ctx context.Context, req dto.ConfirmInventoryRequest) ([]models.InventoryItem, error)
	DisposeItems(ctx context.Context, req dto.DisposeItemsRequest) ([]*models.InventoryItem, error)
}

type inventoryService struct {
	inventoryRepo     repository.InventoryRepository
	inventoryItemRepo repository.InventoryItemRepository
	productRepo       repository.ProductRepository
}

func NewInventoryService(
	inventoryRepo repository.InventoryRepository,
	inventoryItemRepo repository.InventoryItemRepository,
	productRepo repository.ProductRepository,
) InventoryService {
	return &inventoryService{
		inventoryRepo:     inventoryRepo,
		productRepo:       productRepo,
		inventoryItemRepo: inventoryItemRepo,
	}
}

func (s *inventoryService) CreateInventory(ctx context.Context, inventory *models.Inventory) error {
	return s.inventoryRepo.Create(ctx, inventory)
}

func (s *inventoryService) ListInventory(ctx context.Context, limit, offset int) ([]models.Inventory, error) {
	return s.inventoryRepo.List(ctx, limit, offset)
}

func (s *inventoryService) GetInventoryByID(ctx context.Context, id uint) (*models.Inventory, error) {
	return s.inventoryRepo.GetByID(ctx, id)
}

func (s *inventoryService) DeleteInventory(ctx context.Context, id uint) error {
	return s.inventoryRepo.Delete(ctx, id)
}

func (s *inventoryService) UpdateInventory(ctx context.Context, inventory *models.Inventory) error {
	return s.inventoryRepo.Update(ctx, inventory)
}

func (s *inventoryService) AddInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType, notes string) error {
	// Create transaction record
	return s.inventoryRepo.AddInventory(ctx, productID, quantity, referenceID, referenceType)
}

func (s *inventoryService) RemoveInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType, notes string) error {
	return s.inventoryRepo.RemoveInventory(ctx, productID, quantity, referenceID, referenceType)
}

// DisposeInventoryItems disposes multiple inventory items by reducing their quantities
func (s *inventoryService) DisposeItems(ctx context.Context, req dto.DisposeItemsRequest) ([]*models.InventoryItem, error) {
	// Get inventory with preloaded inventory items
	inventory, err := s.inventoryRepo.GetByID(ctx, req.InventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}
	if inventory == nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "inventory not found", nil)
	}

	// Build a map of inventory item ID -> inventory item pointer for quick lookup
	itemMap := make(map[uint]*models.InventoryItem)
	for i := range inventory.Items {
		itemMap[inventory.Items[i].ID] = inventory.Items[i]
	}

	var updatedItems []*models.InventoryItem
	var inventoryTransactions []*models.InventoryTransaction

	// Process each item in the disposal request
	for _, disposalItem := range req.Items {
		// Find the inventory item in the map
		item, exists := itemMap[disposalItem.InventoryItemID]
		if !exists {
			return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, fmt.Sprintf("inventory item %d not found", disposalItem.InventoryItemID), nil)
		}

		// Check if there's sufficient quantity to dispose
		if item.Quantity < disposalItem.Quantity {
			return nil, pkg.NewAppError(pkg.ErrorCodeValidation, fmt.Sprintf("insufficient quantity for inventory item %d. Available: %d, Requested: %d", disposalItem.InventoryItemID, item.Quantity, disposalItem.Quantity), nil)
		}

		// Reduce the quantity in memory
		item.Quantity -= disposalItem.Quantity

		// Add to updated items list
		updatedItems = append(updatedItems, item)

		// Create inventory transaction record
		transaction := models.InventoryTransaction{
			InventoryItemID: item.ID,
			TransactionType: models.InventoryTransactionTypeDisposal,
			Quantity:        -disposalItem.Quantity, // Negative for disposal
		}
		inventoryTransactions = append(inventoryTransactions, &transaction)
	}

	// Batch update inventory items and create transactions
	if err := s.inventoryItemRepo.Update(ctx, updatedItems, inventoryTransactions); err != nil {
		return nil, fmt.Errorf("failed to update inventory items and create transactions: %w", err)
	}

	return updatedItems, nil
}

func (s *inventoryService) ConfirmInventory(ctx context.Context, req dto.ConfirmInventoryRequest) ([]models.InventoryItem, error) {

	return nil, nil
}
