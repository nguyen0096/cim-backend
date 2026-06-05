package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

//go:generate mockery --name=SellingPriceService --structname=SellingPriceService --output=../mocks/servicemocks --outpkg=servicemocks
type SellingPriceService interface {
	CreateSellingPrice(ctx context.Context, req dto.CreateSellingPriceRequest) (*models.SellingPrice, error)
	UpdateSellingPrice(ctx context.Context, id uint, req dto.UpdateSellingPriceRequest) (*models.SellingPrice, error)
	DeleteSellingPrice(ctx context.Context, id uint) error
	GetSellingPrice(ctx context.Context, id uint) (*models.SellingPrice, error)
	ListByProductID(ctx context.Context, productID uint) ([]*models.SellingPrice, error)
	UpsertPOItemSellingPrice(ctx context.Context, poID uint, poItemID uint, sellingPrice decimal.Decimal) (*models.POItemSellingPrice, error)
	CountUnlinkedPOItems(ctx context.Context, sellingPriceID uint) (int64, error)
	BackfillPOItems(ctx context.Context, sellingPriceID uint) (int64, error)
}

type sellingPriceService struct {
	sellingPriceRepo repository.SellingPriceRepository
	productRepo      repository.ProductRepository
	db               *gorm.DB
}

func NewSellingPriceService(
	sellingPriceRepo repository.SellingPriceRepository,
	productRepo repository.ProductRepository,
	db *gorm.DB,
) SellingPriceService {
	return &sellingPriceService{
		sellingPriceRepo: sellingPriceRepo,
		productRepo:      productRepo,
		db:               db,
	}
}

func (s *sellingPriceService) CreateSellingPrice(ctx context.Context, req dto.CreateSellingPriceRequest) (*models.SellingPrice, error) {
	if req.Price.LessThan(decimal.Zero) {
		return nil, fmt.Errorf("price must be 0 or greater")
	}

	// Validate product exists
	_, err := s.productRepo.GetByID(ctx, req.ProductID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("product with ID %d not found", req.ProductID)
		}
		return nil, fmt.Errorf("failed to validate product: %w", err)
	}

	effectiveFrom, err := time.Parse("2006-01-02", req.EffectiveFrom)
	if err != nil {
		return nil, fmt.Errorf("invalid effective_from date format, expected YYYY-MM-DD: %w", err)
	}

	sp := &models.SellingPrice{
		ProductID:     req.ProductID,
		InventoryID:   req.InventoryID,
		Price:         req.Price,
		EffectiveFrom: effectiveFrom,
		Notes:         req.Notes,
	}

	if err := s.sellingPriceRepo.Create(ctx, sp); err != nil {
		return nil, fmt.Errorf("failed to create selling price: %w", err)
	}

	return sp, nil
}

func (s *sellingPriceService) UpdateSellingPrice(ctx context.Context, id uint, req dto.UpdateSellingPriceRequest) (*models.SellingPrice, error) {
	if req.Price.LessThan(decimal.Zero) {
		return nil, fmt.Errorf("price must be 0 or greater")
	}

	sp, err := s.sellingPriceRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkg.ErrNotFound(fmt.Sprintf("selling price with ID %d not found", id), err)
		}
		return nil, fmt.Errorf("failed to get selling price: %w", err)
	}

	effectiveFrom, err := time.Parse("2006-01-02", req.EffectiveFrom)
	if err != nil {
		return nil, fmt.Errorf("invalid effective_from date format, expected YYYY-MM-DD: %w", err)
	}

	sp.Price = req.Price
	sp.EffectiveFrom = effectiveFrom
	sp.Notes = req.Notes

	if err := s.sellingPriceRepo.Update(ctx, sp); err != nil {
		return nil, fmt.Errorf("failed to update selling price: %w", err)
	}

	return sp, nil
}

func (s *sellingPriceService) DeleteSellingPrice(ctx context.Context, id uint) error {
	return s.sellingPriceRepo.Delete(ctx, id)
}

func (s *sellingPriceService) GetSellingPrice(ctx context.Context, id uint) (*models.SellingPrice, error) {
	sp, err := s.sellingPriceRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkg.ErrNotFound(fmt.Sprintf("selling price with ID %d not found", id), err)
		}
		return nil, fmt.Errorf("failed to get selling price: %w", err)
	}
	return sp, nil
}

func (s *sellingPriceService) ListByProductID(ctx context.Context, productID uint) ([]*models.SellingPrice, error) {
	return s.sellingPriceRepo.ListByProductID(ctx, productID)
}

func (s *sellingPriceService) UpsertPOItemSellingPrice(ctx context.Context, poID uint, poItemID uint, sellingPrice decimal.Decimal) (*models.POItemSellingPrice, error) {
	// Validate that the item belongs to the specified PO
	var count int64
	if err := s.db.WithContext(ctx).
		Raw(`SELECT COUNT(*) FROM purchase_order_items WHERE id = ? AND purchase_order_id = ? AND deleted_at IS NULL`, poItemID, poID).
		Scan(&count).Error; err != nil {
		return nil, fmt.Errorf("failed to validate PO item ownership: %w", err)
	}
	if count == 0 {
		return nil, pkg.ErrNotFound(fmt.Sprintf("purchase order item %d not found in purchase order %d", poItemID, poID), nil)
	}

	var existing models.POItemSellingPrice
	err := s.db.WithContext(ctx).
		Where("purchase_order_item_id = ?", poItemID).
		First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// Create new record
		record := &models.POItemSellingPrice{
			PurchaseOrderItemID: poItemID,
			SellingPrice:        &sellingPrice,
		}
		if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
			return nil, fmt.Errorf("failed to create PO item selling price: %w", err)
		}
		return record, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to look up PO item selling price: %w", err)
	}

	// Update existing record — only the override value
	existing.SellingPrice = &sellingPrice
	if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
		return nil, fmt.Errorf("failed to update PO item selling price: %w", err)
	}
	return &existing, nil
}

// unlinkedPOItemsQuery returns the shared SQL for finding PO items that can be linked to a selling price.
// Matches PO items where:
// - product matches the selling price's product
// - po.created_at >= selling price's effective_from
// - po.created_at < next selling price's effective_from (or no upper bound if no next price)
// - the item has no effective selling price yet. This means either no
//   purchase_order_item_selling_prices row, OR a row that is "empty": no override
//   (selling_price) and no live linked price (selling_price_id pointing at a
//   non-deleted selling_prices row). createPOItemSellingPrices always inserts a
//   row at PO creation — empty when no price existed yet — so matching only on
//   "no row" would miss those items.
const unlinkedPOItemsSQL = `
	FROM purchase_order_items poi
	JOIN purchase_orders po ON po.id = poi.purchase_order_id
	WHERE poi.product_id = ?
	AND poi.deleted_at IS NULL
	AND po.deleted_at IS NULL
	AND po.created_at >= ?::date
	AND (?::date IS NULL OR po.created_at < ?::date)
	AND NOT EXISTS (
		SELECT 1 FROM purchase_order_item_selling_prices pisp
		WHERE pisp.purchase_order_item_id = poi.id
		AND pisp.deleted_at IS NULL
		AND (
			pisp.selling_price IS NOT NULL
			OR EXISTS (
				SELECT 1 FROM selling_prices sp
				WHERE sp.id = pisp.selling_price_id AND sp.deleted_at IS NULL
			)
		)
	)
`

// getNextEffectiveFrom returns the effective_from of the next selling price after the given one
// for the same product (and same inventory_id scope). Returns nil if there's no next price.
func (s *sellingPriceService) getNextEffectiveFrom(ctx context.Context, sp *models.SellingPrice) *time.Time {
	var nextDate *time.Time
	query := s.db.WithContext(ctx).
		Raw(`SELECT effective_from FROM selling_prices
			WHERE product_id = ? AND effective_from > ? AND deleted_at IS NULL
			AND (inventory_id IS NOT DISTINCT FROM ?)
			ORDER BY effective_from ASC LIMIT 1`,
			sp.ProductID, sp.EffectiveFrom, sp.InventoryID)
	query.Scan(&nextDate)
	return nextDate
}

func (s *sellingPriceService) CountUnlinkedPOItems(ctx context.Context, sellingPriceID uint) (int64, error) {
	sp, err := s.sellingPriceRepo.GetByID(ctx, sellingPriceID)
	if err != nil {
		return 0, fmt.Errorf("selling price not found: %w", err)
	}

	nextDate := s.getNextEffectiveFrom(ctx, sp)

	var count int64
	s.db.WithContext(ctx).
		Raw("SELECT COUNT(*) "+unlinkedPOItemsSQL, sp.ProductID, sp.EffectiveFrom, nextDate, nextDate).
		Scan(&count)
	return count, nil
}

func (s *sellingPriceService) BackfillPOItems(ctx context.Context, sellingPriceID uint) (int64, error) {
	sp, err := s.sellingPriceRepo.GetByID(ctx, sellingPriceID)
	if err != nil {
		return 0, fmt.Errorf("selling price not found: %w", err)
	}

	nextDate := s.getNextEffectiveFrom(ctx, sp)

	// Upsert: items with no pisp row get one inserted; items that already have an
	// empty row (no override, no link) get that row linked to this price. The
	// WHERE on DO UPDATE guards against overwriting a real override or live link.
	result := s.db.WithContext(ctx).Exec(
		"INSERT INTO purchase_order_item_selling_prices (purchase_order_item_id, selling_price_id, created_by, created_at, updated_by, updated_at) SELECT poi.id, ?, po.created_by, NOW(), po.created_by, NOW() "+unlinkedPOItemsSQL+
			` ON CONFLICT (purchase_order_item_id) DO UPDATE
			SET selling_price_id = EXCLUDED.selling_price_id,
			    updated_by = EXCLUDED.updated_by,
			    updated_at = NOW()
			WHERE purchase_order_item_selling_prices.deleted_at IS NULL
			AND purchase_order_item_selling_prices.selling_price IS NULL
			AND purchase_order_item_selling_prices.selling_price_id IS NULL`,
		sp.ID, sp.ProductID, sp.EffectiveFrom, nextDate, nextDate,
	)

	if result.Error != nil {
		return 0, fmt.Errorf("failed to backfill PO items: %w", result.Error)
	}
	return result.RowsAffected, nil
}

