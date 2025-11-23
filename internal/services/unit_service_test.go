package services

import (
	repositorymocks "cim-backend/internal/mocks/repositories"
	"cim-backend/internal/models"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestCreateUnit(t *testing.T) {
	ctx := context.Background()

	t.Run("should create unit successfully", func(t *testing.T) {
		unitRepo := repositorymocks.NewUnitRepository(t)
		productRepo := repositorymocks.NewProductRepository(t)
		service := NewUnitService(unitRepo, productRepo)

		unit := &models.Unit{
			UnitType:         "mass",
			Name:             "kilogram",
			Symbol:           "kg",
			ConversionFactor: 1,
		}

		unitRepo.On("GetByTypeAndName", ctx, "mass", "kilogram").Return((*models.Unit)(nil), gorm.ErrRecordNotFound)
		unitRepo.On("Create", ctx, unit).Return(nil)

		err := service.CreateUnit(ctx, unit)
		assert.NoError(t, err)
		unitRepo.AssertExpectations(t)
	})

	t.Run("should create derived unit when base unit is valid", func(t *testing.T) {
		unitRepo := repositorymocks.NewUnitRepository(t)
		productRepo := repositorymocks.NewProductRepository(t)
		service := NewUnitService(unitRepo, productRepo)

		baseUnitID := uint(2)
		unit := &models.Unit{
			UnitType:         "mass",
			Name:             "gram",
			Symbol:           "g",
			BaseUnitID:       &baseUnitID,
			ConversionFactor: 0.001,
		}

		unitRepo.On("GetByTypeAndName", ctx, "mass", "gram").Return((*models.Unit)(nil), gorm.ErrRecordNotFound)
		unitRepo.On("GetByID", ctx, baseUnitID).Return(&models.Unit{
			Base:       models.Base{ID: baseUnitID},
			UnitType:   "mass",
			Name:       "kilogram",
			BaseUnitID: nil,
		}, nil)
		unitRepo.On("Create", ctx, unit).Return(nil)

		err := service.CreateUnit(ctx, unit)
		assert.NoError(t, err)
		unitRepo.AssertExpectations(t)
	})

	t.Run("should return validation error when derived unit missing base reference", func(t *testing.T) {
		unitRepo := repositorymocks.NewUnitRepository(t)
		productRepo := repositorymocks.NewProductRepository(t)
		service := NewUnitService(unitRepo, productRepo)

		err := service.CreateUnit(ctx, &models.Unit{
			UnitType:         "mass",
			Name:             "gram",
			Symbol:           "g",
			ConversionFactor: 0.001,
		})
		assert.Error(t, err)
	})

	t.Run("should return validation error when base unit reference invalid", func(t *testing.T) {
		unitRepo := repositorymocks.NewUnitRepository(t)
		productRepo := repositorymocks.NewProductRepository(t)
		service := NewUnitService(unitRepo, productRepo)

		baseUnitID := uint(2)
		unit := &models.Unit{
			UnitType:         "mass",
			Name:             "gram",
			Symbol:           "g",
			BaseUnitID:       &baseUnitID,
			ConversionFactor: 0.001,
		}

		unitRepo.On("GetByID", ctx, baseUnitID).Return(&models.Unit{
			Base:       models.Base{ID: baseUnitID},
			UnitType:   "mass",
			Name:       "kilogram",
			BaseUnitID: &baseUnitID,
		}, nil)

		err := service.CreateUnit(ctx, unit)
		assert.Error(t, err)
		unitRepo.AssertExpectations(t)
	})

	t.Run("should return validation error when required fields missing", func(t *testing.T) {
		unitRepo := repositorymocks.NewUnitRepository(t)
		productRepo := repositorymocks.NewProductRepository(t)
		service := NewUnitService(unitRepo, productRepo)

		err := service.CreateUnit(ctx, &models.Unit{
			UnitType:         "",
			Name:             "",
			Symbol:           "",
			ConversionFactor: 0,
		})
		assert.Error(t, err)
	})

	t.Run("should return duplicate error when unit exists", func(t *testing.T) {
		unitRepo := repositorymocks.NewUnitRepository(t)
		productRepo := repositorymocks.NewProductRepository(t)
		service := NewUnitService(unitRepo, productRepo)

		existing := &models.Unit{
			Base:     models.Base{ID: 1},
			UnitType: "mass",
			Name:     "kilogram",
			Symbol:   "kg",
		}
		unit := &models.Unit{
			UnitType:         "mass",
			Name:             "kilogram",
			Symbol:           "kg",
			ConversionFactor: 1,
		}

		unitRepo.On("GetByTypeAndName", ctx, "mass", "kilogram").Return(existing, nil)

		err := service.CreateUnit(ctx, unit)
		assert.Error(t, err)
		unitRepo.AssertExpectations(t)
	})

	t.Run("should return error when repository create fails", func(t *testing.T) {
		unitRepo := repositorymocks.NewUnitRepository(t)
		productRepo := repositorymocks.NewProductRepository(t)
		service := NewUnitService(unitRepo, productRepo)

		unit := &models.Unit{
			UnitType:         "mass",
			Name:             "kilogram",
			Symbol:           "kg",
			ConversionFactor: 1,
		}
		unitRepo.On("GetByTypeAndName", ctx, "mass", "kilogram").Return((*models.Unit)(nil), gorm.ErrRecordNotFound)
		unitRepo.On("Create", ctx, unit).Return(errors.New("db error"))

		err := service.CreateUnit(ctx, unit)
		assert.Error(t, err)
		unitRepo.AssertExpectations(t)
	})
}

func TestUpdateUnit(t *testing.T) {
	ctx := context.Background()

	t.Run("should update unit successfully", func(t *testing.T) {
		unitRepo := repositorymocks.NewUnitRepository(t)
		productRepo := repositorymocks.NewProductRepository(t)
		service := NewUnitService(unitRepo, productRepo)

		unit := &models.Unit{
			Base:             models.Base{ID: 1},
			UnitType:         "volume",
			Name:             "liter",
			Symbol:           "L",
			ConversionFactor: 1,
		}

		unitRepo.On("GetByID", ctx, uint(1)).Return(&models.Unit{
			Base:       models.Base{ID: 1},
			UnitType:   "volume",
			Name:       "liter",
			Symbol:     "L",
			BaseUnitID: nil,
		}, nil)
		unitRepo.On("Update", ctx, unit).Return(nil)

		err := service.UpdateUnit(ctx, unit)
		assert.NoError(t, err)
		unitRepo.AssertExpectations(t)
	})

	t.Run("should error when unit not found", func(t *testing.T) {
		unitRepo := repositorymocks.NewUnitRepository(t)
		productRepo := repositorymocks.NewProductRepository(t)
		service := NewUnitService(unitRepo, productRepo)

		unit := &models.Unit{
			Base:             models.Base{ID: 99},
			UnitType:         "volume",
			Name:             "liter",
			Symbol:           "L",
			ConversionFactor: 1,
		}

		unitRepo.On("GetByID", ctx, uint(99)).Return((*models.Unit)(nil), gorm.ErrRecordNotFound)

		err := service.UpdateUnit(ctx, unit)
		assert.Error(t, err)
		unitRepo.AssertExpectations(t)
	})

	t.Run("should validate derived unit base reference on update", func(t *testing.T) {
		unitRepo := repositorymocks.NewUnitRepository(t)
		productRepo := repositorymocks.NewProductRepository(t)
		service := NewUnitService(unitRepo, productRepo)

		baseUnitID := uint(3)
		unit := &models.Unit{
			Base:             models.Base{ID: 4},
			UnitType:         "mass",
			Name:             "gram",
			Symbol:           "g",
			BaseUnitID:       &baseUnitID,
			ConversionFactor: 0.001,
		}

		unitRepo.On("GetByID", ctx, uint(4)).Return(&models.Unit{
			Base:       models.Base{ID: 4},
			UnitType:   "mass",
			Name:       "gram",
			Symbol:     "g",
			BaseUnitID: &baseUnitID,
		}, nil)
		unitRepo.On("GetByID", ctx, baseUnitID).Return(&models.Unit{
			Base:       models.Base{ID: baseUnitID},
			UnitType:   "mass",
			Name:       "kilogram",
			BaseUnitID: nil,
		}, nil)
		unitRepo.On("Update", ctx, unit).Return(nil)

		err := service.UpdateUnit(ctx, unit)
		assert.NoError(t, err)
		unitRepo.AssertExpectations(t)
	})
}

func TestDeleteUnit(t *testing.T) {
	ctx := context.Background()

	unitRepo := repositorymocks.NewUnitRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewUnitService(unitRepo, productRepo)

	unitRepo.On("Delete", ctx, uint(1)).Return(nil)

	err := service.DeleteUnit(ctx, 1)
	assert.NoError(t, err)
	unitRepo.AssertExpectations(t)
}
