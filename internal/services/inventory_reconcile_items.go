package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// This file implements the STAFF reconciliation child-item lifecycle (epic #38,
// Part 4): create / update / mark ready / mark not_ready / soft-delete of
// reconciliation_request_items under a parent initiated-reconcile submission,
// plus the per-row state machine and ownership/status guards.
//
// Locked rules (from issue #38):
//   - State machine: in_progress -> ready -> approved -> applied.
//     Staff transitions in scope here: in_progress <-> ready, and the escape
//     hatch approved -> in_progress triggered automatically when a staff member
//     edits the payload of an approved row. (admin ready->approved is Part 6;
//     approved->applied is Part 7 — NOT implemented here, but the state model is
//     built so those parts can layer on cleanly.)
//   - Ownership: a caller may only update / ready / delete rows they created
//     (models.Base.CreatedBy == caller email).
//   - Status guards: an `applied` row is immutable; only in_progress/ready rows
//     may be soft-deleted.
//   - Per-item validation: counted quantity must be >= 0 and must NOT exceed the
//     snapshot baseline for that item (the S2 "counted > snapshot is rejected"
//     rule). The baseline is the parent snapshot captured at initiate.

// loadActiveReconcileParent loads the parent submission for a child-item op and
// confirms it is a reconciliation started via the initiate endpoint (i.e. it has
// snapshot rows). Child items only ever belong to that flow; rejecting otherwise
// keeps a legacy single-payload reconcile (or a dispose/transfer) from acquiring
// child rows.
//
// The parent is loaded with a row-level write lock (SELECT ... FOR UPDATE) and
// the pending assertion below runs under that lock. Every caller invokes this
// inside the same WithinTx as the subsequent child write, so the lock is held
// until that tx commits. This closes the terminal-parent RACE: a concurrent
// ProcessSubmission reject/cancel of the same parent either (a) has already
// committed, in which case this lock acquires and reads the terminal status and
// we reject, or (b) is still in flight, in which case its approval-status UPDATE
// blocks on our row lock until our child write commits, then sees a child it can
// no longer prevent — but our tx, having read pending, is the one that wins, and
// any later child op re-locks and sees the now-terminal parent. No deadlock:
// this tx locks only the single parent submission row before touching child rows
// (a different table), and ProcessSubmission's reject path only ever touches the
// same parent row via standalone UPDATEs, so there is no cyclic lock ordering.
func (s *inventoryService) loadActiveReconcileParent(ctx context.Context, submissionID uint) (*models.InventorySubmission, error) {
	submission, err := s.inventorySubmissionRepo.GetByIDForUpdate(ctx, submissionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.ErrReconParentNotFound(ctx, submissionID)
		}
		return nil, fmt.Errorf("failed to load reconciliation submission: %w", err)
	}

	if submission.SubmissionType != models.InventorySubmissionTypeReconcile {
		return nil, pkg.ErrReconParentNotInitiated(ctx, submissionID)
	}
	hasSnapshots, err := s.snapshotRepo.ExistsForSubmission(ctx, submissionID)
	if err != nil {
		return nil, fmt.Errorf("failed to check reconciliation snapshots: %w", err)
	}
	if !hasSnapshots {
		return nil, pkg.ErrReconParentNotInitiated(ctx, submissionID)
	}

	// The parent must still be in-flight (approval pending). An initiated reconcile
	// stays at approval_status=pending until the Part 6/7 synthesize→approve→apply
	// path, but it can already be REJECTED today via ProcessSubmission (which sets
	// approval_status=rejected + processing_status=canceled while leaving the
	// snapshot rows in place). Without this guard, staff could keep creating/
	// editing/deleting child items under a closed/terminal parent. Any non-pending
	// approval status (rejected, or a future approved/applied) closes the window.
	// This check is evaluated while holding the FOR UPDATE lock acquired above and
	// in the same tx as the child write, so a concurrent terminal transition is
	// serialized against it rather than slipping through a stale read.
	if submission.ApprovalStatus != models.InventorySubmissionApprovalStatusPending {
		return nil, pkg.ErrReconParentNotInFlight(ctx, submissionID, string(submission.ApprovalStatus))
	}
	return submission, nil
}

// loadOwnedItemInParent loads a child item, then enforces (in order): it belongs
// to the path-scoped parent, the caller owns it, and it is not applied
// (immutable). Returns the loaded row for the caller's status-transition logic.
func (s *inventoryService) loadOwnedItemInParent(ctx context.Context, submissionID, itemID uint) (*models.ReconciliationRequestItem, error) {
	item, err := s.reconItemRepo.GetByID(ctx, itemID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.ErrReconItemNotFound(ctx, itemID)
		}
		return nil, fmt.Errorf("failed to load reconciliation item: %w", err)
	}

	// The item must live under the path-scoped parent; a mismatch is treated as
	// not-found-in-parent so a caller cannot probe items across submissions.
	if item.SubmissionID != submissionID {
		return nil, pkg.ErrReconItemNotInParent(ctx, itemID, submissionID)
	}

	if err := s.guardOwnership(ctx, item); err != nil {
		return nil, err
	}

	// Applied rows are terminal/immutable — never editable by staff or admin.
	if item.Status == models.ReconciliationRequestItemStatusApplied {
		return nil, pkg.ErrReconItemImmutable(ctx, itemID)
	}
	return item, nil
}

// guardOwnership enforces that the caller created the row. Ownership is keyed on
// models.Base.CreatedBy (the contributor's email), matching how every other
// table records its actor. Applies to all callers (staff and admin/accountant):
// Part 4 is the staff self-service lifecycle; admin approval of others' rows is
// Part 6.
func (s *inventoryService) guardOwnership(ctx context.Context, item *models.ReconciliationRequestItem) error {
	userEmail, err := pkg.GetUserEmailFromContext(ctx)
	if err != nil {
		return pkg.ErrUnauthorized("user not authenticated", err)
	}
	if item.CreatedBy != userEmail {
		return pkg.ErrReconItemNotOwned(ctx)
	}
	return nil
}

// validateCountsAgainstSnapshot enforces the per-item validation for a child
// payload: non-negative, no duplicate lines, every counted item has a snapshot
// baseline, and — the data-correctness guard — the AGGREGATE counted quantity per
// item across ALL live (non-deleted) sibling child rows of the same parent plus
// this payload must not exceed the snapshot baseline (S2: counted > snapshot is
// rejected; there is no positive-adjustment mechanism in this epic). The snapshot
// baseline is read from the parent's snapshot rows (the sole source of truth for
// prev_quantity).
//
// Why the aggregate (not just per-row): multiple live child rows can count the
// SAME inventory_item_id and are summed by item at synthesis. A per-row-only check
// lets two rows of 80 against a baseline of 100 both pass yet sum to 160,
// corrupting/blocking the synthesized reconcile. So for each incoming item we add
// this row's count to SUM(counted across the OTHER live child rows for the same
// parent+item) and reject if the total exceeds the baseline.
//
// excludeItemID is the reconciliation_request_item row id being replaced on UPDATE
// (its current persisted counts are being overwritten by this payload, so they
// must NOT be double-counted in the sibling sum); pass 0 on CREATE (no row to
// exclude).
//
// Race-safety: this runs inside the same WithinTx as the child write, under the
// parent submission's FOR UPDATE lock (acquired by loadActiveReconcileParent).
// All child writes for a parent serialize on that lock, so the sibling rows read
// here (via DB(ctx), enlisted in this tx) cannot change underneath us — two
// concurrent staff submissions for the same parent are ordered, and the second
// sees the first's committed row in its sum.
//
// Returns the normalized counted payload bytes to persist.
func (s *inventoryService) validateCountsAgainstSnapshot(ctx context.Context, submissionID uint, items []dto.ReconciliationCountItem, excludeItemID uint) (json.RawMessage, error) {
	baselines, err := s.snapshotRepo.GetPrevQuantitiesBySubmission(ctx, submissionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load snapshot baselines: %w", err)
	}

	// Sum counted quantities, per inventory_item_id, across all OTHER live child
	// rows of this parent (excluding the row being updated). Read under the parent
	// FOR UPDATE lock in this tx; siblings are stable for the duration.
	siblingTotals, err := s.sumLiveSiblingCounts(ctx, submissionID, excludeItemID)
	if err != nil {
		return nil, err
	}

	seen := make(map[uint]struct{}, len(items))
	for _, item := range items {
		if _, dup := seen[item.InventoryItemID]; dup {
			return nil, pkg.ErrReconItemDuplicateLine(ctx, item.InventoryItemID)
		}
		seen[item.InventoryItemID] = struct{}{}

		// Quantity is a pointer: nil means the field was omitted entirely, which is
		// rejected here so a malformed payload can never be silently read as a
		// full-shrinkage zero count. An explicit 0 (non-nil) is a valid count.
		if item.Quantity == nil {
			return nil, pkg.ErrReconItemMissingQuantity(ctx, item.InventoryItemID)
		}
		quantity := *item.Quantity

		if quantity.IsNegative() {
			return nil, pkg.ErrReconItemNegativeQuantity(ctx, item.InventoryItemID)
		}

		baseline, ok := baselines[item.InventoryItemID]
		if !ok {
			return nil, pkg.ErrReconItemNoSnapshotBaseline(ctx, item.InventoryItemID)
		}
		// Per-row guard first (clearer single-row message when this row alone exceeds).
		if quantity.GreaterThan(baseline) {
			return nil, pkg.ErrReconItemCountExceedsBaseline(ctx, item.InventoryItemID, quantity, baseline)
		}
		// Aggregate guard: this row + every other live sibling row for the same item.
		total := siblingTotals[item.InventoryItemID].Add(quantity)
		if total.GreaterThan(baseline) {
			return nil, pkg.ErrReconItemAggregateExceedsBaseline(ctx, item.InventoryItemID, total, baseline)
		}
	}

	// Persist in the legacy reconcile payload shape (counts only) so
	// SynthesizeSubmissionPayload (Part 5) can sum child payloads by item id.
	payload := buildReconItemPayload(items)
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reconciliation item payload: %w", err)
	}
	return bytes, nil
}

// sumLiveSiblingCounts returns, per inventory_item_id, the total counted quantity
// across all live (non-soft-deleted) child rows of the parent submission, EXCLUDING
// the row with id excludeItemID (0 = exclude nothing). It reads via DB(ctx), so when
// called inside a child-write WithinTx it enlists in that tx and reads under the
// parent FOR UPDATE lock — making the aggregate check race-safe against concurrent
// staff submissions for the same parent.
func (s *inventoryService) sumLiveSiblingCounts(ctx context.Context, submissionID, excludeItemID uint) (map[uint]decimal.Decimal, error) {
	rows, err := s.reconItemRepo.ListBySubmission(ctx, submissionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load sibling reconciliation items: %w", err)
	}

	totals := make(map[uint]decimal.Decimal)
	for _, row := range rows {
		if row.ID == excludeItemID {
			continue
		}
		if len(row.Payload) == 0 {
			continue
		}
		var parsed reconItemPayload
		if err := json.Unmarshal(row.Payload, &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse sibling reconciliation item %d payload: %w", row.ID, err)
		}
		for _, line := range parsed.Items {
			cur, ok := totals[line.InventoryItemID]
			if !ok {
				cur = decimal.Zero
			}
			totals[line.InventoryItemID] = cur.Add(line.Quantity)
		}
	}
	return totals, nil
}

// reconItemPayload is the on-row JSON shape for a child item: COUNTS only, in the
// legacy reconcile {"items":[{"inventory_item_id","quantity"}]} structure so the
// later synthesize step can read every child row uniformly.
type reconItemPayload struct {
	Items []reconItemPayloadLine `json:"items"`
}

type reconItemPayloadLine struct {
	InventoryItemID uint            `json:"inventory_item_id"`
	Quantity        decimal.Decimal `json:"quantity"`
}

// buildReconItemPayload assumes every line has already passed
// validateCountsAgainstSnapshot (which rejects a nil Quantity), so the pointer is
// safe to dereference; a nil that somehow reaches here is normalized to zero
// rather than panicking.
func buildReconItemPayload(items []dto.ReconciliationCountItem) reconItemPayload {
	lines := make([]reconItemPayloadLine, 0, len(items))
	for _, item := range items {
		quantity := decimal.Zero
		if item.Quantity != nil {
			quantity = *item.Quantity
		}
		lines = append(lines, reconItemPayloadLine{
			InventoryItemID: item.InventoryItemID,
			Quantity:        quantity,
		})
	}
	return reconItemPayload{Items: lines}
}

// CreateReconciliationItem files a new staff child item under a parent initiated
// reconcile. The new row starts in_progress and is owned by the caller. Counts
// are validated against the snapshot baseline. Parent load + create run in one
// tx so a parent that disappears (or loses its snapshots) mid-create can't leave
// an orphan row.
func (s *inventoryService) CreateReconciliationItem(ctx context.Context, req dto.CreateReconciliationItemRequest) (*models.ReconciliationRequestItem, error) {
	var created *models.ReconciliationRequestItem
	err := s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		if _, err := s.loadActiveReconcileParent(txCtx, req.SubmissionID); err != nil {
			return err
		}

		// excludeItemID = 0 on create: there is no existing row to exclude from the
		// sibling aggregate; this new row's counts are the payload being validated.
		payloadBytes, err := s.validateCountsAgainstSnapshot(txCtx, req.SubmissionID, req.Items, 0)
		if err != nil {
			return err
		}

		item := &models.ReconciliationRequestItem{
			SubmissionID: req.SubmissionID,
			Payload:      payloadBytes,
			Status:       models.ReconciliationRequestItemStatusInProgress,
		}
		if err := s.reconItemRepo.Create(txCtx, item); err != nil {
			return fmt.Errorf("failed to create reconciliation item: %w", err)
		}
		created = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// UpdateReconciliationItem replaces the counted payload of an owned child item.
// Editing a `ready` or `approved` row resets it to `in_progress` (the approved ->
// in_progress escape hatch); an `in_progress` row stays in_progress; an `applied`
// row is rejected as immutable (guarded in loadOwnedItemInParent). The whole op
// is one tx (load + validate + write).
func (s *inventoryService) UpdateReconciliationItem(ctx context.Context, req dto.UpdateReconciliationItemRequest) (*models.ReconciliationRequestItem, error) {
	var updated *models.ReconciliationRequestItem
	err := s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		if _, err := s.loadActiveReconcileParent(txCtx, req.SubmissionID); err != nil {
			return err
		}
		item, err := s.loadOwnedItemInParent(txCtx, req.SubmissionID, req.ItemID)
		if err != nil {
			return err
		}

		// Exclude this row from the sibling aggregate: its current persisted counts
		// are being REPLACED by req.Items, so the new payload — not the old one — is
		// what counts toward the per-item baseline.
		payloadBytes, err := s.validateCountsAgainstSnapshot(txCtx, req.SubmissionID, req.Items, item.ID)
		if err != nil {
			return err
		}

		// Any payload edit drops the row back to in_progress: a ready row is no
		// longer "ready for review" once its counts changed, and an approved row
		// must be re-reviewed (the escape hatch). in_progress stays in_progress.
		newStatus := models.ReconciliationRequestItemStatusInProgress
		if err := s.reconItemRepo.UpdatePayloadAndStatus(txCtx, item.ID, payloadBytes, newStatus); err != nil {
			return fmt.Errorf("failed to update reconciliation item: %w", err)
		}
		item.Payload = payloadBytes
		item.Status = newStatus
		updated = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// SetReconciliationItemReady transitions an owned child item between in_progress
// and ready (status-only). Ready=true requires in_progress -> ready; Ready=false
// requires ready -> in_progress. Any other source status is an illegal transition
// (e.g. you cannot mark an approved/applied row ready here).
func (s *inventoryService) SetReconciliationItemReady(ctx context.Context, req dto.SetReconciliationItemReadyRequest) (*models.ReconciliationRequestItem, error) {
	var updated *models.ReconciliationRequestItem
	err := s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		if _, err := s.loadActiveReconcileParent(txCtx, req.SubmissionID); err != nil {
			return err
		}
		item, err := s.loadOwnedItemInParent(txCtx, req.SubmissionID, req.ItemID)
		if err != nil {
			return err
		}

		var target models.ReconciliationRequestItemStatus
		if req.Ready {
			if item.Status != models.ReconciliationRequestItemStatusInProgress {
				return pkg.ErrReconItemInvalidTransition(ctx, string(item.Status), string(models.ReconciliationRequestItemStatusReady))
			}
			target = models.ReconciliationRequestItemStatusReady
		} else {
			if item.Status != models.ReconciliationRequestItemStatusReady {
				return pkg.ErrReconItemInvalidTransition(ctx, string(item.Status), string(models.ReconciliationRequestItemStatusInProgress))
			}
			target = models.ReconciliationRequestItemStatusInProgress
		}

		if err := s.reconItemRepo.UpdateStatus(txCtx, item.ID, target); err != nil {
			return fmt.Errorf("failed to update reconciliation item status: %w", err)
		}
		item.Status = target
		updated = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// DeleteReconciliationItem soft-deletes an owned child item. Only in_progress or
// ready rows may be deleted; approved and applied rows are preserved (an approved
// row's escape hatch is to edit it back to in_progress, not delete it, which
// keeps the audit intent — locked decision #3).
func (s *inventoryService) DeleteReconciliationItem(ctx context.Context, req dto.DeleteReconciliationItemRequest) error {
	return s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		if _, err := s.loadActiveReconcileParent(txCtx, req.SubmissionID); err != nil {
			return err
		}
		item, err := s.loadOwnedItemInParent(txCtx, req.SubmissionID, req.ItemID)
		if err != nil {
			return err
		}

		switch item.Status {
		case models.ReconciliationRequestItemStatusInProgress,
			models.ReconciliationRequestItemStatusReady:
			// deletable
		default:
			return pkg.ErrReconItemCannotDeleteStatus(ctx, item.ID, string(item.Status))
		}

		if err := s.reconItemRepo.SoftDelete(txCtx, item.ID); err != nil {
			return fmt.Errorf("failed to soft-delete reconciliation item: %w", err)
		}
		return nil
	})
}
