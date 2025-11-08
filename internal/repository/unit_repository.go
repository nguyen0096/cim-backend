package repository

import (
	"cim-backend/internal/models"
	"context"
	"fmt"

	"gorm.io/gorm"
)

//go:generate mockery --name=UnitRepository --structname=UnitRepository --output=../mocks/repositories --outpkg=repositorymocks
type UnitRepository interface {
	Create(ctx context.Context, unit *models.Unit) error
	GetByID(ctx context.Context, id uint) (*models.Unit, error)
	GetByTypeAndName(ctx context.Context, unitType, name string) (*models.Unit, error)
	Update(ctx context.Context, unit *models.Unit) error
	Delete(ctx context.Context, id uint) error
	Restore(ctx context.Context, id uint) error
	List(ctx context.Context, limit, offset int, sortBy, sortOrder, unitType string) ([]models.Unit, error)
	Search(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, unitType string) ([]models.Unit, error)
	Count(ctx context.Context, unitType string) (int64, error)
	CountSearch(ctx context.Context, query, unitType string) (int64, error)
}

type unitRepository struct {
	db *gorm.DB
}

func NewUnitRepository(db *gorm.DB) UnitRepository {
	return &unitRepository{db: db}
}

func (r *unitRepository) Create(ctx context.Context, unit *models.Unit) error {
	return r.db.WithContext(ctx).Create(unit).Error
}

func (r *unitRepository) GetByID(ctx context.Context, id uint) (*models.Unit, error) {
	var unit models.Unit
	err := r.db.WithContext(ctx).First(&unit, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &unit, nil
}

func (r *unitRepository) GetByTypeAndName(ctx context.Context, unitType, name string) (*models.Unit, error) {
	var unit models.Unit
	err := r.db.WithContext(ctx).
		Where("unit_type = ? AND (name = ? OR symbol = ?)", unitType, name, name).
		First(&unit).Error
	if err != nil {
		return nil, err
	}
	return &unit, nil
}

func (r *unitRepository) Update(ctx context.Context, unit *models.Unit) error {
	result := r.db.WithContext(ctx).Save(unit)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("failed to update unit %d: not found", unit.ID)
	}
	return nil
}

func (r *unitRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Unit{}, id).Error
}

func (r *unitRepository) Restore(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Unscoped().
		Model(&models.Unit{}).
		Where("id = ?", id).
		Update("deleted_at", nil).
		Error
}

func (r *unitRepository) List(ctx context.Context, limit, offset int, sortBy, sortOrder, unitType string) ([]models.Unit, error) {
	var units []models.Unit
	query := r.db.WithContext(ctx)

	if unitType != "" {
		query = query.Where("unit_type = ?", unitType)
	}

	if sortBy != "" {
		if sortOrder == "" {
			sortOrder = "asc"
		}
		query = query.Order(sortBy + " " + sortOrder)
	} else {
		query = query.Order("created_at desc")
	}

	err := query.Limit(limit).Offset(offset).Find(&units).Error
	return units, err
}

func (r *unitRepository) Search(ctx context.Context, queryText string, limit, offset int, sortBy, sortOrder, unitType string) ([]models.Unit, error) {
	var units []models.Unit
	query := r.db.WithContext(ctx).
		Where("name ILIKE ? OR symbol ILIKE ?", "%"+queryText+"%", "%"+queryText+"%")

	if unitType != "" {
		query = query.Where("unit_type = ?", unitType)
	}

	if sortBy != "" {
		if sortOrder == "" {
			sortOrder = "asc"
		}
		query = query.Order(sortBy + " " + sortOrder)
	} else {
		query = query.Order("created_at desc")
	}

	err := query.Limit(limit).Offset(offset).Find(&units).Error
	return units, err
}

func (r *unitRepository) Count(ctx context.Context, unitType string) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&models.Unit{})
	if unitType != "" {
		query = query.Where("unit_type = ?", unitType)
	}
	err := query.Count(&count).Error
	return count, err
}

func (r *unitRepository) CountSearch(ctx context.Context, queryText, unitType string) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&models.Unit{}).
		Where("name ILIKE ? OR symbol ILIKE ?", "%"+queryText+"%", "%"+queryText+"%")
	if unitType != "" {
		query = query.Where("unit_type = ?", unitType)
	}
	err := query.Count(&count).Error
	return count, err
}

