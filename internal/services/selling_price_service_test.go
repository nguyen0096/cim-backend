package services

import (
	"cim-backend/internal/mocks/repositorymocks"
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newServiceTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	require.NoError(t, err)

	return gormDB, mock
}

func TestGetSellingPrice_NotFound_ReturnsAppError(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil)

	spRepo.On("GetByID", ctx, uint(999)).Return(nil, gorm.ErrRecordNotFound)

	sp, err := service.GetSellingPrice(ctx, 999)
	assert.Nil(t, sp)
	assert.Error(t, err)

	appErr, ok := err.(*pkg.AppError)
	assert.True(t, ok, "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeNotFound, appErr.Code)
}

func TestRefFor_FormatsDate(t *testing.T) {
	sp := &models.SellingPrice{
		Base:          models.Base{ID: 7},
		Price:         decimal.NewFromInt(120),
		EffectiveFrom: time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
	}
	ref := refFor(sp)
	assert.Equal(t, uint(7), ref.ID)
	assert.Equal(t, "2026-04-11", ref.EffectiveFrom)
	assert.True(t, ref.Price.Equal(decimal.NewFromInt(120)))
}

func TestScopeArgs_InventorySpecific(t *testing.T) {
	inv := uint(3)
	next := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	rng := SellingPriceRange{
		Price: &models.SellingPrice{
			Base:          models.Base{ID: 9},
			ProductID:     42,
			InventoryID:   &inv,
			EffectiveFrom: from,
		},
		EffectiveEndAt: &next,
	}
	args := rng.scopeArgs()
	// $1 product, $2 from, $3 end, $4 end, $5 inv, $6 inv, $6b inv, $7 product
	assert.Len(t, args, 8)
	assert.Equal(t, uint(42), args[0])
	assert.Equal(t, from, args[1])
	assert.Equal(t, &next, args[2])
	assert.Equal(t, &next, args[3])
	assert.Equal(t, &inv, args[4])
	assert.Equal(t, &inv, args[5])
	assert.Equal(t, &inv, args[6])
	assert.Equal(t, uint(42), args[7])
}

func TestScopeArgs_Global_NilInventory(t *testing.T) {
	rng := SellingPriceRange{
		Price: &models.SellingPrice{
			Base:          models.Base{ID: 9},
			ProductID:     42,
			EffectiveFrom: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	args := rng.scopeArgs()
	assert.Len(t, args, 8)
	// open-ended: end date placeholders are nil
	assert.Nil(t, args[2])
	assert.Nil(t, args[3])
	// global: inventory placeholders are nil (drives the NOT EXISTS branch)
	assert.Nil(t, args[4])
	assert.Nil(t, args[5])
	assert.Nil(t, args[6])
}

// resolveEffectiveRange is the single date→range translator: when a next price
// exists in scope, the range's EXCLUSIVE end is that price's effective_from and
// Next carries the boundary price itself — both from the same lookup.
func TestResolveEffectiveRange_NextExists_ExclusiveEnd(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil).(*sellingPriceService)

	sp := &models.SellingPrice{
		Base:          models.Base{ID: 1},
		ProductID:     42,
		EffectiveFrom: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	}
	nextFrom := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	next := &models.SellingPrice{
		Base:          models.Base{ID: 2},
		ProductID:     42,
		EffectiveFrom: nextFrom,
	}
	spRepo.On("GetNextInScope", ctx, sp).Return(next, nil)

	rng, err := service.resolveEffectiveRange(ctx, sp)
	assert.NoError(t, err)
	assert.Same(t, sp, rng.Price)
	assert.Same(t, next, rng.Next)
	require.NotNil(t, rng.EffectiveEndAt)
	assert.True(t, rng.EffectiveEndAt.Equal(nextFrom), "end must be the next price's effective_from (exclusive)")
}

// No next price in scope → open-ended range: nil end, nil next.
func TestResolveEffectiveRange_NoNext_OpenEnded(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil).(*sellingPriceService)

	sp := &models.SellingPrice{
		Base:          models.Base{ID: 1},
		ProductID:     42,
		EffectiveFrom: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	}
	spRepo.On("GetNextInScope", ctx, sp).Return(nil, nil)

	rng, err := service.resolveEffectiveRange(ctx, sp)
	assert.NoError(t, err)
	assert.Same(t, sp, rng.Price)
	assert.Nil(t, rng.EffectiveEndAt)
	assert.Nil(t, rng.Next)
}

// A failed next-price lookup must surface as an error, not silently degrade to
// an open-ended range (which would make counts/applies span too far).
func TestResolveEffectiveRange_LookupError_Propagates(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil).(*sellingPriceService)

	sp := &models.SellingPrice{
		Base:          models.Base{ID: 1},
		ProductID:     42,
		EffectiveFrom: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	}
	spRepo.On("GetNextInScope", ctx, sp).Return(nil, gorm.ErrInvalidDB)

	_, err := service.resolveEffectiveRange(ctx, sp)
	assert.Error(t, err)
}

func TestApplyMassiveLinks_StartNotFound_Returns404(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil)

	spRepo.On("GetByID", ctx, uint(999)).Return(nil, gorm.ErrRecordNotFound)

	n, err := service.ApplyMassiveLinks(ctx, 999, nil)
	assert.Equal(t, int64(0), n)
	appErr, ok := err.(*pkg.AppError)
	assert.True(t, ok, "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeNotFound, appErr.Code)
}

// applyMassiveLinksBoundaryFixture wires a start price whose server-resolved
// range has `next` as its boundary (nil = open-ended) and returns the service.
func applyMassiveLinksBoundaryFixture(t *testing.T, next *models.SellingPrice) SellingPriceService {
	t.Helper()
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil)

	start := &models.SellingPrice{
		Base:          models.Base{ID: 1},
		ProductID:     42,
		EffectiveFrom: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	}
	spRepo.On("GetByID", ctx, uint(1)).Return(start, nil)
	spRepo.On("GetNextInScope", ctx, start).Return(next, nil)
	return service
}

func assertBoundaryConflict(t *testing.T, n int64, err error) {
	t.Helper()
	assert.Equal(t, int64(0), n)
	appErr, ok := err.(*pkg.AppError)
	require.True(t, ok, "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeConflict, appErr.Code)
	assert.Contains(t, appErr.Message, "re-fetch the preview")
}

// The client previewed an end date of 2026-06-01, but the ledger's resolved
// boundary DATE is now 2026-05-01 (e.g. a price was inserted since the
// preview) → conflict: the previewed set no longer equals the applied set.
func TestApplyMassiveLinks_EndDateMismatch_ReturnsConflict(t *testing.T) {
	next := &models.SellingPrice{
		Base:          models.Base{ID: 3},
		ProductID:     42,
		EffectiveFrom: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	service := applyMassiveLinksBoundaryFixture(t, next)

	endDate := "2026-06-01"
	n, err := service.ApplyMassiveLinks(context.Background(), 1, &endDate)
	assertBoundaryConflict(t, n, err)
}

// The boundary price's effective_from was EDITED between preview and apply: the
// boundary row is the same (same id, still the next price in scope) but it no
// longer sits where the user saw it. Asserting by boundary ID passed here —
// both the claimed price's date and the resolved end derive from the same live
// row and move together — which silently applied a window the user never
// previewed. Pinning the previewed DATE catches it: claimed 2026-05-01 (what
// the preview showed) vs resolved 2026-05-15 (the edited date) → conflict.
func TestApplyMassiveLinks_BoundaryDateEditedSameRow_ReturnsConflict(t *testing.T) {
	next := &models.SellingPrice{
		Base:          models.Base{ID: 2},
		ProductID:     42,
		EffectiveFrom: time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC), // edited after preview
	}
	service := applyMassiveLinksBoundaryFixture(t, next)

	endDate := "2026-05-01" // the date the preview showed for price 2
	n, err := service.ApplyMassiveLinks(context.Background(), 1, &endDate)
	assertBoundaryConflict(t, n, err)
}

// The client previewed a bounded range, but the range is now open-ended (the
// boundary price was deleted since the preview) → conflict.
func TestApplyMassiveLinks_EndDateButRangeOpenEnded_ReturnsConflict(t *testing.T) {
	service := applyMassiveLinksBoundaryFixture(t, nil)

	endDate := "2026-05-01"
	n, err := service.ApplyMassiveLinks(context.Background(), 1, &endDate)
	assertBoundaryConflict(t, n, err)
}

// The client previewed an open-ended range, but a next price now exists (one
// was inserted since the preview) → conflict.
func TestApplyMassiveLinks_OmittedEndDateButNextExists_ReturnsConflict(t *testing.T) {
	next := &models.SellingPrice{
		Base:          models.Base{ID: 3},
		ProductID:     42,
		EffectiveFrom: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	service := applyMassiveLinksBoundaryFixture(t, next)

	n, err := service.ApplyMassiveLinks(context.Background(), 1, nil)
	assertBoundaryConflict(t, n, err)
}

// A malformed end_effective_from is a client bug, not a stale preview: it must
// surface as a validation error (400), not a conflict (409) that would loop the
// client through pointless re-previews.
func TestApplyMassiveLinks_InvalidEndDateFormat_ReturnsValidationError(t *testing.T) {
	next := &models.SellingPrice{
		Base:          models.Base{ID: 3},
		ProductID:     42,
		EffectiveFrom: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	service := applyMassiveLinksBoundaryFixture(t, next)

	endDate := "05/01/2026"
	n, err := service.ApplyMassiveLinks(context.Background(), 1, &endDate)
	assert.Equal(t, int64(0), n)
	appErr, ok := err.(*pkg.AppError)
	require.True(t, ok, "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
}

// Matching boundary date → the assertion holds and the apply proceeds, with
// the range's upper bound coming from the SERVER-resolved next price.
func TestApplyMassiveLinks_MatchingEndDate_Applies(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	gormDB, mock := newServiceTestDB(t)
	service := NewSellingPriceService(spRepo, productRepo, gormDB)

	start := &models.SellingPrice{
		Base:          models.Base{ID: 1},
		ProductID:     42,
		EffectiveFrom: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	}
	next := &models.SellingPrice{
		Base:          models.Base{ID: 2},
		ProductID:     42,
		EffectiveFrom: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	spRepo.On("GetByID", ctx, uint(1)).Return(start, nil)
	spRepo.On("GetNextInScope", ctx, start).Return(next, nil)

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO purchase_order_item_selling_prices`).
		WillReturnResult(sqlmock.NewResult(0, 5))
	mock.ExpectCommit()

	endDate := "2026-05-01"
	applied, err := service.ApplyMassiveLinks(ctx, 1, &endDate)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), applied)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateSellingPrice_InventorySpecific_ReturnsValidationError(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil)

	inv := uint(5)
	sp, err := service.CreateSellingPrice(ctx, dto.CreateSellingPriceRequest{
		ProductID:     42,
		InventoryID:   &inv,
		Price:         decimal.NewFromInt(100),
		EffectiveFrom: "2026-01-01",
	})
	assert.Nil(t, sp)
	assert.Error(t, err)

	appErr, ok := err.(*pkg.AppError)
	assert.True(t, ok, "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
}

// A selling price of 0 is a valid value: only NEGATIVE prices are rejected.
// Zero must pass validation and be persisted.
func TestCreateSellingPrice_Zero_Accepted(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil)

	productRepo.On("GetByID", ctx, uint(42)).Return(&models.Product{}, nil)
	spRepo.On("Create", ctx, mock.MatchedBy(func(sp *models.SellingPrice) bool {
		return sp.Price.Equal(decimal.Zero)
	})).Return(nil)

	sp, err := service.CreateSellingPrice(ctx, dto.CreateSellingPriceRequest{
		ProductID:     42,
		Price:         decimal.Zero,
		EffectiveFrom: "2026-01-01",
	})
	assert.NoError(t, err)
	require.NotNil(t, sp)
	assert.True(t, sp.Price.Equal(decimal.Zero))
}

func TestCreateSellingPrice_Positive_Accepted(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil)

	productRepo.On("GetByID", ctx, uint(42)).Return(&models.Product{}, nil)
	spRepo.On("Create", ctx, mock.Anything).Return(nil)

	sp, err := service.CreateSellingPrice(ctx, dto.CreateSellingPriceRequest{
		ProductID:     42,
		Price:         decimal.NewFromInt(100),
		EffectiveFrom: "2026-01-01",
	})
	assert.NoError(t, err)
	require.NotNil(t, sp)
	assert.True(t, sp.Price.Equal(decimal.NewFromInt(100)))
}

// Negative prices are rejected BEFORE any repo lookup: no product/create mock is
// set, so the test also asserts the early-return ordering.
func TestCreateSellingPrice_Negative_Rejected(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil)

	sp, err := service.CreateSellingPrice(ctx, dto.CreateSellingPriceRequest{
		ProductID:     42,
		Price:         decimal.NewFromInt(-1),
		EffectiveFrom: "2026-01-01",
	})
	assert.Nil(t, sp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "0 or greater")
}

// A 0 price update is valid (only NEGATIVE is rejected): the service must
// persist it. Mirrors TestCreateSellingPrice_Zero_Accepted for the update path.
func TestUpdateSellingPrice_Zero_Accepted(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil)

	spRepo.On("GetByID", ctx, uint(7)).Return(&models.SellingPrice{Price: decimal.NewFromInt(100)}, nil)
	spRepo.On("Update", ctx, mock.MatchedBy(func(sp *models.SellingPrice) bool {
		return sp.Price.Equal(decimal.Zero)
	})).Return(nil)

	sp, err := service.UpdateSellingPrice(ctx, 7, dto.UpdateSellingPriceRequest{
		Price:         decimal.Zero,
		EffectiveFrom: "2026-01-01",
	})
	assert.NoError(t, err)
	require.NotNil(t, sp)
	assert.True(t, sp.Price.Equal(decimal.Zero))
}

func TestUpdateSellingPrice_Negative_Rejected(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil)

	sp, err := service.UpdateSellingPrice(ctx, 1, dto.UpdateSellingPriceRequest{
		Price:         decimal.NewFromInt(-1),
		EffectiveFrom: "2026-01-01",
	})
	assert.Nil(t, sp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "0 or greater")
}

func TestUpdateSellingPrice_NotFound_ReturnsAppError(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil)

	spRepo.On("GetByID", ctx, uint(999)).Return(nil, gorm.ErrRecordNotFound)

	sp, err := service.UpdateSellingPrice(ctx, 999, dto.UpdateSellingPriceRequest{
		EffectiveFrom: "2026-01-01",
	})
	assert.Nil(t, sp)
	assert.Error(t, err)

	appErr, ok := err.(*pkg.AppError)
	assert.True(t, ok, "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeNotFound, appErr.Code)
}

// Moving the EARLIEST price later leaves the vacated leading window with no
// previous price to take over: the update must be rejected (validation error)
// BEFORE persisting — symmetric with the DELETE block.
func TestUpdateSellingPriceWithApplying_MoveEarliestLater_Rejected(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil)

	old := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	before := &models.SellingPrice{
		Base:          models.Base{ID: 1},
		ProductID:     42,
		EffectiveFrom: old,
	}
	spRepo.On("GetByID", ctx, uint(1)).Return(before, nil)

	// resolvePreviousPrice probes for an earlier same-scope price; none exists =>
	// this is the earliest price.
	spRepo.On("GetPrevInScope", ctx, before).Return(nil, nil)

	sp, _, err := service.UpdateSellingPriceWithApplying(ctx, 1, dto.UpdateSellingPriceRequest{
		Price:         decimal.NewFromInt(100),
		EffectiveFrom: "2026-05-01", // later than old (2026-04-01)
	})
	assert.Nil(t, sp)
	assert.Error(t, err)

	appErr, ok := err.(*pkg.AppError)
	assert.True(t, ok, "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)

	// The update must not have been persisted.
	spRepo.AssertNotCalled(t, "Update")
}

// Moving a price later when a previous same-scope price exists is allowed: the
// previous price takes over the vacated window, so the no-takeover block must
// NOT fire.
func TestUpdateSellingPriceWithApplying_MoveLaterWithPrevious_NotRejected(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil)

	old := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	before := &models.SellingPrice{
		Base:          models.Base{ID: 2},
		ProductID:     42,
		EffectiveFrom: old,
	}
	// GetByID is called in the with-applying wrapper and again inside UpdateSellingPrice.
	spRepo.On("GetByID", ctx, uint(2)).Return(before, nil)

	// Pre-commit probe (and later the vacated-window probe): a previous same-scope
	// price exists => takeover available, so the block does NOT fire.
	prev := &models.SellingPrice{
		Base:          models.Base{ID: 1},
		ProductID:     42,
		EffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	spRepo.On("GetPrevInScope", ctx, mock.Anything).Return(prev, nil)

	// The update is then persisted.
	spRepo.On("Update", ctx, before).Return(nil)

	// Past commit, the preview path is best-effort; a range-resolution error
	// degrades to an empty preview rather than failing the operation.
	spRepo.On("GetNextInScope", ctx, mock.Anything).Return(nil, gorm.ErrInvalidDB)

	sp, _, err := service.UpdateSellingPriceWithApplying(ctx, 2, dto.UpdateSellingPriceRequest{
		Price:         decimal.NewFromInt(100),
		EffectiveFrom: "2026-05-01", // later than old (2026-04-01)
	})
	assert.NoError(t, err)
	assert.NotNil(t, sp)
}

// TestApplyMassiveLinks_SkipsManualOverrides asserts the WRITE statement that
// applyRangeLinks emits carries the override guard, so a massive apply never
// re-points pisp rows that have a manual per-item override (selling_price NOT
// NULL). The DO UPDATE WHERE clause must contain "selling_price IS NULL" — that
// guard is what makes the write set equal countAffected's counted set, so
// RowsAffected (applied) == affected_po_item_count.
//
// It also locks the conflict target onto the PARTIAL unique index predicate
// (ON CONFLICT (purchase_order_item_id) WHERE deleted_at IS NULL), which Postgres
// requires to infer the partial arbiter index.
//
// NOTE: this harness is go-sqlmock, which matches the emitted SQL but does not
// execute Postgres ON CONFLICT ... WHERE semantics against real rows. It can
// therefore prove the guard/predicate are PRESENT in the statement (a regression
// lock on the load-bearing SQL) but cannot behaviorally prove the row-count
// equivalence end-to-end (including the soft-deleted-only INSERT path that makes
// applied == affected unconditional); that needs a real DB harness, which this
// repo does not have.
func TestApplyMassiveLinks_SkipsManualOverrides(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	gormDB, mock := newServiceTestDB(t)
	service := NewSellingPriceService(spRepo, productRepo, gormDB)

	start := &models.SellingPrice{
		Base:          models.Base{ID: 1},
		ProductID:     42,
		EffectiveFrom: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	}
	spRepo.On("GetByID", ctx, uint(1)).Return(start, nil)
	// Server-resolved range: no next price → open-ended. The omitted end date
	// (nil) asserts the same, so the boundary check passes and the apply runs.
	spRepo.On("GetNextInScope", ctx, start).Return(nil, nil)

	// The single apply runs in a transaction. The DO UPDATE WHERE clause MUST
	// include the override guard so manual overrides are left untouched.
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)ON CONFLICT \(purchase_order_item_id\) WHERE deleted_at IS NULL DO UPDATE.*selling_price_id IS DISTINCT FROM.*selling_price IS NULL`).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectCommit()

	applied, err := service.ApplyMassiveLinks(ctx, 1, nil)
	assert.NoError(t, err)
	// RowsAffected from the guarded write is returned verbatim as "applied"; with
	// the guard in place this equals what countAffected reports (the counted set).
	assert.Equal(t, int64(3), applied)
	assert.NoError(t, mock.ExpectationsWereMet())
}
