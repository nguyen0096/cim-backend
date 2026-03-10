package repository

import (
	"cim-backend/internal/models"
	"context"

	"gorm.io/gorm"
)

//go:generate mockery --name=RevenueExpenseFinalizationRepository --structname=RevenueExpenseFinalizationRepository --output=../mocks/repositorymocks --outpkg=repositorymocks
type RevenueExpenseFinalizationRepository interface {
	Create(ctx context.Context, finalization *models.RevenueExpenseFinalization) error
	Update(ctx context.Context, finalization *models.RevenueExpenseFinalization) error
	List(ctx context.Context, limit, offset int) ([]models.RevenueExpenseFinalization, int64, error)
	GetLastest(ctx context.Context) (*models.RevenueExpenseFinalization, error)
}

type revenueExpenseFinalizationRepository struct {
	db *gorm.DB
}

// NewRevenueExpenseFinalizationRepository creates a new revenue expense finalization repository
func NewRevenueExpenseFinalizationRepository(db *gorm.DB) RevenueExpenseFinalizationRepository {
	return &revenueExpenseFinalizationRepository{
		db: db,
	}
}

// Create inserts a new revenue expense finalization record
func (r *revenueExpenseFinalizationRepository) Create(ctx context.Context, finalization *models.RevenueExpenseFinalization) error {
	return r.db.WithContext(ctx).Create(finalization).Error
}

// Update updates an existing revenue expense finalization record
func (r *revenueExpenseFinalizationRepository) Update(ctx context.Context, finalization *models.RevenueExpenseFinalization) error {
	return r.db.WithContext(ctx).Save(finalization).Error
}

// List retrieves revenue expense finalizations with pagination
func (r *revenueExpenseFinalizationRepository) List(ctx context.Context, limit, offset int) ([]models.RevenueExpenseFinalization, int64, error) {
	var finalizations []models.RevenueExpenseFinalization
	var total int64

	if err := r.db.WithContext(ctx).Model(&models.RevenueExpenseFinalization{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.db.WithContext(ctx).
		Order("finalized_date DESC").
		Limit(limit).
		Offset(offset).
		Find(&finalizations).Error; err != nil {
		return nil, 0, err
	}

	return finalizations, total, nil
}

// GetLastest retrieves the most recent successful finalization
func (r *revenueExpenseFinalizationRepository) GetLastest(ctx context.Context) (*models.RevenueExpenseFinalization, error) {
	var finalization models.RevenueExpenseFinalization
	err := r.db.WithContext(ctx).
		Order("finalized_date DESC").
		First(&finalization).Error
	if err != nil {
		return nil, err
	}
	return &finalization, nil
}
