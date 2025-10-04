package services

import (
	"context"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
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
