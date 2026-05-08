package repository

import (
	"cim-backend/internal/models"
	"context"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// POItemSellingPriceInfo bundles PO/POI metadata with its effective selling price.
// EffectivePrice is COALESCE(pisp.selling_price, sp.price) — nil when both are null
// or when no pisp row exists for the POI.
// PurchasePrice is the POI's unit cost (purchase_order_items.unit_price) — used
// by export consumers that need cost data for rows with no in-window purchase
// txn (e.g. carry-over rows).
type POItemSellingPriceInfo struct {
	POItemID         uint
	POID             uint
	PONumber         string
	POStatus         string // delivery status of the PO (purchase_orders.status)
	POItemStatus     string // delivery status of the POI (purchase_order_items.status)
	ProductID        uint
	QuantityOrdered  decimal.Decimal
	QuantityReceived decimal.Decimal
	PurchasePrice    decimal.Decimal
	EffectivePrice   *decimal.Decimal
}

//go:generate mockery --name=SellingPriceRepository --structname=SellingPriceRepository --output=../mocks/repositorymocks --outpkg=repositorymocks
type SellingPriceRepository interface {
	Create(ctx context.Context, sp *models.SellingPrice) error
	Update(ctx context.Context, sp *models.SellingPrice) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*models.SellingPrice, error)
	ListByProductID(ctx context.Context, productID uint) ([]*models.SellingPrice, error)

	// GetLatestForProduct returns the latest selling price for a product as of asOfDate.
	// Fallback: inventory-specific (matching inventoryID) first, then global (inventory_id IS NULL).
	// inventoryID can be nil to only search global prices.
	GetLatestForProduct(ctx context.Context, productID uint, inventoryID *uint, asOfDate time.Time) (*models.SellingPrice, error)

	// GetLatestForProducts returns the latest selling price for each product as of asOfDate.
	// Same fallback logic as GetLatestForProduct, applied per product.
	GetLatestForProducts(ctx context.Context, productIDs []uint, inventoryID *uint, asOfDate time.Time) (map[uint]*models.SellingPrice, error)

	// GetSellingPricesForSellTransactions returns the effective selling price for each sell transaction ID.
	// Resolves through: sell txn → counter txn (purchase) → PO item → POItemSellingPrice.
	// Uses COALESCE(pisp.selling_price, sp.price) for the fallback logic.
	GetSellingPricesForSellTransactions(ctx context.Context, sellTxnIDs []uint) (map[uint]decimal.Decimal, error)

	// GetPOItemSellingPricesByPOItemIDs returns POItemSellingPrice rows for the given purchase_order_item_ids.
	GetPOItemSellingPricesByPOItemIDs(ctx context.Context, poItemIDs []uint) ([]*models.POItemSellingPrice, error)

	// CreatePOItemSellingPrice inserts a POItemSellingPrice row.
	CreatePOItemSellingPrice(ctx context.Context, record *models.POItemSellingPrice) error

	// GetPOItemsWithPriceByIDs returns PO + POI metadata and the effective selling price
	// for the given purchase_order_item_ids, scoped to the given inventoryID.
	// POIs whose parent PO belongs to a different inventory are filtered out (handles
	// e.g. transfer-in transactions whose counter purchase is in another inventory).
	// Soft-deleted POIs and POs are excluded.
	GetPOItemsWithPriceByIDs(ctx context.Context, poItemIDs []uint, inventoryID uint) (map[uint]*POItemSellingPriceInfo, error)

	// GetPOItemsWithPriceByIDsAcrossInventories returns the same payload as
	// GetPOItemsWithPriceByIDs but without the inventory_id filter. Use this
	// when callers need to resolve source-PO metadata across inventory
	// boundaries — e.g. a destination-inventory transfer-in row whose source
	// POI lives in another inventory. Soft-deleted POIs and POs are still
	// excluded.
	GetPOItemsWithPriceByIDsAcrossInventories(ctx context.Context, poItemIDs []uint) (map[uint]*POItemSellingPriceInfo, error)
}

type sellingPriceRepository struct {
	db *gorm.DB
}

func NewSellingPriceRepository(db *gorm.DB) SellingPriceRepository {
	return &sellingPriceRepository{db: db}
}

func (r *sellingPriceRepository) Create(ctx context.Context, sp *models.SellingPrice) error {
	return r.db.WithContext(ctx).Create(sp).Error
}

func (r *sellingPriceRepository) Update(ctx context.Context, sp *models.SellingPrice) error {
	return r.db.WithContext(ctx).Save(sp).Error
}

func (r *sellingPriceRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.SellingPrice{}, "id = ?", id).Error
}

func (r *sellingPriceRepository) GetByID(ctx context.Context, id uint) (*models.SellingPrice, error) {
	var sp models.SellingPrice
	err := r.db.WithContext(ctx).
		Preload("Product").
		First(&sp, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &sp, nil
}

func (r *sellingPriceRepository) ListByProductID(ctx context.Context, productID uint) ([]*models.SellingPrice, error) {
	var prices []*models.SellingPrice
	err := r.db.WithContext(ctx).
		Where("product_id = ?", productID).
		Order("effective_from DESC").
		Find(&prices).Error
	return prices, err
}

func (r *sellingPriceRepository) GetLatestForProduct(ctx context.Context, productID uint, inventoryID *uint, asOfDate time.Time) (*models.SellingPrice, error) {
	var sp models.SellingPrice

	// Try inventory-specific first
	if inventoryID != nil {
		err := r.db.WithContext(ctx).
			Where("product_id = ? AND inventory_id = ? AND effective_from <= ?", productID, *inventoryID, asOfDate).
			Order("effective_from DESC").
			First(&sp).Error
		if err == nil {
			return &sp, nil
		}
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
	}

	// Fall back to global (inventory_id IS NULL)
	err := r.db.WithContext(ctx).
		Where("product_id = ? AND inventory_id IS NULL AND effective_from <= ?", productID, asOfDate).
		Order("effective_from DESC").
		First(&sp).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &sp, nil
}

func (r *sellingPriceRepository) GetLatestForProducts(ctx context.Context, productIDs []uint, inventoryID *uint, asOfDate time.Time) (map[uint]*models.SellingPrice, error) {
	if len(productIDs) == 0 {
		return make(map[uint]*models.SellingPrice), nil
	}

	result := make(map[uint]*models.SellingPrice)

	// Try inventory-specific first
	if inventoryID != nil {
		var inventoryPrices []*models.SellingPrice
		err := r.db.WithContext(ctx).
			Raw(`SELECT DISTINCT ON (product_id) *
				FROM selling_prices
				WHERE product_id IN ?
				AND inventory_id = ?
				AND effective_from <= ?
				AND deleted_at IS NULL
				ORDER BY product_id, effective_from DESC`, productIDs, *inventoryID, asOfDate).
			Scan(&inventoryPrices).Error
		if err != nil {
			return nil, err
		}
		for _, sp := range inventoryPrices {
			result[sp.ProductID] = sp
		}
	}

	// Find products still missing — need global fallback
	var missingIDs []uint
	for _, pid := range productIDs {
		if _, found := result[pid]; !found {
			missingIDs = append(missingIDs, pid)
		}
	}

	if len(missingIDs) > 0 {
		var globalPrices []*models.SellingPrice
		err := r.db.WithContext(ctx).
			Raw(`SELECT DISTINCT ON (product_id) *
				FROM selling_prices
				WHERE product_id IN ?
				AND inventory_id IS NULL
				AND effective_from <= ?
				AND deleted_at IS NULL
				ORDER BY product_id, effective_from DESC`, missingIDs, asOfDate).
			Scan(&globalPrices).Error
		if err != nil {
			return nil, err
		}
		for _, sp := range globalPrices {
			result[sp.ProductID] = sp
		}
	}

	return result, nil
}

func (r *sellingPriceRepository) GetSellingPricesForSellTransactions(ctx context.Context, sellTxnIDs []uint) (map[uint]decimal.Decimal, error) {
	if len(sellTxnIDs) == 0 {
		return make(map[uint]decimal.Decimal), nil
	}

	type result struct {
		ID           uint            `gorm:"column:id"`
		SellingPrice decimal.Decimal `gorm:"column:selling_price"`
	}

	var results []result
	err := r.db.WithContext(ctx).
		Raw(`SELECT st.id, COALESCE(pisp.selling_price, sp.price) as selling_price
			FROM inventory_transactions st
			JOIN inventory_transactions pt ON pt.id = st.counter_transaction_id
			JOIN purchase_order_item_selling_prices pisp ON pisp.purchase_order_item_id = pt.purchase_order_item_id
			LEFT JOIN selling_prices sp ON sp.id = pisp.selling_price_id
			WHERE st.id IN ?
			AND COALESCE(pisp.selling_price, sp.price) IS NOT NULL`, sellTxnIDs).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	priceMap := make(map[uint]decimal.Decimal, len(results))
	for _, r := range results {
		priceMap[r.ID] = r.SellingPrice
	}
	return priceMap, nil
}

func (r *sellingPriceRepository) GetPOItemSellingPricesByPOItemIDs(ctx context.Context, poItemIDs []uint) ([]*models.POItemSellingPrice, error) {
	if len(poItemIDs) == 0 {
		return nil, nil
	}
	var rows []*models.POItemSellingPrice
	if err := r.db.WithContext(ctx).
		Where("purchase_order_item_id IN ?", poItemIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *sellingPriceRepository) CreatePOItemSellingPrice(ctx context.Context, record *models.POItemSellingPrice) error {
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *sellingPriceRepository) GetPOItemsWithPriceByIDs(ctx context.Context, poItemIDs []uint, inventoryID uint) (map[uint]*POItemSellingPriceInfo, error) {
	return r.queryPOItemsWithPrice(ctx, poItemIDs, &inventoryID)
}

func (r *sellingPriceRepository) GetPOItemsWithPriceByIDsAcrossInventories(ctx context.Context, poItemIDs []uint) (map[uint]*POItemSellingPriceInfo, error) {
	return r.queryPOItemsWithPrice(ctx, poItemIDs, nil)
}

// queryPOItemsWithPrice is the shared implementation. When inventoryID is nil
// the inventory_id predicate is omitted, returning POIs across all inventories.
func (r *sellingPriceRepository) queryPOItemsWithPrice(ctx context.Context, poItemIDs []uint, inventoryID *uint) (map[uint]*POItemSellingPriceInfo, error) {
	if len(poItemIDs) == 0 {
		return make(map[uint]*POItemSellingPriceInfo), nil
	}

	type row struct {
		POItemID         uint             `gorm:"column:po_item_id"`
		POID             uint             `gorm:"column:po_id"`
		PONumber         string           `gorm:"column:po_number"`
		POStatus         string           `gorm:"column:po_status"`
		POItemStatus     string           `gorm:"column:po_item_status"`
		ProductID        uint             `gorm:"column:product_id"`
		QuantityOrdered  decimal.Decimal  `gorm:"column:quantity_ordered"`
		QuantityReceived decimal.Decimal  `gorm:"column:quantity_received"`
		PurchasePrice    decimal.Decimal  `gorm:"column:purchase_price"`
		EffectivePrice   *decimal.Decimal `gorm:"column:effective_price"`
	}

	// LEFT JOIN soft-delete filters live in the ON clause so soft-deleted pisp/sp
	// rows are treated as absent (LEFT JOIN returns the POI row with NULL price)
	// instead of converting the LEFT JOIN to INNER JOIN semantics.
	const baseSQL = `SELECT
			poi.id AS po_item_id,
			po.id AS po_id,
			po.order_number AS po_number,
			po.status AS po_status,
			poi.status AS po_item_status,
			poi.product_id AS product_id,
			poi.quantity AS quantity_ordered,
			poi.received_quantity AS quantity_received,
			poi.unit_price AS purchase_price,
			COALESCE(pisp.selling_price, sp.price) AS effective_price
		FROM purchase_order_items poi
		JOIN purchase_orders po ON po.id = poi.purchase_order_id
		LEFT JOIN purchase_order_item_selling_prices pisp
			ON pisp.purchase_order_item_id = poi.id AND pisp.deleted_at IS NULL
		LEFT JOIN selling_prices sp
			ON sp.id = pisp.selling_price_id AND sp.deleted_at IS NULL
		WHERE poi.id IN ?
		AND poi.deleted_at IS NULL
		AND po.deleted_at IS NULL`

	var rows []row
	var err error
	if inventoryID != nil {
		err = r.db.WithContext(ctx).
			Raw(baseSQL+` AND po.inventory_id = ?`, poItemIDs, *inventoryID).
			Scan(&rows).Error
	} else {
		err = r.db.WithContext(ctx).
			Raw(baseSQL, poItemIDs).
			Scan(&rows).Error
	}
	if err != nil {
		return nil, err
	}

	result := make(map[uint]*POItemSellingPriceInfo, len(rows))
	for _, r := range rows {
		result[r.POItemID] = &POItemSellingPriceInfo{
			POItemID:         r.POItemID,
			POID:             r.POID,
			PONumber:         r.PONumber,
			POStatus:         r.POStatus,
			POItemStatus:     r.POItemStatus,
			ProductID:        r.ProductID,
			QuantityOrdered: r.QuantityOrdered,
			QuantityReceived: r.QuantityReceived,
			PurchasePrice:    r.PurchasePrice,
			EffectivePrice:   r.EffectivePrice,
		}
	}
	return result, nil
}
