package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/pkg"
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

//go:generate mockery --name=UnitService --structname=UnitService --output=./servicemocks --outpkg=servicemocks
type UnitService interface {
	CreateUnit(ctx context.Context, unit *models.Unit) error
	GetUnitByID(ctx context.Context, id uint) (*models.Unit, error)
	UpdateUnit(ctx context.Context, unit *models.Unit) error
	DeleteUnit(ctx context.Context, id uint) error
	ListUnits(ctx context.Context, limit, offset int, sortBy, sortOrder, unitType string) ([]models.Unit, error)
	SearchUnits(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, unitType string) ([]models.Unit, error)
	CountUnits(ctx context.Context, unitType string) (int64, error)
	CountSearchUnits(ctx context.Context, query string, unitType string) (int64, error)
}

type unitService struct {
	unitRepo repository.UnitRepository
}

func NewUnitService(unitRepo repository.UnitRepository) UnitService {
	return &unitService{
		unitRepo: unitRepo,
	}
}

func (s *unitService) CreateUnit(ctx context.Context, unit *models.Unit) error {
	if err := s.ensureBaseUnitRelationship(ctx, unit); err != nil {
		return err
	}

	if unit.BaseUnitID == nil {
		unit.ConversionFactor = 1
	}

	existing, err := s.unitRepo.GetByTypeAndName(ctx, unit.UnitType, unit.Name)
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check existing unit: %w", err)
	}
	if existing != nil && err == nil {
		return pkg.ErrDuplicate(fmt.Sprintf("unit '%s' already exists for type '%s'", unit.Name, unit.UnitType), nil)
	}

	if err := s.unitRepo.Create(ctx, unit); err != nil {
		return fmt.Errorf("failed to create unit: %w", err)
	}
	return nil
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
		return pkg.ErrValidation("unit ID is required", nil)
	}
	if err := s.unitRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete unit %d: %w", id, err)
	}
	return nil
}

func (s *unitService) ListUnits(ctx context.Context, limit, offset int, sortBy, sortOrder, unitType string) ([]models.Unit, error) {
	units, err := s.unitRepo.List(ctx, limit, offset, sortBy, sortOrder, unitType)
	if err != nil {
		return nil, fmt.Errorf("failed to list units: %w", err)
	}
	return units, nil
}

func (s *unitService) SearchUnits(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, unitType string) ([]models.Unit, error) {
	units, err := s.unitRepo.Search(ctx, query, limit, offset, sortBy, sortOrder, unitType)
	if err != nil {
		return nil, fmt.Errorf("failed to search units: %w", err)
	}
	return units, nil
}

func (s *unitService) CountUnits(ctx context.Context, unitType string) (int64, error) {
	count, err := s.unitRepo.Count(ctx, unitType)
	if err != nil {
		return 0, fmt.Errorf("failed to count units: %w", err)
	}
	return count, nil
}

func (s *unitService) CountSearchUnits(ctx context.Context, query string, unitType string) (int64, error) {
	count, err := s.unitRepo.CountSearch(ctx, query, unitType)
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
		return nil
	}

	if unit.ID != 0 && unit.BaseUnitID != nil && *unit.BaseUnitID == unit.ID {
		return pkg.ErrValidation("base_unit_id cannot reference the same unit", nil)
	}

	baseUnit, err := s.unitRepo.GetByID(ctx, *unit.BaseUnitID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return pkg.ErrValidation("base_unit_id must reference an existing base unit", err)
		}
		return fmt.Errorf("failed to load base unit %d: %w", *unit.BaseUnitID, err)
	}

	if baseUnit.BaseUnitID != nil {
		return pkg.ErrValidation("base_unit_id must reference a base unit", nil)
	}

	if !strings.EqualFold(baseUnit.UnitType, unit.UnitType) {
		return pkg.ErrValidation("base unit must have the same unit_type", nil)
	}

	return nil
}
