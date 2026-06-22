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
}

type inventorySubmissionRepository struct {
	*baseRepository
}

// NewInventorySubmissionRepository creates a new inventory submission repository
func NewInventorySubmissionRepository(base BaseRepository) InventorySubmissionRepository {
	return &inventorySubmissionRepository{baseRepository: asBase(base)}
}

// Create creates a new inventory submission. It is transaction-aware: when the
// context carries a BaseRepository.WithinTx transaction it runs inside that
// transaction, otherwise it uses the repository's own connection.
func (r *inventorySubmissionRepository) Create(ctx context.Context, submission *models.InventorySubmission) error {
	return r.DB(ctx).WithContext(ctx).Create(submission).Error
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
