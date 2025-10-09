package models

// InventoryTransaction represents an inventory transaction
type InventoryTransaction struct {
	Base
	InventoryItemID uint          `json:"inventory_item_id"`
	InventoryItem   InventoryItem `json:"inventory_item" gorm:"foreignKey:InventoryItemID"`
	TransactionType string        `json:"transaction_type" gorm:"not null;check:transaction_type IN ('purchase')"`
	Quantity        int           `json:"quantity" gorm:"not null"`
}
