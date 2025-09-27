package services

import (
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"

	"github.com/google/uuid"
)

//go:generate mockery --name=InventoryService --structname=InventoryService --output=./servicemocks --outpkg=servicemocks
type InventoryService interface {
	GetInventory(limit, offset int) ([]models.Inventory, error)
	GetInventoryByID(id uuid.UUID) (*models.Inventory, error)
	GetInventoryByProductID(productID uuid.UUID) (*models.Inventory, error)
	UpdateInventory(inventory *models.Inventory) error
	AdjustInventory(productID uuid.UUID, quantity int, notes string, userID string) error
	GetLowStock() ([]models.Inventory, error)
	GetTransactions(productID uuid.UUID, limit, offset int) ([]models.InventoryTransaction, error)
	AddInventory(productID uuid.UUID, quantity int, referenceID uuid.UUID, referenceType, notes string, userID string) error
	RemoveInventory(productID uuid.UUID, quantity int, referenceID uuid.UUID, referenceType, notes string, userID string) error
	CountInventory() (int64, error)
	CountTransactions(productID uuid.UUID) (int64, error)
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

func (s *inventoryService) GetInventory(limit, offset int) ([]models.Inventory, error) {
	return s.inventoryRepo.List(limit, offset)
}

func (s *inventoryService) GetInventoryByID(id uuid.UUID) (*models.Inventory, error) {
	return s.inventoryRepo.GetByID(id)
}

func (s *inventoryService) GetInventoryByProductID(productID uuid.UUID) (*models.Inventory, error) {
	return s.inventoryRepo.GetByProductID(productID)
}

func (s *inventoryService) UpdateInventory(inventory *models.Inventory) error {
	return s.inventoryRepo.Update(inventory)
}

func (s *inventoryService) AdjustInventory(productID uuid.UUID, quantity int, notes string, userID string) error {
	inventory, err := s.inventoryRepo.GetByProductID(productID)
	if err != nil {
		return err
	}

	// Create transaction record
	transaction := &models.InventoryTransaction{
		ProductID:       productID,
		TransactionType: "adjustment",
		Quantity:        quantity,
		Notes:           notes,
		CreatedBy:       userID,
	}

	if err := s.inventoryRepo.CreateTransaction(transaction); err != nil {
		return err
	}

	// Update inventory quantity
	inventory.Quantity += quantity
	return s.inventoryRepo.Update(inventory)
}

func (s *inventoryService) GetLowStock() ([]models.Inventory, error) {
	return s.inventoryRepo.GetLowStock()
}

func (s *inventoryService) GetTransactions(productID uuid.UUID, limit, offset int) ([]models.InventoryTransaction, error) {
	return s.inventoryRepo.GetTransactions(productID, limit, offset)
}

func (s *inventoryService) AddInventory(productID uuid.UUID, quantity int, referenceID uuid.UUID, referenceType, notes string, userID string) error {
	inventory, err := s.inventoryRepo.GetByProductID(productID)
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
		CreatedBy:       userID,
	}

	if err := s.inventoryRepo.CreateTransaction(transaction); err != nil {
		return err
	}

	// Update inventory quantity
	inventory.Quantity += quantity
	return s.inventoryRepo.Update(inventory)
}

func (s *inventoryService) RemoveInventory(productID uuid.UUID, quantity int, referenceID uuid.UUID, referenceType, notes string, userID string) error {
	inventory, err := s.inventoryRepo.GetByProductID(productID)
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
		CreatedBy:       userID,
	}

	if err := s.inventoryRepo.CreateTransaction(transaction); err != nil {
		return err
	}

	// Update inventory quantity
	inventory.Quantity -= quantity
	return s.inventoryRepo.Update(inventory)
}

// Custom error for insufficient inventory
type InsufficientInventoryError struct {
	Available int
	Requested int
}

func (e *InsufficientInventoryError) Error() string {
	return "insufficient inventory"
}

func (s *inventoryService) CountInventory() (int64, error) {
	return s.inventoryRepo.Count()
}

func (s *inventoryService) CountTransactions(productID uuid.UUID) (int64, error) {
	return s.inventoryRepo.CountTransactions(productID)
}
