package models

type Supplier struct {
	Base
	Name         string     `json:"name" gorm:"not null"`
	ContactEmail string     `json:"contact_email"`
	ContactPhone string     `json:"contact_phone"`
	Address      string     `json:"address"`
	Status       string     `json:"status" gorm:"default:active;check:status IN ('active', 'inactive')"`
	Products     []*Product `json:"products,omitempty" gorm:"many2many:product_suppliers;" validate:"-"`
}
