package services

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_consumeFIFO(t *testing.T) {
	// Setup specific to this test
	ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")
	_ = ctx

	// Create test service
	service := &inventoryService{}

	t.Run("should consume from single item with single transaction", func(t *testing.T) {
		// Setup test data
		txn1 := &models.InventoryTransaction{
			Base:             models.Base{ID: 1},
			Quantity:         10,
			ConsumedQuantity: 0,
			Price:            100.0,
		}

		item1 := &models.InventoryItem{
			Base:                   models.Base{ID: 1},
			ProductID:              1,
			Quantity:               10,
			ConsumableTransactions: []*models.InventoryTransaction{txn1},
		}

		activeItems := []*models.InventoryItem{item1}
		itemConsumeQuantity := map[uint]int{1: 5}

		consumeHandler := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity int) []*models.InventoryTransaction {
			return []*models.InventoryTransaction{
				{
					InventoryItemID:      item.ID,
					TransactionType:      models.InventoryTransactionTypeSell,
					Price:                consumeTxn.Price,
					Quantity:             quantity,
					CounterTransactionID: &consumeTxn.ID,
				},
			}
		}

		// Execute
		ivtrItemChanges, txns, err := service.consumeFIFO(
			newProcessingState(nil, nil),
			activeItems,
			itemConsumeQuantity,
			consumeHandler,
		)

		// Assertions
		require.NoError(t, err)
		assert.Len(t, ivtrItemChanges, 1, "Should have 1 inventory item change")
		assert.Equal(t, 5, ivtrItemChanges[0].Quantity, "Item quantity should be reduced to 5")
		assert.Equal(t, 10, ivtrItemChanges[0].OriginalQuantity, "Original quantity should be 10")
		assert.Len(t, txns, 2, "Should have 2 transactions (1 sell + 1 update)")
		assert.Equal(t, models.InventoryTransactionTypeSell, txns[0].TransactionType)
		assert.Equal(t, 5, txns[0].Quantity)
		assert.Equal(t, 5, txns[1].ConsumedQuantity, "Purchase transaction should have ConsumedQuantity = 5")
		assert.Equal(t, uint(1), ivtrItemChanges[0].ConsumingTransactionID, "ConsumingTransactionID should be set to txn1 ID")
	})

	t.Run("should consume from single item with multiple transactions in FIFO order", func(t *testing.T) {
		// Setup test data - multiple transactions
		txn1 := &models.InventoryTransaction{
			Base:             models.Base{ID: 1},
			Quantity:         10,
			ConsumedQuantity: 0,
			Price:            100.0,
		}
		txn2 := &models.InventoryTransaction{
			Base:             models.Base{ID: 2},
			Quantity:         8,
			ConsumedQuantity: 0,
			Price:            105.0,
		}

		item1 := &models.InventoryItem{
			Base:                   models.Base{ID: 1},
			ProductID:              1,
			Quantity:               18,
			ConsumableTransactions: []*models.InventoryTransaction{txn1, txn2},
		}

		activeItems := []*models.InventoryItem{item1}
		itemConsumeQuantity := map[uint]int{1: 15}

		consumeHandler := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity int) []*models.InventoryTransaction {
			return []*models.InventoryTransaction{
				{
					InventoryItemID:      item.ID,
					TransactionType:      models.InventoryTransactionTypeSell,
					Price:                consumeTxn.Price,
					Quantity:             quantity,
					CounterTransactionID: &consumeTxn.ID,
				},
			}
		}

		// Execute
		ivtrItemChanges, txns, err := service.consumeFIFO(
			newProcessingState(nil, nil),
			activeItems,
			itemConsumeQuantity,
			consumeHandler,
		)

		// Assertions
		require.NoError(t, err)
		assert.Len(t, ivtrItemChanges, 1, "Should have 1 inventory item change")
		assert.Equal(t, 3, ivtrItemChanges[0].Quantity, "Item quantity should be reduced to 3")
		assert.Equal(t, 18, ivtrItemChanges[0].OriginalQuantity, "Original quantity should be 18")

		// Should have 4 transactions: 2 sell transactions + 2 updated purchase transactions
		assert.Len(t, txns, 4, "Should have 4 transactions")

		// Verify first transaction consumed (10 units from txn1)
		assert.Equal(t, models.InventoryTransactionTypeSell, txns[0].TransactionType)
		assert.Equal(t, 10, txns[0].Quantity)
		assert.Equal(t, 100.0, txns[0].Price)
		assert.Equal(t, 10, txns[1].ConsumedQuantity, "First purchase transaction should be fully consumed")

		// Verify second transaction consumed (5 units from txn2)
		assert.Equal(t, models.InventoryTransactionTypeSell, txns[2].TransactionType)
		assert.Equal(t, 5, txns[2].Quantity)
		assert.Equal(t, 105.0, txns[2].Price)
		assert.Equal(t, 5, txns[3].ConsumedQuantity, "Second purchase transaction should have 5 consumed")
		assert.Equal(t, uint(2), ivtrItemChanges[0].ConsumingTransactionID, "ConsumingTransactionID should be set to txn2 ID (last consumed transaction)")
	})

	t.Run("should consume from multiple items", func(t *testing.T) {
		// Setup test data - two items
		txn1 := &models.InventoryTransaction{
			Base:             models.Base{ID: 1},
			Quantity:         10,
			ConsumedQuantity: 0,
			Price:            100.0,
		}
		txn2 := &models.InventoryTransaction{
			Base:             models.Base{ID: 2},
			Quantity:         20,
			ConsumedQuantity: 0,
			Price:            200.0,
		}

		item1 := &models.InventoryItem{
			Base:                   models.Base{ID: 1},
			ProductID:              1,
			Quantity:               10,
			ConsumableTransactions: []*models.InventoryTransaction{txn1},
		}
		item2 := &models.InventoryItem{
			Base:                   models.Base{ID: 2},
			ProductID:              2,
			Quantity:               20,
			ConsumableTransactions: []*models.InventoryTransaction{txn2},
		}

		activeItems := []*models.InventoryItem{item1, item2}
		itemConsumeQuantity := map[uint]int{1: 5, 2: 10}

		consumeHandler := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity int) []*models.InventoryTransaction {
			return []*models.InventoryTransaction{
				{
					InventoryItemID:      item.ID,
					TransactionType:      models.InventoryTransactionTypeSell,
					Price:                consumeTxn.Price,
					Quantity:             quantity,
					CounterTransactionID: &consumeTxn.ID,
				},
			}
		}

		// Execute
		ivtrItemChanges, txns, err := service.consumeFIFO(
			newProcessingState(nil, nil),
			activeItems,
			itemConsumeQuantity,
			consumeHandler,
		)

		// Assertions
		require.NoError(t, err)
		assert.Len(t, ivtrItemChanges, 2, "Should have 2 inventory item changes")
		assert.Equal(t, 5, ivtrItemChanges[0].Quantity, "First item quantity should be 5")
		assert.Equal(t, 10, ivtrItemChanges[1].Quantity, "Second item quantity should be 10")
		assert.Len(t, txns, 4, "Should have 4 transactions (2 sell + 2 update)")
		assert.Equal(t, uint(1), ivtrItemChanges[0].ConsumingTransactionID, "First item ConsumingTransactionID should be set to txn1 ID")
		assert.Equal(t, uint(2), ivtrItemChanges[1].ConsumingTransactionID, "Second item ConsumingTransactionID should be set to txn2 ID")
	})

	t.Run("should skip items with zero consume quantity", func(t *testing.T) {
		// Setup test data
		txn1 := &models.InventoryTransaction{
			Base:             models.Base{ID: 1},
			Quantity:         10,
			ConsumedQuantity: 0,
			Price:            100.0,
		}
		txn2 := &models.InventoryTransaction{
			Base:             models.Base{ID: 2},
			Quantity:         20,
			ConsumedQuantity: 0,
			Price:            200.0,
		}

		item1 := &models.InventoryItem{
			Base:                   models.Base{ID: 1},
			ProductID:              1,
			Quantity:               10,
			ConsumableTransactions: []*models.InventoryTransaction{txn1},
		}
		item2 := &models.InventoryItem{
			Base:                   models.Base{ID: 2},
			ProductID:              2,
			Quantity:               20,
			ConsumableTransactions: []*models.InventoryTransaction{txn2},
		}

		activeItems := []*models.InventoryItem{item1, item2}
		itemConsumeQuantity := map[uint]int{1: 0, 2: 10} // item1 has 0 consume quantity

		consumeHandler := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity int) []*models.InventoryTransaction {
			return []*models.InventoryTransaction{
				{
					InventoryItemID:      item.ID,
					TransactionType:      models.InventoryTransactionTypeSell,
					Price:                consumeTxn.Price,
					Quantity:             quantity,
					CounterTransactionID: &consumeTxn.ID,
				},
			}
		}

		// Execute
		ivtrItemChanges, txns, err := service.consumeFIFO(
			newProcessingState(nil, nil),
			activeItems,
			itemConsumeQuantity,
			consumeHandler,
		)

		// Assertions
		require.NoError(t, err)
		assert.Len(t, ivtrItemChanges, 1, "Should have only 1 inventory item change (item2)")
		assert.Equal(t, uint(2), ivtrItemChanges[0].ID, "Should be item2")
		assert.Len(t, txns, 2, "Should have 2 transactions (1 sell + 1 update)")
	})

	t.Run("should skip items not in itemConsumeQuantity map", func(t *testing.T) {
		// Setup test data
		txn1 := &models.InventoryTransaction{
			Base:             models.Base{ID: 1},
			Quantity:         10,
			ConsumedQuantity: 0,
			Price:            100.0,
		}
		txn2 := &models.InventoryTransaction{
			Base:             models.Base{ID: 2},
			Quantity:         20,
			ConsumedQuantity: 0,
			Price:            200.0,
		}

		item1 := &models.InventoryItem{
			Base:                   models.Base{ID: 1},
			ProductID:              1,
			Quantity:               10,
			ConsumableTransactions: []*models.InventoryTransaction{txn1},
		}
		item2 := &models.InventoryItem{
			Base:                   models.Base{ID: 2},
			ProductID:              2,
			Quantity:               20,
			ConsumableTransactions: []*models.InventoryTransaction{txn2},
		}

		activeItems := []*models.InventoryItem{item1, item2}
		itemConsumeQuantity := map[uint]int{2: 10} // only item2 in map

		consumeHandler := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity int) []*models.InventoryTransaction {
			return []*models.InventoryTransaction{
				{
					InventoryItemID:      item.ID,
					TransactionType:      models.InventoryTransactionTypeSell,
					Price:                consumeTxn.Price,
					Quantity:             quantity,
					CounterTransactionID: &consumeTxn.ID,
				},
			}
		}

		// Execute
		ivtrItemChanges, _, err := service.consumeFIFO(
			newProcessingState(nil, nil),
			activeItems,
			itemConsumeQuantity,
			consumeHandler,
		)

		// Assertions
		require.NoError(t, err)
		assert.Len(t, ivtrItemChanges, 1, "Should have only 1 inventory item change (item2)")
		assert.Equal(t, uint(2), ivtrItemChanges[0].ID, "Should be item2")
	})

	t.Run("should skip transactions with fully consumed quantity", func(t *testing.T) {
		// Setup test data - first transaction is fully consumed
		txn1 := &models.InventoryTransaction{
			Base:             models.Base{ID: 1},
			Quantity:         10,
			ConsumedQuantity: 10, // fully consumed
			Price:            100.0,
		}
		txn2 := &models.InventoryTransaction{
			Base:             models.Base{ID: 2},
			Quantity:         8,
			ConsumedQuantity: 3,
			Price:            105.0,
		}

		item1 := &models.InventoryItem{
			Base:                   models.Base{ID: 1},
			ProductID:              1,
			Quantity:               5,
			ConsumableTransactions: []*models.InventoryTransaction{txn1, txn2},
		}

		activeItems := []*models.InventoryItem{item1}
		itemConsumeQuantity := map[uint]int{1: 3}

		consumeHandler := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity int) []*models.InventoryTransaction {
			return []*models.InventoryTransaction{
				{
					InventoryItemID:      item.ID,
					TransactionType:      models.InventoryTransactionTypeSell,
					Price:                consumeTxn.Price,
					Quantity:             quantity,
					CounterTransactionID: &consumeTxn.ID,
				},
			}
		}

		// Execute
		ivtrItemChanges, txns, err := service.consumeFIFO(
			newProcessingState(nil, nil),
			activeItems,
			itemConsumeQuantity,
			consumeHandler,
		)

		// Assertions
		require.NoError(t, err)
		assert.Len(t, ivtrItemChanges, 1, "Should have 1 inventory item change")
		assert.Equal(t, 2, ivtrItemChanges[0].Quantity, "Item quantity should be reduced to 2")
		assert.Len(t, txns, 2, "Should have 2 transactions (1 sell + 1 update)")
		// Should only consume from txn2 (not txn1 which is fully consumed)
		assert.Equal(t, 6, txns[1].ConsumedQuantity, "Transaction 2 should have ConsumedQuantity = 6")
		assert.Equal(t, uint(2), ivtrItemChanges[0].ConsumingTransactionID, "ConsumingTransactionID should be set to txn2 ID (skipped fully consumed txn1)")
	})

	t.Run("should return error when item not found in active items", func(t *testing.T) {
		// Setup test data
		txn1 := &models.InventoryTransaction{
			Base:             models.Base{ID: 1},
			Quantity:         10,
			ConsumedQuantity: 0,
			Price:            100.0,
		}

		item1 := &models.InventoryItem{
			Base:                   models.Base{ID: 1},
			ProductID:              1,
			Quantity:               10,
			ConsumableTransactions: []*models.InventoryTransaction{txn1},
		}

		activeItems := []*models.InventoryItem{item1}
		itemConsumeQuantity := map[uint]int{999: 5} // item ID 999 not in activeItems

		consumeHandler := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity int) []*models.InventoryTransaction {
			return []*models.InventoryTransaction{}
		}

		// Execute
		ivtrItemChanges, txns, err := service.consumeFIFO(
			newProcessingState(nil, nil),
			activeItems,
			itemConsumeQuantity,
			consumeHandler,
		)

		// Assertions
		require.Error(t, err)
		assert.Nil(t, ivtrItemChanges)
		assert.Nil(t, txns)
		assert.Contains(t, err.Error(), "failed to validate consume FIFO")
	})

	t.Run("should return error when consume quantity exceeds available quantity", func(t *testing.T) {
		// Setup test data
		txn1 := &models.InventoryTransaction{
			Base:             models.Base{ID: 1},
			Quantity:         10,
			ConsumedQuantity: 0,
			Price:            100.0,
		}

		item1 := &models.InventoryItem{
			Base:                   models.Base{ID: 1},
			ProductID:              1,
			Quantity:               10,
			ConsumableTransactions: []*models.InventoryTransaction{txn1},
		}

		activeItems := []*models.InventoryItem{item1}
		itemConsumeQuantity := map[uint]int{1: 15} // trying to consume 15 but only 10 available

		consumeHandler := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity int) []*models.InventoryTransaction {
			return []*models.InventoryTransaction{}
		}

		// Execute
		ivtrItemChanges, txns, err := service.consumeFIFO(
			newProcessingState(nil, nil),
			activeItems,
			itemConsumeQuantity,
			consumeHandler,
		)

		// Assertions
		require.Error(t, err)
		assert.Nil(t, ivtrItemChanges)
		assert.Nil(t, txns)
		assert.Contains(t, err.Error(), "failed to validate consume FIFO")
	})

	t.Run("should update ConsumingTransactionID correctly", func(t *testing.T) {
		// Setup test data
		txn1 := &models.InventoryTransaction{
			Base:             models.Base{ID: 1},
			Quantity:         10,
			ConsumedQuantity: 0,
			Price:            100.0,
		}

		item1 := &models.InventoryItem{
			Base:                   models.Base{ID: 1},
			ProductID:              1,
			Quantity:               10,
			ConsumableTransactions: []*models.InventoryTransaction{txn1},
		}

		activeItems := []*models.InventoryItem{item1}
		itemConsumeQuantity := map[uint]int{1: 5}

		consumeHandler := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity int) []*models.InventoryTransaction {
			return []*models.InventoryTransaction{
				{
					InventoryItemID:      item.ID,
					TransactionType:      models.InventoryTransactionTypeSell,
					Price:                consumeTxn.Price,
					Quantity:             quantity,
					CounterTransactionID: &consumeTxn.ID,
				},
			}
		}

		// Execute
		ivtrItemChanges, txns, err := service.consumeFIFO(
			newProcessingState(nil, nil),
			activeItems,
			itemConsumeQuantity,
			consumeHandler,
		)

		// Assertions
		require.NoError(t, err)
		assert.Equal(t, uint(1), ivtrItemChanges[0].ConsumingTransactionID, "ConsumingTransactionID should be set to txn1 ID")
		assert.Len(t, txns, 2)
	})

	t.Run("should handle empty active items list", func(t *testing.T) {
		// Setup test data
		activeItems := []*models.InventoryItem{}
		itemConsumeQuantity := map[uint]int{}

		consumeHandler := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity int) []*models.InventoryTransaction {
			return []*models.InventoryTransaction{}
		}

		// Execute
		ivtrItemChanges, txns, err := service.consumeFIFO(
			newProcessingState(nil, nil),
			activeItems,
			itemConsumeQuantity,
			consumeHandler,
		)

		// Assertions
		require.NoError(t, err)
		assert.Empty(t, ivtrItemChanges)
		assert.Empty(t, txns)
	})
}
