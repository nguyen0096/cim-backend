package models

// Unit represents a measurement unit definition used by products
type Unit struct {
	Base
	UnitType         string     `json:"unit_type" gorm:"type:varchar(50);not null"`
	Name             string     `json:"name" gorm:"type:varchar(100);not null"`
	Symbol           string     `json:"symbol" gorm:"type:varchar(20);not null"`
	ConversionFactor float64    `json:"conversion_factor" gorm:"not null;default:1"`
	BaseUnitID       *uint      `json:"base_unit_id,omitempty" gorm:"column:base_unit_id"`
	BaseUnit         *Unit      `json:"base_unit,omitempty" gorm:"foreignKey:BaseUnitID;references:ID" validate:"-"`
	DerivedUnits     []*Unit    `json:"derived_units,omitempty" gorm:"foreignKey:BaseUnitID" validate:"-"`
	Products         []*Product `json:"products,omitempty" gorm:"foreignKey:UnitID" validate:"-"`
}
