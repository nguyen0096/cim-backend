package repository

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"context"
	"fmt"
	"slices"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

//go:generate mockery --name=UnitRepository --structname=UnitRepository --output=../mocks/repositorymocks --outpkg=repositorymocks
type UnitRepository interface {
	GetByID(ctx context.Context, id uint) (*models.Unit, error)
	GetByTypeAndName(ctx context.Context, unitType, name string) (*models.Unit, error)
	GetByNames(ctx context.Context, names []string) ([]models.Unit, error)
	Delete(ctx context.Context, id uint) error
	Restore(ctx context.Context, id uint) error
	List(ctx context.Context, limit, offset int, sortBy, sortOrder, unitType string, baseOnly bool) ([]models.Unit, error)
	Search(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, unitType string, baseOnly bool) ([]models.Unit, error)
	Count(ctx context.Context, unitType string, baseOnly bool) (int64, error)
	CountSearch(ctx context.Context, query, unitType string, baseOnly bool) (int64, error)

	// v1 - AGENTS MUST CONFIRM BEFORE MODIFYING SECTION BELOW THIS LINE

	// Unit
	Create(ctx context.Context, unit *models.Unit) error
	Update(ctx context.Context, unit *models.Unit) error

	// UnitConversion

	GetConversionsFrom(ctx context.Context, unitID uint) ([]models.UnitConversion, error)
	CreateConversion(ctx context.Context, conversion *models.UnitConversion) error
}

type unitRepository struct {
	db *gorm.DB
}

func NewUnitRepository(db *gorm.DB) UnitRepository {
	return &unitRepository{db: db}
}

func (r *unitRepository) Create(ctx context.Context, unit *models.Unit) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(unit).Error; err != nil {
			if isDuplicateError(err, nil) {
				return pkg.ErrUnitAlreadyExists(ctx, unit.Name, unit.UnitType)
			}
			return err
		}

		// Only create unit conversion if BaseUnitID is not nil (not a root base unit)
		if unit.BaseUnitID != nil {
			if err := r.txnCreateConversion(tx, &models.UnitConversion{
				FromUnitID:       unit.ID,
				ToUnitID:         *unit.BaseUnitID,
				ConversionFactor: decimal.NewFromFloat(unit.ConversionFactor),
			}); err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *unitRepository) GetByID(ctx context.Context, id uint) (*models.Unit, error) {
	var unit models.Unit
	err := r.db.WithContext(ctx).
		Preload("BaseUnit").
		Preload("DerivedUnits").
		First(&unit, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	// Load all units in the hierarchy and cache them to avoid repeated queries
	unitCache := make(map[uint]*models.Unit)
	unitCache[unit.ID] = &unit
	r.loadAllHierarchyUnits(ctx, &unit, unitCache)

	// Get the root base unit ID to populate DerivedUnits with all units in the hierarchy
	rootBaseUnitID := unit.GetRootBaseUnitID()
	allDerivedUnits, err := r.findAllUnitsWithRootBaseUnit(ctx, rootBaseUnitID, unit.UnitType, unitCache)
	if err != nil {
		return nil, fmt.Errorf("failed to load all derived units: %w", err)
	}

	// Filter to only include units that are descendants of the current unit
	derivedUnits := r.filterDescendantUnitsWithCache(allDerivedUnits, id, unitCache)

	// Sort derived units by ID
	slices.SortFunc(derivedUnits, func(a, b *models.Unit) int {
		return int(a.ID) - int(b.ID)
	})

	// Set DerivedUnits on the original unit
	unit.DerivedUnits = derivedUnits

	// Set conversion_factor_to_current for the current unit (always 1, since converting to itself)
	unit.ConversionFactorToCurrent = floatPtr(1.0)

	// Calculate conversion_factor_to_current for derived units
	r.calculateDerivedUnitsConversionFactors(&unit, id)

	// Calculate conversion factor from base unit chain to current unit (recursively)
	r.calculateBaseUnitConversionFactors(&unit, &unit)

	// Break cycles for JSON serialization by clearing BaseUnit on derived units
	// This prevents "encountered a cycle" errors during JSON marshaling
	for _, derivedUnit := range unit.DerivedUnits {
		derivedUnit.BaseUnit = nil
	}

	return &unit, nil
}

// floatPtr creates a pointer to a float64 value
func floatPtr(v float64) *float64 {
	return &v
}

// calculateDerivedUnitsConversionFactors calculates conversion_factor_to_current for all derived units
func (r *unitRepository) calculateDerivedUnitsConversionFactors(unit *models.Unit, currentUnitID uint) {
	if len(unit.DerivedUnits) == 0 {
		return
	}

	// Calculate conversion factor for each derived unit individually
	// Each derived unit's conversion factor is calculated based on its path to the current unit
	// CalculateConversionFactorToCurrent returns the factor FROM derived unit TO current unit
	// conversion_factor_to_current should be the inverse (1/factor) to represent "1 of current = X of derived"
	for _, derivedUnit := range unit.DerivedUnits {
		if factor := derivedUnit.CalculateConversionFactorToCurrent(currentUnitID); factor > 0 {
			inverseFactor := 1.0 / factor
			derivedUnit.ConversionFactorToCurrent = &inverseFactor
		}
	}
}

// calculateBaseUnitConversionFactors recursively calculates conversion_factor_to_current for all base units
func (r *unitRepository) calculateBaseUnitConversionFactors(unit *models.Unit, originalCurrentUnit *models.Unit) {
	if unit.BaseUnit == nil {
		return
	}

	var factor float64
	if originalCurrentUnit.BaseUnitID != nil && *originalCurrentUnit.BaseUnitID == unit.BaseUnit.ID {
		// Direct base unit: use current unit's conversion factor
		factor = float64(originalCurrentUnit.ConversionFactor)
	} else {
		// Nested base unit: multiply conversion factors along the path
		factor = 1.0
		for current := originalCurrentUnit; current != nil && current.ID != unit.BaseUnit.ID; current = current.BaseUnit {
			factor *= float64(current.ConversionFactor)
			if current.BaseUnitID == nil || current.BaseUnit == nil {
				break
			}
		}
	}
	unit.BaseUnit.ConversionFactorToCurrent = &factor

	// Recursively calculate for nested base units
	r.calculateBaseUnitConversionFactors(unit.BaseUnit, originalCurrentUnit)
}

// loadAllHierarchyUnits loads all units in the hierarchy into the cache
func (r *unitRepository) loadAllHierarchyUnits(ctx context.Context, unit *models.Unit, cache map[uint]*models.Unit) {
	// Load base unit chain
	if unit.BaseUnitID != nil {
		if _, exists := cache[*unit.BaseUnitID]; !exists {
			var baseUnit models.Unit
			if err := r.db.WithContext(ctx).First(&baseUnit, "id = ?", *unit.BaseUnitID).Error; err == nil {
				cache[baseUnit.ID] = &baseUnit
				unit.BaseUnit = &baseUnit
				r.loadAllHierarchyUnits(ctx, &baseUnit, cache)
			}
		} else {
			unit.BaseUnit = cache[*unit.BaseUnitID]
		}
	}
}

// filterDescendantUnitsWithCache filters units using the cache to avoid database queries
func (r *unitRepository) filterDescendantUnitsWithCache(units []*models.Unit, ancestorID uint, cache map[uint]*models.Unit) []*models.Unit {
	filtered := make([]*models.Unit, 0, len(units))
	for _, unit := range units {
		if r.isDescendantOfWithCache(unit.ID, ancestorID, cache) {
			// Ensure base unit chain is loaded from cache
			r.loadBaseUnitChainFromCache(unit, cache)
			filtered = append(filtered, unit)
		}
	}
	return filtered
}

// loadBaseUnitChainFromCache loads base unit chain from cache
func (r *unitRepository) loadBaseUnitChainFromCache(unit *models.Unit, cache map[uint]*models.Unit) {
	if unit.BaseUnitID == nil {
		return
	}
	if baseUnit, exists := cache[*unit.BaseUnitID]; exists {
		unit.BaseUnit = baseUnit
		r.loadBaseUnitChainFromCache(baseUnit, cache)
	}
}

// isDescendantOfWithCache checks if descendantID is a descendant of ancestorID using cache
func (r *unitRepository) isDescendantOfWithCache(descendantID, ancestorID uint, cache map[uint]*models.Unit) bool {
	if descendantID == ancestorID {
		return false
	}

	currentID := descendantID
	visited := make(map[uint]bool)

	for currentID != 0 {
		if visited[currentID] {
			return false
		}
		visited[currentID] = true

		unit, exists := cache[currentID]
		if !exists {
			return false
		}

		if unit.BaseUnitID == nil {
			return false
		}

		if *unit.BaseUnitID == ancestorID {
			return true
		}

		currentID = *unit.BaseUnitID
	}

	return false
}

// findAllUnitsWithRootBaseUnit finds all units that have the same root base unit
// It uses batch loading and cache to minimize database queries
func (r *unitRepository) findAllUnitsWithRootBaseUnit(ctx context.Context, rootBaseUnitID uint, unitType string, cache map[uint]*models.Unit) ([]*models.Unit, error) {
	// Ensure root base unit is in cache
	if _, exists := cache[rootBaseUnitID]; !exists {
		var rootUnit models.Unit
		if err := r.db.WithContext(ctx).First(&rootUnit, "id = ?", rootBaseUnitID).Error; err != nil {
			return nil, fmt.Errorf("failed to load root base unit: %w", err)
		}
		cache[rootBaseUnitID] = &rootUnit
	}

	// Load all units of this type in one query (they might be in different hierarchies, but we'll filter)
	var allUnitsInHierarchy []models.Unit
	if err := r.db.WithContext(ctx).
		Where("unit_type = ?", unitType).
		Find(&allUnitsInHierarchy).Error; err != nil {
		return nil, fmt.Errorf("failed to load units: %w", err)
	}

	// Build a map of all units and add to cache
	unitMap := make(map[uint]*models.Unit)
	for i := range allUnitsInHierarchy {
		unitMap[allUnitsInHierarchy[i].ID] = &allUnitsInHierarchy[i]
		if _, exists := cache[allUnitsInHierarchy[i].ID]; !exists {
			cache[allUnitsInHierarchy[i].ID] = &allUnitsInHierarchy[i]
		}
	}

	// Build relationships using the map and cache
	// Clear DerivedUnits to avoid cycles during relationship building
	for _, unit := range unitMap {
		unit.DerivedUnits = nil
		if unit.BaseUnitID != nil {
			if baseUnit, exists := unitMap[*unit.BaseUnitID]; exists {
				unit.BaseUnit = baseUnit
			} else if cachedBaseUnit, exists := cache[*unit.BaseUnitID]; exists {
				unit.BaseUnit = cachedBaseUnit
			}
		}
	}

	// Collect all units that are descendants of the root base unit
	var allUnits []*models.Unit
	visited := make(map[uint]bool)

	var collectUnits func(unitID uint)
	collectUnits = func(unitID uint) {
		if visited[unitID] {
			return
		}
		visited[unitID] = true

		// Find all units that have this unit as their base unit
		for _, unit := range unitMap {
			if unit.BaseUnitID != nil && *unit.BaseUnitID == unitID {
				allUnits = append(allUnits, unit)
				collectUnits(unit.ID)
			}
		}
	}

	collectUnits(rootBaseUnitID)
	return allUnits, nil
}

func (r *unitRepository) GetByTypeAndName(ctx context.Context, unitType, name string) (*models.Unit, error) {
	var unit models.Unit
	err := r.db.WithContext(ctx).
		Preload("BaseUnit").
		Preload("DerivedUnits").
		Where("unit_type = ? AND (name = ? OR symbol = ?)", unitType, name, name).
		First(&unit).Error
	if err != nil {
		return nil, err
	}

	// Load all units in the hierarchy and cache them
	unitCache := make(map[uint]*models.Unit)
	unitCache[unit.ID] = &unit
	r.loadAllHierarchyUnits(ctx, &unit, unitCache)

	// Get the root base unit ID to populate DerivedUnits with all units in the hierarchy
	rootBaseUnitID := unit.GetRootBaseUnitID()
	allDerivedUnits, err := r.findAllUnitsWithRootBaseUnit(ctx, rootBaseUnitID, unit.UnitType, unitCache)
	if err != nil {
		return nil, fmt.Errorf("failed to load all derived units: %w", err)
	}

	// Filter to only include units that are descendants of the current unit
	unit.DerivedUnits = r.filterDescendantUnitsWithCache(allDerivedUnits, unit.ID, unitCache)

	// Set conversion_factor_to_current for the current unit (always 1, since converting to itself)
	unit.ConversionFactorToCurrent = floatPtr(1.0)

	// Calculate conversion factors for derived units
	// CalculateConversionFactorToCurrent returns the factor FROM derived unit TO current unit
	// conversion_factor_to_current should be the inverse (1/factor) to represent "1 of current = X of derived"
	for _, derivedUnit := range unit.DerivedUnits {
		if factor := derivedUnit.CalculateConversionFactorToCurrent(unit.ID); factor > 0 {
			inverseFactor := 1.0 / factor
			derivedUnit.ConversionFactorToCurrent = &inverseFactor
		}
	}

	return &unit, nil
}

// GetByNames retrieves multiple units by their names (matches both name and symbol)
func (r *unitRepository) GetByNames(ctx context.Context, names []string) ([]models.Unit, error) {
	if len(names) == 0 {
		return nil, nil
	}
	units := []models.Unit{}
	if err := r.db.WithContext(ctx).
		Where("name IN ? OR symbol IN ?", names, names).
		Find(&units).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch units by names: %w", err)
	}
	return units, nil
}

func (r *unitRepository) Update(ctx context.Context, unit *models.Unit) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Save(unit)
		if result.Error != nil {
			if isDuplicateError(result.Error, nil) {
				return pkg.ErrUnitAlreadyExists(ctx, unit.Name, unit.UnitType)
			}
			return fmt.Errorf("failed to update unit: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			return fmt.Errorf("failed to update unit %d: not found", unit.ID)
		}

		if unit.BaseUnitID == nil {
			return nil
		}

		if err := r.txnCreateConversion(tx, &models.UnitConversion{
			FromUnitID:       unit.ID,
			ToUnitID:         *unit.BaseUnitID,
			ConversionFactor: decimal.NewFromFloat(unit.ConversionFactor),
		}); err != nil {
			return err
		}

		return nil
	})
}

func (r *unitRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&models.Unit{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete unit %d: %w", id, err)
	}
	return nil
}

func (r *unitRepository) Restore(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).
		Unscoped().
		Model(&models.Unit{}).
		Where("id = ?", id).
		Update("deleted_at", nil).
		Error; err != nil {
		return fmt.Errorf("failed to restore unit %d: %w", id, err)
	}
	return nil
}

func (r *unitRepository) List(ctx context.Context, limit, offset int, sortBy, sortOrder, unitType string, baseOnly bool) ([]models.Unit, error) {
	return r.queryUnits(ctx, limit, offset, sortBy, sortOrder, unitType, baseOnly, "")
}

func (r *unitRepository) Search(ctx context.Context, queryText string, limit, offset int, sortBy, sortOrder, unitType string, baseOnly bool) ([]models.Unit, error) {
	return r.queryUnits(ctx, limit, offset, sortBy, sortOrder, unitType, baseOnly, queryText)
}

// queryUnits builds and executes a query for units with optional search text
func (r *unitRepository) queryUnits(ctx context.Context, limit, offset int, sortBy, sortOrder, unitType string, baseOnly bool, searchText string) ([]models.Unit, error) {
	query := r.db.WithContext(ctx).
		Preload("BaseUnit").
		Preload("DerivedUnits")

	if searchText != "" {
		query = query.Where("name ILIKE ? OR symbol ILIKE ?", "%"+searchText+"%", "%"+searchText+"%")
	}
	if unitType != "" {
		query = query.Where("unit_type = ?", unitType)
	}
	if baseOnly {
		query = query.Where("base_unit_id IS NULL")
	}

	// Set default sort order
	if sortBy == "" {
		sortBy = "created_at"
		sortOrder = "desc"
	} else if sortOrder == "" {
		sortOrder = "asc"
	}
	query = query.Order(sortBy + " " + sortOrder)

	var units []models.Unit
	if err := query.Limit(limit).Offset(offset).Find(&units).Error; err != nil {
		return nil, fmt.Errorf("failed to query units: %w", err)
	}
	return units, nil
}

func (r *unitRepository) Count(ctx context.Context, unitType string, baseOnly bool) (int64, error) {
	return r.countUnits(ctx, unitType, baseOnly, "")
}

func (r *unitRepository) CountSearch(ctx context.Context, queryText, unitType string, baseOnly bool) (int64, error) {
	return r.countUnits(ctx, unitType, baseOnly, queryText)
}

// countUnits counts units matching the given filters
func (r *unitRepository) countUnits(ctx context.Context, unitType string, baseOnly bool, searchText string) (int64, error) {
	query := r.db.WithContext(ctx).Model(&models.Unit{})
	if searchText != "" {
		query = query.Where("name ILIKE ? OR symbol ILIKE ?", "%"+searchText+"%", "%"+searchText+"%")
	}
	if unitType != "" {
		query = query.Where("unit_type = ?", unitType)
	}
	if baseOnly {
		query = query.Where("base_unit_id IS NULL")
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count units: %w", err)
	}
	return count, nil
}
