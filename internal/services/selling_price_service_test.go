package services

import (
	"cim-backend/internal/mocks/repositorymocks"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestGetSellingPrice_NotFound_ReturnsAppError(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil)

	spRepo.On("GetByID", ctx, uint(999)).Return(nil, gorm.ErrRecordNotFound)

	sp, err := service.GetSellingPrice(ctx, 999)
	assert.Nil(t, sp)
	assert.Error(t, err)

	appErr, ok := err.(*pkg.AppError)
	assert.True(t, ok, "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeNotFound, appErr.Code)
}

func TestUpdateSellingPrice_NotFound_ReturnsAppError(t *testing.T) {
	ctx := context.Background()
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	productRepo := repositorymocks.NewProductRepository(t)
	service := NewSellingPriceService(spRepo, productRepo, nil)

	spRepo.On("GetByID", ctx, uint(999)).Return(nil, gorm.ErrRecordNotFound)

	sp, err := service.UpdateSellingPrice(ctx, 999, dto.UpdateSellingPriceRequest{
		EffectiveFrom: "2026-01-01",
	})
	assert.Nil(t, sp)
	assert.Error(t, err)

	appErr, ok := err.(*pkg.AppError)
	assert.True(t, ok, "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeNotFound, appErr.Code)
}
