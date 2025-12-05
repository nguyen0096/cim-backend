package repository

import (
	"cim-backend/internal/models"
	"context"

	"gorm.io/gorm"
)

//go:generate mockery --name=UnitConversionRepository --structname=UnitConversionRepository --output=../mocks/repositorymocks --outpkg=repositorymocks
type UnitConversionRepository interface {
	GetConversionsFrom(ctx context.Context, unitID uint) ([]models.UnitConversion, error)
}

type unitConversionRepository struct {
	db *gorm.DB
}

func NewUnitConversionRepository(db *gorm.DB) UnitConversionRepository {
	return &unitConversionRepository{db: db}
}

func (r *unitConversionRepository) GetConversionsFrom(ctx context.Context, unitID uint) ([]models.UnitConversion, error) {
	var conversions []models.UnitConversion
	err := r.db.WithContext(ctx).Where("from_unit_id = ?", unitID).Find(&conversions).Error
	return conversions, err
}
