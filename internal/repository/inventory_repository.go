package repository

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"context"
	"fmt"

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
	GetLastPurchasePrices(ctx context.Context) ([]*dto.LastPurchasePriceResponse, error)
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

// GetLastPurchasePrices retrieves the last purchase transaction price for each product_id + supplier_id combination
func (r *inventoryRepository) GetLastPurchasePrices(ctx context.Context) ([]*dto.LastPurchasePriceResponse, error) {
	var results []*dto.LastPurchasePriceResponse

	err := r.db.WithContext(ctx).
		Table("inventory_transactions AS it").
		Select(`
			ii.product_id,
			it.supplier_id,
			it.price AS last_price,
			it.created_at AS last_purchase_date
		`).
		Joins("INNER JOIN inventory_items AS ii ON it.inventory_item_id = ii.id").
		Joins("INNER JOIN products AS p ON ii.product_id = p.id").
		Joins("INNER JOIN suppliers AS s ON it.supplier_id = s.id").
		Joins(`INNER JOIN (
			SELECT ii2.product_id, it2.supplier_id, MAX(it2.created_at) AS max_created_at
			FROM inventory_transactions AS it2
			INNER JOIN inventory_items AS ii2 ON it2.inventory_item_id = ii2.id
			WHERE it2.transaction_type = ?
			GROUP BY ii2.product_id, it2.supplier_id
		) AS latest ON ii.product_id = latest.product_id
			AND it.supplier_id = latest.supplier_id
			AND it.created_at = latest.max_created_at`, models.InventoryTransactionTypePurchase).
		Where("it.transaction_type = ?", models.InventoryTransactionTypePurchase).
		Where("p.status = ?", "active").
		Where("s.status = ?", "active").
		Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get last purchase prices: %w", err)
	}

	return results, nil
}
