package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/pkg"
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const (
	// MaxUnitHierarchyDepth is the maximum allowed depth for unit hierarchy
	// Level 1: Root base unit, Level 2-4: Derived units
	MaxUnitHierarchyDepth = 4
)

//go:generate mockery --name=UnitService --structname=UnitService --output=../mocks/servicemocks --outpkg=servicemocks
type UnitService interface {
	CreateUnit(ctx context.Context, unit *models.Unit) (*models.Unit, error)
	GetUnitByID(ctx context.Context, id uint) (*models.Unit, error)
	GetMapByNames(ctx context.Context, names []string) (map[string]*models.Unit, error)
	UpdateUnit(ctx context.Context, unit *models.Unit) error
	DeleteUnit(ctx context.Context, id uint) error
	ListUnits(ctx context.Context, limit, offset int, sortBy, sortOrder, unitType string, baseOnly bool) ([]models.Unit, error)
	SearchUnits(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, unitType string, baseOnly bool) ([]models.Unit, error)
	CountUnits(ctx context.Context, unitType string, baseOnly bool) (int64, error)
	CountSearchUnits(ctx context.Context, query string, unitType string, baseOnly bool) (int64, error)
}

type unitService struct {
	unitRepo    repository.UnitRepository
	productRepo repository.ProductRepository
}

func NewUnitService(unitRepo repository.UnitRepository, productRepo repository.ProductRepository) UnitService {
	return &unitService{
		unitRepo:    unitRepo,
		productRepo: productRepo,
	}
}

// CreateUnit creates a new unit or returns existing unit if one with the same name and type already exists
// Unit names are automatically uppercased for consistency
func (s *unitService) CreateUnit(ctx context.Context, unit *models.Unit) (*models.Unit, error) {
	// Validate base unit relationship for new units
	if err := s.ensureBaseUnitRelationship(ctx, unit); err != nil {
		return nil, err
	}

	// Uppercase and trim the unit name for consistency
	unit.Name = strings.ToUpper(strings.TrimSpace(unit.Name))

	// Check if unit already exists (get-or-create pattern)
	existing, err := s.unitRepo.GetByTypeAndName(ctx, unit.UnitType, unit.Name)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check existing unit: %w", err)
	}
	if err == nil && existing != nil {
		// Unit already exists, return it
		// (this supports get-or-create behavior)
		return existing, pkg.ErrUnitAlreadyExists(ctx, unit.Name, unit.UnitType)
	}

	// Set defaults for base units
	if unit.BaseUnitID == nil {
		unit.Level = 1
		unit.ConversionFactor = 1
	}

	// Create the unit
	if err := s.unitRepo.Create(ctx, unit); err != nil {
		return nil, fmt.Errorf("failed to create unit: %w", err)
	}

	return unit, nil
}

func (s *unitService) GetUnitByID(ctx context.Context, id uint) (*models.Unit, error) {
	unit, err := s.unitRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkg.ErrNotFound(fmt.Sprintf("unit %d not found", id), err)
		}
		return nil, fmt.Errorf("failed to get unit %d: %w", id, err)
	}
	return unit, nil
}

func (s *unitService) UpdateUnit(ctx context.Context, unit *models.Unit) error {
	if unit.ID == 0 {
		return pkg.ErrValidation("unit ID is required", nil)
	}

	existing, err := s.unitRepo.GetByID(ctx, unit.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return pkg.ErrNotFound(fmt.Sprintf("unit %d not found", unit.ID), err)
		}
		return fmt.Errorf("failed to load unit %d: %w", unit.ID, err)
	}

	if err := s.ensureBaseUnitRelationship(ctx, unit); err != nil {
		return err
	}

	if unit.BaseUnitID == nil {
		unit.ConversionFactor = 1
	}

	if !strings.EqualFold(existing.Name, unit.Name) || !strings.EqualFold(existing.UnitType, unit.UnitType) {
		conflict, err := s.unitRepo.GetByTypeAndName(ctx, unit.UnitType, unit.Name)
		if err != nil && err != gorm.ErrRecordNotFound {
			return fmt.Errorf("failed to check unit uniqueness: %w", err)
		}
		if conflict != nil && err == nil && conflict.ID != unit.ID {
			return pkg.ErrDuplicate(fmt.Sprintf("unit '%s' already exists for type '%s'", unit.Name, unit.UnitType), nil)
		}
	}

	if err := s.unitRepo.Update(ctx, unit); err != nil {
		return fmt.Errorf("failed to update unit %d: %w", unit.ID, err)
	}
	return nil
}

func (s *unitService) DeleteUnit(ctx context.Context, id uint) error {
	if id == 0 {
		return pkg.ErrUnitIDRequired(ctx)
	}

	// Check if any products reference this unit
	exists, err := s.productRepo.AnyProductUsingUnitID(ctx, id)
	if err != nil {
		return pkg.ErrFailedToCheckProductReferences(ctx, id, err)
	}

	if exists {
		return pkg.ErrCannotDeleteUnitProductsReference(ctx)
	}

	if err := s.unitRepo.Delete(ctx, id); err != nil {
		return pkg.ErrFailedToDeleteUnit(ctx, id, err)
	}
	return nil
}

func (s *unitService) ListUnits(ctx context.Context, limit, offset int, sortBy, sortOrder, unitType string, baseOnly bool) ([]models.Unit, error) {
	units, err := s.unitRepo.List(ctx, limit, offset, sortBy, sortOrder, unitType, baseOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to list units: %w", err)
	}
	return units, nil
}

func (s *unitService) SearchUnits(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, unitType string, baseOnly bool) ([]models.Unit, error) {
	units, err := s.unitRepo.Search(ctx, query, limit, offset, sortBy, sortOrder, unitType, baseOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to search units: %w", err)
	}
	return units, nil
}

func (s *unitService) CountUnits(ctx context.Context, unitType string, baseOnly bool) (int64, error) {
	count, err := s.unitRepo.Count(ctx, unitType, baseOnly)
	if err != nil {
		return 0, fmt.Errorf("failed to count units: %w", err)
	}
	return count, nil
}

func (s *unitService) CountSearchUnits(ctx context.Context, query string, unitType string, baseOnly bool) (int64, error) {
	count, err := s.unitRepo.CountSearch(ctx, query, unitType, baseOnly)
	if err != nil {
		return 0, fmt.Errorf("failed to count units search result: %w", err)
	}
	return count, nil
}

func (s *unitService) ensureBaseUnitRelationship(ctx context.Context, unit *models.Unit) error {
	if unit.BaseUnitID == nil {
		if unit.ConversionFactor != 1 {
			return pkg.ErrValidation("conversion_factor must be 1 for base units", nil)
		}
		unit.Level = 1
		return nil
	}

	if unit.ID != 0 && unit.BaseUnitID != nil && *unit.BaseUnitID == unit.ID {
		return pkg.ErrValidation("base_unit_id cannot reference the same unit", nil)
	}

	baseUnit, err := s.unitRepo.GetByID(ctx, *unit.BaseUnitID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return pkg.ErrValidation("base_unit_id must reference an existing unit", err)
		}
		return fmt.Errorf("failed to load base unit %d: %w", *unit.BaseUnitID, err)
	}

	if !strings.EqualFold(baseUnit.UnitType, unit.UnitType) {
		return pkg.ErrValidation("base unit must have the same unit_type", nil)
	}

	// Check for circular references by traversing up the hierarchy
	if err := s.checkCircularReference(ctx, unit.ID, *unit.BaseUnitID); err != nil {
		return err
	}

	// Calculate and validate the level based on base unit's level
	expectedLevel := baseUnit.Level + 1
	if expectedLevel > MaxUnitHierarchyDepth {
		return pkg.ErrValidation(fmt.Sprintf("cannot create/update unit: base unit is at level %d, which would result in level %d. Maximum allowed hierarchy depth is %d levels", baseUnit.Level, expectedLevel, MaxUnitHierarchyDepth), nil)
	}

	// Set the level for the unit
	unit.Level = expectedLevel

	return nil
}

// checkCircularReference checks if setting baseUnitID would create a circular reference
func (s *unitService) checkCircularReference(ctx context.Context, unitID uint, baseUnitID uint) error {
	visited := make(map[uint]bool)
	currentID := baseUnitID

	for currentID != 0 {
		// If we've seen this unit before, we have a cycle
		if visited[currentID] {
			return pkg.ErrValidation("circular reference detected in unit hierarchy", nil)
		}

		// If the current unit is the one we're trying to set as base, we have a cycle
		if currentID == unitID {
			return pkg.ErrValidation("circular reference detected: unit would reference itself through base unit chain", nil)
		}

		visited[currentID] = true

		// Get the current unit and check its base unit
		currentUnit, err := s.unitRepo.GetByID(ctx, currentID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// If unit not found, we've reached the end of the chain
				break
			}
			return fmt.Errorf("failed to check circular reference: %w", err)
		}

		if currentUnit.BaseUnitID == nil {
			// Reached a root base unit, no cycle
			break
		}

		currentID = *currentUnit.BaseUnitID
	}

	return nil
}

// GetUnitMapByNames returns a map of units by their names (matches both name and symbol)
func (s *unitService) GetMapByNames(ctx context.Context, names []string) (map[string]*models.Unit, error) {
	units, err := s.unitRepo.GetByNames(ctx, names)
	if err != nil {
		return nil, fmt.Errorf("failed to get unit map by names: %w", err)
	}

	// Build map by both name and symbol for O(1) lookup
	result := make(map[string]*models.Unit)
	for _, u := range units {
		result[u.Name] = &u
		if u.Symbol != "" {
			result[u.Symbol] = &u
		}
	}
	return result, nil
}
