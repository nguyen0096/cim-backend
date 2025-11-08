package models

// Unit represents a measurement unit definition used by products
type Unit struct {
	Base
	UnitType         string     `json:"unit_type" gorm:"type:varchar(50);not null"`
	Name             string     `json:"name" gorm:"type:varchar(100);not null"`
	Symbol           string     `json:"symbol" gorm:"type:varchar(20);not null"`
	IsBase           bool       `json:"is_base" gorm:"default:false"`
	ConversionFactor float64    `json:"conversion_factor" gorm:"not null;default:1"`
	Products         []*Product `json:"products,omitempty" gorm:"foreignKey:UnitID" validate:"-"`
}

