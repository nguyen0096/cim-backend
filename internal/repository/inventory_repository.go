package repository

import (
	"context"
	"import-export-backend/internal/models"

	"gorm.io/gorm"
)

type InventoryRepository interface {
	Create(ctx context.Context, inventory *models.Inventory) error
	GetByID(ctx context.Context, id uint) (*models.Inventory, error)
	GetByProductID(ctx context.Context, productID uint) (*models.Inventory, error)
	Update(ctx context.Context, inventory *models.Inventory) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, limit, offset int) ([]models.Inventory, error)
	GetLowStock(ctx context.Context) ([]models.Inventory, error)
	GetTransactions(ctx context.Context, productID uint, limit, offset int) ([]models.InventoryTransaction, error)
	CreateTransaction(ctx context.Context, transaction *models.InventoryTransaction) error
	Count(ctx context.Context) (int64, error)
	CountTransactions(ctx context.Context, productID uint) (int64, error)
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
	err := r.db.WithContext(ctx).Preload("Product").First(&inventory, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &inventory, nil
}

func (r *inventoryRepository) GetByProductID(ctx context.Context, productID uint) (*models.Inventory, error) {
	var inventory models.Inventory
	err := r.db.WithContext(ctx).Preload("Product").First(&inventory, "product_id = ?", productID).Error
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
	err := r.db.WithContext(ctx).Preload("Product").Limit(limit).Offset(offset).Find(&inventories).Error
	return inventories, err
}

func (r *inventoryRepository) GetLowStock(ctx context.Context) ([]models.Inventory, error) {
	var inventories []models.Inventory
	err := r.db.WithContext(ctx).Preload("Product").Where("quantity <= reorder_level").Find(&inventories).Error
	return inventories, err
}

func (r *inventoryRepository) GetTransactions(ctx context.Context, productID uint, limit, offset int) ([]models.InventoryTransaction, error) {
	var transactions []models.InventoryTransaction
	query := r.db.WithContext(ctx).Preload("Product")
	if productID != 0 {
		query = query.Where("product_id = ?", productID)
	}
	err := query.Limit(limit).Offset(offset).Order("created_at DESC").Find(&transactions).Error
	return transactions, err
}

func (r *inventoryRepository) CreateTransaction(ctx context.Context, transaction *models.InventoryTransaction) error {
	return r.db.WithContext(ctx).Create(transaction).Error
}

func (r *inventoryRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Inventory{}).Count(&count).Error
	return count, err
}

func (r *inventoryRepository) CountTransactions(ctx context.Context, productID uint) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&models.InventoryTransaction{})
	if productID != 0 {
		query = query.Where("product_id = ?", productID)
	}
	err := query.Count(&count).Error
	return count, err
}
