package models

// MenuItem represents a menu item that can contain products and belong to menus
type MenuItem struct {
	Base
	Name     string     `json:"name" gorm:"not null"`
	Menus    []*Menu    `json:"menus,omitempty" gorm:"many2many:menu_menu_items;" validate:"-"`
	Products []*Product `json:"products,omitempty" gorm:"many2many:menu_item_products;" validate:"-"`
}
