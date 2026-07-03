package models

import "strings"

// Unit represents a measurement unit definition used by products
// Supports a 4-level hierarchy: Level 1 (root base unit) through Level 4 (leaf units)
type Unit struct {
	Base
	UnitType                  string     `json:"unit_type" gorm:"type:varchar(50);not null"`
	Name                      string     `json:"name" gorm:"type:varchar(100);not null"`
	Symbol                    string     `json:"symbol" gorm:"type:varchar(20);not null"`
	ConversionFactor          float64    `json:"conversion_factor" gorm:"not null;default:1"`
	ConversionFactorToCurrent *float64   `json:"conversion_factor_to_current,omitempty" gorm:"-"` // Calculated field, not stored in DB
	Level                     int        `json:"level" gorm:"type:smallint;not null;default:1;check:level >= 1 AND level <= 4"`
	DecimalPlaces             int        `json:"decimal_places" gorm:"type:smallint;not null;default:2;check:decimal_places >= 0 AND decimal_places <= 10"`
	BaseUnitID                *uint      `json:"base_unit_id,omitempty" gorm:"column:base_unit_id"`
	BaseUnit                  *Unit      `json:"base_unit,omitempty" gorm:"foreignKey:BaseUnitID;references:ID" validate:"-"`
	DerivedUnits              []*Unit    `json:"derived_units,omitempty" gorm:"foreignKey:BaseUnitID" validate:"-"`
	Products                  []*Product `json:"products,omitempty" gorm:"foreignKey:UnitID" validate:"-"`
}

// IsRootBaseUnit returns true if this unit is a root base unit (Level 1, has no base unit)
func (u *Unit) IsRootBaseUnit() bool {
	return u.Level == 1 && u.BaseUnitID == nil
}

// IsLeafUnit returns true if this unit is at the maximum level (Level 4)
func (u *Unit) IsLeafUnit() bool {
	return u.Level == 4
}

// GetRootBaseUnitID returns the ID of the root base unit in the hierarchy.
func (u *Unit) GetRootBaseUnitID() uint {
	if u.IsRootBaseUnit() {
		return u.ID
	}
	if u.BaseUnit != nil {
		return u.BaseUnit.GetRootBaseUnitID()
	}
	return u.ID
}

// GetTotalConversionFactorToRoot returns the product of conversion factors from
// this unit up to the root base unit.
func (u *Unit) GetTotalConversionFactorToRoot() float64 {
	if u.IsRootBaseUnit() {
		return 1.0
	}
	if u.BaseUnit != nil {
		return u.ConversionFactor * u.BaseUnit.GetTotalConversionFactorToRoot()
	}
	return u.ConversionFactor
}

// GetExpectedLevel returns the expected level: 1 when BaseUnitID is nil, else
// base unit's level + 1.
func (u *Unit) GetExpectedLevel() int {
	if u.BaseUnitID == nil {
		return 1
	}
	if u.BaseUnit != nil {
		return u.BaseUnit.Level + 1
	}
	return u.Level
}

// CalculateConversionFactorToCurrent returns the factor to convert from this unit
// to the target unit (e.g. this=102, target=101 -> 3), or 0 when no path exists.
func (u *Unit) CalculateConversionFactorToCurrent(targetUnitID uint) float64 {
	if u.ID == targetUnitID {
		return 1.0
	}

	currentUnit := u
	visited := make(map[uint]bool)
	factor := 1.0

	for currentUnit != nil && currentUnit.ID != targetUnitID {
		if visited[currentUnit.ID] {
			return 0 // Circular reference
		}
		visited[currentUnit.ID] = true

		if currentUnit.BaseUnitID == nil {
			return 0
		}

		factor *= currentUnit.ConversionFactor

		if currentUnit.BaseUnit == nil {
			return 0 // Base unit not loaded
		}
		currentUnit = currentUnit.BaseUnit
	}

	if currentUnit != nil && currentUnit.ID == targetUnitID {
		return factor
	}

	return 0
}

// StandardizeName standardizes the unit name to uppercase and trims the whitespace.
func (u *Unit) StandardizeName() {
	u.Name = strings.ToUpper(strings.TrimSpace(u.Name))
}

// DeduceDefaultsFromBaseUnit sets level and conversion factor to 1 for a base unit.
func (u *Unit) DeduceDefaultsFromBaseUnit() {
	if u.BaseUnitID == nil {
		u.Level = 1
		u.ConversionFactor = 1
	}
}
