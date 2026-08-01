package models

import "github.com/shopspring/decimal"

type InventoryTransactionType string

const (
	InventoryTransactionTypePurchase    InventoryTransactionType = "purchase"
	InventoryTransactionTypeDisposal    InventoryTransactionType = "disposal"
	InventoryTransactionTypeSell        InventoryTransactionType = "sell"
	InventoryTransactionTypeTransferOut InventoryTransactionType = "transfer_out"
	InventoryTransactionTypeTransferIn  InventoryTransactionType = "transfer_in"
	// InventoryTransactionTypeReconcileStockUp raises app stock to match a counted
	// surplus found during reconciliation. Zero-cost, consumable; surfaced as its
	// own "adjustment" category in reports, timeline, and the in/out export.
	InventoryTransactionTypeReconcileStockUp InventoryTransactionType = "reconcile_stock_up"
	// InventoryTransactionTypeInitial loads pre-app opening stock. Zero-cost,
	// consumable, stamped at run time so it sorts last in FIFO. Invisible in every
	// reporting surface: on-hand and FIFO are its only visible effects.
	InventoryTransactionTypeInitial InventoryTransactionType = "initial"
)

// StockDelta returns the signed quantity for this transaction type.
// Positive for types that increase stock (purchase, transfer_in, reconcile_stock_up, initial),
// negative for types that decrease stock (sell, disposal, transfer_out).
// Returns zero for unknown types.
func (t InventoryTransactionType) StockDelta(qty decimal.Decimal) decimal.Decimal {
	switch t {
	case InventoryTransactionTypePurchase, InventoryTransactionTypeTransferIn,
		InventoryTransactionTypeReconcileStockUp, InventoryTransactionTypeInitial:
		return qty
	case InventoryTransactionTypeSell, InventoryTransactionTypeDisposal, InventoryTransactionTypeTransferOut:
		return qty.Neg()
	default:
		return decimal.Zero
	}
}

// GetConsumableTransactionTypes returns the transaction types that can be consumed by an inventory item.
func GetConsumableTransactionTypes() []InventoryTransactionType {
	return []InventoryTransactionType{
		InventoryTransactionTypePurchase,
		InventoryTransactionTypeTransferIn,
		InventoryTransactionTypeReconcileStockUp,
		InventoryTransactionTypeInitial,
	}
}

// InventoryTransaction represents an inventory transaction
type InventoryTransaction struct {
	Base
	InventoryItemID      uint                     `json:"inventory_item_id" gorm:"not null"`
	InventoryItem        *InventoryItem           `json:"inventory_item" gorm:"foreignKey:InventoryItemID"`
	SupplierID           *uint                    `json:"supplier_id"`
	Supplier             *Supplier                `json:"supplier,omitempty" gorm:"foreignKey:SupplierID" validate:"-"`
	TransactionType      InventoryTransactionType `json:"transaction_type" gorm:"not null;check:transaction_type IN ('purchase', 'disposal', 'sell', 'transfer_out', 'transfer_in', 'reconcile_stock_up', 'initial')"`
	Price                float64                  `json:"price" gorm:"not null"`
	Quantity             decimal.Decimal          `json:"quantity" gorm:"type:decimal(10,2);not null"`
	ConsumedQuantity     decimal.Decimal          `json:"consumed_quantity" gorm:"type:decimal(10,2)"`
	CounterTransactionID *uint                    `json:"counter_transaction_id"`
	PurchaseOrderItemID  *uint                    `json:"purchase_order_item_id"`
	// IsAdjustment marks a zero-cost reconcile-correction stock layer: a
	// reconcile_stock_up, or a transfer_in that consumed such a layer (set at
	// transfer time so it propagates across any number of transfer hops).
	IsAdjustment bool `json:"is_adjustment" gorm:"not null;default:false"`
}

// AggTxnQuantities aggregates net quantities by inventory item ID from a slice of transactions.
// Purchases and transfers-in add to quantity, while sells, disposals, and transfers-out subtract.
func AggTxnQuantities(
	txns []*InventoryTransaction,
) map[uint]decimal.Decimal {
	quantities := make(map[uint]decimal.Decimal)

	for _, txn := range txns {
		current := quantities[txn.InventoryItemID]
		quantities[txn.InventoryItemID] = current.Add(txn.TransactionType.StockDelta(txn.Quantity))
	}

	return quantities
}
