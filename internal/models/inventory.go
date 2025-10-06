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
		if item != nil && item.Product != nil {
			total += float64(item.Quantity) * item.Product.UnitPrice
		}
	}
	return total
}

// GetLowStockItems returns items that are below reorder level
func (i *Inventory) GetLowStockItems() []*InventoryItem {
	var lowStockItems []*InventoryItem
	for _, item := range i.Items {
		if item != nil && item.Quantity <= item.ReorderLevel {
			lowStockItems = append(lowStockItems, item)
		}
	}
	return lowStockItems
}
