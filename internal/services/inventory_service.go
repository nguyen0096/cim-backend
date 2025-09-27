package services

import (
	"context"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
)

//go:generate mockery --name=InventoryService --structname=InventoryService --output=./servicemocks --outpkg=servicemocks
type InventoryService interface {
	GetInventory(ctx context.Context, limit, offset int) ([]models.Inventory, error)
	GetInventoryByID(ctx context.Context, id uint) (*models.Inventory, error)
	GetInventoryByProductID(ctx context.Context, productID uint) (*models.Inventory, error)
	UpdateInventory(ctx context.Context, inventory *models.Inventory) error
	AdjustInventory(ctx context.Context, productID uint, quantity int, notes string) error
	GetLowStock(ctx context.Context) ([]models.Inventory, error)
	GetTransactions(ctx context.Context, productID uint, limit, offset int) ([]models.InventoryTransaction, error)
	AddInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType, notes string) error
	RemoveInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType, notes string) error
	CountInventory(ctx context.Context) (int64, error)
	CountTransactions(ctx context.Context, productID uint) (int64, error)
}

type inventoryService struct {
	inventoryRepo repository.InventoryRepository
	productRepo   repository.ProductRepository
}

func NewInventoryService(inventoryRepo repository.InventoryRepository, productRepo repository.ProductRepository) InventoryService {
	return &inventoryService{
		inventoryRepo: inventoryRepo,
		productRepo:   productRepo,
	}
}

func (s *inventoryService) GetInventory(ctx context.Context, limit, offset int) ([]models.Inventory, error) {
	return s.inventoryRepo.List(ctx, limit, offset)
}

func (s *inventoryService) GetInventoryByID(ctx context.Context, id uint) (*models.Inventory, error) {
	return s.inventoryRepo.GetByID(ctx, id)
}

func (s *inventoryService) GetInventoryByProductID(ctx context.Context, productID uint) (*models.Inventory, error) {
	return s.inventoryRepo.GetByProductID(ctx, productID)
}

func (s *inventoryService) UpdateInventory(ctx context.Context, inventory *models.Inventory) error {
	return s.inventoryRepo.Update(ctx, inventory)
}

func (s *inventoryService) AdjustInventory(ctx context.Context, productID uint, quantity int, notes string) error {
	inventory, err := s.inventoryRepo.GetByProductID(ctx, productID)
	if err != nil {
		return err
	}

	// Create transaction record
	transaction := &models.InventoryTransaction{
		ProductID:       productID,
		TransactionType: "adjustment",
		Quantity:        quantity,
		Notes:           notes,
	}

	if err := s.inventoryRepo.CreateTransaction(ctx, transaction); err != nil {
		return err
	}

	// Update inventory quantity
	inventory.Quantity += quantity
	return s.inventoryRepo.Update(ctx, inventory)
}

func (s *inventoryService) GetLowStock(ctx context.Context) ([]models.Inventory, error) {
	return s.inventoryRepo.GetLowStock(ctx)
}

func (s *inventoryService) GetTransactions(ctx context.Context, productID uint, limit, offset int) ([]models.InventoryTransaction, error) {
	return s.inventoryRepo.GetTransactions(ctx, productID, limit, offset)
}

func (s *inventoryService) AddInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType, notes string) error {
	inventory, err := s.inventoryRepo.GetByProductID(ctx, productID)
	if err != nil {
		return err
	}

	// Create transaction record
	transaction := &models.InventoryTransaction{
		ProductID:       productID,
		TransactionType: "purchase",
		Quantity:        quantity,
		ReferenceID:     referenceID,
		ReferenceType:   referenceType,
		Notes:           notes,
	}

	if err := s.inventoryRepo.CreateTransaction(ctx, transaction); err != nil {
		return err
	}

	// Update inventory quantity
	inventory.Quantity += quantity
	return s.inventoryRepo.Update(ctx, inventory)
}

func (s *inventoryService) RemoveInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType, notes string) error {
	inventory, err := s.inventoryRepo.GetByProductID(ctx, productID)
	if err != nil {
		return err
	}

	// Check if enough inventory
	if inventory.Quantity < quantity {
		return &InsufficientInventoryError{Available: inventory.Quantity, Requested: quantity}
	}

	// Create transaction record
	transaction := &models.InventoryTransaction{
		ProductID:       productID,
		TransactionType: "sale",
		Quantity:        -quantity, // Negative for removal
		ReferenceID:     referenceID,
		ReferenceType:   referenceType,
		Notes:           notes,
	}

	if err := s.inventoryRepo.CreateTransaction(ctx, transaction); err != nil {
		return err
	}

	// Update inventory quantity
	inventory.Quantity -= quantity
	return s.inventoryRepo.Update(ctx, inventory)
}

// Custom error for insufficient inventory
type InsufficientInventoryError struct {
	Available int
	Requested int
}

func (e *InsufficientInventoryError) Error() string {
	return "insufficient inventory"
}

func (s *inventoryService) CountInventory(ctx context.Context) (int64, error) {
	return s.inventoryRepo.Count(ctx)
}

func (s *inventoryService) CountTransactions(ctx context.Context, productID uint) (int64, error) {
	return s.inventoryRepo.CountTransactions(ctx, productID)
}
