package services

import (
	"cim-backend/internal/models"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

type mockUnitRepository struct {
	mock.Mock
}

func (m *mockUnitRepository) Create(ctx context.Context, unit *models.Unit) error {
	args := m.Called(ctx, unit)
	return args.Error(0)
}

func (m *mockUnitRepository) GetByID(ctx context.Context, id uint) (*models.Unit, error) {
	args := m.Called(ctx, id)
	unit, _ := args.Get(0).(*models.Unit)
	return unit, args.Error(1)
}

func (m *mockUnitRepository) GetByTypeAndName(ctx context.Context, unitType, name string) (*models.Unit, error) {
	args := m.Called(ctx, unitType, name)
	unit, _ := args.Get(0).(*models.Unit)
	return unit, args.Error(1)
}

func (m *mockUnitRepository) Update(ctx context.Context, unit *models.Unit) error {
	args := m.Called(ctx, unit)
	return args.Error(0)
}

func (m *mockUnitRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUnitRepository) Restore(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUnitRepository) List(ctx context.Context, limit, offset int, sortBy, sortOrder, unitType string) ([]models.Unit, error) {
	args := m.Called(ctx, limit, offset, sortBy, sortOrder, unitType)
	units, _ := args.Get(0).([]models.Unit)
	return units, args.Error(1)
}

func (m *mockUnitRepository) Search(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, unitType string) ([]models.Unit, error) {
	args := m.Called(ctx, query, limit, offset, sortBy, sortOrder, unitType)
	units, _ := args.Get(0).([]models.Unit)
	return units, args.Error(1)
}

func (m *mockUnitRepository) Count(ctx context.Context, unitType string) (int64, error) {
	args := m.Called(ctx, unitType)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockUnitRepository) CountSearch(ctx context.Context, query, unitType string) (int64, error) {
	args := m.Called(ctx, query, unitType)
	return args.Get(0).(int64), args.Error(1)
}

func TestCreateUnit(t *testing.T) {
	ctx := context.Background()

	t.Run("should create unit successfully", func(t *testing.T) {
		repo := new(mockUnitRepository)
		service := NewUnitService(repo)

		unit := &models.Unit{
			UnitType:         "mass",
			Name:             "kilogram",
			Symbol:           "kg",
			ConversionFactor: 1,
		}

		repo.On("GetByTypeAndName", ctx, "mass", "kilogram").Return((*models.Unit)(nil), gorm.ErrRecordNotFound)
		repo.On("Create", ctx, unit).Return(nil)

		err := service.CreateUnit(ctx, unit)
		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("should create derived unit when base unit is valid", func(t *testing.T) {
		repo := new(mockUnitRepository)
		service := NewUnitService(repo)

		baseUnitID := uint(2)
		unit := &models.Unit{
			UnitType:         "mass",
			Name:             "gram",
			Symbol:           "g",
			BaseUnitID:       &baseUnitID,
			ConversionFactor: 0.001,
		}

		repo.On("GetByTypeAndName", ctx, "mass", "gram").Return((*models.Unit)(nil), gorm.ErrRecordNotFound)
		repo.On("GetByID", ctx, baseUnitID).Return(&models.Unit{
			Base:       models.Base{ID: baseUnitID},
			UnitType:   "mass",
			Name:       "kilogram",
			BaseUnitID: nil,
		}, nil)
		repo.On("Create", ctx, unit).Return(nil)

		err := service.CreateUnit(ctx, unit)
		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("should return validation error when derived unit missing base reference", func(t *testing.T) {
		repo := new(mockUnitRepository)
		service := NewUnitService(repo)

		err := service.CreateUnit(ctx, &models.Unit{
			UnitType:         "mass",
			Name:             "gram",
			Symbol:           "g",
			ConversionFactor: 0.001,
		})
		assert.Error(t, err)
	})

	t.Run("should return validation error when base unit reference invalid", func(t *testing.T) {
		repo := new(mockUnitRepository)
		service := NewUnitService(repo)

		baseUnitID := uint(2)
		unit := &models.Unit{
			UnitType:         "mass",
			Name:             "gram",
			Symbol:           "g",
			BaseUnitID:       &baseUnitID,
			ConversionFactor: 0.001,
		}

		repo.On("GetByID", ctx, baseUnitID).Return(&models.Unit{
			Base:       models.Base{ID: baseUnitID},
			UnitType:   "mass",
			Name:       "kilogram",
			BaseUnitID: &baseUnitID,
		}, nil)

		err := service.CreateUnit(ctx, unit)
		assert.Error(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("should return validation error when required fields missing", func(t *testing.T) {
		repo := new(mockUnitRepository)
		service := NewUnitService(repo)

		err := service.CreateUnit(ctx, &models.Unit{
			UnitType:         "",
			Name:             "",
			Symbol:           "",
			ConversionFactor: 0,
		})
		assert.Error(t, err)
	})

	t.Run("should return duplicate error when unit exists", func(t *testing.T) {
		repo := new(mockUnitRepository)
		service := NewUnitService(repo)

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

		repo.On("GetByTypeAndName", ctx, "mass", "kilogram").Return(existing, nil)

		err := service.CreateUnit(ctx, unit)
		assert.Error(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("should return error when repository create fails", func(t *testing.T) {
		repo := new(mockUnitRepository)
		service := NewUnitService(repo)

		unit := &models.Unit{
			UnitType:         "mass",
			Name:             "kilogram",
			Symbol:           "kg",
			ConversionFactor: 1,
		}
		repo.On("GetByTypeAndName", ctx, "mass", "kilogram").Return((*models.Unit)(nil), gorm.ErrRecordNotFound)
		repo.On("Create", ctx, unit).Return(errors.New("db error"))

		err := service.CreateUnit(ctx, unit)
		assert.Error(t, err)
		repo.AssertExpectations(t)
	})
}

func TestUpdateUnit(t *testing.T) {
	ctx := context.Background()

	t.Run("should update unit successfully", func(t *testing.T) {
		repo := new(mockUnitRepository)
		service := NewUnitService(repo)

		unit := &models.Unit{
			Base:             models.Base{ID: 1},
			UnitType:         "volume",
			Name:             "liter",
			Symbol:           "L",
			ConversionFactor: 1,
		}

		repo.On("GetByID", ctx, uint(1)).Return(&models.Unit{
			Base:       models.Base{ID: 1},
			UnitType:   "volume",
			Name:       "liter",
			Symbol:     "L",
			BaseUnitID: nil,
		}, nil)
		repo.On("Update", ctx, unit).Return(nil)

		err := service.UpdateUnit(ctx, unit)
		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("should error when unit not found", func(t *testing.T) {
		repo := new(mockUnitRepository)
		service := NewUnitService(repo)

		unit := &models.Unit{
			Base:             models.Base{ID: 99},
			UnitType:         "volume",
			Name:             "liter",
			Symbol:           "L",
			ConversionFactor: 1,
		}

		repo.On("GetByID", ctx, uint(99)).Return((*models.Unit)(nil), gorm.ErrRecordNotFound)

		err := service.UpdateUnit(ctx, unit)
		assert.Error(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("should validate derived unit base reference on update", func(t *testing.T) {
		repo := new(mockUnitRepository)
		service := NewUnitService(repo)

		baseUnitID := uint(3)
		unit := &models.Unit{
			Base:             models.Base{ID: 4},
			UnitType:         "mass",
			Name:             "gram",
			Symbol:           "g",
			BaseUnitID:       &baseUnitID,
			ConversionFactor: 0.001,
		}

		repo.On("GetByID", ctx, uint(4)).Return(&models.Unit{
			Base:       models.Base{ID: 4},
			UnitType:   "mass",
			Name:       "gram",
			Symbol:     "g",
			BaseUnitID: &baseUnitID,
		}, nil)
		repo.On("GetByID", ctx, baseUnitID).Return(&models.Unit{
			Base:       models.Base{ID: baseUnitID},
			UnitType:   "mass",
			Name:       "kilogram",
			BaseUnitID: nil,
		}, nil)
		repo.On("Update", ctx, unit).Return(nil)

		err := service.UpdateUnit(ctx, unit)
		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})
}

func TestDeleteUnit(t *testing.T) {
	ctx := context.Background()

	repo := new(mockUnitRepository)
	service := NewUnitService(repo)

	repo.On("Delete", ctx, uint(1)).Return(nil)

	err := service.DeleteUnit(ctx, 1)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
