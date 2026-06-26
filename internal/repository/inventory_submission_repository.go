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
	// GetByIDForUpdate loads a submission while holding a row-level write lock
	// (SELECT ... FOR UPDATE). It is tx-aware via DB(ctx): inside a WithinTx block
	// the lock is held for the life of that transaction, so a concurrent writer of
	// the same row blocks until commit. Callers must invoke it inside WithinTx to
	// get any locking benefit (outside a tx the lock releases immediately).
	GetByIDForUpdate(ctx context.Context, id uint) (*models.InventorySubmission, error)
	UpdateApprovalStatus(ctx context.Context, id uint, status models.SubmissionApprovalStatus, reason string) error
	UpdateProcessingStatus(ctx context.Context, id uint, status models.SubmissionProcessingStatus) error
	FailSubmissionProcessingWithErrors(ctx context.Context, id uint, errors []error) error
	ListSubmissions(ctx context.Context, params models.ListParams, inventoryID uint, approvalStatuses []string, submissionTypes []string) ([]models.InventorySubmission, int64, error)
	UpdateSubmissionPayload(ctx context.Context, id uint, payload []byte) error
	// ExistsActivePending reports whether a live pending RECONCILE submission
	// already exists for the inventory (one-active-pending guard, #38 P3,
	// reconcile-only). Tx-aware via DB(ctx).
	ExistsActivePending(ctx context.Context, inventoryID uint) (bool, error)
	// ListConsumingProcessedSince is the locked event-based drift re-check (epic
	// #38, Part 6 redesign). It returns every OTHER inventory_submissions row for
	// the same inventory that is CONSUMING (submission_type in dispose/transfer/
	// reconcile — all drop source stock; transfer's row is keyed to the source
	// inventory) AND was approved (approval_status='approved') AND PROCESSED
	// (processing_status='completed') with processed_at inside the reconcile
	// window [since, now]. Such a sibling created consuming inventory transactions
	// during the count, invalidating the snapshot baseline. PO receipts raise stock
	// OUTSIDE the submissions table and so never appear here (received-PO-during-
	// reconcile is allowed). A submission that was approved but FAILED processing
	// created no consuming transaction and has a nil processed_at, so it is
	// excluded. excludeSubmissionID is the parent reconcile itself. Tx-aware via
	// DB(ctx) — the caller runs it under the advisory lock so the read is
	// authoritative.
	ListConsumingProcessedSince(ctx context.Context, inventoryID, excludeSubmissionID uint, since time.Time) ([]models.InventorySubmission, error)
	// UpdateReconcileStatus sets the reconciliation lifecycle status (epic #38,
	// Part 6: open/closed/processing/processed). Tx-aware via DB(ctx).
	UpdateReconcileStatus(ctx context.Context, id uint, status models.ReconcileLifecycleStatus) error
	// MarkProcessed atomically finalizes a submission's processing: sets
	// processing_status='completed', approval_status='approved',
	// processed_at=clock_timestamp() (the DATABASE clock, so it shares a clock with
	// the snapshot created_at it is compared against — no cross-machine skew), and
	// (for a reconcile) reconcile_status='processed'. Tx-aware via DB(ctx) so it
	// commits inside the caller's apply transaction. Returns the DB-stamped
	// processed_at it read back. reconcileStatus is empty for non-reconcile callers.
	MarkProcessed(ctx context.Context, id uint, reconcileStatus models.ReconcileLifecycleStatus) (time.Time, error)
	// AcquireInventoryAdvisoryLock takes a transaction-scoped advisory lock keyed
	// on the inventory id (pg_advisory_xact_lock). Both the reconcile Start-
	// Processing apply and the general consuming ProcessSubmission apply take it, so
	// they serialize per inventory and the drift re-check is race-free (closes the
	// TOCTOU the review raised). MUST be called inside a transaction (DB(ctx)); the
	// lock auto-releases on commit/rollback.
	AcquireInventoryAdvisoryLock(ctx context.Context, inventoryID uint) error
	// SetProcessedAt stamps processed_at (and processing_status='completed') for a
	// consuming submission processed via the legacy ProcessSubmission path, so it
	// becomes visible to a concurrent reconcile's drift re-check. processed_at is
	// stamped with the DATABASE clock (clock_timestamp()) so it shares a clock with
	// the snapshot created_at it is compared against (no cross-machine skew).
	// Tx-aware.
	SetProcessedAt(ctx context.Context, id uint) error
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
	// Use DB(ctx) so the read enlists in the caller's transaction when there is
	// one (and the base connection otherwise — identical behavior outside a tx).
	// This keeps a tx-bound caller (e.g. synthesize invoked inside WithinTx after
	// mutating the parent) consistent with the child/snapshot reads, which already
	// go through DB(ctx).
	err := r.DB(ctx).WithContext(ctx).First(&submission, id).Error
	if err != nil {
		return nil, err
	}
	return &submission, nil
}

// GetByIDForUpdate retrieves a submission by ID while holding a row-level write
// lock (SELECT ... FOR UPDATE), tx-aware via DB(ctx). Used by the staff
// child-item write path: it loads the parent inside the same WithinTx as the
// child write so a concurrent ProcessSubmission reject/cancel of that parent
// either blocks until this tx commits or is observed by this tx, closing the
// terminal-parent race.
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

	// Tx-aware via DB(ctx): when the caller wraps the reject in a WithinTx that has
	// already taken the parent FOR UPDATE lock (the legacy-reject TOCTOU fix), this
	// write enlists in that transaction so the lock/re-check/write commit atomically.
	// Outside a tx the behavior is identical (base connection).
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
	// On a SUCCESSFUL completion, clear any stale failure audit. A prior atomic apply
	// failure records processing_status=failed + the error JSON while rolling approval
	// back to pending; gating is only on approval status, so the same submission can be
	// fixed and processed successfully via this path. Without resetting the error here,
	// list/detail would show a completed submission still carrying the previous failure.
	// Only a subsequent success clears it; the failure path still records the error.
	if status == models.InventorySubmissionStatusCompleted {
		fields["error"] = gorm.Expr("NULL")
	}
	updates, err := pkg.WithUpdateFields(ctx, fields)
	if err != nil {
		return fmt.Errorf("failed to prepare update fields: %w", err)
	}

	// Tx-aware via DB(ctx) so the reject path's canceled-status write commits in the
	// same transaction (and under the same parent lock) as the approval-status write.
	// Identical to the base connection when no tx is in flight.
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

// consumingSubmissionTypes are the submission types that DROP source stock when
// processed: dispose, transfer (keyed to the source inventory), and another
// reconcile. PO receipts are NOT submissions, so they never appear here.
var consumingSubmissionTypes = []models.SubmissionType{
	models.InventorySubmissionTypeDispose,
	models.InventorySubmissionTypeTransfer,
	models.InventorySubmissionTypeReconcile,
}

// ListConsumingProcessedSince implements the event-based drift re-check. See the
// interface doc: any OTHER consuming submission for the inventory, approved AND
// processed (processing_status='completed') with processed_at in [since, now].
// GORM's soft-delete scope excludes deleted rows. Tx-aware via DB(ctx).
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

// MarkProcessed finalizes a submission's processing atomically inside the
// caller's tx. See the interface doc.
func (r *inventorySubmissionRepository) MarkProcessed(ctx context.Context, id uint, reconcileStatus models.ReconcileLifecycleStatus) (time.Time, error) {
	// Stamp processed_at with the DATABASE clock (clock_timestamp()) rather than
	// the API host clock: it is compared against snapshot created_at values that
	// are themselves stamped by the DB clock, so a host-clock value would make the
	// drift window depend on cross-machine clock skew (a lagging API clock could
	// record processed_at < capturedAt and hide real drift). clock_timestamp()
	// advances within the tx, so it lands at the actual UPDATE instant.
	fields := map[string]interface{}{
		"processing_status": models.InventorySubmissionStatusCompleted,
		"approval_status":   models.InventorySubmissionApprovalStatusApproved,
		"processed_at":      gorm.Expr("clock_timestamp()"),
		// Clear any stale failure audit from an earlier failed apply: a prior atomic
		// apply failure records processing_status=failed + the error JSON while rolling
		// approval back to pending (FailSubmissionProcessingWithErrors). Gating is only
		// on approval status, so the same submission can later be fixed and processed
		// successfully — at which point the old error must be reset, otherwise list/detail
		// would show a completed/approved submission still carrying the previous failure.
		// Only a SUBSEQUENT success clears it; the failure path still records the error.
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
	// Read back the DB-stamped processed_at so callers can surface it (e.g. in the
	// response payload) without re-deriving it from the host clock.
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

// SetProcessedAt stamps processed_at + processing_status='completed' for a
// consuming submission processed via the legacy path. Tx-aware.
func (r *inventorySubmissionRepository) SetProcessedAt(ctx context.Context, id uint) error {
	// Stamp with the DATABASE clock (clock_timestamp()), NOT the API host clock:
	// processed_at is compared against snapshot created_at values that are stamped
	// by the DB clock, so a host-clock value would make the reconcile drift window
	// depend on cross-machine clock skew (a lagging API clock could record
	// processed_at < capturedAt and let StartProcessing miss real drift / apply a
	// stale baseline). clock_timestamp() advances within the tx and lands at the
	// actual UPDATE instant, keeping both sides of the comparison on one clock.
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

	// DB(ctx) so this enlists in the caller's transaction when present (the
	// reconcile Start-Processing apply persists the synthesized payload inside its
	// one tx, which already holds FOR UPDATE on this submission row — using the
	// base connection here would self-deadlock against that lock).
	return r.DB(ctx).WithContext(ctx).
		Model(&models.InventorySubmission{}).
		Where("id = ?", id).
		Updates(updates).Error
}
