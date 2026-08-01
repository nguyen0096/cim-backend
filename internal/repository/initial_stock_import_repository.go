package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"cim-backend/internal/models"
)

//go:generate mockery --name=InitialStockImportRepository --structname=InitialStockImportRepository --output=../mocks/repositorymocks --outpkg=repositorymocks
type InitialStockImportRepository interface {
	// GetReceipt returns the committed receipt for (inventoryID, key), or nil when none.
	GetReceipt(ctx context.Context, inventoryID uint, key string) (*models.InitialStockImport, error)
	// CreateReceipt records an applied run. Reports false when a concurrent run
	// already recorded the same (inventoryID, key), so the caller replays instead
	// of surfacing a unique-violation.
	CreateReceipt(ctx context.Context, receipt *models.InitialStockImport) (bool, error)
	// ExistsInitialForInventory reports whether any `initial` transaction already
	// backs stock in the inventory. Counts transactions of soft-deleted items too:
	// a soft-deleted item would otherwise hide a prior load and let a second run
	// double that product's stock.
	ExistsInitialForInventory(ctx context.Context, inventoryID uint) (bool, error)
	// CreateUnits and CreateProducts insert inside the caller's transaction. The
	// unit and product repositories' own Create paths hold the base connection, so
	// they would commit independently and leave a partial load behind on rollback.
	CreateUnits(ctx context.Context, units []*models.Unit) error
	CreateProducts(ctx context.Context, products []*models.Product) error
}

type initialStockImportRepository struct {
	*baseRepository
}

func NewInitialStockImportRepository(base BaseRepository) InitialStockImportRepository {
	return &initialStockImportRepository{baseRepository: asBase(base)}
}

func (r *initialStockImportRepository) GetReceipt(ctx context.Context, inventoryID uint, key string) (*models.InitialStockImport, error) {
	var receipt models.InitialStockImport
	err := r.DB(ctx).WithContext(ctx).
		Where("inventory_id = ? AND idempotency_key = ?", inventoryID, key).
		First(&receipt).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load initial stock import receipt: %w", err)
	}
	return &receipt, nil
}

func (r *initialStockImportRepository) CreateReceipt(ctx context.Context, receipt *models.InitialStockImport) (bool, error) {
	res := r.DB(ctx).WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "inventory_id"}, {Name: "idempotency_key"}},
		DoNothing: true,
	}).Create(receipt)
	if res.Error != nil {
		return false, fmt.Errorf("failed to record initial stock import receipt: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (r *initialStockImportRepository) CreateUnits(ctx context.Context, units []*models.Unit) error {
	if len(units) == 0 {
		return nil
	}
	if err := r.DB(ctx).WithContext(ctx).Create(units).Error; err != nil {
		return fmt.Errorf("failed to create units: %w", err)
	}
	return nil
}

func (r *initialStockImportRepository) CreateProducts(ctx context.Context, products []*models.Product) error {
	if len(products) == 0 {
		return nil
	}
	if err := r.DB(ctx).WithContext(ctx).Omit("Suppliers", "Unit").Create(products).Error; err != nil {
		return fmt.Errorf("failed to create products: %w", err)
	}
	return nil
}

func (r *initialStockImportRepository) ExistsInitialForInventory(ctx context.Context, inventoryID uint) (bool, error) {
	var found int
	err := r.DB(ctx).WithContext(ctx).
		Model(&models.InventoryTransaction{}).
		Unscoped().
		Select("1").
		Joins("JOIN inventory_items ii ON ii.id = inventory_transactions.inventory_item_id").
		Where("ii.inventory_id = ?", inventoryID).
		Where("inventory_transactions.transaction_type = ?", models.InventoryTransactionTypeInitial).
		Where("inventory_transactions.deleted_at IS NULL").
		Limit(1).
		Scan(&found).Error
	if err != nil {
		return false, fmt.Errorf("failed to check existing initial stock transactions: %w", err)
	}
	return found == 1, nil
}
