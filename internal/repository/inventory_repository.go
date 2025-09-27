package repository

import (
	"import-export-backend/internal/models"

	"gorm.io/gorm"
)

type InventoryRepository interface {
	Create(inventory *models.Inventory) error
	GetByID(id uint) (*models.Inventory, error)
	GetByProductID(productID uint) (*models.Inventory, error)
	Update(inventory *models.Inventory) error
	Delete(id uint) error
	List(limit, offset int) ([]models.Inventory, error)
	GetLowStock() ([]models.Inventory, error)
	GetTransactions(productID uint, limit, offset int) ([]models.InventoryTransaction, error)
	CreateTransaction(transaction *models.InventoryTransaction) error
	Count() (int64, error)
	CountTransactions(productID uint) (int64, error)
}

type inventoryRepository struct {
	db *gorm.DB
}

func NewInventoryRepository(db *gorm.DB) InventoryRepository {
	return &inventoryRepository{db: db}
}

func (r *inventoryRepository) Create(inventory *models.Inventory) error {
	return r.db.Create(inventory).Error
}

func (r *inventoryRepository) GetByID(id uint) (*models.Inventory, error) {
	var inventory models.Inventory
	err := r.db.Preload("Product").First(&inventory, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &inventory, nil
}

func (r *inventoryRepository) GetByProductID(productID uint) (*models.Inventory, error) {
	var inventory models.Inventory
	err := r.db.Preload("Product").First(&inventory, "product_id = ?", productID).Error
	if err != nil {
		return nil, err
	}
	return &inventory, nil
}

func (r *inventoryRepository) Update(inventory *models.Inventory) error {
	return r.db.Save(inventory).Error
}

func (r *inventoryRepository) Delete(id uint) error {
	return r.db.Delete(&models.Inventory{}, "id = ?", id).Error
}

func (r *inventoryRepository) List(limit, offset int) ([]models.Inventory, error) {
	var inventories []models.Inventory
	err := r.db.Preload("Product").Limit(limit).Offset(offset).Find(&inventories).Error
	return inventories, err
}

func (r *inventoryRepository) GetLowStock() ([]models.Inventory, error) {
	var inventories []models.Inventory
	err := r.db.Preload("Product").Where("quantity <= reorder_level").Find(&inventories).Error
	return inventories, err
}

func (r *inventoryRepository) GetTransactions(productID uint, limit, offset int) ([]models.InventoryTransaction, error) {
	var transactions []models.InventoryTransaction
	query := r.db.Preload("Product")
	if productID != 0 {
		query = query.Where("product_id = ?", productID)
	}
	err := query.Limit(limit).Offset(offset).Order("created_at DESC").Find(&transactions).Error
	return transactions, err
}

func (r *inventoryRepository) CreateTransaction(transaction *models.InventoryTransaction) error {
	return r.db.Create(transaction).Error
}

func (r *inventoryRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.Inventory{}).Count(&count).Error
	return count, err
}

func (r *inventoryRepository) CountTransactions(productID uint) (int64, error) {
	var count int64
	query := r.db.Model(&models.InventoryTransaction{})
	if productID != 0 {
		query = query.Where("product_id = ?", productID)
	}
	err := query.Count(&count).Error
	return count, err
}
