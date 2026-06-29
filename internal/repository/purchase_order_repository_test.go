package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"cim-backend/internal/models"
	"cim-backend/pkg"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPurchaseOrderRepository_Create_TranslatesOrderNumberConflict verifies the
// repository owns the DB-specific detection: an order_number unique violation is
// translated to the typed pkg.ErrDuplicateOrderNumber domain error (the #84 race
// signal the service retries on), the item-constraint violation
// (idx_product_supplier_po) and other unique violations pass through untranslated,
// and a clean insert returns no error.
func TestPurchaseOrderRepository_Create_TranslatesOrderNumberConflict(t *testing.T) {
	order := func() *models.PurchaseOrder {
		inventoryID := uint(1)
		return &models.PurchaseOrder{
			OrderNumber: "PO-240101-120000-ABCD",
			InventoryID: &inventoryID,
			Status:      models.PurchaseOrderStatusOrderPlaced,
		}
	}

	// An authenticated context is required so models.Base.BeforeCreate passes and
	// the INSERT (and the simulated violation) actually runs.
	ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")

	t.Run("order_number violation -> typed domain error", func(t *testing.T) {
		gormDB, mock := setupTestDB(t)
		repo := NewPurchaseOrderRepository(NewBaseRepository(gormDB))

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "purchase_orders"`)).
			WillReturnError(errors.New(`ERROR: duplicate key value violates unique constraint "purchase_orders_order_number_key" (SQLSTATE 23505)`))
		mock.ExpectRollback()

		err := repo.Create(ctx, order())
		require.Error(t, err)
		assert.True(t, pkg.IsErrorCode(err, pkg.ErrorCodeDuplicateOrderNumber),
			"expected ErrorCodeDuplicateOrderNumber, got %v", err)
	})

	t.Run("item-constraint violation passes through untranslated", func(t *testing.T) {
		gormDB, mock := setupTestDB(t)
		repo := NewPurchaseOrderRepository(NewBaseRepository(gormDB))

		raw := errors.New(`ERROR: duplicate key value violates unique constraint "idx_product_supplier_po" (SQLSTATE 23505)`)
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "purchase_orders"`)).
			WillReturnError(raw)
		mock.ExpectRollback()

		err := repo.Create(ctx, order())
		require.Error(t, err)
		assert.False(t, pkg.IsErrorCode(err, pkg.ErrorCodeDuplicateOrderNumber),
			"item-constraint violation must not be mapped to the order-number conflict")
		assert.Equal(t, raw, err, "item-constraint violation must surface unwrapped")
	})

	t.Run("unrelated unique violation passes through", func(t *testing.T) {
		gormDB, mock := setupTestDB(t)
		repo := NewPurchaseOrderRepository(NewBaseRepository(gormDB))

		raw := errors.New(`ERROR: duplicate key value violates unique constraint "some_other_unique" (SQLSTATE 23505)`)
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "purchase_orders"`)).
			WillReturnError(raw)
		mock.ExpectRollback()

		err := repo.Create(ctx, order())
		require.Error(t, err)
		assert.False(t, pkg.IsErrorCode(err, pkg.ErrorCodeDuplicateOrderNumber),
			"unrelated unique violation must not be mapped to the order-number conflict")
	})

	t.Run("successful insert returns no error", func(t *testing.T) {
		gormDB, mock := setupTestDB(t)
		repo := NewPurchaseOrderRepository(NewBaseRepository(gormDB))

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "purchase_orders"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		mock.ExpectCommit()

		po := order()
		err := repo.Create(ctx, po)
		require.NoError(t, err)
		assert.Equal(t, uint(1), po.ID)
	})
}
