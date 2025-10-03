package models

// InventoryTransaction represents an inventory transaction
type InventoryTransaction struct {
	Base
	InventoryID     uint      `json:"inventory_id"`
	Inventory       Inventory `json:"inventory" gorm:"foreignKey:InventoryID"`
	ProductID       uint      `json:"product_id"`
	Product         Product   `json:"product" gorm:"foreignKey:ProductID"`
	TransactionType string    `json:"transaction_type" gorm:"not null;check:transaction_type IN ('purchase')"`
	Quantity        int       `json:"quantity" gorm:"not null"`
}
