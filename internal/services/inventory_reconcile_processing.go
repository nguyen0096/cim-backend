package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// errReconDriftRollback forces the apply tx to roll back on drift while still
// returning the warning-shaped result to the handler.
var errReconDriftRollback = errors.New("reconciliation drift detected; rolling back")

// CloseReconciliation moves an initiated reconcile open -> closed.
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

// buildNotReadySessionWarnings returns one advisory line per live staff session
// not yet ready for review, or nil when all are ready. Manager-owned sessions are excluded.
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

// managerOwnedSessions returns the set of row IDs whose creator holds recon_manage.
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

// ReopenReconciliation moves an initiated reconcile closed -> open.
func (s *inventoryService) ReopenReconciliation(ctx context.Context, submissionID uint) (*models.InventorySubmission, error) {
	return s.transitionReconcileStatus(ctx, submissionID,
		models.ReconcileLifecycleStatusClosed, models.ReconcileLifecycleStatusOpen)
}

// transitionReconcileStatus asserts the source lifecycle status and writes the target.
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

// CancelReconciliation abandons an active reconcile (open/closed -> canceled) with
// no stock change; child count rows are retained.
func (s *inventoryService) CancelReconciliation(ctx context.Context, submissionID uint) (*models.InventorySubmission, error) {
	if !pkg.HasPermission(ctx, pkg.RBACResourceInventorySubmissions, pkg.RBACActionReconManage) {
		return nil, pkg.ErrForbidden("user does not have permission to manage reconciliations", nil)
	}

	var result *models.InventorySubmission
	err := s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		parent, err := s.loadActiveReconcileParent(txCtx, submissionID)
		if err != nil {
			return err
		}
		if parent.ReconcileStatus != models.ReconcileLifecycleStatusOpen &&
			parent.ReconcileStatus != models.ReconcileLifecycleStatusClosed {
			return pkg.ErrReconInvalidLifecycleTransition(txCtx, submissionID,
				string(parent.ReconcileStatus), string(models.ReconcileLifecycleStatusCanceled))
		}

		if err := s.inventorySubmissionRepo.AcquireInventoryAdvisoryLock(txCtx, parent.InventoryID); err != nil {
			return fmt.Errorf("failed to acquire inventory advisory lock: %w", err)
		}

		if err := s.inventorySubmissionRepo.CancelReconciliation(txCtx, submissionID); err != nil {
			return fmt.Errorf("failed to cancel reconciliation: %w", err)
		}
		parent.ReconcileStatus = models.ReconcileLifecycleStatusCanceled
		parent.ProcessingStatus = models.InventorySubmissionStatusCanceled
		parent.ApprovalStatus = models.InventorySubmissionApprovalStatusRejected
		result = parent
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// StartProcessing runs the atomic apply tx: advisory lock, drift re-check, then
// synthesize + snapshot-aware apply + finalize. On drift it rolls back and returns
// a warning-shaped result with no inventory mutated.
func (s *inventoryService) StartProcessing(ctx context.Context, submissionID uint) (*dto.StartProcessingResult, error) {
	if !pkg.HasPermission(ctx, pkg.RBACResourceInventorySubmissions, pkg.RBACActionReconManage) {
		return nil, pkg.ErrForbidden("user does not have permission to manage reconciliations", nil)
	}

	var result *dto.StartProcessingResult
	err := s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		parent, err := s.loadActiveReconcileParent(txCtx, submissionID)
		if err != nil {
			return err
		}
		if parent.ReconcileStatus != models.ReconcileLifecycleStatusClosed {
			return pkg.ErrReconInvalidLifecycleTransition(txCtx, submissionID,
				string(parent.ReconcileStatus), string(models.ReconcileLifecycleStatusProcessing))
		}

		if err := s.inventorySubmissionRepo.AcquireInventoryAdvisoryLock(txCtx, parent.InventoryID); err != nil {
			return fmt.Errorf("failed to acquire inventory advisory lock: %w", err)
		}

		// Drift window lower bound is the snapshot-capture instant, not parent.CreatedAt:
		// a consume committed before capture is already in the baseline and must not be flagged.
		capturedAt, ok, err := s.snapshotRepo.GetSnapshotCapturedAt(txCtx, parent.ID)
		if err != nil {
			return fmt.Errorf("failed to load reconciliation snapshot capture time: %w", err)
		}
		if !ok {
			// Defensive: no snapshot rows; fall back to the wider parent.CreatedAt window.
			capturedAt = parent.CreatedAt
		}
		drifted, err := s.inventorySubmissionRepo.ListConsumingProcessedSince(
			txCtx, parent.InventoryID, parent.ID, capturedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to evaluate reconciliation drift: %w", err)
		}
		if len(drifted) > 0 {
			result = &dto.StartProcessingResult{
				DriftDetected: true,
				Warnings:      buildDriftWarnings(txCtx, drifted),
			}
			return errReconDriftRollback
		}

		syn, err := s.SynthesizeSubmissionPayload(txCtx, submissionID)
		if err != nil {
			return fmt.Errorf("failed to synthesize reconciliation payload: %w", err)
		}

		if err := s.applyReconcileSnapshotAware(txCtx, parent, syn); err != nil {
			return err
		}

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

		// Mirror the persisted fields onto the returned struct so it reflects the committed row.
		parent.Payload = payloadBytes
		parent.ApprovalStatus = models.InventorySubmissionApprovalStatusApproved
		parent.ProcessingStatus = models.InventorySubmissionStatusCompleted
		parent.ReconcileStatus = models.ReconcileLifecycleStatusProcessed
		parent.ProcessedAt = &processedAt
		parent.Error = nil
		result = &dto.StartProcessingResult{Submission: parent}
		return nil
	})
	if err != nil && err != errReconDriftRollback {
		return nil, err
	}
	return result, nil
}

// applyReconcileSnapshotAware applies the synthesized reconcile per item: when counted
// is below snapshot it consumes the shortfall FIFO (sell); when counted is above snapshot
// it raises stock by the surplus with a reconcile_stock_up txn. Both the sell and stock-up
// txns are backdated to the parent submission's created_at (reconciliation initiation time).
func (s *inventoryService) applyReconcileSnapshotAware(
	ctx context.Context,
	parent *models.InventorySubmission,
	syn *dto.SynthesizedReconcile,
) error {
	itemIDs := models.GetIDs(syn.Request.Items)
	if len(itemIDs) == 0 {
		return nil
	}

	activeItems, err := s.getActiveInventoryItems(ctx, parent.InventoryID, itemIDs)
	if err != nil {
		return fmt.Errorf("failed to load active inventory items for reconcile apply: %w", err)
	}
	activeItemMap := s.buildItemMap(activeItems)

	itemConsumeQuantity := make(map[uint]decimal.Decimal)
	itemStockUpQuantity := make(map[uint]decimal.Decimal)
	for _, line := range syn.Request.Items {
		if line.Quantity == nil {
			return pkg.ErrInvalidRequestBody(fmt.Errorf("counted quantity missing for inventory item %d", line.InventoryItemID))
		}
		if _, exists := activeItemMap[line.InventoryItemID]; !exists {
			return pkg.ErrInventoryItemNotFound(ctx, line.InventoryItemID)
		}
		delta := line.PrevQuantity.Sub(*line.Quantity)
		switch {
		case delta.IsPositive():
			itemConsumeQuantity[line.InventoryItemID] = delta
		case delta.IsNegative():
			itemStockUpQuantity[line.InventoryItemID] = delta.Neg()
		}
	}

	ps := newProcessingState(s, parent)

	consumeHandler := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity decimal.Decimal) []*models.InventoryTransaction {
		return []*models.InventoryTransaction{
			{
				Base:                 models.Base{CreatedAt: parent.CreatedAt},
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

	stockUpChanges, stockUpTxns := buildReconcileStockUps(activeItemMap, itemStockUpQuantity, parent.CreatedAt)
	ivtrItemChanges = append(ivtrItemChanges, stockUpChanges...)
	txns = append(txns, stockUpTxns...)

	if err := s.inventoryItemRepo.SaveInventoryItemChanges(ctx, ivtrItemChanges, txns); err != nil {
		return fmt.Errorf("failed to save inventory item changes for reconcile apply: %w", err)
	}
	return nil
}

// buildReconcileStockUps raises each item's on-hand by its surplus and emits a backdated,
// consumable reconcile_stock_up txn (Price 0) so the on-hand/consumable invariant holds.
func buildReconcileStockUps(
	activeItemMap map[uint]*models.InventoryItem,
	itemStockUpQuantity map[uint]decimal.Decimal,
	backdatedAt time.Time,
) ([]*models.InventoryItemChange, []*models.InventoryTransaction) {
	changes := make([]*models.InventoryItemChange, 0, len(itemStockUpQuantity))
	txns := make([]*models.InventoryTransaction, 0, len(itemStockUpQuantity))
	for itemID, surplus := range itemStockUpQuantity {
		item := activeItemMap[itemID]
		changes = append(changes, &models.InventoryItemChange{
			InventoryItem:    item,
			OriginalQuantity: item.Quantity,
		})
		item.Quantity = item.Quantity.Add(surplus)
		txns = append(txns, &models.InventoryTransaction{
			Base:            models.Base{CreatedAt: backdatedAt},
			InventoryItemID: itemID,
			TransactionType: models.InventoryTransactionTypeReconcileStockUp,
			Price:           0,
			Quantity:        surplus,
			IsAdjustment:    true,
		})
	}
	return changes, txns
}

// buildDriftWarnings renders one localized warning line per offending consuming submission.
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
