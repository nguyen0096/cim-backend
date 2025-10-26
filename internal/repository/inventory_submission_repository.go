package repository

import (
	"cim-backend/internal/models"
	"context"
	"fmt"

	"gorm.io/gorm"
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
}

type inventorySubmissionRepository struct {
	db *gorm.DB
}

// NewInventorySubmissionRepository creates a new inventory submission repository
func NewInventorySubmissionRepository(db *gorm.DB) InventorySubmissionRepository {
	return &inventorySubmissionRepository{db: db}
}

// Create creates a new inventory submission
func (r *inventorySubmissionRepository) Create(ctx context.Context, submission *models.InventorySubmission) error {
	return r.db.WithContext(ctx).Create(submission).Error
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
	updates := map[string]interface{}{
		"approval_status": status,
		"reason":          reason,
	}
	return r.db.WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateProcessingStatus updates the processing status of a submission
func (r *inventorySubmissionRepository) UpdateProcessingStatus(ctx context.Context, id uint, status models.SubmissionProcessingStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("id = ?", id).
		Update("processing_status", status).Error
}

func (r *inventorySubmissionRepository) FailSubmissionProcessingWithErrors(ctx context.Context, id uint, errors []error) error {
	errorsJSON, err := models.MarshalErrors(errors)
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("id = ?", id).
		Update("processing_status", models.InventorySubmissionStatusFailed).
		Update("error", errorsJSON).Error
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

	query = query.
		Order("created_at DESC").
		Limit(params.Limit).
		Offset(params.GetOffset()).
		Preload("Inventory")
	if err := query.Find(&submissions).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list submissions: %w", err)
	}

	return submissions, total, nil
}
