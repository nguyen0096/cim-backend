package models

// Inventory represents inventory for a product
type Inventory struct {
	Base
	ProductID    uint     `json:"product_id" gorm:"unique;not null"`
	Product      *Product `json:"product,omitempty" gorm:"foreignKey:ProductID" validate:"-"`
	Quantity     int      `json:"quantity" gorm:"default:0"`
	ReorderLevel int      `json:"reorder_level" gorm:"default:0"`
	Location     string   `json:"location"`
}
