package models

import "github.com/shopspring/decimal"

type UnitConversion struct {
	Base
	FromUnitID       uint            `json:"from_unit_id" gorm:"index:idx_unit_conversions_unique,unique;not null"`
	FromUnit         *Unit           `json:"from_unit" gorm:"foreignKey:FromUnitID"`
	ToUnitID         uint            `json:"to_unit_id" gorm:"index:idx_unit_conversions_unique,unique;not null"`
	ToUnit           *Unit           `json:"to_unit" gorm:"foreignKey:ToUnitID"`
	ConversionFactor decimal.Decimal `json:"conversion_factor" gorm:"not null"`
}
