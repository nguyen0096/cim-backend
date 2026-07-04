package repository

import (
	"context"
	"regexp"
	"testing"

	"cim-backend/internal/services/dto"
	"cim-backend/pkg"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReceiveInventory_RejectsNegativeReceivedQuantity rejects a negative
// received_quantity before any write (zero stays allowed).
func TestReceiveInventory_RejectsNegativeReceivedQuantity(t *testing.T) {
	for _, q := range []int64{-60, -1} {
		gormDB, mock := setupTestDB(t)
		repo := NewPurchaseOrderRepository(NewBaseRepository(gormDB))
		ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`FROM "purchase_orders"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "inventory_id", "status"}).
				AddRow(1, 1, "partially_delivered"))
		mock.ExpectQuery(`FROM purchase_order_items poi`).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "purchase_order_id", "product_id", "supplier_id", "unit_id", "received_quantity",
				"inventory_item_id", "unit_id", "unit_name", "unit_decimal_places",
			}).AddRow(1, 1, 1, 1, 1, "50.00", nil, 1, "Kg", 2))
		mock.ExpectRollback()

		_, err := repo.ReceiveInventory(ctx, dto.UpdatePurchaseOrderDeliveryStatusRequest{
			PurchaseOrderID: 1,
			Items: []struct {
				ID               uint            `json:"id" validate:"required"`
				ReceivedQuantity decimal.Decimal `json:"received_quantity" validate:"required"`
			}{{ID: 1, ReceivedQuantity: decimal.NewFromInt(q)}},
		})

		require.Error(t, err, "negative received_quantity %d must be rejected", q)
		var appErr *pkg.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
		assert.NoError(t, mock.ExpectationsWereMet())
	}
}

// TestIncreaseQuantityInventoryItems_FloorsAtZero asserts the receive UPDATE
// floors on-hand at zero (GREATEST).
func TestIncreaseQuantityInventoryItems_FloorsAtZero(t *testing.T) {
	gormDB, mock := setupTestDB(t)
	repo := NewPurchaseOrderRepository(NewBaseRepository(gormDB)).(*purchaseOrderRepository)

	mock.ExpectExec(`GREATEST\(0, ii\.quantity \+ payload\.delta`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.increaseQuantityInventoryItems(gormDB, map[uint]decimal.Decimal{1: decimal.NewFromInt(5)})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
