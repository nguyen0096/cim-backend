package scenario

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"cim-backend/internal/services/dto"

	"github.com/shopspring/decimal"
)

// inventoryMu serializes reconciles (write lock, held for a whole lifecycle) and
// excludes stock-mutating PO receives (read lock) from a reconcile's apply
// window, keeping item quantities stable from snapshot to apply.
var inventoryMu sync.RWMutex

// ReconciliationScenario drives one reconciliation lifecycle on the shared
// inventory, alternating between a straight path and a reopen+adjust path.
type ReconciliationScenario struct {
	iteration int
}

// Name implements Scenario.
func (s *ReconciliationScenario) Name() string { return "reconciliation" }

// inventoryItem is an inventory item's ID and live quantity.
type inventoryItem struct {
	ID       uint            `json:"id"`
	Quantity decimal.Decimal `json:"quantity"`
}

// submissionResponse is a reconcile submission's ID and lifecycle status.
type submissionResponse struct {
	ID              uint   `json:"id"`
	ReconcileStatus string `json:"reconcile_status"`
}

// reconItemResponse is a count row's ID.
type reconItemResponse struct {
	ID uint `json:"id"`
}

// Run drives one reconciliation lifecycle on the shared inventory.
func (s *ReconciliationScenario) Run(ctx context.Context, env *Env) error {
	variant := s.iteration % 2
	s.iteration++

	ref := env.RefIDs
	if ref == nil || ref.InventoryID == 0 || len(ref.ProductIDs) == 0 || len(ref.SupplierIDs) == 0 || ref.UnitID == 0 {
		return fmt.Errorf("reconciliation: reference data not initialized")
	}
	inventoryID := ref.InventoryID

	// Ensure the inventory holds items to count: add a fully-received PO.
	po, err := createPO(ctx, env, s.iteration, inventoryID)
	if err != nil {
		return err
	}
	if err := receivePO(ctx, env, po, true); err != nil {
		return err
	}

	// Write lock for the whole lifecycle: serializes reconciles and keeps item
	// quantities stable from snapshot (initiate) through apply (start-processing).
	inventoryMu.Lock()
	defer inventoryMu.Unlock()

	items, err := listInventoryItems(ctx, env, inventoryID)
	if err != nil {
		return fmt.Errorf("list inventory items: %w", err)
	}
	if len(items) == 0 {
		return fmt.Errorf("reconciliation: inventory %d has no items to count", inventoryID)
	}

	// 1. Initiate.
	var sub submissionResponse
	initiatePath := fmt.Sprintf("/inventories/%d/reconcile/initiate", inventoryID)
	if err := env.Client.Do(ctx, "POST", initiatePath, struct{}{}, &sub, "POST /inventories/:id/reconcile/initiate"); err != nil {
		return fmt.Errorf("initiate reconcile: %w", err)
	}
	if sub.ID == 0 {
		return fmt.Errorf("reconciliation: initiate returned no submission id")
	}
	env.Report.Created("reconciliation")

	// 2. File one labeled count row at countFraction of the baseline.
	itemID, err := s.createCountRow(ctx, env, sub.ID, items, "Morning — Zone A", "shelf-1", countFraction)
	if err != nil {
		return fmt.Errorf("count items: %w", err)
	}

	// 3. Close.
	closePath := fmt.Sprintf("/inventories/submissions/%d/close", sub.ID)
	if err := env.Client.Do(ctx, "POST", closePath, nil, nil, "POST /inventories/submissions/:id/close"); err != nil {
		return fmt.Errorf("close reconcile %d: %w", sub.ID, err)
	}
	env.Report.Created("reconciliation_closed")

	// variant 1: reopen -> adjust -> close again.
	if variant == 1 {
		reopenPath := fmt.Sprintf("/inventories/submissions/%d/reopen", sub.ID)
		if err := env.Client.Do(ctx, "POST", reopenPath, nil, nil, "POST /inventories/submissions/:id/reopen"); err != nil {
			return fmt.Errorf("reopen reconcile %d: %w", sub.ID, err)
		}
		env.Report.Created("reconciliation_reopened")

		if err := s.updateCountRow(ctx, env, sub.ID, itemID, items, "Afternoon — Zone B", "dock-2", adjustFraction); err != nil {
			return fmt.Errorf("adjust count items: %w", err)
		}
		env.Report.Created("reconciliation_adjusted")

		if err := env.Client.Do(ctx, "POST", closePath, nil, nil, "POST /inventories/submissions/:id/close"); err != nil {
			return fmt.Errorf("re-close reconcile %d: %w", sub.ID, err)
		}
	}

	// 4. Start processing. A 409 (drift) is an expected non-failure outcome.
	startPath := fmt.Sprintf("/inventories/submissions/%d/start-processing", sub.ID)
	var result dto.StartProcessingResult
	status, err := env.Client.DoExpectingStatus(ctx, "POST", startPath, nil, &result,
		"POST /inventories/submissions/:id/start-processing", http.StatusConflict)
	if err != nil {
		return fmt.Errorf("start processing %d: %w", sub.ID, err)
	}
	if status == http.StatusConflict || result.DriftDetected {
		env.Report.Created("reconciliation_drift")
		return nil
	}
	env.Report.Created("reconciliation_processed")
	return nil
}

// createCountRow files one count row at the given fraction of each item's
// baseline and returns the created row's ID.
func (s *ReconciliationScenario) createCountRow(ctx context.Context, env *Env, subID uint, items []inventoryItem, rowLabel, countLabel string, fraction float64) (uint, error) {
	req := dto.CreateReconciliationItemRequest{
		SubmissionID: subID,
		Label:        rowLabel,
		Items:        countItems(items, countLabel, fraction),
	}
	path := fmt.Sprintf("/inventories/submissions/%d/reconciliation-items", subID)
	var resp reconItemResponse
	if err := env.Client.Do(ctx, "POST", path, req, &resp, "POST /inventories/submissions/:id/reconciliation-items"); err != nil {
		return 0, err
	}
	return resp.ID, nil
}

// updateCountRow full-replaces an existing count row with a new fraction of the
// baseline.
func (s *ReconciliationScenario) updateCountRow(ctx context.Context, env *Env, subID, itemID uint, items []inventoryItem, rowLabel, countLabel string, fraction float64) error {
	req := dto.UpdateReconciliationItemRequest{
		SubmissionID: subID,
		ItemID:       itemID,
		Label:        rowLabel,
		Items:        countItems(items, countLabel, fraction),
	}
	path := fmt.Sprintf("/inventories/submissions/%d/reconciliation-items/%d", subID, itemID)
	return env.Client.Do(ctx, "PUT", path, req, nil, "PUT /inventories/submissions/:id/reconciliation-items/:item_id")
}

// countItems builds the count lines for a row: each item at the given fraction
// of its baseline, all carrying countLabel.
func countItems(items []inventoryItem, countLabel string, fraction float64) []dto.ReconciliationCountItem {
	counts := make([]dto.ReconciliationCountItem, 0, len(items))
	for _, it := range items {
		q := countedQuantity(it.Quantity, fraction)
		counts = append(counts, dto.ReconciliationCountItem{
			InventoryItemID: it.ID,
			Quantity:        &q,
			Label:           countLabel,
		})
	}
	return counts
}

// countFraction is the share of the baseline the count reports. 1.0 counts
// exactly what is on hand, so applying the reconcile never shrinks stock.
const countFraction = 1.0

// adjustFraction is the share used by the reopen+update adjust path.
const adjustFraction = 1.0

// countedQuantity is fraction of baseline, floored and clamped to the baseline;
// a zero/negative baseline yields zero.
func countedQuantity(baseline decimal.Decimal, fraction float64) decimal.Decimal {
	if baseline.IsNegative() {
		return decimal.Zero
	}
	counted := baseline.Mul(decimal.NewFromFloat(fraction)).Floor()
	if counted.GreaterThan(baseline) {
		counted = baseline
	}
	return counted
}

// listInventoryItems reads an inventory's active items (ID + quantity).
func listInventoryItems(ctx context.Context, env *Env, inventoryID uint) ([]inventoryItem, error) {
	path := fmt.Sprintf("/inventories/%d/inventory-items?limit=200", inventoryID)
	var resp struct {
		Data []inventoryItem `json:"data"`
	}
	if err := env.Client.Do(ctx, "GET", path, nil, &resp, "GET /inventories/:id/inventory-items"); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
