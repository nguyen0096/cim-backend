package models

import "github.com/shopspring/decimal"

type InventoryTransactionType string

const (
	InventoryTransactionTypePurchase    InventoryTransactionType = "purchase"
	InventoryTransactionTypeDisposal    InventoryTransactionType = "disposal"
	InventoryTransactionTypeSell        InventoryTransactionType = "sell"
	InventoryTransactionTypeTransferOut InventoryTransactionType = "transfer_out"
	InventoryTransactionTypeTransferIn  InventoryTransactionType = "transfer_in"
)

// GetConsumableTransactionTypes returns the transaction types that can be consumed by an inventory item.
func GetConsumableTransactionTypes() []InventoryTransactionType {
	return []InventoryTransactionType{
		InventoryTransactionTypePurchase,
		InventoryTransactionTypeTransferIn,
	}
}

// InventoryTransaction represents an inventory transaction
type InventoryTransaction struct {
	Base
	InventoryItemID      uint                     `json:"inventory_item_id" gorm:"not null"`
	InventoryItem        *InventoryItem           `json:"inventory_item" gorm:"foreignKey:InventoryItemID"`
	SupplierID           *uint                    `json:"supplier_id"`
	Supplier             *Supplier                `json:"supplier,omitempty" gorm:"foreignKey:SupplierID" validate:"-"`
	TransactionType      InventoryTransactionType `json:"transaction_type" gorm:"not null;check:transaction_type IN ('purchase', 'disposal', 'sell', 'transfer_out', 'transfer_in')"`
	Price                float64                  `json:"price" gorm:"not null"`
	Quantity             decimal.Decimal          `json:"quantity" gorm:"type:decimal(10,2);not null"`
	ConsumedQuantity     decimal.Decimal          `json:"consumed_quantity" gorm:"type:decimal(10,2)"`
	CounterTransactionID *uint                    `json:"counter_transaction_id"`
	PurchaseOrderItemID  *uint                    `json:"purchase_order_item_id"`
}

// AggTxnQuantities aggregates net quantities by inventory item ID from a slice of transactions.
// Purchases and transfers-in add to quantity, while sells, disposals, and transfers-out subtract.
func AggTxnQuantities(
	txns []*InventoryTransaction,
) map[uint]decimal.Decimal {
	quantities := make(map[uint]decimal.Decimal)

	for _, txn := range txns {
		current, exists := quantities[txn.InventoryItemID]
		if !exists {
			current = decimal.Zero
		}

		switch txn.TransactionType {
		case InventoryTransactionTypePurchase, InventoryTransactionTypeTransferIn:
			quantities[txn.InventoryItemID] = current.Add(txn.Quantity)
		case InventoryTransactionTypeSell, InventoryTransactionTypeDisposal, InventoryTransactionTypeTransferOut:
			quantities[txn.InventoryItemID] = current.Sub(txn.Quantity)
		}
	}

	return quantities
}
