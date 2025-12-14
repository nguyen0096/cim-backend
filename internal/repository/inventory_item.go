package repository

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
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
	Search      string
	Sort        string
	Order       string
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

	// v1 - AGENTS MUST CONFIRM BEFORE MODIFYING SECTION BELOW THIS LINE
	GetByInventoryIDWithFilters(ctx context.Context, inventoryID uint, filters InventoryItemFilters, limit, offset int) ([]models.InventoryItem, error)
	CountByInventoryIDWithFilters(ctx context.Context, inventoryID uint, filters InventoryItemFilters) (int64, error)
	GetActiveInventoryItems(ctx context.Context, inventoryID uint, ids []uint) ([]*models.InventoryItem, error)
	GetActiveInventoryItemsByProductIDs(ctx context.Context, inventoryID uint, productIDs []uint) ([]*models.InventoryItem, error)
	GetByIDs(ctx context.Context, ids []uint) ([]*models.InventoryItem, error)
	Update(ctx context.Context, items []*models.InventoryItem, transactions []*models.InventoryTransaction) error
	SaveInventoryItemChanges(ctx context.Context, items []*models.InventoryItemChange, transactions []*models.InventoryTransaction) error
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
		Preload("Unit").
		Where("inventory_items.inventory_id = ?", inventoryID)

	// Apply status filter
	if filters.Status != "" {
		query = query.Where("inventory_items.status = ?", filters.Status)
	}

	// Determine if we need to join products table
	needsProductJoin := filters.ProductType != "" || filters.Sort == string(InventoryItemSortFieldProductName) || filters.Search != ""

	// Apply product_type filter by joining with products table
	if needsProductJoin {
		query = query.Joins("JOIN products ON products.id = inventory_items.product_id")
		if filters.ProductType != "" {
			query = query.Where("products.product_type = ?", filters.ProductType)
		}
	}

	// Apply search filter
	if filters.Search != "" {
		searchPattern := "%" + filters.Search + "%"
		query = query.Where("products.name ILIKE ?", searchPattern)
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

	// Apply product_type filter and search filter by joining with products table
	needsProductJoin := filters.ProductType != "" || filters.Search != ""
	if needsProductJoin {
		query = query.Joins("JOIN products ON products.id = inventory_items.product_id")
		if filters.ProductType != "" {
			query = query.Where("products.product_type = ?", filters.ProductType)
		}
		if filters.Search != "" {
			searchPattern := "%" + filters.Search + "%"
			query = query.Where("products.name ILIKE ?", searchPattern)
		}
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

// GetActiveInventoryItems returns active inventory items for the given inventory ID and item IDs
func (r *inventoryItemRepository) GetActiveInventoryItems(
	ctx context.Context,
	inventoryID uint,
	ids []uint,
) ([]*models.InventoryItem, error) {
	var items []*models.InventoryItem
	return items, r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.
			Preload("Inventory").
			Preload("Product").
			Where("inventory_id = ?", inventoryID).
			Where("id IN ?", ids).
			Where("status = ?", models.InventoryItemStatusActive).
			Find(&items).Error
		if err != nil {
			return fmt.Errorf("failed to get active inventory items by inventory IDs: %w", err)
		}

		if len(items) == 0 {
			return pkg.ErrNotFound("no active inventory items found", nil)
		}

		err = r.populateConsumableTransactions(tx, items)
		if err != nil {
			return fmt.Errorf("failed to populate consumable transactions: %w", err)
		}
		return nil
	})
}

// GetActiveInventoryItemsByProductIDs returns active inventory items for the given inventory ID and product IDs.
func (r *inventoryItemRepository) GetActiveInventoryItemsByProductIDs(
	ctx context.Context,
	inventoryID uint,
	productIDs []uint,
) ([]*models.InventoryItem, error) {
	var items []*models.InventoryItem
	return items, r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.
			Preload("Inventory").
			Preload("Product").
			Where("inventory_id = ?", inventoryID).
			Where("product_id IN ?", productIDs).
			Where("status = ?", models.InventoryItemStatusActive).
			Find(&items).Error
		if err != nil {
			return fmt.Errorf("failed to get active inventory items by inventory IDs: %w", err)
		}

		if len(items) == 0 {
			return pkg.ErrNotFound("no active inventory items found", nil)
		}

		err = r.populateConsumableTransactions(tx, items)
		if err != nil {
			return fmt.Errorf("failed to populate consumable transactions: %w", err)
		}
		return nil
	})
}

func (r *inventoryItemRepository) populateConsumableTransactions(
	tx *gorm.DB,
	items []*models.InventoryItem,
) error {
	// Fetch consumable transactions for the active inventory items.
	var transactions []*models.InventoryTransaction
	err := tx.
		Table("inventory_transactions").
		Joins("JOIN inventory_items ON inventory_items.id = inventory_transactions.inventory_item_id").
		Where("inventory_transactions.inventory_item_id IN ?",
			models.GetIDs(items)).
		Where("inventory_transactions.transaction_type IN ?",
			models.GetConsumableTransactionTypes()).
		Where("inventory_transactions.id >= COALESCE(inventory_items.consuming_transaction_id, 0)").
		Where("inventory_transactions.consumed_quantity < inventory_transactions.quantity").
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
		item.ConsumableTransactions = transactionMap[item.ID]
	}
	return nil
}

// GetByIDs retrieves inventory items by IDs, including soft-deleted records
func (r *inventoryItemRepository) GetByIDs(ctx context.Context, ids []uint) ([]*models.InventoryItem, error) {
	if len(ids) == 0 {
		return []*models.InventoryItem{}, nil
	}

	var items []*models.InventoryItem
	err := r.db.WithContext(ctx).
		Unscoped().
		Preload("Inventory").
		Preload("Product").
		Preload("Unit").
		Where("id IN ?", ids).
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory items by IDs: %w", err)
	}

	return items, nil
}

// updateInventoryItems updates inventory items with optimistic locking.
func (r *inventoryItemRepository) updateInventoryItems(
	ctx context.Context,
	db *gorm.DB,
	changes []*models.InventoryItemChange,
) error {
	if len(changes) == 0 {
		return nil
	}

	var existingItems []*models.InventoryItem
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", models.GetIDs(changes)).
		Find(&existingItems).Error
	if err != nil {
		return fmt.Errorf("failed to fetch inventory items for update: %w", err)
	}

	existingItemMap := models.BuildIDMap(existingItems)
	for _, change := range changes {
		if change.InventoryItem.IsNew() {
			continue
		}

		existingItem, exists := existingItemMap[change.ID]
		if !exists {
			return fmt.Errorf("inventory item with ID %d not found", change.ID)
		}

		if !existingItem.Quantity.Equal(change.OriginalQuantity) {
			return pkg.ErrOptimisticLockConflict(ctx, "inventory item", change.ID, change.OriginalQuantity, existingItem.Quantity)
		}
	}

	ivtrItems := make([]*models.InventoryItem, len(changes))
	for i, change := range changes {
		ivtrItems[i] = change.InventoryItem
	}
	err = db.Save(ivtrItems).Error
	if err != nil {
		return fmt.Errorf("failed to update inventory items: %w", err)
	}
	return nil
}

func (r *inventoryItemRepository) updateTransactions(
	db *gorm.DB,
	transactions []*models.InventoryTransaction,
) error {
	if len(transactions) == 0 {
		return nil
	}
	err := db.Save(transactions).Error
	if err != nil {
		return fmt.Errorf("failed to update transactions: %w", err)
	}
	return nil
}

func (r *inventoryItemRepository) SaveInventoryItemChanges(
	ctx context.Context,
	changes []*models.InventoryItemChange,
	txns []*models.InventoryTransaction,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := r.updateInventoryItems(ctx, tx, changes)
		if err != nil {
			return fmt.Errorf("failed to update inventory items: %w", err)
		}

		for _, txn := range txns {
			if txn.InventoryItem != nil && txn.InventoryItem.ID != 0 {
				txn.InventoryItemID = txn.InventoryItem.ID
			}
		}

		err = r.updateTransactions(tx, txns)
		if err != nil {
			return fmt.Errorf("failed to update transactions: %w", err)
		}
		return nil
	})
}
