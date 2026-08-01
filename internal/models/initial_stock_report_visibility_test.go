package models

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A mixed month (purchase + sell + an opening-stock load) must render the load,
// treated exactly as a reconcile stock-up is: present in the transaction list and
// carrying no PO. Parity with stock-up is asserted rather than a bucket total,
// because the legacy view has no bucket for either type.
func TestBuildLegacyView_RendersInitialLikeStockUp(t *testing.T) {
	from := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	at := func(day int) time.Time { return time.Date(2026, time.July, day, 12, 0, 0, 0, time.UTC) }

	unit := &Unit{Base: Base{ID: 5}, Name: "KG"}
	product := &Product{Base: Base{ID: 9}, Name: "CÀ PHÊ"}

	build := func(itemID uint, extra *InventoryTransaction) *TxnReportInventoryItem {
		item := &InventoryItem{Base: Base{ID: itemID}, ProductID: product.ID, Product: product, Unit: unit}
		purchase := &InventoryTransaction{
			Base: Base{ID: 100, CreatedAt: at(3)}, InventoryItemID: itemID,
			TransactionType: InventoryTransactionTypePurchase, Quantity: decimal.NewFromInt(20), Price: 10,
		}
		sell := &InventoryTransaction{
			Base: Base{ID: 101, CreatedAt: at(10)}, InventoryItemID: itemID,
			TransactionType: InventoryTransactionTypeSell, Quantity: decimal.NewFromInt(5), Price: 10,
		}
		extra.InventoryItemID = itemID
		rb, err := NewReportBuilder(&Inventory{Base: Base{ID: 3}, Name: "KHO"}, from, to).
			Txns([]*InventoryTransaction{purchase, sell, extra}).
			ConsumeTxns([]*InventoryTransaction{sell}).
			HistoricalTxns([]*InventoryTransaction{}).
			InventoryItemLookup(map[uint]*InventoryItem{itemID: item}).
			PurchaseOrderItemLookup(map[uint]*PurchaseOrderItem{}).
			Build()
		require.NoError(t, err)
		out, err := rb.GetOutput()
		require.NoError(t, err)
		require.Len(t, out.Items, 1)
		return out.Items[0]
	}

	withInitial := build(41, &InventoryTransaction{
		Base:            Base{ID: 200, CreatedAt: at(5)},
		TransactionType: InventoryTransactionTypeInitial,
		Quantity:        decimal.NewFromInt(30), Price: 0, IsAdjustment: true,
	})
	withStockUp := build(42, &InventoryTransaction{
		Base:            Base{ID: 201, CreatedAt: at(5)},
		TransactionType: InventoryTransactionTypeReconcileStockUp,
		Quantity:        decimal.NewFromInt(30), Price: 0, IsAdjustment: true,
	})

	var found *InventoryTransaction
	for _, txn := range withInitial.Transactions {
		if txn.TransactionType == InventoryTransactionTypeInitial {
			found = txn
		}
	}
	require.NotNil(t, found, "an opening-stock load must appear in the report transactions")
	assert.True(t, found.Quantity.Equal(decimal.NewFromInt(30)))
	assert.Nil(t, found.PurchaseOrderItemID, "opening stock has no purchase order")

	// Identical treatment to a stock-up of the same size.
	assert.Len(t, withInitial.Transactions, len(withStockUp.Transactions))
	assert.True(t, withInitial.PurchaseQuantity.Equal(withStockUp.PurchaseQuantity))
	assert.True(t, withInitial.ReconcileQuantity.Equal(withStockUp.ReconcileQuantity))
	assert.True(t, withInitial.EndQuantity.Equal(withStockUp.EndQuantity),
		"initial must foot exactly as reconcile_stock_up does, got %s vs %s",
		withInitial.EndQuantity, withStockUp.EndQuantity)
}
