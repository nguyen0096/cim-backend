package models

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestStockDelta(t *testing.T) {
	qty := decimal.NewFromInt(10)

	tests := []struct {
		name     string
		txnType  InventoryTransactionType
		expected decimal.Decimal
	}{
		{"purchase increases stock", InventoryTransactionTypePurchase, qty},
		{"transfer_in increases stock", InventoryTransactionTypeTransferIn, qty},
		{"sell decreases stock", InventoryTransactionTypeSell, qty.Neg()},
		{"disposal decreases stock", InventoryTransactionTypeDisposal, qty.Neg()},
		{"transfer_out decreases stock", InventoryTransactionTypeTransferOut, qty.Neg()},
		{"unknown type returns zero", InventoryTransactionType("unknown"), decimal.Zero},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.txnType.StockDelta(qty)
			assert.True(t, tt.expected.Equal(result), "expected %s, got %s", tt.expected, result)
		})
	}
}

func TestAggTxnQuantities_IncludesTransferIn(t *testing.T) {
	itemID := uint(1)
	qty := decimal.NewFromInt(10)

	txns := []*InventoryTransaction{
		{InventoryItemID: itemID, TransactionType: InventoryTransactionTypePurchase, Quantity: qty},
		{InventoryItemID: itemID, TransactionType: InventoryTransactionTypeTransferIn, Quantity: qty},
		{InventoryItemID: itemID, TransactionType: InventoryTransactionTypeSell, Quantity: decimal.NewFromInt(5)},
		{InventoryItemID: itemID, TransactionType: InventoryTransactionTypeTransferOut, Quantity: decimal.NewFromInt(3)},
	}

	result := AggTxnQuantities(txns)

	// 10 (purchase) + 10 (transfer_in) - 5 (sell) - 3 (transfer_out) = 12
	expected := decimal.NewFromInt(12)
	assert.True(t, expected.Equal(result[itemID]), "expected %s, got %s", expected, result[itemID])
}
