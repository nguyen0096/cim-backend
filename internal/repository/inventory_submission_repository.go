package repository

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// InventorySubmissionRepository handles inventory submission persistence
type InventorySubmissionRepository interface {
	Create(ctx context.Context, submission *models.InventorySubmission) error
	GetPendingSubmissions(ctx context.Context, inventoryID uint) ([]models.InventorySubmission, error)
	GetByID(ctx context.Context, id uint) (*models.InventorySubmission, error)
	// GetByIDForUpdate loads a submission holding a row-level write lock. Tx-aware via DB(ctx).
	GetByIDForUpdate(ctx context.Context, id uint) (*models.InventorySubmission, error)
	UpdateApprovalStatus(ctx context.Context, id uint, status models.SubmissionApprovalStatus, reason string) error
	UpdateProcessingStatus(ctx context.Context, id uint, status models.SubmissionProcessingStatus) error
	FailSubmissionProcessingWithErrors(ctx context.Context, id uint, errors []error) error
	ListSubmissions(ctx context.Context, params models.ListParams, inventoryID uint, approvalStatuses []string, submissionTypes []string) ([]models.InventorySubmission, int64, error)
	// ListActiveReconciliations returns active reconcile submissions across all inventories.
	ListActiveReconciliations(ctx context.Context, params models.ListParams, reconcileStatuses []string) ([]models.InventorySubmission, int64, error)
	UpdateSubmissionPayload(ctx context.Context, id uint, payload []byte) error
	// ExistsActivePending reports whether a live pending reconcile submission exists for the inventory. Tx-aware via DB(ctx).
	ExistsActivePending(ctx context.Context, inventoryID uint) (bool, error)
	// ListConsumingProcessedSince returns other consuming submissions for the inventory that were approved and processed since the given time. Tx-aware via DB(ctx).
	ListConsumingProcessedSince(ctx context.Context, inventoryID, excludeSubmissionID uint, since time.Time) ([]models.InventorySubmission, error)
	// UpdateReconcileStatus sets the reconciliation lifecycle status. Tx-aware via DB(ctx).
	UpdateReconcileStatus(ctx context.Context, id uint, status models.ReconcileLifecycleStatus) error
	// CancelReconciliation sets the terminal cancel state without mutating inventory. Tx-aware via DB(ctx).
	CancelReconciliation(ctx context.Context, id uint) error
	// MarkProcessed finalizes a submission's processing and returns the DB-stamped processed_at. reconcileStatus is empty for non-reconcile callers. Tx-aware via DB(ctx).
	MarkProcessed(ctx context.Context, id uint, reconcileStatus models.ReconcileLifecycleStatus) (time.Time, error)
	// AcquireInventoryAdvisoryLock takes pg_advisory_xact_lock(inventory_id). Must be called inside a transaction.
	AcquireInventoryAdvisoryLock(ctx context.Context, inventoryID uint) error
	// SetProcessedAt stamps processed_at and processing_status='completed' for a consuming submission processed via the legacy path. Tx-aware.
	SetProcessedAt(ctx context.Context, id uint) error
}

type inventorySubmissionRepository struct {
	*baseRepository
}

// NewInventorySubmissionRepository creates a new inventory submission repository
func NewInventorySubmissionRepository(base BaseRepository) InventorySubmissionRepository {
	return &inventorySubmissionRepository{baseRepository: asBase(base)}
}

// uqOneActivePendingReconcile is the partial unique index enforcing one active pending reconcile per inventory.
const uqOneActivePendingReconcile = "uq_inventory_submissions_one_active_pending"

// Create creates a new inventory submission. Tx-aware via DB(ctx). A one-active-pending-reconcile index violation is translated to ErrActivePendingReconcileConflict.
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
	err := r.DB(ctx).WithContext(ctx).First(&submission, id).Error
	if err != nil {
		return nil, err
	}
	return &submission, nil
}

// GetByIDForUpdate retrieves a submission by ID holding a row-level write lock. Tx-aware via DB(ctx).
func (r *inventorySubmissionRepository) GetByIDForUpdate(ctx context.Context, id uint) (*models.InventorySubmission, error) {
	var submission models.InventorySubmission
	err := r.DB(ctx).WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&submission, id).Error
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

	return r.DB(ctx).WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateProcessingStatus updates the processing status of a submission
func (r *inventorySubmissionRepository) UpdateProcessingStatus(ctx context.Context, id uint, status models.SubmissionProcessingStatus) error {
	fields := map[string]interface{}{
		"processing_status": status,
	}
	// Clear any stale failure audit on successful completion.
	if status == models.InventorySubmissionStatusCompleted {
		fields["error"] = gorm.Expr("NULL")
	}
	updates, err := pkg.WithUpdateFields(ctx, fields)
	if err != nil {
		return fmt.Errorf("failed to prepare update fields: %w", err)
	}

	return r.DB(ctx).WithContext(ctx).
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

// activeReconcileStatuses is the reconcile_status set of an active reconcile.
var activeReconcileStatuses = []models.ReconcileLifecycleStatus{
	models.ReconcileLifecycleStatusOpen,
	models.ReconcileLifecycleStatusClosed,
}

// ListActiveReconciliations returns active reconcile submissions across all inventories.
func (r *inventorySubmissionRepository) ListActiveReconciliations(
	ctx context.Context,
	params models.ListParams,
	reconcileStatuses []string,
) ([]models.InventorySubmission, int64, error) {
	var submissions []models.InventorySubmission
	var total int64

	statuses := reconcileStatuses
	if len(statuses) == 0 {
		statuses = make([]string, len(activeReconcileStatuses))
		for i, s := range activeReconcileStatuses {
			statuses[i] = string(s)
		}
	}

	query := r.db.WithContext(ctx).Model(&models.InventorySubmission{}).
		Where("submission_type = ?", models.InventorySubmissionTypeReconcile).
		Where("processing_status = ?", models.InventorySubmissionStatusPending).
		Where("reconcile_status IN ?", statuses)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count active reconciliations: %w", err)
	}

	orderClause := fmt.Sprintf("%s %s", params.Sort, strings.ToUpper(params.Order))
	query = query.
		Order(orderClause).
		Limit(params.Limit).
		Offset(params.GetOffset()).
		Preload("Inventory")
	if err := query.Find(&submissions).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list active reconciliations: %w", err)
	}

	return submissions, total, nil
}

// ExistsActivePending reports whether a live pending reconcile submission exists for the inventory. Tx-aware via DB(ctx).
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

// consumingSubmissionTypes are the submission types that drop source stock when processed.
var consumingSubmissionTypes = []models.SubmissionType{
	models.InventorySubmissionTypeDispose,
	models.InventorySubmissionTypeTransfer,
	models.InventorySubmissionTypeReconcile,
}

// ListConsumingProcessedSince returns other consuming submissions for the inventory that were approved and processed since the given time. Tx-aware via DB(ctx).
func (r *inventorySubmissionRepository) ListConsumingProcessedSince(ctx context.Context, inventoryID, excludeSubmissionID uint, since time.Time) ([]models.InventorySubmission, error) {
	var rows []models.InventorySubmission
	err := r.DB(ctx).WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("inventory_id = ?", inventoryID).
		Where("id <> ?", excludeSubmissionID).
		Where("submission_type IN ?", consumingSubmissionTypes).
		Where("approval_status = ?", models.InventorySubmissionApprovalStatusApproved).
		Where("processing_status = ?", models.InventorySubmissionStatusCompleted).
		Where("processed_at IS NOT NULL").
		Where("processed_at >= ?", since).
		Order("processed_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// UpdateReconcileStatus sets the reconciliation lifecycle status. Tx-aware.
func (r *inventorySubmissionRepository) UpdateReconcileStatus(ctx context.Context, id uint, status models.ReconcileLifecycleStatus) error {
	updates, err := pkg.WithUpdateFields(ctx, map[string]interface{}{
		"reconcile_status": status,
	})
	if err != nil {
		return fmt.Errorf("failed to prepare update fields: %w", err)
	}
	return r.DB(ctx).WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// CancelReconciliation sets the terminal cancel state without mutating inventory. Tx-aware via DB(ctx).
func (r *inventorySubmissionRepository) CancelReconciliation(ctx context.Context, id uint) error {
	updates, err := pkg.WithUpdateFields(ctx, map[string]interface{}{
		"reconcile_status":  models.ReconcileLifecycleStatusCanceled,
		"processing_status": models.InventorySubmissionStatusCanceled,
	})
	if err != nil {
		return fmt.Errorf("failed to prepare update fields: %w", err)
	}
	return r.DB(ctx).WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// MarkProcessed finalizes a submission's processing and returns the DB-stamped processed_at. Tx-aware via DB(ctx).
func (r *inventorySubmissionRepository) MarkProcessed(ctx context.Context, id uint, reconcileStatus models.ReconcileLifecycleStatus) (time.Time, error) {
	// processed_at uses the DB clock (clock_timestamp()) so it shares a clock with snapshot created_at.
	fields := map[string]interface{}{
		"processing_status": models.InventorySubmissionStatusCompleted,
		"approval_status":   models.InventorySubmissionApprovalStatusApproved,
		"processed_at":      gorm.Expr("clock_timestamp()"),
		// Clear any stale failure audit from an earlier failed apply.
		"error": gorm.Expr("NULL"),
	}
	if reconcileStatus != "" {
		fields["reconcile_status"] = reconcileStatus
	}
	updates, err := pkg.WithUpdateFields(ctx, fields)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to prepare update fields: %w", err)
	}
	if err := r.DB(ctx).WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return time.Time{}, err
	}
	var processedAt time.Time
	if err := r.DB(ctx).WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Select("processed_at").
		Where("id = ?", id).
		Scan(&processedAt).Error; err != nil {
		return time.Time{}, err
	}
	return processedAt, nil
}

// SetProcessedAt stamps processed_at and processing_status='completed' for a consuming submission processed via the legacy path. Tx-aware.
func (r *inventorySubmissionRepository) SetProcessedAt(ctx context.Context, id uint) error {
	// processed_at uses the DB clock (clock_timestamp()) so it shares a clock with snapshot created_at.
	updates, err := pkg.WithUpdateFields(ctx, map[string]interface{}{
		"processing_status": models.InventorySubmissionStatusCompleted,
		"processed_at":      gorm.Expr("clock_timestamp()"),
	})
	if err != nil {
		return fmt.Errorf("failed to prepare update fields: %w", err)
	}
	return r.DB(ctx).WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// AcquireInventoryAdvisoryLock takes pg_advisory_xact_lock(inventory_id). Must be
// called inside a transaction; the lock auto-releases at commit/rollback.
func (r *inventorySubmissionRepository) AcquireInventoryAdvisoryLock(ctx context.Context, inventoryID uint) error {
	return r.DB(ctx).WithContext(ctx).
		Exec("SELECT pg_advisory_xact_lock(?)", int64(inventoryID)).Error
}

// UpdateSubmissionPayload updates the payload of a submission
func (r *inventorySubmissionRepository) UpdateSubmissionPayload(ctx context.Context, id uint, payload []byte) error {
	updates, err := pkg.WithUpdateFields(ctx, map[string]interface{}{
		"payload": payload,
	})
	if err != nil {
		return fmt.Errorf("failed to prepare update fields: %w", err)
	}

	return r.DB(ctx).WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("id = ?", id).
		Updates(updates).Error
}
