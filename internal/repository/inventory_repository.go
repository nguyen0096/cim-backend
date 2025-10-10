package repository

import (
	"context"
	"fmt"
	"import-export-backend/internal/models"

	"gorm.io/gorm"
)

type InventoryRepository interface {
	Create(ctx context.Context, inventory *models.Inventory) error
	Update(ctx context.Context, inventory *models.Inventory) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, limit, offset int) ([]models.Inventory, error)
	AddInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType string) error
	RemoveInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType string) error

	// v1
	GetByID(ctx context.Context, id uint) (*models.Inventory, error)
	GetTransactionsByInventoryItemIDs(ctx context.Context, inventoryItemIDs []uint) ([]models.InventoryTransaction, error)
}

type inventoryRepository struct {
	db *gorm.DB
}

func NewInventoryRepository(db *gorm.DB) InventoryRepository {
	return &inventoryRepository{db: db}
}

func (r *inventoryRepository) Create(ctx context.Context, inventory *models.Inventory) error {
	return r.db.WithContext(ctx).Create(inventory).Error
}

func (r *inventoryRepository) GetByID(ctx context.Context, id uint) (*models.Inventory, error) {
	var inventory models.Inventory
	err := r.db.WithContext(ctx).Preload("Items").Preload("Items.Product").First(&inventory, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &inventory, nil
}

func (r *inventoryRepository) Update(ctx context.Context, inventory *models.Inventory) error {
	return r.db.WithContext(ctx).Save(inventory).Error
}

func (r *inventoryRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Inventory{}, "id = ?", id).Error
}

func (r *inventoryRepository) List(ctx context.Context, limit, offset int) ([]models.Inventory, error) {
	var inventories []models.Inventory
	err := r.db.WithContext(ctx).Preload("Items").Preload("Items.Product").Limit(limit).Offset(offset).Find(&inventories).Error
	return inventories, err
}

func (r *inventoryRepository) AddInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType string) error {
	// Find the inventory item for this product
	var inventoryItem models.InventoryItem
	err := r.db.WithContext(ctx).Where("product_id = ?", productID).First(&inventoryItem).Error
	if err != nil {
		return err
	}

	// Update the quantity
	inventoryItem.Quantity += quantity

	return r.db.WithContext(ctx).Save(&inventoryItem).Error
}

func (r *inventoryRepository) RemoveInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType string) error {
	// Find the inventory item for this product
	var inventoryItem models.InventoryItem
	err := r.db.WithContext(ctx).Where("product_id = ?", productID).First(&inventoryItem).Error
	if err != nil {
		return err
	}

	// Check if there's enough inventory
	if inventoryItem.Quantity < quantity {
		return fmt.Errorf("insufficient inventory: available %d, requested %d", inventoryItem.Quantity, quantity)
	}

	// Update the quantity
	inventoryItem.Quantity -= quantity

	return r.db.WithContext(ctx).Save(&inventoryItem).Error
}

func (r *inventoryRepository) GetTransactionsByInventoryItemIDs(ctx context.Context, inventoryItemIDs []uint) ([]models.InventoryTransaction, error) {
	var transactions []models.InventoryTransaction
	err := r.db.WithContext(ctx).Where("inventory_item_id IN ?", inventoryItemIDs).Find(&transactions).Error
	return transactions, err
}
