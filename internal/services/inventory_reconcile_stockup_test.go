package services

import (
	"testing"
	"time"

	"cim-backend/internal/models"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildReconcileStockUps_RaisesStockWithBackdatedConsumableTxn(t *testing.T) {
	backdatedAt := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	item := &models.InventoryItem{Base: models.Base{ID: 10}, Quantity: decimal.NewFromInt(100)}
	activeItemMap := map[uint]*models.InventoryItem{10: item}

	changes, txns := buildReconcileStockUps(
		activeItemMap,
		map[uint]decimal.Decimal{10: decimal.NewFromInt(20)},
		backdatedAt,
	)

	require.Len(t, changes, 1)
	require.Len(t, txns, 1)

	assert.True(t, changes[0].OriginalQuantity.Equal(decimal.NewFromInt(100)),
		"optimistic-lock baseline must be the pre-stock-up on-hand")
	assert.True(t, item.Quantity.Equal(decimal.NewFromInt(120)), "on-hand must rise by the surplus")

	txn := txns[0]
	assert.Equal(t, models.InventoryTransactionTypeReconcileStockUp, txn.TransactionType)
	assert.Equal(t, uint(10), txn.InventoryItemID)
	assert.True(t, txn.Quantity.Equal(decimal.NewFromInt(20)), "txn quantity must be the surplus")
	assert.True(t, txn.ConsumedQuantity.IsZero(), "stock-up txn must be fully unconsumed")
	assert.Equal(t, float64(0), txn.Price, "stock-up price must be 0")
	assert.Equal(t, backdatedAt, txn.CreatedAt, "txn must be backdated to reconciliation initiation")
	assert.Nil(t, txn.CounterTransactionID)

	// Invariant: the raised on-hand equals the sum of unconsumed consumable txn quantity.
	assert.True(t, txn.Quantity.Sub(txn.ConsumedQuantity).Equal(item.Quantity.Sub(decimal.NewFromInt(100))))
}

func TestBuildReconcileStockUps_NoSurplus_NoOp(t *testing.T) {
	item := &models.InventoryItem{Base: models.Base{ID: 10}, Quantity: decimal.NewFromInt(50)}
	changes, txns := buildReconcileStockUps(
		map[uint]*models.InventoryItem{10: item},
		map[uint]decimal.Decimal{},
		time.Now(),
	)
	assert.Empty(t, changes)
	assert.Empty(t, txns)
	assert.True(t, item.Quantity.Equal(decimal.NewFromInt(50)), "on-hand must be untouched")
}
