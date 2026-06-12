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
	// ApplyMassiveLinks re-points PO items to startID across the price's effective
	// range, resolved SERVER-SIDE from the current ledger. endEffectiveFrom is the
	// client's optimistic-concurrency assertion of the exclusive end DATE it
	// previewed ("YYYY-MM-DD", nil = previewed open-ended); a mismatch with the
	// resolved boundary date returns a conflict error ("re-preview"). Runs in a
	// single transaction.
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

	// DECISION: inventory-scoped selling prices are deliberately blocked for now
	// to keep the initial feature simple — every price is global (inventory_id
	// NULL). The schema and apply-scope SQL already handle inventory-specific vs
	// global precedence, so this validation is the only gate to lift later.
	if req.InventoryID != nil {
		return nil, pkg.ErrValidation("inventory-specific selling price is not supported yet", nil)
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

	// Inventory-scoped prices are temporarily blocked (see the DECISION note in
	// CreateSellingPrice). The update DTO carries no InventoryID and we never
	// mutate sp.InventoryID here, so an update cannot change the scope — the
	// create guard is sufficient.
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

// SellingPriceRange is a selling price translated from its single user-supplied
// effective date into an effective RANGE [Price.EffectiveFrom, EffectiveEndAt):
// the user only provides the start date, the app decides the end.
// EffectiveEndAt is the EXCLUSIVE upper bound — the next price's effective_from
// in the same product+inventory scope — and nil when the range is open-ended.
// Next is that boundary price itself (nil iff EffectiveEndAt is nil). Both are
// derived from ONE lookup in resolveEffectiveRange, so the count boundary and
// the displayed end ref can never disagree under a concurrent write.
type SellingPriceRange struct {
	Price          *models.SellingPrice
	EffectiveEndAt *time.Time
	Next           *models.SellingPrice
}

// resolveEffectiveRange is the single date→range translator: every
// effective-range calculation (create/update/delete previews, unlinked counts,
// backfill) goes through here. It takes a price carrying only effective_from
// and resolves the exclusive end boundary from the next price in scope.
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

// resolvePreviousPrice is resolveEffectiveRange's symmetric twin for the
// VACATED-window logic: the previous price in scope is the one that takes over
// a window vacated by an update/delete. nil when none exists — and, fail-closed,
// on lookup failure (callers either block the mutation or skip a best-effort
// preview entry). It may return the probed price itself when probing from a
// stale (pre-update) date — the caller dedupes (see UpdateSellingPriceWithApplying).
func (s *sellingPriceService) resolvePreviousPrice(ctx context.Context, sp *models.SellingPrice) *models.SellingPrice {
	prev, err := s.sellingPriceRepo.GetPrevInScope(ctx, sp)
	if err != nil {
		return nil
	}
	return prev
}

// affectedPOItemsFromSQL is the shared FROM+WHERE that selects the PO items in a
// price's range and inventory scope. Placeholder order (per SellingPriceRange.scopeArgs):
//
//	$1 product_id        (range/scope)
//	$2 effective_from    (lower bound, inclusive)
//	$3 next_date         (NULL check for open-ended)
//	$4 next_date         (exclusive upper bound)
//	$5 inventory_id      (inventory-specific match; NULL => global branch)
//	$6 inventory_id      (global branch: IS NULL test)
//	$7 product_id        (global branch: no inventory-specific price covering po.created_at)
//
// Inventory scope (issue #40 decision B) mirrors PO-creation precedence:
//   - inventory-specific price (inventory_id set): po.inventory_id = price.inventory_id
//   - global price (inventory_id NULL): POIs in inventories that have NO
//     inventory-specific selling price for the product covering po.created_at.
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
// if the range's price were applied. It EXCLUDES override rows (their displayed
// price won't change) and uses IS DISTINCT FROM so rows already linked to the
// target are not counted.
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
// PO item in range+scope whose link differs AND has no manual override. Manual
// per-item overrides (selling_price NOT NULL) are LEFT COMPLETELY UNTOUCHED — the
// DO UPDATE WHERE clause skips them — so the write set exactly equals what
// countAffected reports: RowsAffected (applied) == affected_po_item_count. It
// never writes the override column. Items with no LIVE pisp row get one inserted.
//
// The conflict target is the PARTIAL unique index uq_pisp_po_item_id_active
// (purchase_order_item_id WHERE deleted_at IS NULL), so the predicate is repeated
// in the ON CONFLICT clause as Postgres requires for arbiter-index inference.
// Because the index ignores soft-deleted rows, a PO item whose ONLY pisp row is
// soft-deleted has NO live conflict: the statement INSERTs a fresh live row. That
// fresh row is part of the counted set (no override) AND is written, so
// RowsAffected (applied) == affected_po_item_count holds UNCONDITIONALLY — the
// soft-deleted-only case is no longer an exception the guards have to absorb.
//
// The executor tx is the *gorm.DB to run against; pass the transaction handle so
// the whole apply is atomic.
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

// ApplyMassiveLinks re-points PO items to the START price (startID) across its
// CURRENT effective range, resolved server-side via resolveEffectiveRange — the
// app decides the boundary, never the client. The client's endEffectiveFrom is
// an optimistic-concurrency ASSERTION of the exclusive end DATE it previewed
// (end_selling_price.effective_from in the massive_applying entry, nil =
// open-ended): when it no longer matches the resolved boundary date the ledger
// changed since the preview, so the apply is rejected with a conflict error and
// the client must re-preview. When the assertion holds, the applied set equals
// the previewed set. The whole apply runs in a single transaction.
func (s *sellingPriceService) ApplyMassiveLinks(ctx context.Context, startID uint, endEffectiveFrom *string) (int64, error) {
	rng, err := s.resolveEffectiveRangeByID(ctx, startID)
	if err != nil {
		return 0, err
	}
	if err := assertRangeBoundary(rng, endEffectiveFrom); err != nil {
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

// assertRangeBoundary checks the client's previewed end-boundary claim against
// the server-resolved range. A nil claim asserts "I previewed an open-ended
// range"; a non-nil claim carries the exclusive end DATE the client SAW in the
// preview ("YYYY-MM-DD", end_selling_price.effective_from). The previewed date
// is pinned VERBATIM — never re-derived from live rows — so editing the
// boundary price's date between preview and apply cannot move both sides of
// the comparison together (the failure mode of asserting by boundary id). The
// resolved end is compared in the same "YYYY-MM-DD" rendering the preview
// serialized (refFor), making the assertion exactly "the window still ends
// where the user saw it end". Any disagreement — a claimed date differing from
// the resolved boundary, a date when the range is now open-ended, or no date
// when a next price now exists — means the ledger changed between preview and
// apply: conflict, re-preview.
func assertRangeBoundary(rng SellingPriceRange, claimedEndDate *string) error {
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
		return pkg.ErrValidation("invalid end_effective_from date format, expected YYYY-MM-DD", err)
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

// entryForPrice builds one preview entry for a price acting as the window START.
// EndSellingPrice is the next price in scope (nil if open-ended). The count
// boundary and the end ref both come from ONE resolved range, so they cannot
// disagree under a concurrent write.
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
	// create → 1 entry: start = the input price. The preview is informational, so a
	// failure to compute it must not fail the already-committed create — degrade to
	// an empty preview instead of returning an error (which the handler would 500).
	entry, err := s.entryForPrice(ctx, sp)
	if err != nil {
		return sp, dto.SellingPriceMassiveApplying{}, nil
	}
	return sp, dto.SellingPriceMassiveApplying{entry}, nil
}

func (s *sellingPriceService) UpdateSellingPriceWithApplying(ctx context.Context, id uint, req dto.UpdateSellingPriceRequest) (*models.SellingPrice, dto.SellingPriceMassiveApplying, error) {
	// Capture the pre-update effective_from to detect a date change.
	before, err := s.sellingPriceRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, pkg.ErrNotFound(fmt.Sprintf("selling price with ID %d not found", id), err)
		}
		return nil, nil, fmt.Errorf("failed to get selling price: %w", err)
	}
	oldEffectiveFrom := before.EffectiveFrom

	// Reject the no-takeover move-later case BEFORE committing: if the earliest
	// price in scope is moved to a later effective_from, the vacated leading window
	// [oldEffectiveFrom, newEffectiveFrom) has no previous price to take over, so
	// PO items there would keep a stale link the client can't re-point or clear.
	// This mirrors the DELETE block. Probe with the OLD effective_from (the
	// pre-mutation row) to detect "this is the earliest price". Move-earlier and
	// price-only updates are unaffected. A bad date format is surfaced by
	// UpdateSellingPrice below, so ignore the parse error here.
	if newEffectiveFrom, perr := time.Parse("2006-01-02", req.EffectiveFrom); perr == nil &&
		newEffectiveFrom.After(oldEffectiveFrom) &&
		s.resolvePreviousPrice(ctx, before) == nil {
		return nil, nil, pkg.ErrValidation("cannot move the earliest selling price later: the vacated window would have no selling price to take over", nil)
	}

	sp, err := s.UpdateSellingPrice(ctx, id, req)
	if err != nil {
		return nil, nil, err
	}

	dateChanged := !sp.EffectiveFrom.Equal(oldEffectiveFrom)

	// The update is committed past this point, so preview-query failures degrade to
	// an empty/partial preview rather than failing the operation.
	if !dateChanged {
		// price-only update → 1 entry: start = this price.
		entry, err := s.entryForPrice(ctx, sp)
		if err != nil {
			return sp, dto.SellingPriceMassiveApplying{}, nil
		}
		return sp, dto.SellingPriceMassiveApplying{entry}, nil
	}

	// effective_from changed → delete+insert semantics:
	//   1) the vacated old window — now covered by the previous price in scope
	//   2) the new window — covered by this price
	entries := dto.SellingPriceMassiveApplying{}

	// Entry for the vacated old window: start = the previous price in scope now
	// covering the old date. The move-earlier dedupe (prev.ID != sp.ID) skips the
	// vacated entry when this price itself moved earlier and now covers the old
	// window (the new-window entry below already covers it). Best-effort: a
	// preview-query failure degrades to skipping the entry.
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

	// Entry for the new window: start = this price.
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

	// For delete, the start price is the previous price in scope that now covers
	// the vacated window. Block when there is none (e.g. the first/only price) —
	// no price would take over the vacated window.
	start := s.resolvePreviousPrice(ctx, sp)
	if start == nil {
		return nil, pkg.ErrValidation("cannot delete: no previous selling price to take over the vacated window", nil)
	}

	if err := s.DeleteSellingPrice(ctx, id); err != nil {
		return nil, fmt.Errorf("failed to delete selling price: %w", err)
	}

	// After deletion, start's next is the deleted price's former next; entryForPrice
	// recomputes range/scope/count from the post-delete state. The delete is already
	// committed, so a preview-query failure degrades to an empty preview.
	entry, err := s.entryForPrice(ctx, start)
	if err != nil {
		return dto.SellingPriceMassiveApplying{}, nil
	}
	return dto.SellingPriceMassiveApplying{entry}, nil
}
