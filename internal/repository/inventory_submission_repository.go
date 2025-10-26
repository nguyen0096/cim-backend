package repository

import (
	"cim-backend/internal/models"
	"context"

	"gorm.io/gorm"
)

// InventorySubmissionRepository handles inventory submission persistence
type InventorySubmissionRepository interface {
	Create(ctx context.Context, submission *models.InventorySubmission) error
	GetPendingSubmissions(ctx context.Context, inventoryID uint) ([]models.InventorySubmission, error)
	GetByID(ctx context.Context, id uint) (*models.InventorySubmission, error)
	UpdateApprovalStatus(ctx context.Context, id uint, status models.InventorySubmissionApprovalStatus, reason string) error
	UpdateProcessingStatus(ctx context.Context, id uint, status models.InventorySubmissionStatus) error
	FailSubmissionProcessingWithErrors(ctx context.Context, id uint, errors []error) error
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
func (r *inventorySubmissionRepository) UpdateApprovalStatus(ctx context.Context, id uint, status models.InventorySubmissionApprovalStatus, reason string) error {
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
func (r *inventorySubmissionRepository) UpdateProcessingStatus(ctx context.Context, id uint, status models.InventorySubmissionStatus) error {
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
