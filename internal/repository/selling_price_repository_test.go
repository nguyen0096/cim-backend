package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"cim-backend/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetSellingPricesForSellTransactions_FiltersSoftDeletedPISP asserts that the
// raw pisp join in GetSellingPricesForSellTransactions filters soft-deleted
// purchase_order_item_selling_prices rows (pisp.deleted_at IS NULL). Raw SQL
// bypasses GORM's soft-delete scope, and the partial unique index lets a
// soft-deleted AND a live pisp row coexist for the same PO item; without the
// filter the join could match the stale soft-deleted row and return a wrong price.
func TestGetSellingPricesForSellTransactions_FiltersSoftDeletedPISP(t *testing.T) {
	gormDB, mock := setupTestDB(t)
	repo := NewSellingPriceRepository(NewBaseRepository(gormDB))
	ctx := context.Background()

	// The query expectation requires "pisp.deleted_at IS NULL" to appear in the
	// JOIN. If the live-row filter is missing, this regex will not match the
	// executed SQL and sqlmock fails the test.
	queryRe := regexp.MustCompile(
		`(?s)JOIN purchase_order_item_selling_prices pisp ` +
			`ON pisp\.purchase_order_item_id = pt\.purchase_order_item_id ` +
			`AND pisp\.deleted_at IS NULL`)

	mock.ExpectQuery(queryRe.String()).
		WithArgs(uint(1)).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "selling_price"}).
				AddRow(uint(1), "42.50"),
		)

	result, err := repo.GetSellingPricesForSellTransactions(ctx, []uint{1})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	require.Contains(t, result, uint(1))
	assert.True(t, result[1].Equal(decimal.RequireFromString("42.50")),
		"expected live pisp price 42.50, got %s", result[1])
}

// nextPrevScopeRe asserts the load-bearing parts of the adjacent-price lookup
// SQL: live rows only, same product, and the inventory scope compared with
// IS NOT DISTINCT FROM so a global ledger (NULL) never matches an
// inventory-specific one. cmp/ord distinguish next (>, ASC) from prev (<, DESC).
// The id tie-break in the ORDER BY is required: two same-scope prices can share
// an effective_from, and without it the picked row is query-plan-dependent.
func nextPrevScopeRe(cmp, ord string) string {
	return `(?s)SELECT \* FROM selling_prices\s+` +
		`WHERE product_id = \$1 AND effective_from ` + cmp + ` \$2 AND deleted_at IS NULL\s+` +
		`AND \(inventory_id IS NOT DISTINCT FROM \$3\)\s+` +
		`ORDER BY effective_from ` + ord + `, id ` + ord + ` LIMIT 1`
}

func TestGetNextInScope_ReturnsNextRow(t *testing.T) {
	gormDB, mock := setupTestDB(t)
	repo := NewSellingPriceRepository(NewBaseRepository(gormDB))

	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	nextFrom := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	sp := &models.SellingPrice{ProductID: 42, EffectiveFrom: from}

	mock.ExpectQuery(nextPrevScopeRe(">", "ASC")).
		WithArgs(uint(42), from, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "effective_from"}).
			AddRow(uint(9), uint(42), nextFrom))

	next, err := repo.GetNextInScope(context.Background(), sp)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.NotNil(t, next)
	assert.Equal(t, uint(9), next.ID)
	assert.True(t, next.EffectiveFrom.Equal(nextFrom))
}

func TestGetNextInScope_NoNext_ReturnsNil(t *testing.T) {
	gormDB, mock := setupTestDB(t)
	repo := NewSellingPriceRepository(NewBaseRepository(gormDB))

	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	inv := uint(3)
	sp := &models.SellingPrice{ProductID: 42, InventoryID: &inv, EffectiveFrom: from}

	mock.ExpectQuery(nextPrevScopeRe(">", "ASC")).
		WithArgs(uint(42), from, &inv).
		WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "effective_from"}))

	next, err := repo.GetNextInScope(context.Background(), sp)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Nil(t, next)
}

func TestGetPrevInScope_ReturnsPrevRow(t *testing.T) {
	gormDB, mock := setupTestDB(t)
	repo := NewSellingPriceRepository(NewBaseRepository(gormDB))

	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	prevFrom := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sp := &models.SellingPrice{ProductID: 42, EffectiveFrom: from}

	mock.ExpectQuery(nextPrevScopeRe("<", "DESC")).
		WithArgs(uint(42), from, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "effective_from"}).
			AddRow(uint(4), uint(42), prevFrom))

	prev, err := repo.GetPrevInScope(context.Background(), sp)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.NotNil(t, prev)
	assert.Equal(t, uint(4), prev.ID)
	assert.True(t, prev.EffectiveFrom.Equal(prevFrom))
}

func TestGetPrevInScope_NoPrev_ReturnsNil(t *testing.T) {
	gormDB, mock := setupTestDB(t)
	repo := NewSellingPriceRepository(NewBaseRepository(gormDB))

	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	sp := &models.SellingPrice{ProductID: 42, EffectiveFrom: from}

	mock.ExpectQuery(nextPrevScopeRe("<", "DESC")).
		WithArgs(uint(42), from, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "effective_from"}))

	prev, err := repo.GetPrevInScope(context.Background(), sp)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Nil(t, prev)
}
