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
)

// errReconDriftRollback is an internal sentinel: StartProcessing returns it from
// inside WithinTx to force the apply transaction to ROLL BACK (no inventory
// mutated) when the drift re-check fires, while still handing the warning-shaped
// result back to the handler. It never escapes the service.
var errReconDriftRollback = errors.New("reconciliation drift detected; rolling back")

// This file implements the ADMIN/accountant reconciliation MANAGEMENT flow (epic
// #38, Part 6 redesign — folds the former Part 6 "approve gate" + Part 7
// "apply"). The per-row approve gate is gone; the submission-level lifecycle is:
//
//	open       staff freely edit child count rows
//	  -close-> closed      staff LOCKED; admin/accountant may still edit; admin reviews
//	  -reopen-> open        (Q4: reopen allowed)
//	  -start-> processing -> processed   ONE atomic apply tx (advisory-locked,
//	                                     event-based drift re-check, snapshot-aware
//	                                     consume), creating the real consuming
//	                                     inventory transactions (irreversible).
//
// All three endpoints require recon_manage (admin + accountant only, Q5). The
// service re-asserts the permission as the correctness backstop, mirroring
// ProcessSubmission.

// CloseReconciliation moves an initiated reconcile open -> closed (admin/
// accountant), in one tx under the parent FOR UPDATE lock. The close always
// succeeds; the result carries an advisory Warnings list naming any live session
// still in_progress at close time (warn, not block).
func (s *inventoryService) CloseReconciliation(ctx context.Context, submissionID uint) (*dto.CloseReconciliationResult, error) {
	if !pkg.HasPermission(ctx, pkg.RBACResourceInventorySubmissions, pkg.RBACActionReconManage) {
		return nil, pkg.ErrForbidden("user does not have permission to manage reconciliations", nil)
	}

	var result *dto.CloseReconciliationResult
	err := s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		parent, err := s.loadActiveReconcileParent(txCtx, submissionID)
		if err != nil {
			return err
		}
		if parent.ReconcileStatus != models.ReconcileLifecycleStatusOpen {
			return pkg.ErrReconInvalidLifecycleTransition(txCtx, submissionID,
				string(parent.ReconcileStatus), string(models.ReconcileLifecycleStatusClosed))
		}

		// Read sessions under the lock so the warning is consistent with the close.
		rows, err := s.reconItemRepo.ListBySubmission(txCtx, submissionID)
		if err != nil {
			return fmt.Errorf("failed to load reconciliation sessions: %w", err)
		}

		if err := s.inventorySubmissionRepo.UpdateReconcileStatus(txCtx, submissionID, models.ReconcileLifecycleStatusClosed); err != nil {
			return fmt.Errorf("failed to update reconcile status: %w", err)
		}
		parent.ReconcileStatus = models.ReconcileLifecycleStatusClosed
		managerOwned, err := s.managerOwnedSessions(txCtx, rows)
		if err != nil {
			return err
		}
		result = &dto.CloseReconciliationResult{
			Submission: parent,
			Warnings:   buildNotReadySessionWarnings(txCtx, rows, managerOwned),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// buildNotReadySessionWarnings returns one advisory line per live STAFF session
// that is not yet ready for review, or nil when all are ready. Manager-owned
// sessions (managerOwned[row.ID]) are excluded: readiness is a staff concept and
// managers cannot toggle, so a manager session must never trigger the warning.
func buildNotReadySessionWarnings(ctx context.Context, rows []models.ReconciliationRequestItem, managerOwned map[uint]bool) []string {
	warnings := make([]string, 0)
	for i := range rows {
		row := rows[i]
		if managerOwned[row.ID] {
			continue
		}
		if row.Status != models.ReconciliationRequestItemStatusReadyForReview {
			warnings = append(warnings, pkg.ReconNotReadySessionsWarning(ctx, row.ID, row.Label, row.CreatedBy))
		}
	}
	if len(warnings) == 0 {
		return nil
	}
	return warnings
}

// managerOwnedSessions returns the set of row IDs whose creator holds recon_manage
// (admin/accountant). It batches the distinct creator emails into one role lookup
// and evaluates each distinct role against the policy once, so it adds at most one
// query plus a handful of enforce checks regardless of row count.
func (s *inventoryService) managerOwnedSessions(ctx context.Context, rows []models.ReconciliationRequestItem) (map[uint]bool, error) {
	managerOwned := make(map[uint]bool, len(rows))
	if len(rows) == 0 {
		return managerOwned, nil
	}

	emailSet := make(map[string]struct{}, len(rows))
	for i := range rows {
		if e := rows[i].CreatedBy; e != "" {
			emailSet[e] = struct{}{}
		}
	}
	emails := make([]string, 0, len(emailSet))
	for e := range emailSet {
		emails = append(emails, e)
	}

	roleByEmail, err := s.userRepo.GetRolesByEmails(ctx, emails)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve session creator roles: %w", err)
	}

	manageByRole := make(map[models.UserRole]bool, len(roleByEmail))
	roleManages := func(role models.UserRole) (bool, error) {
		if v, ok := manageByRole[role]; ok {
			return v, nil
		}
		ok, err := s.casbinService.Enforce(string(role), pkg.RBACResourceInventorySubmissions, pkg.RBACActionReconManage)
		if err != nil {
			return false, fmt.Errorf("failed to evaluate recon_manage for role %q: %w", role, err)
		}
		manageByRole[role] = ok
		return ok, nil
	}

	for i := range rows {
		role, ok := roleByEmail[rows[i].CreatedBy]
		if !ok {
			continue
		}
		manages, err := roleManages(role)
		if err != nil {
			return nil, err
		}
		if manages {
			managerOwned[rows[i].ID] = true
		}
	}
	return managerOwned, nil
}

// ReopenReconciliation moves an initiated reconcile closed -> open (admin/
// accountant, Q4). Staff regain edit access. Runs in one tx under the parent FOR
// UPDATE lock.
func (s *inventoryService) ReopenReconciliation(ctx context.Context, submissionID uint) (*models.InventorySubmission, error) {
	return s.transitionReconcileStatus(ctx, submissionID,
		models.ReconcileLifecycleStatusClosed, models.ReconcileLifecycleStatusOpen)
}

// transitionReconcileStatus backs Reopen: permission check, then one tx (parent
// FOR UPDATE) asserting the expected source lifecycle status and writing the
// target. The parent must still be approval-pending (in-flight) —
// loadActiveReconcileParent enforces that. processing/processed are terminal and
// never a valid source.
func (s *inventoryService) transitionReconcileStatus(
	ctx context.Context,
	submissionID uint,
	from, to models.ReconcileLifecycleStatus,
) (*models.InventorySubmission, error) {
	if !pkg.HasPermission(ctx, pkg.RBACResourceInventorySubmissions, pkg.RBACActionReconManage) {
		return nil, pkg.ErrForbidden("user does not have permission to manage reconciliations", nil)
	}

	var result *models.InventorySubmission
	err := s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		parent, err := s.loadActiveReconcileParent(txCtx, submissionID)
		if err != nil {
			return err
		}
		if parent.ReconcileStatus != from {
			return pkg.ErrReconInvalidLifecycleTransition(txCtx, submissionID, string(parent.ReconcileStatus), string(to))
		}
		if err := s.inventorySubmissionRepo.UpdateReconcileStatus(txCtx, submissionID, to); err != nil {
			return fmt.Errorf("failed to update reconcile status: %w", err)
		}
		parent.ReconcileStatus = to
		result = parent
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// StartProcessing is the ONE atomic apply transaction (locked decisions Q5–Q8).
// It takes the per-inventory advisory lock FIRST, then performs the event-based
// drift re-check, then (if clean) synthesizes the payload, applies it with
// snapshot-aware consume sizing (snapshot − counted, Reading B), stamps
// processed_at, and moves the submission to processed — all in the one tx. On
// drift it ROLLS BACK and returns a warning-shaped result (no inventory mutated).
//
// Why the lock + re-check are authoritative (closes the TOCTOU the review
// raised): pg_advisory_xact_lock(inventory_id) is taken by BOTH this path and the
// general consuming ProcessSubmission path before either reads/writes stock. So a
// concurrent consuming apply for this inventory cannot interleave: either it
// committed before us — and its processed_at sits in our window, so our re-check
// sees it and we roll back — or it is blocked on the lock until we commit. There
// is no window in which a consuming sibling commits between our re-check and our
// apply.
func (s *inventoryService) StartProcessing(ctx context.Context, submissionID uint) (*dto.StartProcessingResult, error) {
	if !pkg.HasPermission(ctx, pkg.RBACResourceInventorySubmissions, pkg.RBACActionReconManage) {
		return nil, pkg.ErrForbidden("user does not have permission to manage reconciliations", nil)
	}

	var result *dto.StartProcessingResult
	err := s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		// Step 0: parent lock + lifecycle assertion. (loadActiveReconcileParent also
		// confirms it is an initiated reconcile that is still approval-pending.)
		parent, err := s.loadActiveReconcileParent(txCtx, submissionID)
		if err != nil {
			return err
		}
		if parent.ReconcileStatus != models.ReconcileLifecycleStatusClosed {
			return pkg.ErrReconInvalidLifecycleTransition(txCtx, submissionID,
				string(parent.ReconcileStatus), string(models.ReconcileLifecycleStatusProcessing))
		}

		// Step 1: advisory lock FIRST (per-inventory serialization, Q7).
		if err := s.inventorySubmissionRepo.AcquireInventoryAdvisoryLock(txCtx, parent.InventoryID); err != nil {
			return fmt.Errorf("failed to acquire inventory advisory lock: %w", err)
		}

		// Step 2: event-based drift re-check (Q5/Q8) under the lock. The window opens
		// at the ACTUAL snapshot-capture instant and runs to now.
		//
		// It is NOT parent.CreatedAt: the parent row is inserted FIRST in InitiateReconcile,
		// then the per-inventory advisory lock is acquired, and only THEN are the snapshots
		// captured (BuildReconciliationSnapshots stamps created_at = clock_timestamp() under the lock).
		// A consuming apply can stamp its processed_at after parent.CreatedAt yet COMMIT
		// before the snapshot read — its stock effect is then already in the baseline, so
		// counting it as drift is a FALSE positive that forces a needless discard/recreate.
		// Using the snapshot-capture time as the lower bound makes the invariant exact: a
		// consuming apply already reflected in the baseline (committed before capture) is
		// NOT flagged, and one committing after capture IS.
		capturedAt, ok, err := s.snapshotRepo.GetSnapshotCapturedAt(txCtx, parent.ID)
		if err != nil {
			return fmt.Errorf("failed to load reconciliation snapshot capture time: %w", err)
		}
		if !ok {
			// No live snapshot rows: not an initiated reconcile this flow can process.
			// loadActiveReconcileParent already asserts the lifecycle, so this is a
			// defensive guard; fall back to parent.CreatedAt so the re-check stays a
			// (conservative, never-too-narrow) superset window rather than panicking.
			capturedAt = parent.CreatedAt
		}
		drifted, err := s.inventorySubmissionRepo.ListConsumingProcessedSince(
			txCtx, parent.InventoryID, parent.ID, capturedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to evaluate reconciliation drift: %w", err)
		}
		if len(drifted) > 0 {
			// Roll back the whole tx (no apply) and surface the warning-shaped
			// payload so the admin can edit or reopen, then restart processing. We
			// return a sentinel so the WithinTx rolls back while still letting us hand
			// the result back to the handler (the sentinel is swallowed below).
			result = &dto.StartProcessingResult{
				DriftDetected: true,
				Warnings:      buildDriftWarnings(txCtx, drifted),
			}
			return errReconDriftRollback
		}

		// Step 3: synthesize the finalized payload from the live child rows.
		syn, err := s.SynthesizeSubmissionPayload(txCtx, submissionID)
		if err != nil {
			return fmt.Errorf("failed to synthesize reconciliation payload: %w", err)
		}

		// Step 4: apply with snapshot-aware consume sizing (snapshot − counted).
		if err := s.applyReconcileSnapshotAware(txCtx, parent.InventoryID, syn); err != nil {
			return err
		}

		// Step 5: persist the finalized payload + finalize statuses (processed_at,
		// approval_status=approved, processing_status=completed, reconcile=processed).
		payloadBytes, err := json.Marshal(syn.Request)
		if err != nil {
			return fmt.Errorf("failed to marshal synthesized payload: %w", err)
		}
		if err := s.inventorySubmissionRepo.UpdateSubmissionPayload(txCtx, submissionID, payloadBytes); err != nil {
			return fmt.Errorf("failed to persist synthesized payload: %w", err)
		}
		processedAt, err := s.inventorySubmissionRepo.MarkProcessed(txCtx, submissionID, models.ReconcileLifecycleStatusProcessed)
		if err != nil {
			return fmt.Errorf("failed to finalize reconciliation processing: %w", err)
		}

		// Mirror EVERY field MarkProcessed/UpdateSubmissionPayload persisted onto the
		// returned struct so the response reflects the committed row (epic #38, Part 6).
		// MarkProcessed also clears `error` to NULL, so clear it here too — `parent` is
		// read fresh under FOR UPDATE in this tx and an initiated reconcile never carries
		// a stale error today, but mirroring it keeps the response authoritative if that
		// ever changes (no field left echoing pre-tx state).
		parent.Payload = payloadBytes
		parent.ApprovalStatus = models.InventorySubmissionApprovalStatusApproved
		parent.ProcessingStatus = models.InventorySubmissionStatusCompleted
		parent.ReconcileStatus = models.ReconcileLifecycleStatusProcessed
		parent.ProcessedAt = &processedAt
		parent.Error = nil
		result = &dto.StartProcessingResult{Submission: parent}
		return nil
	})
	// errReconDriftRollback is our internal sentinel to force a rollback while
	// returning the warning payload as a successful (200/409-shaped) result, not an
	// error. Any other error propagates.
	if err != nil && err != errReconDriftRollback {
		return nil, err
	}
	return result, nil
}

// applyReconcileSnapshotAware applies the synthesized reconcile with Reading-B
// sizing: each consume = snapshot (PrevQuantity) − counted (Quantity), consumed
// FIFO against LIVE stock via the shared consumeFIFO + SaveInventoryItemChanges.
// This is the snapshot-aware replacement for the legacy reconcileInventory's
// `live − counted` sizing + `live == prev` optimistic lock (both of which would
// be wrong here: a PO received during the count legitimately raised live above
// snapshot, and must SURVIVE on top of the counted figure). It runs inside the
// caller's StartProcessing tx (advisory lock + the FOR UPDATE taken by
// SaveInventoryItemChanges), so the apply is atomic and serialized.
//
// Synthesis already capped counted at the snapshot baseline (and rejects counts
// over snapshot at write time, S2), so snapshot − counted is always >= 0; and
// since snapshot <= live (PO receipts only add), consume <= live, so consumeFIFO's
// safety bound is never exceeded.
func (s *inventoryService) applyReconcileSnapshotAware(
	ctx context.Context,
	inventoryID uint,
	syn *dto.SynthesizedReconcile,
) error {
	itemIDs := models.GetIDs(syn.Request.Items)
	if len(itemIDs) == 0 {
		// Nothing counted — no consume to apply. (An initiated reconcile with no
		// child rows; the lifecycle still completes.)
		return nil
	}

	activeItems, err := s.getActiveInventoryItems(ctx, inventoryID, itemIDs)
	if err != nil {
		return fmt.Errorf("failed to load active inventory items for reconcile apply: %w", err)
	}
	activeItemMap := s.buildItemMap(activeItems)

	itemConsumeQuantity := make(map[uint]decimal.Decimal)
	for _, line := range syn.Request.Items {
		if line.Quantity == nil {
			return pkg.ErrInvalidRequestBody(fmt.Errorf("counted quantity missing for inventory item %d", line.InventoryItemID))
		}
		if _, exists := activeItemMap[line.InventoryItemID]; !exists {
			return pkg.ErrInventoryItemNotFound(ctx, line.InventoryItemID)
		}
		// Reading B: consume = snapshot − counted.
		consume := line.PrevQuantity.Sub(*line.Quantity)
		if consume.IsNegative() {
			// Defensive: synthesis caps counted at snapshot, so this is unreachable;
			// clamp to zero rather than attempt a negative (stock-increasing) consume,
			// which has no mechanism in this epic (S2).
			consume = decimal.Zero
		}
		itemConsumeQuantity[line.InventoryItemID] = consume
	}

	ps := newProcessingState(s, &models.InventorySubmission{InventoryID: inventoryID})

	consumeHandler := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity decimal.Decimal) []*models.InventoryTransaction {
		return []*models.InventoryTransaction{
			{
				InventoryItemID:      item.ID,
				TransactionType:      models.InventoryTransactionTypeSell,
				Price:                consumeTxn.Price,
				Quantity:             quantity,
				CounterTransactionID: &consumeTxn.ID,
			},
		}
	}

	ivtrItemChanges, txns, err := s.consumeFIFO(ctx, ps, activeItems, itemConsumeQuantity, consumeHandler)
	if err != nil {
		return fmt.Errorf("failed to consume FIFO for reconcile apply: %w", err)
	}
	if err := s.inventoryItemRepo.SaveInventoryItemChanges(ctx, ivtrItemChanges, txns); err != nil {
		return fmt.Errorf("failed to save inventory item changes for reconcile apply: %w", err)
	}
	return nil
}

// buildDriftWarnings renders the offending consuming siblings into the warning-
// shaped payload the admin sees (Q8: same shape as the existing reconcile
// warning) — one localized line per invalid consuming submission, enumerating its
// id, type, and processed-at so the UI shows "these consuming actions happened
// during your count."
func buildDriftWarnings(ctx context.Context, drifted []models.InventorySubmission) []string {
	warnings := make([]string, 0, len(drifted))
	for i := range drifted {
		d := drifted[i]
		processedAt := ""
		if d.ProcessedAt != nil {
			processedAt = d.ProcessedAt.Format("2006-01-02 15:04:05")
		}
		warnings = append(warnings, pkg.ReconDriftWarning(ctx, d.ID, string(d.SubmissionType), processedAt))
	}
	return warnings
}
