package models

type InventoryTransactionType string

const (
	InventoryTransactionTypePurchase InventoryTransactionType = "purchase"
	InventoryTransactionTypeDisposal InventoryTransactionType = "disposal"
	InventoryTransactionTypeSell     InventoryTransactionType = "sell"
)

// InventoryTransaction represents an inventory transaction
type InventoryTransaction struct {
	Base
	InventoryItemID      uint                     `json:"inventory_item_id"`
	InventoryItem        *InventoryItem           `json:"inventory_item" gorm:"foreignKey:InventoryItemID"`
	SupplierID           *uint                    `json:"supplier_id"`
	Supplier             *Supplier                `json:"supplier,omitempty" gorm:"foreignKey:SupplierID" validate:"-"`
	TransactionType      InventoryTransactionType `json:"transaction_type" gorm:"not null;check:transaction_type IN ('purchase', 'disposal', 'sell')"`
	Price                float64                  `json:"price" gorm:"not null"`
	Quantity             int                      `json:"quantity" gorm:"not null"`
	ConsumedQuantity     int                      `json:"consumed_quantity"`
	CounterTransactionID *uint                    `json:"counter_transaction_id"`
	PurchaseOrderItemID  *uint                    `json:"purchase_order_item_id"`
}
