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
	InventoryItemID     uint                     `json:"inventory_item_id"`
	InventoryItem       InventoryItem            `json:"inventory_item" gorm:"foreignKey:InventoryItemID"`
	TransactionType     InventoryTransactionType `json:"transaction_type" gorm:"not null;check:transaction_type IN ('purchase', 'disposal')"`
	Price               float64                  `json:"price" gorm:"not null"`
	Quantity            int                      `json:"quantity" gorm:"not null"`
	PurchaseOrderItemID *uint                    `json:"purchase_order_item_id"`
}
