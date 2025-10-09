package models

// InventoryStatus represents the status of inventory
type InventoryStatus string

const (
	InventoryStatusActive   InventoryStatus = "active"
	InventoryStatusInactive InventoryStatus = "inactive"
)

// Inventory represents a warehouse or storage location
type Inventory struct {
	Base
	Name        string           `json:"name" gorm:"not null"`
	Description string           `json:"description"`
	Location    string           `json:"location" gorm:"not null"`
	Status      InventoryStatus  `json:"status" gorm:"default:active"`
	Items       []*InventoryItem `json:"items,omitempty" gorm:"foreignKey:InventoryID" validate:"-"`
}

// GetTotalItems returns the total number of items in this inventory
func (i *Inventory) GetTotalItems() int {
	return len(i.Items)
}

// GetTotalValue calculates the total value of all items in this inventory
func (i *Inventory) GetTotalValue() float64 {
	var total float64
	for _, item := range i.Items {
		if item != nil {
			total += item.CalculateTotalValue()
		}
	}
	return total
}
