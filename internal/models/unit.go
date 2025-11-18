package models

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

// GetRootBaseUnitID returns the ID of the root base unit in the hierarchy
// If this unit is a root base unit (Level 1), it returns its own ID
func (u *Unit) GetRootBaseUnitID() uint {
	if u.IsRootBaseUnit() {
		return u.ID
	}
	if u.BaseUnit != nil {
		return u.BaseUnit.GetRootBaseUnitID()
	}
	// If BaseUnit is not loaded, we can't traverse, return current ID
	// This should not happen if the unit hierarchy is properly loaded
	return u.ID
}

// GetTotalConversionFactorToRoot calculates the total conversion factor from this unit to the root base unit
// This multiplies all conversion factors along the hierarchy chain
func (u *Unit) GetTotalConversionFactorToRoot() float64 {
	if u.IsRootBaseUnit() {
		return 1.0
	}
	if u.BaseUnit != nil {
		return u.ConversionFactor * u.BaseUnit.GetTotalConversionFactorToRoot()
	}
	// If BaseUnit is not loaded, return current conversion factor
	// This should not happen if the unit hierarchy is properly loaded
	return u.ConversionFactor
}

// GetExpectedLevel calculates the expected level based on the base unit's level
// Returns 1 if BaseUnitID is nil, otherwise returns base unit's level + 1
func (u *Unit) GetExpectedLevel() int {
	if u.BaseUnitID == nil {
		return 1
	}
	if u.BaseUnit != nil {
		return u.BaseUnit.Level + 1
	}
	// If BaseUnit is not loaded, we can't determine expected level
	// This should not happen if the unit hierarchy is properly loaded
	return u.Level
}

// CalculateConversionFactorToCurrent calculates the conversion factor from this unit to the target unit
// Returns the factor needed to convert from this unit to the target unit
// For example, if this unit is 102 and target is 101, it returns 3 (3 of 102 = 1 of 101)
// This traverses up the BaseUnit chain from this unit to the target unit
func (u *Unit) CalculateConversionFactorToCurrent(targetUnitID uint) float64 {
	if u.ID == targetUnitID {
		return 1.0
	}

	// Traverse up the hierarchy from this unit to the target unit
	// Multiply conversion factors along the path
	currentUnit := u
	visited := make(map[uint]bool)
	factor := 1.0

	for currentUnit != nil && currentUnit.ID != targetUnitID {
		// Prevent infinite loops
		if visited[currentUnit.ID] {
			return 0 // Circular reference
		}
		visited[currentUnit.ID] = true

		// If this unit has no base unit, we can't reach the target
		if currentUnit.BaseUnitID == nil {
			return 0
		}

		// Multiply by the conversion factor to get to the base unit
		factor *= currentUnit.ConversionFactor

		// Move to the base unit
		if currentUnit.BaseUnit == nil {
			return 0 // Base unit not loaded
		}
		currentUnit = currentUnit.BaseUnit
	}

	if currentUnit != nil && currentUnit.ID == targetUnitID {
		return factor
	}

	// If we couldn't find a path, return 0 to indicate error
	return 0
}
