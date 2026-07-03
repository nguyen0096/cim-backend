package repository

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// ReconciliationSnapshotRepository persists the per-item baseline quantities
// captured when a reconciliation is initiated.
//
//go:generate mockery --name=ReconciliationSnapshotRepository --structname=ReconciliationSnapshotRepository --output=../mocks/repositorymocks --outpkg=repositorymocks
type ReconciliationSnapshotRepository interface {
	// BuildReconciliationSnapshots captures the per-item baseline for a
	// reconciliation and returns the number of snapshot rows inserted.
	BuildReconciliationSnapshots(ctx context.Context, submissionID, inventoryID uint) (int64, error)
	// ExistsForSubmission reports whether the submission has any snapshot row.
	ExistsForSubmission(ctx context.Context, submissionID uint) (bool, error)
	// GetPrevQuantitiesBySubmission returns the per-item baseline as
	// inventory_item_id -> prev_quantity.
	GetPrevQuantitiesBySubmission(ctx context.Context, submissionID uint) (map[uint]decimal.Decimal, error)
	// GetSnapshotCapturedAt returns the instant the submission's baseline snapshot
	// was captured, or ok=false when it has no live snapshot rows.
	GetSnapshotCapturedAt(ctx context.Context, submissionID uint) (capturedAt time.Time, ok bool, err error)
}

type reconciliationSnapshotRepository struct {
	*baseRepository
}

// NewReconciliationSnapshotRepository creates a new reconciliation snapshot repository.
func NewReconciliationSnapshotRepository(base BaseRepository) ReconciliationSnapshotRepository {
	return &reconciliationSnapshotRepository{baseRepository: asBase(base)}
}

// buildReconciliationSnapshotsSQL inserts one snapshot per active item, copying
// the item's live quantity into prev_quantity. clock_timestamp() (not NOW()) stamps
// the real post-lock capture instant.
const buildReconciliationSnapshotsSQL = `
INSERT INTO reconciliation_snapshots
	(submission_id, inventory_item_id, prev_quantity, created_by, updated_by, created_at, updated_at)
SELECT ?, id, quantity, ?, ?, clock_timestamp(), clock_timestamp()
FROM inventory_items
WHERE inventory_id = ?
  AND status = ?
  AND deleted_at IS NULL`

func (r *reconciliationSnapshotRepository) BuildReconciliationSnapshots(ctx context.Context, submissionID, inventoryID uint) (int64, error) {
	// Raw INSERT bypasses models.Base.BeforeCreate; set audit columns explicitly.
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
	// Nullable target distinguishes "no live snapshot rows" from a real timestamp.
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
