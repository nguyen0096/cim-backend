package repository

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"context"
)

// ReconciliationSnapshotRepository persists the per-item baseline quantities
// captured when a reconciliation is initiated (epic #38, Part 2). The snapshot
// is the sole source of truth for prev_quantity during reconciliation.
//
//go:generate mockery --name=ReconciliationSnapshotRepository --structname=ReconciliationSnapshotRepository --output=../mocks/repositorymocks --outpkg=repositorymocks
type ReconciliationSnapshotRepository interface {
	// BuildReconciliationSnapshots captures the per-item baseline for a
	// reconciliation in a single set-based INSERT ... SELECT, so the snapshot is
	// never materialised in application memory. For every active, non-deleted
	// inventory_items row of the inventory it inserts one reconciliation_snapshots
	// row with prev_quantity = that item's current quantity, FK'd to submissionID.
	//
	// It is transaction-aware (DB(ctx)): run inside the initiate transaction so
	// the snapshots commit/roll back atomically with the parent submission, and so
	// prev_quantity is read against the same committed state. Returns the number of
	// snapshot rows inserted (= number of active items), which the caller uses to
	// detect the "no active items" case without a separate count query.
	BuildReconciliationSnapshots(ctx context.Context, submissionID, inventoryID uint) (int64, error)
	// ExistsForSubmission reports whether any (non-soft-deleted) snapshot row is
	// associated with the submission. The presence of snapshot rows is the
	// new-model marker that distinguishes a reconciliation started via the
	// initiate endpoint from a legacy single-payload reconcile submission.
	ExistsForSubmission(ctx context.Context, submissionID uint) (bool, error)
}

type reconciliationSnapshotRepository struct {
	*baseRepository
}

// NewReconciliationSnapshotRepository creates a new reconciliation snapshot repository.
func NewReconciliationSnapshotRepository(base BaseRepository) ReconciliationSnapshotRepository {
	return &reconciliationSnapshotRepository{baseRepository: asBase(base)}
}

// buildReconciliationSnapshotsSQL inserts one snapshot per active, non-deleted
// inventory item of the inventory, copying the item's live quantity into
// prev_quantity. created_at/updated_at are stamped with NOW() and
// created_by/updated_by with the initiating user (the raw INSERT bypasses
// models.Base.BeforeCreate, so the audit columns are populated explicitly here to
// keep the captured baseline auditable). deleted_at is left NULL. The source rows
// are filtered to status='active' AND deleted_at IS NULL so the baseline matches
// the active-item set the legacy paths use.
const buildReconciliationSnapshotsSQL = `
INSERT INTO reconciliation_snapshots
	(submission_id, inventory_item_id, prev_quantity, created_by, updated_by, created_at, updated_at)
SELECT ?, id, quantity, ?, ?, NOW(), NOW()
FROM inventory_items
WHERE inventory_id = ?
  AND status = ?
  AND deleted_at IS NULL`

func (r *reconciliationSnapshotRepository) BuildReconciliationSnapshots(ctx context.Context, submissionID, inventoryID uint) (int64, error) {
	// Populate the audit columns from context, mirroring models.Base.BeforeCreate
	// (which this raw INSERT bypasses). The initiate flow always runs with an
	// authenticated user; surface a clear error rather than silently writing NULL.
	userEmail, err := pkg.GetUserEmailFromContext(ctx)
	if err != nil {
		return 0, err
	}

	res := r.DB(ctx).WithContext(ctx).Exec(
		buildReconciliationSnapshotsSQL,
		submissionID, userEmail, userEmail, inventoryID, models.InventoryItemStatusActive,
	)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

func (r *reconciliationSnapshotRepository) ExistsForSubmission(ctx context.Context, submissionID uint) (bool, error) {
	var count int64
	err := r.DB(ctx).WithContext(ctx).
		Model(&models.ReconciliationSnapshot{}).
		Where("submission_id = ?", submissionID).
		Limit(1).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
