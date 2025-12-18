package models

// Menu represents a menu that can contain menu items and be associated with inventories
type Menu struct {
	Base
	Name        string       `json:"name" gorm:"not null"`
	MenuItems   []*MenuItem  `json:"menu_items,omitempty" gorm:"many2many:menu_menu_items;" validate:"-"`
	Inventories []*Inventory `json:"inventories,omitempty" gorm:"many2many:menu_inventories;" validate:"-"`
}
