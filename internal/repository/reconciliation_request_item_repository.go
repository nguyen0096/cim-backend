package repository

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"context"
	"encoding/json"
)

// ReconciliationRequestItemRepository persists the per-staff/batch child rows of
// an in-flight reconciliation (epic #38, Part 4). Each row carries COUNTED
// quantities only (legacy reconcile payload shape) and a status in the
// in_progress -> ready -> approved -> applied state machine. The contributing
// staff member is recorded via models.Base.CreatedBy (user email).
//
//go:generate mockery --name=ReconciliationRequestItemRepository --structname=ReconciliationRequestItemRepository --output=../mocks/repositorymocks --outpkg=repositorymocks
type ReconciliationRequestItemRepository interface {
	// Create inserts a new child row. Status/CreatedBy are stamped by the caller /
	// the models.Base BeforeCreate hook. Tx-aware via DB(ctx).
	Create(ctx context.Context, item *models.ReconciliationRequestItem) error
	// GetByID loads a single child row by id (excludes soft-deleted rows via the
	// gorm.DeletedAt scope). Returns gorm.ErrRecordNotFound when absent. Tx-aware.
	GetByID(ctx context.Context, id uint) (*models.ReconciliationRequestItem, error)
	// ListBySubmission returns all live (non-soft-deleted) child rows for a parent
	// submission, oldest first. Tx-aware via DB(ctx).
	ListBySubmission(ctx context.Context, submissionID uint) ([]models.ReconciliationRequestItem, error)
	// UpdatePayloadAndStatus writes both the counted-quantity payload and the new
	// status of a child row in one update (used by the staff edit path, including
	// the approved -> in_progress escape hatch). Tx-aware via DB(ctx).
	UpdatePayloadAndStatus(ctx context.Context, id uint, payload json.RawMessage, status models.ReconciliationRequestItemStatus) error
	// UpdateStatus transitions only the status of a child row (mark ready /
	// not_ready). Tx-aware via DB(ctx).
	UpdateStatus(ctx context.Context, id uint, status models.ReconciliationRequestItemStatus) error
	// SoftDelete soft-deletes a child row (sets deleted_at). Tx-aware via DB(ctx).
	SoftDelete(ctx context.Context, id uint) error
}

type reconciliationRequestItemRepository struct {
	*baseRepository
}

// NewReconciliationRequestItemRepository creates a new reconciliation request
// item repository sharing the process-wide BaseRepository.
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

func (r *reconciliationRequestItemRepository) UpdatePayloadAndStatus(ctx context.Context, id uint, payload json.RawMessage, status models.ReconciliationRequestItemStatus) error {
	updates, err := pkg.WithUpdateFields(ctx, map[string]interface{}{
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
	// Stamp updated_by/updated_at alongside the soft-delete so the deleting actor
	// is auditable; gorm's Delete on a soft-delete model only sets deleted_at. Both
	// statements run on DB(ctx), so they enlist in the caller's transaction when one
	// is in flight (WithinTx) and are otherwise applied directly.
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
