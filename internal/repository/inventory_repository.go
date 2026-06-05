package repository

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

//go:generate mockery --name=InventoryRepository --structname=InventoryRepository --output=../mocks/repositorymocks --outpkg=repositorymocks
type InventoryRepository interface {
	Create(ctx context.Context, inventory *models.Inventory) error
	Update(ctx context.Context, inventory *models.Inventory) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, limit, offset int) ([]models.Inventory, error)
	AddInventory(ctx context.Context, productID uint, quantity decimal.Decimal, referenceID uint, referenceType string) error
	RemoveInventory(ctx context.Context, productID uint, quantity decimal.Decimal, referenceID uint, referenceType string) error

	// v1 - AGENTS MUST CONFIRM BEFORE MODIFYING SECTION BELOW THIS LINE

	GetByID(ctx context.Context, id uint) (*models.Inventory, error)
	GetByName(ctx context.Context, name string) (*models.Inventory, error)
	GetLastPurchasePrices(ctx context.Context, supplierID uint, limit uint) ([]*dto.LastPurchasePriceResponse, error)

	GetTransactionsByInventoryItemIDs(ctx context.Context, inventoryItemIDs []uint) ([]models.InventoryTransaction, error)
	// GetTransactionsByInventoryIDs returns transactions for an inventory in [from, to).
	// When itemIDs is non-empty the result is further scoped to those inventory_item ids
	// (used to fetch only a page's worth of items); empty itemIDs means the whole inventory.
	GetTransactionsByInventoryIDs(ctx context.Context, inventoryID uint, from, to *time.Time, itemIDs ...uint) ([]*models.InventoryTransaction, error)
	GetTransactionsByIDs(ctx context.Context, txnIDs []uint) ([]*models.InventoryTransaction, error)

	// GetTransactionsByInventoryIDsWithCounter returns transactions for an inventory in [from, to)
	// along with the counter transaction's purchase_order_item_id (if any). For sells, disposals,
	// and transfers, this exposes the originating purchase POI without a follow-up query — the
	// timeline service uses it to attribute every txn to its source PO.
	// When itemIDs is non-empty the result is scoped to those inventory_item ids.
	GetTransactionsByInventoryIDsWithCounter(ctx context.Context, inventoryID uint, from, to *time.Time, itemIDs ...uint) ([]*InventoryTransactionWithCounter, error)
}

// InventoryTransactionWithCounter is an InventoryTransaction with its counter transaction's
// purchase_order_item_id resolved in the same query.
type InventoryTransactionWithCounter struct {
	*models.InventoryTransaction
	CounterPOIID *uint
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
	err := r.db.WithContext(ctx).Preload("Items").Preload("Items.Product").Preload("Items.Unit").First(&inventory, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &inventory, nil
}

// GetByName retrieves an inventory by name (case-insensitive)
func (r *inventoryRepository) GetByName(ctx context.Context, name string) (*models.Inventory, error) {
	var inventory models.Inventory
	err := r.db.WithContext(ctx).
		Where("LOWER(name) = LOWER(?)", name).
		First(&inventory).Error
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

func (r *inventoryRepository) AddInventory(ctx context.Context, productID uint, quantity decimal.Decimal, referenceID uint, referenceType string) error {
	// Find the inventory item for this product
	var inventoryItem models.InventoryItem
	err := r.db.WithContext(ctx).Where("product_id = ?", productID).First(&inventoryItem).Error
	if err != nil {
		return err
	}

	// Update the quantity
	inventoryItem.Quantity = inventoryItem.Quantity.Add(quantity)

	return r.db.WithContext(ctx).Save(&inventoryItem).Error
}

func (r *inventoryRepository) RemoveInventory(ctx context.Context, productID uint, quantity decimal.Decimal, referenceID uint, referenceType string) error {
	// Find the inventory item for this product
	var inventoryItem models.InventoryItem
	err := r.db.WithContext(ctx).Where("product_id = ?", productID).First(&inventoryItem).Error
	if err != nil {
		return err
	}

	// Check if there's enough inventory
	if inventoryItem.Quantity.LessThan(quantity) {
		return fmt.Errorf("insufficient inventory: available %s, requested %s", inventoryItem.Quantity.String(), quantity.String())
	}

	// Update the quantity
	inventoryItem.Quantity = inventoryItem.Quantity.Sub(quantity)

	return r.db.WithContext(ctx).Save(&inventoryItem).Error
}

func (r *inventoryRepository) GetTransactionsByInventoryItemIDs(ctx context.Context, inventoryItemIDs []uint) ([]models.InventoryTransaction, error) {
	var transactions []models.InventoryTransaction
	err := r.db.WithContext(ctx).Where("inventory_item_id IN ?", inventoryItemIDs).Find(&transactions).Error
	return transactions, err
}

// GetLastPurchasePrices retrieves the most recent purchase transaction prices for each product_id + supplier_id combination
func (r *inventoryRepository) GetLastPurchasePrices(ctx context.Context, supplierID uint, limit uint) ([]*dto.LastPurchasePriceResponse, error) {
	var results []*dto.LastPurchasePriceResponse

	// First subquery: get the latest created_at for each distinct price per product-supplier pair
	distinctPricesSubquery := r.db.WithContext(ctx).
		Table("inventory_transactions AS it").
		Select(`
			ii.product_id,
			it.supplier_id,
			it.price,
			MAX(it.created_at) AS latest_created_at
		`).
		Joins("INNER JOIN inventory_items AS ii ON it.inventory_item_id = ii.id").
		Joins("INNER JOIN products AS p ON ii.product_id = p.id").
		Joins("INNER JOIN suppliers AS s ON it.supplier_id = s.id").
		Where("it.transaction_type = ?", models.InventoryTransactionTypePurchase).
		Where("p.status = ?", "active").
		Where("s.status = ?", "active")

	// Filter by supplier_id if provided
	if supplierID > 0 {
		distinctPricesSubquery = distinctPricesSubquery.Where("it.supplier_id = ?", supplierID)
	}

	distinctPricesSubquery = distinctPricesSubquery.Group("ii.product_id, it.supplier_id, it.price")

	// Second subquery: rank distinct prices by their latest occurrence
	rankedSubquery := r.db.WithContext(ctx).
		Table("(?) AS distinct_prices", distinctPricesSubquery).
		Select(`
			product_id,
			supplier_id,
			price,
			latest_created_at,
			ROW_NUMBER() OVER (PARTITION BY product_id, supplier_id ORDER BY latest_created_at DESC) AS rn
		`)

	// Main query: select only top N distinct prices per product-supplier pair
	query := r.db.WithContext(ctx).
		Table("(?) AS ranked", rankedSubquery).
		Select(`
			product_id,
			supplier_id,
			price AS last_price,
			latest_created_at AS last_purchase_date
		`).
		Where("rn <= ?", limit).
		Order("product_id, supplier_id, latest_created_at DESC")

	err := query.Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get last purchase prices: %w", err)
	}

	return results, nil
}

func (r *inventoryRepository) GetTransactionsByInventoryIDs(ctx context.Context, inventoryID uint, from, to *time.Time, itemIDs ...uint) ([]*models.InventoryTransaction, error) {
	var txns []*models.InventoryTransaction
	q := r.db.WithContext(ctx).
		Table("inventory_transactions it").
		Joins("INNER JOIN inventory_items ii ON it.inventory_item_id = ii.id").
		Where("ii.inventory_id = ?", inventoryID)

	if len(itemIDs) > 0 {
		q = q.Where("ii.id IN ?", itemIDs)
	}

	if from != nil {
		q = q.Where("it.created_at >= ?", from)
	}

	if to != nil {
		q = q.Where("it.created_at < ?", to)
	}

	if err := q.Find(&txns).Error; err != nil {
		return nil, fmt.Errorf("failed to get inventory transactions before date: %w", err)
	}
	return txns, nil
}

func (r *inventoryRepository) GetTransactionsByInventoryIDsWithCounter(ctx context.Context, inventoryID uint, from, to *time.Time, itemIDs ...uint) ([]*InventoryTransactionWithCounter, error) {
	type row struct {
		models.InventoryTransaction
		CounterPOIID *uint `gorm:"column:counter_poi_id"`
	}

	// Soft-delete filters: it/ii in WHERE (INNER JOIN-equivalent), counter in JOIN
	// ON clause to preserve LEFT JOIN semantics (soft-deleted counter → main txn
	// still returned, CounterPOIID is nil).
	q := r.db.WithContext(ctx).
		Table("inventory_transactions it").
		Select("it.*, counter.purchase_order_item_id AS counter_poi_id").
		Joins("INNER JOIN inventory_items ii ON it.inventory_item_id = ii.id AND ii.deleted_at IS NULL").
		Joins("LEFT JOIN inventory_transactions counter ON counter.id = it.counter_transaction_id AND counter.deleted_at IS NULL").
		Where("ii.inventory_id = ? AND it.deleted_at IS NULL", inventoryID)

	if len(itemIDs) > 0 {
		q = q.Where("ii.id IN ?", itemIDs)
	}

	if from != nil {
		q = q.Where("it.created_at >= ?", from)
	}
	if to != nil {
		q = q.Where("it.created_at < ?", to)
	}

	var rows []row
	if err := q.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to get inventory transactions with counter: %w", err)
	}

	result := make([]*InventoryTransactionWithCounter, 0, len(rows))
	for i := range rows {
		txn := rows[i].InventoryTransaction
		result = append(result, &InventoryTransactionWithCounter{
			InventoryTransaction: &txn,
			CounterPOIID:         rows[i].CounterPOIID,
		})
	}
	return result, nil
}

func (r *inventoryRepository) GetTransactionsByIDs(ctx context.Context, txnIDs []uint) ([]*models.InventoryTransaction, error) {
	if len(txnIDs) == 0 {
		return []*models.InventoryTransaction{}, nil
	}

	var txns []*models.InventoryTransaction
	if err := r.db.WithContext(ctx).
		Where("id IN ?", txnIDs).
		Preload("InventoryItem").
		Preload("InventoryItem.Inventory").
		Preload("InventoryItem.Product").
		Preload("InventoryItem.Unit").
		Find(&txns).Error; err != nil {
		return nil, fmt.Errorf("failed to get transactions by IDs: %w", err)
	}
	return txns, nil
}
