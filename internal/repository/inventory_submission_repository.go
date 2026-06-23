package repository

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"context"
	"fmt"
	"strings"
)

// InventorySubmissionRepository handles inventory submission persistence
type InventorySubmissionRepository interface {
	Create(ctx context.Context, submission *models.InventorySubmission) error
	GetPendingSubmissions(ctx context.Context, inventoryID uint) ([]models.InventorySubmission, error)
	GetByID(ctx context.Context, id uint) (*models.InventorySubmission, error)
	UpdateApprovalStatus(ctx context.Context, id uint, status models.SubmissionApprovalStatus, reason string) error
	UpdateProcessingStatus(ctx context.Context, id uint, status models.SubmissionProcessingStatus) error
	FailSubmissionProcessingWithErrors(ctx context.Context, id uint, errors []error) error
	ListSubmissions(ctx context.Context, params models.ListParams, inventoryID uint, approvalStatuses []string, submissionTypes []string) ([]models.InventorySubmission, int64, error)
	UpdateSubmissionPayload(ctx context.Context, id uint, payload []byte) error
	// ExistsActivePending reports whether a live pending RECONCILE submission
	// already exists for the inventory (one-active-pending guard, #38 P3,
	// reconcile-only). Tx-aware via DB(ctx).
	ExistsActivePending(ctx context.Context, inventoryID uint) (bool, error)
}

type inventorySubmissionRepository struct {
	*baseRepository
}

// NewInventorySubmissionRepository creates a new inventory submission repository
func NewInventorySubmissionRepository(base BaseRepository) InventorySubmissionRepository {
	return &inventorySubmissionRepository{baseRepository: asBase(base)}
}

// uqOneActivePendingReconcile is the partial unique index that enforces the
// one-active-pending-reconcile-per-inventory rule (migration 20260622000002).
const uqOneActivePendingReconcile = "uq_inventory_submissions_one_active_pending"

// Create creates a new inventory submission. Transaction-aware via DB(ctx). A
// violation of the one-active-pending-reconcile index is translated to the
// ErrActivePendingReconcileConflict domain error (so a concurrent race loser
// yields a clean conflict, not a raw DB error); other errors pass through.
func (r *inventorySubmissionRepository) Create(ctx context.Context, submission *models.InventorySubmission) error {
	err := r.DB(ctx).WithContext(ctx).Create(submission).Error
	constraint := uqOneActivePendingReconcile
	if err != nil && isDuplicateError(err, &constraint) {
		return pkg.ErrActivePendingReconcileConflict(submission.InventoryID, err)
	}
	return err
}

// GetPendingSubmissions retrieves all pending submissions for an inventory
func (r *inventorySubmissionRepository) GetPendingSubmissions(ctx context.Context, inventoryID uint) ([]models.InventorySubmission, error) {
	var submissions []models.InventorySubmission
	query := r.db.WithContext(ctx).
		Where("approval_status = ?", models.InventorySubmissionApprovalStatusPending).
		Order("created_at DESC")

	if inventoryID > 0 {
		query = query.Where("inventory_id = ?", inventoryID)
	}

	err := query.Preload("Inventory").Find(&submissions).Error
	return submissions, err
}

// GetByID retrieves a submission by ID
func (r *inventorySubmissionRepository) GetByID(ctx context.Context, id uint) (*models.InventorySubmission, error) {
	var submission models.InventorySubmission
	err := r.db.WithContext(ctx).First(&submission, id).Error
	if err != nil {
		return nil, err
	}
	return &submission, nil
}

// UpdateApprovalStatus updates the approval status and reason of a submission
func (r *inventorySubmissionRepository) UpdateApprovalStatus(ctx context.Context, id uint, status models.SubmissionApprovalStatus, reason string) error {
	updates, err := pkg.WithUpdateFields(ctx, map[string]interface{}{
		"approval_status": status,
		"reason":          reason,
	})
	if err != nil {
		return fmt.Errorf("failed to prepare update fields: %w", err)
	}

	return r.db.WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateProcessingStatus updates the processing status of a submission
func (r *inventorySubmissionRepository) UpdateProcessingStatus(ctx context.Context, id uint, status models.SubmissionProcessingStatus) error {
	updates, err := pkg.WithUpdateFields(ctx, map[string]interface{}{
		"processing_status": status,
	})
	if err != nil {
		return fmt.Errorf("failed to prepare update fields: %w", err)
	}

	return r.db.WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *inventorySubmissionRepository) FailSubmissionProcessingWithErrors(ctx context.Context, id uint, errors []error) error {
	errorsJSON, err := models.MarshalErrors(errors)
	if err != nil {
		return err
	}

	updates, err := pkg.WithUpdateFields(ctx, map[string]interface{}{
		"processing_status": models.InventorySubmissionStatusFailed,
		"error":             errorsJSON,
	})
	if err != nil {
		return fmt.Errorf("failed to prepare update fields: %w", err)
	}

	return r.db.WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// ListSubmissions retrieves submissions with pagination and filtering
func (r *inventorySubmissionRepository) ListSubmissions(
	ctx context.Context,
	params models.ListParams,
	inventoryID uint,
	approvalStatuses []string,
	submissionTypes []string,
) ([]models.InventorySubmission, int64, error) {
	var submissions []models.InventorySubmission
	var total int64

	query := r.db.WithContext(ctx).Model(&models.InventorySubmission{}).
		Where("inventory_id = ?", inventoryID)

	// Apply filters
	if len(approvalStatuses) > 0 {
		query = query.Where("approval_status IN ?", approvalStatuses)
	}

	if len(submissionTypes) > 0 {
		query = query.Where("submission_type IN ?", submissionTypes)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count submissions: %w", err)
	}

	// Build order clause using params Sort and Order
	orderClause := fmt.Sprintf("%s %s",
		params.Sort, strings.ToUpper(params.Order))
	query = query.
		Order(orderClause).
		Limit(params.Limit).
		Offset(params.GetOffset()).
		Preload("Inventory")
	if err := query.Find(&submissions).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list submissions: %w", err)
	}

	return submissions, total, nil
}

// ExistsActivePending reports whether a live pending RECONCILE submission exists
// for the inventory. The predicate mirrors the partial unique index (GORM adds
// the deleted_at IS NULL scope), so terminal reconciles and pending
// dispose/transfer never count. Tx-aware via DB(ctx).
func (r *inventorySubmissionRepository) ExistsActivePending(ctx context.Context, inventoryID uint) (bool, error) {
	var count int64
	err := r.DB(ctx).WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("inventory_id = ?", inventoryID).
		Where("processing_status = ?", models.InventorySubmissionStatusPending).
		Where("submission_type = ?", models.InventorySubmissionTypeReconcile).
		Limit(1).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateSubmissionPayload updates the payload of a submission
func (r *inventorySubmissionRepository) UpdateSubmissionPayload(ctx context.Context, id uint, payload []byte) error {
	updates, err := pkg.WithUpdateFields(ctx, map[string]interface{}{
		"payload": payload,
	})
	if err != nil {
		return fmt.Errorf("failed to prepare update fields: %w", err)
	}

	return r.db.WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("id = ?", id).
		Updates(updates).Error
}
