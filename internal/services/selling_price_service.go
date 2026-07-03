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
	// ApplyMassiveLinks re-points PO items to startID across the price's
	// server-resolved effective range. endEffectiveFrom is the client's
	// optimistic-concurrency assertion of the previewed exclusive end date (nil =
	// open-ended); a mismatch returns a conflict error. Runs in a single transaction.
	ApplyMassiveLinks(ctx context.Context, startID uint, endEffectiveFrom *string) (int64, error)

	// CreateSellingPriceWithApplying creates a price and returns the massive-apply preview.
	CreateSellingPriceWithApplying(ctx context.Context, req dto.CreateSellingPriceRequest) (*models.SellingPrice, dto.SellingPriceMassiveApplying, error)
	// UpdateSellingPriceWithApplying updates a price and returns the massive-apply preview.
	UpdateSellingPriceWithApplying(ctx context.Context, id uint, req dto.UpdateSellingPriceRequest) (*models.SellingPrice, dto.SellingPriceMassiveApplying, error)
	// DeleteSellingPriceWithApplying deletes a price and returns the massive-apply preview.
	// Blocked (validation error) when the deleted price has no previous price in scope.
	DeleteSellingPriceWithApplying(ctx context.Context, id uint) (dto.SellingPriceMassiveApplying, error)
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

	// Inventory-scoped selling prices are blocked for now; only global prices
	// (inventory_id NULL) are allowed.
	if req.InventoryID != nil {
		return nil, pkg.ErrSellingPriceInventorySpecificUnsupported(ctx)
	}

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

	// Update cannot change scope: the DTO carries no InventoryID and sp.InventoryID
	// is never mutated here.
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

	existing.SellingPrice = &sellingPrice
	if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
		return nil, fmt.Errorf("failed to update PO item selling price: %w", err)
	}
	return &existing, nil
}

// SellingPriceRange translates a price's single effective date into an effective
// range [EffectiveFrom, EffectiveEndAt). EffectiveEndAt is the exclusive upper
// bound (the next price's effective_from in scope, nil when open-ended); Next is
// that boundary price. Both come from one lookup in resolveEffectiveRange.
type SellingPriceRange struct {
	Price          *models.SellingPrice
	EffectiveEndAt *time.Time
	Next           *models.SellingPrice
}

// resolveEffectiveRange is the single date->range translator; it resolves the
// exclusive end boundary from the next price in scope.
func (s *sellingPriceService) resolveEffectiveRange(ctx context.Context, sp *models.SellingPrice) (SellingPriceRange, error) {
	next, err := s.sellingPriceRepo.GetNextInScope(ctx, sp)
	if err != nil {
		return SellingPriceRange{}, fmt.Errorf("failed to resolve effective range: %w", err)
	}
	rng := SellingPriceRange{Price: sp, Next: next}
	if next != nil {
		end := next.EffectiveFrom
		rng.EffectiveEndAt = &end
	}
	return rng, nil
}

// resolveEffectiveRangeByID resolves the range for the price :id from its
// CURRENT persisted state, so callers never trust client-supplied ranges.
func (s *sellingPriceService) resolveEffectiveRangeByID(ctx context.Context, sellingPriceID uint) (SellingPriceRange, error) {
	sp, err := s.sellingPriceRepo.GetByID(ctx, sellingPriceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return SellingPriceRange{}, pkg.ErrNotFound(fmt.Sprintf("selling price with ID %d not found", sellingPriceID), err)
		}
		return SellingPriceRange{}, fmt.Errorf("failed to get selling price: %w", err)
	}
	return s.resolveEffectiveRange(ctx, sp)
}

// resolvePreviousPrice returns the previous price in scope (the one that takes
// over a vacated window), or nil when none exists or on lookup failure
// (fail-closed). May return the probed price itself; callers dedupe.
func (s *sellingPriceService) resolvePreviousPrice(ctx context.Context, sp *models.SellingPrice) *models.SellingPrice {
	prev, err := s.sellingPriceRepo.GetPrevInScope(ctx, sp)
	if err != nil {
		return nil
	}
	return prev
}

// affectedPOItemsFromSQL is the shared FROM+WHERE selecting the PO items in a
// price's range and inventory scope. Placeholder order matches
// SellingPriceRange.scopeArgs; global vs inventory-specific scope mirrors
// PO-creation precedence.
const affectedPOItemsFromSQL = `
	FROM purchase_order_items poi
	JOIN purchase_orders po ON po.id = poi.purchase_order_id
	WHERE poi.product_id = ?
	AND poi.deleted_at IS NULL
	AND po.deleted_at IS NULL
	AND po.created_at >= ?::date
	AND (?::date IS NULL OR po.created_at < ?::date)
	AND (
		-- inventory-specific price: match POs in that inventory
		(?::bigint IS NOT NULL AND po.inventory_id = ?::bigint)
		OR
		-- global price: match POs in inventories with no inventory-specific
		-- selling price for the product covering po.created_at
		(?::bigint IS NULL AND NOT EXISTS (
			SELECT 1 FROM selling_prices sp2
			WHERE sp2.product_id = ?
			AND sp2.inventory_id = po.inventory_id
			AND sp2.deleted_at IS NULL
			AND sp2.effective_from <= po.created_at
		))
	)
`

// scopeArgs returns the 8 positional args for affectedPOItemsFromSQL.
func (rng SellingPriceRange) scopeArgs() []interface{} {
	return []interface{}{
		rng.Price.ProductID,     // $1 range product
		rng.Price.EffectiveFrom, // $2 lower bound
		rng.EffectiveEndAt,      // $3 open-ended check
		rng.EffectiveEndAt,      // $4 upper bound
		rng.Price.InventoryID,   // $5 inventory-specific present?
		rng.Price.InventoryID,   // $6 inventory-specific match value
		rng.Price.InventoryID,   // $6b global branch IS NULL test
		rng.Price.ProductID,     // $7 global branch product
	}
}

// countAffected returns the number of PO items whose displayed price would change
// if the range's price were applied, excluding override rows and rows already
// linked to the target.
func (s *sellingPriceService) countAffected(ctx context.Context, rng SellingPriceRange) (int64, error) {
	args := append(rng.scopeArgs(), rng.Price.ID)
	var count int64
	err := s.db.WithContext(ctx).
		Raw(`SELECT COUNT(*) `+affectedPOItemsFromSQL+`
			AND NOT EXISTS (
				SELECT 1 FROM purchase_order_item_selling_prices pisp
				WHERE pisp.purchase_order_item_id = poi.id
				AND pisp.deleted_at IS NULL
				AND (
					pisp.selling_price IS NOT NULL
					OR pisp.selling_price_id IS NOT DISTINCT FROM ?
				)
			)`, args...).
		Scan(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count affected PO items: %w", err)
	}
	return count, nil
}

// applyRangeLinks re-points pisp.selling_price_id to the range's price for every
// PO item in range+scope whose link differs and has no manual override; overrides
// are left untouched and items with no live pisp row get one inserted. So
// RowsAffected == affected_po_item_count. Runs against tx for atomicity.
func (s *sellingPriceService) applyRangeLinks(ctx context.Context, tx *gorm.DB, rng SellingPriceRange) (int64, error) {
	args := append([]interface{}{rng.Price.ID}, rng.scopeArgs()...)
	args = append(args, rng.Price.ID)
	result := tx.WithContext(ctx).Exec(
		`INSERT INTO purchase_order_item_selling_prices (purchase_order_item_id, selling_price_id, created_by, created_at, updated_by, updated_at)
			SELECT poi.id, ?, po.created_by, NOW(), po.created_by, NOW() `+affectedPOItemsFromSQL+`
		ON CONFLICT (purchase_order_item_id) WHERE deleted_at IS NULL DO UPDATE
			SET selling_price_id = EXCLUDED.selling_price_id,
			    updated_by = EXCLUDED.updated_by,
			    updated_at = NOW()
			WHERE purchase_order_item_selling_prices.deleted_at IS NULL
			AND purchase_order_item_selling_prices.selling_price_id IS DISTINCT FROM ?
			AND purchase_order_item_selling_prices.selling_price IS NULL`,
		args...,
	)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to apply selling price links: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (s *sellingPriceService) CountUnlinkedPOItems(ctx context.Context, sellingPriceID uint) (int64, error) {
	rng, err := s.resolveEffectiveRangeByID(ctx, sellingPriceID)
	if err != nil {
		return 0, err
	}
	return s.countAffected(ctx, rng)
}

// ApplyMassiveLinks re-points PO items to the start price across its current
// server-resolved range. endEffectiveFrom asserts the previewed exclusive end
// date (nil = open-ended); a mismatch is a conflict (re-preview). Runs in a single
// transaction.
func (s *sellingPriceService) ApplyMassiveLinks(ctx context.Context, startID uint, endEffectiveFrom *string) (int64, error) {
	rng, err := s.resolveEffectiveRangeByID(ctx, startID)
	if err != nil {
		return 0, err
	}
	if err := assertRangeBoundary(ctx, rng, endEffectiveFrom); err != nil {
		return 0, err
	}

	var applied int64
	err = s.db.Transaction(func(tx *gorm.DB) error {
		n, err := s.applyRangeLinks(ctx, tx, rng)
		if err != nil {
			return err
		}
		applied = n
		return nil
	})
	if err != nil {
		return 0, err
	}
	return applied, nil
}

// assertRangeBoundary checks the client's previewed end-boundary claim (nil =
// open-ended) against the server-resolved range. The claimed date is compared
// verbatim; any disagreement means the ledger changed since preview -> conflict.
func assertRangeBoundary(ctx context.Context, rng SellingPriceRange, claimedEndDate *string) error {
	boundaryConflict := func() error {
		return pkg.NewAppError(pkg.ErrorCodeConflict,
			"selling price range boundary changed since preview: end_effective_from does not match the current range end in scope — re-fetch the preview and retry", nil)
	}

	if claimedEndDate == nil {
		if rng.EffectiveEndAt == nil {
			return nil
		}
		return boundaryConflict()
	}
	if _, err := time.Parse("2006-01-02", *claimedEndDate); err != nil {
		return pkg.ErrSellingPriceInvalidEndEffectiveFromFormat(ctx, err)
	}
	if rng.EffectiveEndAt == nil {
		return boundaryConflict()
	}
	if rng.EffectiveEndAt.Format("2006-01-02") != *claimedEndDate {
		return boundaryConflict()
	}
	return nil
}

// refFor builds a SellingPriceRef DTO from a model.
func refFor(sp *models.SellingPrice) dto.SellingPriceRef {
	return dto.SellingPriceRef{
		ID:            sp.ID,
		Price:         sp.Price,
		EffectiveFrom: sp.EffectiveFrom.Format("2006-01-02"),
	}
}

// entryForPrice builds one preview entry for a price acting as the window start.
// EndSellingPrice is the next price in scope (nil if open-ended).
func (s *sellingPriceService) entryForPrice(ctx context.Context, start *models.SellingPrice) (dto.SellingPriceMassiveApplyingEntry, error) {
	rng, err := s.resolveEffectiveRange(ctx, start)
	if err != nil {
		return dto.SellingPriceMassiveApplyingEntry{}, err
	}
	count, err := s.countAffected(ctx, rng)
	if err != nil {
		return dto.SellingPriceMassiveApplyingEntry{}, err
	}
	entry := dto.SellingPriceMassiveApplyingEntry{
		StartSellingPrice:   refFor(start),
		AffectedPOItemCount: count,
	}
	if rng.Next != nil {
		ref := refFor(rng.Next)
		entry.EndSellingPrice = &ref
	}
	return entry, nil
}

func (s *sellingPriceService) CreateSellingPriceWithApplying(ctx context.Context, req dto.CreateSellingPriceRequest) (*models.SellingPrice, dto.SellingPriceMassiveApplying, error) {
	sp, err := s.CreateSellingPrice(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	// Preview is informational; a compute failure degrades to an empty preview
	// rather than failing the committed create.
	entry, err := s.entryForPrice(ctx, sp)
	if err != nil {
		return sp, dto.SellingPriceMassiveApplying{}, nil
	}
	return sp, dto.SellingPriceMassiveApplying{entry}, nil
}

func (s *sellingPriceService) UpdateSellingPriceWithApplying(ctx context.Context, id uint, req dto.UpdateSellingPriceRequest) (*models.SellingPrice, dto.SellingPriceMassiveApplying, error) {
	before, err := s.sellingPriceRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, pkg.ErrNotFound(fmt.Sprintf("selling price with ID %d not found", id), err)
		}
		return nil, nil, fmt.Errorf("failed to get selling price: %w", err)
	}
	oldEffectiveFrom := before.EffectiveFrom

	// Reject moving the earliest price to a later date: the vacated leading window
	// would have no previous price to take over. Probe with the old effective_from.
	// Parse errors are surfaced by UpdateSellingPrice below.
	if newEffectiveFrom, perr := time.Parse("2006-01-02", req.EffectiveFrom); perr == nil &&
		newEffectiveFrom.After(oldEffectiveFrom) &&
		s.resolvePreviousPrice(ctx, before) == nil {
		return nil, nil, pkg.ErrSellingPriceMoveEarliestNoTakeover(ctx)
	}

	sp, err := s.UpdateSellingPrice(ctx, id, req)
	if err != nil {
		return nil, nil, err
	}

	dateChanged := !sp.EffectiveFrom.Equal(oldEffectiveFrom)

	// Update is committed; preview-query failures degrade to an empty/partial preview.
	if !dateChanged {
		entry, err := s.entryForPrice(ctx, sp)
		if err != nil {
			return sp, dto.SellingPriceMassiveApplying{}, nil
		}
		return sp, dto.SellingPriceMassiveApplying{entry}, nil
	}

	// effective_from changed: preview the vacated old window and the new window.
	entries := dto.SellingPriceMassiveApplying{}

	// Vacated old window: start = the previous price now covering the old date.
	// Dedupe (prev.ID != sp.ID) when this price itself moved earlier. Best-effort.
	vacated := &models.SellingPrice{
		Base:          models.Base{ID: sp.ID},
		ProductID:     sp.ProductID,
		InventoryID:   sp.InventoryID,
		EffectiveFrom: oldEffectiveFrom,
	}
	if prev := s.resolvePreviousPrice(ctx, vacated); prev != nil && prev.ID != sp.ID {
		if entry, perr := s.entryForPrice(ctx, prev); perr == nil {
			entries = append(entries, entry)
		}
	}

	// New window: start = this price.
	if newEntry, perr := s.entryForPrice(ctx, sp); perr == nil {
		entries = append(entries, newEntry)
	}

	return sp, entries, nil
}

func (s *sellingPriceService) DeleteSellingPriceWithApplying(ctx context.Context, id uint) (dto.SellingPriceMassiveApplying, error) {
	sp, err := s.sellingPriceRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkg.ErrNotFound(fmt.Sprintf("selling price with ID %d not found", id), err)
		}
		return nil, fmt.Errorf("failed to get selling price: %w", err)
	}

	// Start price is the previous price in scope covering the vacated window; block
	// when there is none (e.g. the first/only price).
	start := s.resolvePreviousPrice(ctx, sp)
	if start == nil {
		return nil, pkg.ErrSellingPriceDeleteNoTakeover(ctx)
	}

	if err := s.DeleteSellingPrice(ctx, id); err != nil {
		return nil, fmt.Errorf("failed to delete selling price: %w", err)
	}

	// entryForPrice recomputes range/count from the post-delete state. Delete is
	// committed, so a failure degrades to an empty preview.
	entry, err := s.entryForPrice(ctx, start)
	if err != nil {
		return dto.SellingPriceMassiveApplying{}, nil
	}
	return dto.SellingPriceMassiveApplying{entry}, nil
}
