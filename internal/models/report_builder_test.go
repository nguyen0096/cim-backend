package models

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rbInventory() *Inventory {
	return &Inventory{Base: Base{ID: 1}, Name: "Kho A"}
}

func rbItemLookup() map[uint]*InventoryItem {
	return map[uint]*InventoryItem{
		1: {Base: Base{ID: 1}, Product: &Product{Name: "Widget"}},
	}
}

// Source view: a period reconcile_stock_up (once classified as a period source)
// becomes its own zero-cost source row.
func TestBuildSourceView_ReconcileStockUp_IsSourceRow(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)

	stockUp := &InventoryTransaction{Base: Base{ID: 3, CreatedAt: from}, InventoryItemID: 1, TransactionType: InventoryTransactionTypeReconcileStockUp, Quantity: decimal.NewFromInt(30), Price: 0}
	purchase := &InventoryTransaction{Base: Base{ID: 1, CreatedAt: from}, InventoryItemID: 1, TransactionType: InventoryTransactionTypePurchase, Quantity: decimal.NewFromInt(50), Price: 10}

	rb, err := NewReportBuilder(rbInventory(), from, to).
		Txns([]*InventoryTransaction{purchase, stockUp}).
		SourceTxns([]*InventoryTransaction{purchase, stockUp}).
		InventoryItemLookup(rbItemLookup()).
		Build()
	require.NoError(t, err)
	out, err := rb.GetOutput()
	require.NoError(t, err)

	var found bool
	for _, it := range out.Items {
		if it.SourceTransaction != nil && it.SourceTransaction.TransactionType == InventoryTransactionTypeReconcileStockUp {
			found = true
			assert.Equal(t, float64(0), it.SourceTransaction.Price, "adjustment source layer must be zero-cost")
		}
	}
	assert.True(t, found, "the reconcile_stock_up must appear as its own source row in the source view")
}
