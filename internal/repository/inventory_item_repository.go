package repository

import (
	"context"
	"import-export-backend/internal/models"

	"gorm.io/gorm"
)

type InventoryItemRepository interface {
	Create(ctx context.Context, item *models.InventoryItem) error
	GetByID(ctx context.Context, id uint) (*models.InventoryItem, error)
	Update(ctx context.Context, item *models.InventoryItem) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, limit, offset int) ([]models.InventoryItem, error)
	GetByInventoryID(ctx context.Context, inventoryID uint, limit, offset int) ([]models.InventoryItem, error)
	GetByProductID(ctx context.Context, productID uint) (*models.InventoryItem, error)
	GetLowStockItems(ctx context.Context, limit, offset int) ([]models.InventoryItem, error)
	Count(ctx context.Context) (int64, error)
	CountByInventoryID(ctx context.Context, inventoryID uint) (int64, error)
	CountLowStockItems(ctx context.Context) (int64, error)
}

type inventoryItemRepository struct {
	db *gorm.DB
}

func NewInventoryItemRepository(db *gorm.DB) InventoryItemRepository {
	return &inventoryItemRepository{db: db}
}

func (r *inventoryItemRepository) Create(ctx context.Context, item *models.InventoryItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *inventoryItemRepository) GetByID(ctx context.Context, id uint) (*models.InventoryItem, error) {
	var item models.InventoryItem
	err := r.db.WithContext(ctx).
		Preload("Inventory").
		Preload("Product").
		First(&item, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *inventoryItemRepository) Update(ctx context.Context, item *models.InventoryItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *inventoryItemRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.InventoryItem{}, "id = ?", id).Error
}

func (r *inventoryItemRepository) List(ctx context.Context, limit, offset int) ([]models.InventoryItem, error) {
	var items []models.InventoryItem
	err := r.db.WithContext(ctx).
		Preload("Inventory").
		Preload("Product").
		Limit(limit).
		Offset(offset).
		Find(&items).Error
	return items, err
}

func (r *inventoryItemRepository) GetByInventoryID(ctx context.Context, inventoryID uint, limit, offset int) ([]models.InventoryItem, error) {
	var items []models.InventoryItem
	err := r.db.WithContext(ctx).
		Preload("Inventory").
		Preload("Product").
		Where("inventory_id = ?", inventoryID).
		Limit(limit).
		Offset(offset).
		Find(&items).Error
	return items, err
}

func (r *inventoryItemRepository) GetByProductID(ctx context.Context, productID uint) (*models.InventoryItem, error) {
	var item models.InventoryItem
	err := r.db.WithContext(ctx).
		Preload("Inventory").
		Preload("Product").
		First(&item, "product_id = ?", productID).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *inventoryItemRepository) GetLowStockItems(ctx context.Context, limit, offset int) ([]models.InventoryItem, error) {
	var items []models.InventoryItem
	err := r.db.WithContext(ctx).
		Preload("Inventory").
		Preload("Product").
		Where("quantity <= reorder_level").
		Limit(limit).
		Offset(offset).
		Find(&items).Error
	return items, err
}

func (r *inventoryItemRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.InventoryItem{}).Count(&count).Error
	return count, err
}

func (r *inventoryItemRepository) CountByInventoryID(ctx context.Context, inventoryID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.InventoryItem{}).
		Where("inventory_id = ?", inventoryID).
		Count(&count).Error
	return count, err
}

func (r *inventoryItemRepository) CountLowStockItems(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.InventoryItem{}).
		Where("quantity <= reorder_level").
		Count(&count).Error
	return count, err
}
