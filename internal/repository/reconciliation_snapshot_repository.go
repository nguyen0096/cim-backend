package repository

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"context"
	"time"

	"github.com/shopspring/decimal"
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
	// GetPrevQuantitiesBySubmission returns the per-item baseline captured at
	// initiate as a map inventory_item_id -> prev_quantity, over the submission's
	// live (non-soft-deleted) snapshot rows. It is the sole source of truth for the
	// per-item "counted > snapshot" guard staff child-item writes are validated
	// against (epic #38, Part 4). Tx-aware via DB(ctx).
	GetPrevQuantitiesBySubmission(ctx context.Context, submissionID uint) (map[uint]decimal.Decimal, error)
	// GetSnapshotCapturedAt returns the instant the submission's baseline snapshot
	// was captured — i.e. MIN(created_at) over its live (non-soft-deleted) snapshot
	// rows. BuildReconciliationSnapshots stamps every row of one capture with
	// clock_timestamp() AFTER acquiring the per-inventory advisory lock, so this is
	// the authoritative post-lock capture time and the correct lower bound for the
	// Start-Processing drift window (epic #38, Part 6 redesign). It is strictly the
	// snapshot-capture moment, NOT the parent submission's created_at (which is
	// stamped earlier, before the lock, and would falsely flag a consuming apply
	// that committed before the snapshot read yet after the parent insert). Returns
	// ok=false when the submission has no live snapshot rows (a legacy reconcile, or
	// a baseline with no active items). Tx-aware via DB(ctx).
	GetSnapshotCapturedAt(ctx context.Context, submissionID uint) (capturedAt time.Time, ok bool, err error)
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
// prev_quantity. created_at/updated_at are stamped with clock_timestamp() and
// created_by/updated_by with the initiating user (the raw INSERT bypasses
// models.Base.BeforeCreate, so the audit columns are populated explicitly here to
// keep the captured baseline auditable). deleted_at is left NULL. The source rows
// are filtered to status='active' AND deleted_at IS NULL so the baseline matches
// the active-item set the legacy paths use.
//
// clock_timestamp() (NOT NOW()/transaction_timestamp()): InitiateReconcile opens
// its transaction, THEN waits on pg_advisory_xact_lock(inventory_id), and only
// THEN runs this INSERT. NOW() is fixed at transaction start, so it would stamp a
// time BEFORE the post-lock snapshot read — making MIN(created_at) (the
// Start-Processing drift window-start) earlier than the actual baseline read. A
// consuming apply that committed DURING the lock wait (already reflected in the
// post-lock baseline) would then have processed_at >= window-start and be flagged
// as false drift, blocking the reconciliation. clock_timestamp() advances within
// the transaction and returns the real wall-clock instant of this statement — the
// true post-lock capture time (epic #38, Part 6; Codex P2).
const buildReconciliationSnapshotsSQL = `
INSERT INTO reconciliation_snapshots
	(submission_id, inventory_item_id, prev_quantity, created_by, updated_by, created_at, updated_at)
SELECT ?, id, quantity, ?, ?, clock_timestamp(), clock_timestamp()
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

func (r *reconciliationSnapshotRepository) GetSnapshotCapturedAt(ctx context.Context, submissionID uint) (time.Time, bool, error) {
	// MIN(created_at) over the submission's live snapshot rows. The INSERT ... SELECT
	// stamps created_at = clock_timestamp() (the real post-lock statement instant);
	// rows of one capture differ only by sub-statement clock drift, so MIN is the
	// earliest, conservative capture instant and is robust even if a future change
	// ever re-captured. A nullable scan target distinguishes "no live snapshot rows"
	// (NULL) from a real timestamp.
	var capturedAt *time.Time
	err := r.DB(ctx).WithContext(ctx).
		Model(&models.ReconciliationSnapshot{}).
		Where("submission_id = ?", submissionID).
		Select("MIN(created_at)").
		Scan(&capturedAt).Error
	if err != nil {
		return time.Time{}, false, err
	}
	if capturedAt == nil {
		return time.Time{}, false, nil
	}
	return *capturedAt, true, nil
}

func (r *reconciliationSnapshotRepository) GetPrevQuantitiesBySubmission(ctx context.Context, submissionID uint) (map[uint]decimal.Decimal, error) {
	var rows []models.ReconciliationSnapshot
	err := r.DB(ctx).WithContext(ctx).
		Model(&models.ReconciliationSnapshot{}).
		Select("inventory_item_id", "prev_quantity").
		Where("submission_id = ?", submissionID).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uint]decimal.Decimal, len(rows))
	for _, row := range rows {
		out[row.InventoryItemID] = row.PrevQuantity
	}
	return out, nil
}
