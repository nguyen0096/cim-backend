package models

type Supplier struct {
	Base
	Name         string `json:"name" gorm:"not null"`
	ContactEmail string `json:"contact_email"`
	ContactPhone string `json:"contact_phone"`
	Address      string `json:"address"`
}
