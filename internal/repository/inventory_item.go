package repository

import (
	"context"
	"fmt"
	"import-export-backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InventoryItemRepository interface {
	Create(ctx context.Context, item *models.InventoryItem) error
	GetByID(ctx context.Context, id uint) (*models.InventoryItem, error)
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, limit, offset int) ([]models.InventoryItem, error)
	GetByInventoryID(ctx context.Context, inventoryID uint, limit, offset int) ([]models.InventoryItem, error)
	GetByProductID(ctx context.Context, productID uint) (*models.InventoryItem, error)
	GetLowStockItems(ctx context.Context, limit, offset int) ([]models.InventoryItem, error)
	Count(ctx context.Context) (int64, error)
	CountByInventoryID(ctx context.Context, inventoryID uint) (int64, error)
	CountLowStockItems(ctx context.Context) (int64, error)

	// v1
	Update(ctx context.Context, items []*models.InventoryItem, transactions []*models.InventoryTransaction) error
	GetActiveItemsByInventoryIDs(ctx context.Context, inventoryIDs []uint) ([]*models.InventoryItem, error)
	PersistReconciliation(ctx context.Context, items []*models.InventoryItem, sellTransactions []*models.InventoryTransaction) error
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

func (r *inventoryItemRepository) Update(
	ctx context.Context,
	items []*models.InventoryItem,
	transactions []*models.InventoryTransaction,
) error {
	return r.db.WithContext(ctx).Save(items).Error
}

// GetActiveItemsByInventoryIDs returns active inventory items for the given inventory IDs
func (r *inventoryItemRepository) GetActiveItemsByInventoryIDs(ctx context.Context, inventoryIDs []uint) ([]*models.InventoryItem, error) {
	var items []*models.InventoryItem
	return items, r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.
			Preload("Inventory").
			Preload("Product").
			Preload("Supplier").
			Where("inventory_id IN ?", inventoryIDs).
			Where("quantity > 0").
			Find(&items).Error
		if err != nil {
			return fmt.Errorf("failed to get active inventory items by inventory IDs: %w", err)
		}

		// Return empty slice instead of error when no items found
		if len(items) == 0 {
			return nil
		}

		itemIDs := make([]uint, len(items))
		for i, item := range items {
			itemIDs[i] = item.ID
		}

		// Fetch all purchase transactions for these items with the date condition using JOIN
		var transactions []*models.InventoryTransaction
		err = tx.
			Table("inventory_transactions").
			Joins("JOIN inventory_items ON inventory_items.id = inventory_transactions.inventory_item_id").
			Where("inventory_transactions.inventory_item_id IN ?", itemIDs).
			Where("inventory_transactions.transaction_type = ?", models.InventoryTransactionTypePurchase).
			Where("inventory_transactions.created_at >= COALESCE(inventory_items.latest_active_purchase_at, '-infinity'::timestamptz)").
			Order("inventory_transactions.created_at ASC"). // @todo: add Order test
			Find(&transactions).Error
		if err != nil {
			return fmt.Errorf("failed to get active purchase transactions: %w", err)
		}

		transactionMap := make(map[uint][]*models.InventoryTransaction)
		for _, transaction := range transactions {
			transactionMap[transaction.InventoryItemID] = append(transactionMap[transaction.InventoryItemID], transaction)
		}
		for _, item := range items {
			item.ActivePurchaseTransactions = transactionMap[item.ID]
		}

		return nil
	})
}

// PersistReconciliation persists inventory items and sell transactions with transaction safety
func (r *inventoryItemRepository) PersistReconciliation(ctx context.Context, items []*models.InventoryItem, sellTransactions []*models.InventoryTransaction) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Step 1: Fetch current inventory items with FOR UPDATE to prevent concurrent modifications
		itemIDs := make([]uint, len(items))
		for i, item := range items {
			itemIDs[i] = item.ID
		}

		var currentItems []*models.InventoryItem
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id IN ?", itemIDs).
			Find(&currentItems).Error
		if err != nil {
			return fmt.Errorf("failed to fetch inventory items for update: %w", err)
		}

		// Create a map for quick lookup of current items
		currentItemMap := make(map[uint]*models.InventoryItem)
		for _, item := range currentItems {
			currentItemMap[item.ID] = item
		}

		// Step 2: Validate that no quantities have been changed by other transactions
		for _, updatedItem := range items {
			currentItem, exists := currentItemMap[updatedItem.ID]
			if !exists {
				return fmt.Errorf("inventory item with ID %d not found", updatedItem.ID)
			}

			// Check if quantity has been modified by another transaction
			if currentItem.Quantity != updatedItem.Quantity {
				return fmt.Errorf("inventory item %d quantity has been modified by another transaction. Current: %d, Expected: %d",
					updatedItem.ID, currentItem.Quantity, updatedItem.Quantity)
			}
		}

		// Step 3: Persist updated inventory items
		if len(items) > 0 {
			err = tx.Save(items).Error
			if err != nil {
				return fmt.Errorf("failed to persist inventory items: %w", err)
			}
		}

		// Step 4: Persist sell transactions
		if len(sellTransactions) > 0 {
			err = tx.Create(sellTransactions).Error
			if err != nil {
				return fmt.Errorf("failed to persist sell transactions: %w", err)
			}
		}

		return nil
	})
}
