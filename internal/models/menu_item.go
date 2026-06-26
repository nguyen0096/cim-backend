package models

import (
	"github.com/lib/pq"
)

// MenuItem represents a menu item that can contain products and belong to menus
type MenuItem struct {
	Base
	Name     string         `json:"name" gorm:"not null"`
	Tags     pq.StringArray `json:"tags,omitempty" gorm:"type:text[]" swaggertype:"array,string"`
	Menus    []*Menu        `json:"menus,omitempty" gorm:"many2many:menu_menu_items;" validate:"-"`
	Products []*Product     `json:"products,omitempty" gorm:"many2many:menu_item_products;" validate:"-"`
}
