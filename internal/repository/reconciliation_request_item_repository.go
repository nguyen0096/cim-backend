package repository

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"context"
	"encoding/json"
)

// ReconciliationRequestItemRepository persists the per-staff child rows of an
// in-flight reconciliation.
//
//go:generate mockery --name=ReconciliationRequestItemRepository --structname=ReconciliationRequestItemRepository --output=../mocks/repositorymocks --outpkg=repositorymocks
type ReconciliationRequestItemRepository interface {
	// Create inserts a new child row.
	Create(ctx context.Context, item *models.ReconciliationRequestItem) error
	// GetByID loads a single child row by id.
	GetByID(ctx context.Context, id uint) (*models.ReconciliationRequestItem, error)
	// ListBySubmission returns the submission's child rows, oldest first.
	ListBySubmission(ctx context.Context, submissionID uint) ([]models.ReconciliationRequestItem, error)
	// ListBySubmissionAndCreator returns the submission's child rows created by
	// createdBy, oldest first.
	ListBySubmissionAndCreator(ctx context.Context, submissionID uint, createdBy string) ([]models.ReconciliationRequestItem, error)
	// UpdateLabelPayloadAndStatus writes the row's label, payload, and status.
	UpdateLabelPayloadAndStatus(ctx context.Context, id uint, label string, payload json.RawMessage, status models.ReconciliationRequestItemStatus) error
	// UpdateStatus writes only the row's status.
	UpdateStatus(ctx context.Context, id uint, status models.ReconciliationRequestItemStatus) error
	// SoftDelete soft-deletes a child row.
	SoftDelete(ctx context.Context, id uint) error
}

type reconciliationRequestItemRepository struct {
	*baseRepository
}

// NewReconciliationRequestItemRepository creates a new reconciliation request item repository.
func NewReconciliationRequestItemRepository(base BaseRepository) ReconciliationRequestItemRepository {
	return &reconciliationRequestItemRepository{baseRepository: asBase(base)}
}

func (r *reconciliationRequestItemRepository) Create(ctx context.Context, item *models.ReconciliationRequestItem) error {
	return r.DB(ctx).WithContext(ctx).Create(item).Error
}

func (r *reconciliationRequestItemRepository) GetByID(ctx context.Context, id uint) (*models.ReconciliationRequestItem, error) {
	var item models.ReconciliationRequestItem
	if err := r.DB(ctx).WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *reconciliationRequestItemRepository) ListBySubmission(ctx context.Context, submissionID uint) ([]models.ReconciliationRequestItem, error) {
	var items []models.ReconciliationRequestItem
	err := r.DB(ctx).WithContext(ctx).
		Where("submission_id = ?", submissionID).
		Order("created_at ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *reconciliationRequestItemRepository) ListBySubmissionAndCreator(ctx context.Context, submissionID uint, createdBy string) ([]models.ReconciliationRequestItem, error) {
	var items []models.ReconciliationRequestItem
	err := r.DB(ctx).WithContext(ctx).
		Where("submission_id = ? AND created_by = ?", submissionID, createdBy).
		Order("id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *reconciliationRequestItemRepository) UpdateLabelPayloadAndStatus(ctx context.Context, id uint, label string, payload json.RawMessage, status models.ReconciliationRequestItemStatus) error {
	updates, err := pkg.WithUpdateFields(ctx, map[string]interface{}{
		"label":   label,
		"payload": payload,
		"status":  status,
	})
	if err != nil {
		return err
	}
	return r.DB(ctx).WithContext(ctx).
		Model(&models.ReconciliationRequestItem{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *reconciliationRequestItemRepository) UpdateStatus(ctx context.Context, id uint, status models.ReconciliationRequestItemStatus) error {
	updates, err := pkg.WithUpdateFields(ctx, map[string]interface{}{
		"status": status,
	})
	if err != nil {
		return err
	}
	return r.DB(ctx).WithContext(ctx).
		Model(&models.ReconciliationRequestItem{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *reconciliationRequestItemRepository) SoftDelete(ctx context.Context, id uint) error {
	// Stamp updated_by/updated_at for audit; gorm's Delete only sets deleted_at.
	updates, err := pkg.WithUpdateFields(ctx, map[string]interface{}{})
	if err != nil {
		return err
	}
	db := r.DB(ctx).WithContext(ctx)
	if err := db.Model(&models.ReconciliationRequestItem{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return err
	}
	return db.Where("id = ?", id).Delete(&models.ReconciliationRequestItem{}).Error
}
