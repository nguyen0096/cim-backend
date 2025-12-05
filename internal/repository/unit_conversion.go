package repository

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"context"

	"github.com/shopspring/decimal"
)

func (r *unitRepository) GetConversionsFrom(ctx context.Context, unitID uint) ([]models.UnitConversion, error) {
	var conversions []models.UnitConversion
	err := r.db.WithContext(ctx).Where("from_unit_id = ?", unitID).Find(&conversions).Error
	return conversions, err
}

func (r *unitRepository) CreateConversion(ctx context.Context, conversion *models.UnitConversion) error {
	// flip the from and to unit IDs in order to keep
	// the conversion factor positive.
	if conversion.ConversionFactor.LessThan(decimal.NewFromInt(1)) {
		conversion.FromUnitID, conversion.ToUnitID = conversion.ToUnitID, conversion.FromUnitID
		conversion.ConversionFactor = decimal.NewFromInt(1).Div(conversion.ConversionFactor)
	}
	err := r.db.WithContext(ctx).Create(conversion).Error
	if err != nil {
		if isDuplicateError(err, nil) {
			return pkg.ErrUnitConversionAlreadyExists(ctx, conversion.FromUnitID, conversion.ToUnitID)
		}

		return err
	}
	return nil
}
