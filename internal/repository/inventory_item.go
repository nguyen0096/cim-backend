package repository

import (
	"cim-backend/internal/models"
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InventoryItemSortField string

const (
	InventoryItemSortFieldUpdatedAt   InventoryItemSortField = "inventory_items.updated_at"
	InventoryItemSortFieldCreatedAt   InventoryItemSortField = "inventory_items.created_at"
	InventoryItemSortFieldQuantity    InventoryItemSortField = "inventory_items.quantity"
	InventoryItemSortFieldProductName InventoryItemSortField = "products.name"
)

var (
	sortFieldCollationLookup = map[InventoryItemSortField]string{
		InventoryItemSortFieldProductName: "vi_vn",
	}
)

// InventoryItemFilters represents filters for inventory items
type InventoryItemFilters struct {
	Status      string
	ProductType string
	Sort        string
	Order       string
}

type PersistReconciliationItem struct {
	*models.InventoryItem
	OriginalQuantity int
}

type InventoryItemRepository interface {
	Create(ctx context.Context, item *models.InventoryItem) error
	GetByID(ctx context.Context, id uint) (*models.InventoryItem, error)
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, limit, offset int) ([]models.InventoryItem, error)
	GetByProductID(ctx context.Context, productID uint) (*models.InventoryItem, error)
	GetLowStockItems(ctx context.Context, limit, offset int) ([]models.InventoryItem, error)
	Count(ctx context.Context) (int64, error)
	CountLowStockItems(ctx context.Context) (int64, error)

	// v1
	GetByInventoryIDWithFilters(ctx context.Context, inventoryID uint, filters InventoryItemFilters, limit, offset int) ([]models.InventoryItem, error)
	CountByInventoryIDWithFilters(ctx context.Context, inventoryID uint, filters InventoryItemFilters) (int64, error)
	GetActiveInventoryItems(ctx context.Context, ids []uint) ([]*models.InventoryItem, error)
	GetByIDs(ctx context.Context, ids []uint) ([]*models.InventoryItem, error)
	Update(ctx context.Context, items []*models.InventoryItem, transactions []*models.InventoryTransaction) error
	PersistConsumption(ctx context.Context,
		reconcileItems []*PersistReconciliationItem,
		updateTransactions []*models.InventoryTransaction,
		sellTransactions []*models.InventoryTransaction) error
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

// GetByInventoryIDWithFilters retrieves inventory items by inventory ID with filters
func (r *inventoryItemRepository) GetByInventoryIDWithFilters(ctx context.Context, inventoryID uint, filters InventoryItemFilters, limit, offset int) ([]models.InventoryItem, error) {
	var items []models.InventoryItem
	query := r.db.WithContext(ctx).
		Preload("Inventory").
		Preload("Product").
		Where("inventory_items.inventory_id = ?", inventoryID)

	// Apply status filter
	if filters.Status != "" {
		query = query.Where("inventory_items.status = ?", filters.Status)
	}

	// Determine if we need to join products table
	needsProductJoin := filters.ProductType != "" || filters.Sort == string(InventoryItemSortFieldProductName)

	// Apply product_type filter by joining with products table
	if needsProductJoin {
		query = query.Joins("JOIN products ON products.id = inventory_items.product_id")
		if filters.ProductType != "" {
			query = query.Where("products.product_type = ?", filters.ProductType)
		}
	}

	// Apply sorting
	if filters.Sort != "" && filters.Order != "" {
		// apply collation to the sort field if defined
		collation := sortFieldCollationLookup[InventoryItemSortField(filters.Sort)]
		orderClauseParts := []string{filters.Sort}
		if collation != "" {
			orderClauseParts = append(orderClauseParts, fmt.Sprintf("COLLATE \"%s\"", collation))
		}
		orderClauseParts = append(orderClauseParts, filters.Order)

		orderClause := strings.Join(orderClauseParts, " ")
		query = query.Order(orderClause)
	}

	err := query.
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

// CountByInventoryIDWithFilters counts inventory items by inventory ID with filters
func (r *inventoryItemRepository) CountByInventoryIDWithFilters(ctx context.Context, inventoryID uint, filters InventoryItemFilters) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).
		Model(&models.InventoryItem{}).
		Where("inventory_items.inventory_id = ?", inventoryID)

	// Apply status filter
	if filters.Status != "" {
		query = query.Where("inventory_items.status = ?", filters.Status)
	}

	// Apply product_type filter by joining with products table
	if filters.ProductType != "" {
		query = query.Joins("JOIN products ON products.id = inventory_items.product_id").
			Where("products.product_type = ?", filters.ProductType)
	}

	err := query.Count(&count).Error
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

// GetActiveInventoryItemsByProductIDs returns active inventory items for the given inventory IDs
func (r *inventoryItemRepository) GetActiveInventoryItems(ctx context.Context, ids []uint) ([]*models.InventoryItem, error) {
	var items []*models.InventoryItem
	return items, r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.
			Preload("Inventory").
			Preload("Product").
			Where("id IN ?", ids).
			Where("quantity > 0").
			Where("status = ?", models.InventoryItemStatusActive).
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

		// Fetch all purchase transactions for these items with the ID condition using JOIN
		var transactions []*models.InventoryTransaction
		err = tx.
			Table("inventory_transactions").
			Joins("JOIN inventory_items ON inventory_items.id = inventory_transactions.inventory_item_id").
			Where("inventory_transactions.inventory_item_id IN ?", itemIDs).
			Where("inventory_transactions.transaction_type = ?", models.InventoryTransactionTypePurchase).
			Where("inventory_transactions.id >= COALESCE(inventory_items.consuming_transaction_id, 0)").
			Order("inventory_transactions.created_at ASC").
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

// GetByIDs retrieves inventory items by IDs
func (r *inventoryItemRepository) GetByIDs(ctx context.Context, ids []uint) ([]*models.InventoryItem, error) {
	if len(ids) == 0 {
		return []*models.InventoryItem{}, nil
	}

	var items []*models.InventoryItem
	err := r.db.WithContext(ctx).
		Preload("Inventory").
		Preload("Product").
		Where("id IN ?", ids).
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory items by IDs: %w", err)
	}

	return items, nil
}

// PersistConsumption persists inventory items and insert new transactions with transaction safety
func (r *inventoryItemRepository) PersistConsumption(ctx context.Context,
	reItems []*PersistReconciliationItem,
	updateTransactions []*models.InventoryTransaction,
	newTransactions []*models.InventoryTransaction) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Step 1: Fetch current inventory items with FOR UPDATE to prevent concurrent modifications
		itemIDs := make([]uint, len(reItems))
		for i, item := range reItems {
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
		for _, reItem := range reItems {
			currentItem, exists := currentItemMap[reItem.ID]
			if !exists {
				return fmt.Errorf("inventory item with ID %d not found", reItem.ID)
			}

			// Check if quantity has been modified by another transaction
			if currentItem.Quantity != reItem.OriginalQuantity {
				return fmt.Errorf("inventory item %d quantity has been modified by another transaction. Current: %d, Expected: %d",
					reItem.ID, currentItem.Quantity, reItem.Quantity)
			}
		}

		// Step 3: Persist updated inventory items
		if len(reItems) > 0 {
			updateItems := make([]*models.InventoryItem, len(reItems))
			for i, item := range reItems {
				updateItems[i] = item.InventoryItem
			}
			err = tx.Save(updateItems).Error
			if err != nil {
				return fmt.Errorf("failed to persist inventory items: %w", err)
			}
		}

		if len(updateTransactions) > 0 {
			err = tx.Save(updateTransactions).Error
			if err != nil {
				return fmt.Errorf("failed to persist update transactions: %w", err)
			}
		}

		// Step 4: Persist sell transactions
		if len(newTransactions) > 0 {
			err = tx.Create(newTransactions).Error
			if err != nil {
				return fmt.Errorf("failed to persist sell transactions: %w", err)
			}
		}

		return nil
	})
}
