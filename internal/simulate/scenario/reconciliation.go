package scenario

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"cim-backend/internal/services/dto"

	"github.com/shopspring/decimal"
)

// inventoryMu coordinates access to the single shared inventory between the
// reconcile lifecycle and the stock-mutating PO receives.
//
//   - A reconcile takes the WRITE lock for its whole lifecycle (initiate ->
//     start-processing). This both serializes reconciles (the service allows only
//     one active-pending reconcile per inventory) AND, critically, blocks PO
//     receives for the duration: start-processing applies a snapshot-aware update
//     that optimistic-locks on each item's quantity, so a concurrent receive that
//     changed a quantity mid-window would make the apply 409 and leave the
//     submission stuck pending (poisoning every later reconcile). With receives
//     excluded, quantities are stable from snapshot to apply, so the reconcile
//     reaches a terminal state.
//   - PO receives take the READ lock: they run concurrently with each other but
//     never during a reconcile's apply window.
var inventoryMu sync.RWMutex

// Reconciliation lifecycle driver (epic #38, Part 6 + issue #73 labels).
//
// Endpoints, in order:
//
//	(ensure the shared inventory holds active items via a received PO)
//	POST /inventories/:id/reconcile/initiate
//	  -> POST /inventories/submissions/:id/reconciliation-items  (one labeled row)
//	  -> POST /inventories/submissions/:id/close
//	  [-> POST /inventories/submissions/:id/reopen
//	      -> PUT  /inventories/submissions/:id/reconciliation-items/:item_id (adjust)
//	      -> POST /inventories/submissions/:id/close]
//	  -> POST /inventories/submissions/:id/start-processing
//
// Initiate captures one reconciliation_snapshots row per ACTIVE inventory item
// (prev_quantity = live quantity at initiate). Staff then file count rows; each
// count must be non-negative, and the per-item TOTAL summed across ALL live
// sibling rows must not exceed the snapshot baseline (the service's aggregate
// guard). The scenario counts a fraction of the baseline and, on the adjust
// path, UPDATES the existing row in full (never adds a competing second row) so
// the aggregate stays within the baseline.
//
// All traffic targets ONE shared inventory (ref.InventoryID). Because the service
// allows only one active-pending reconcile per inventory, the reconcile lifecycle
// is serialized (reconMu) and every iteration runs to a TERMINAL state — a parked
// open/closed reconcile would block the next initiate on the same inventory.
// Concurrent PO receives and the other reconcile completing make start-processing
// frequently report drift (409, a non-failure outcome) — expected on a hot shared
// inventory.
//
// The variant alternates to cover the reachable terminal paths:
//
//	variant 0: initiate -> count -> close -> start-processing                                       (processed/drift)
//	variant 1: initiate -> count -> close -> reopen -> adjust(update) -> close -> start-processing   (processed/drift via reopen+update)
type ReconciliationScenario struct {
	iteration int
}

// Name implements Scenario.
func (s *ReconciliationScenario) Name() string { return "reconciliation" }

// inventoryItem is the subset of an inventory item we read back: its real ID and
// its live quantity (the reconcile snapshot baseline at initiate time).
type inventoryItem struct {
	ID       uint            `json:"id"`
	Quantity decimal.Decimal `json:"quantity"`
}

// submissionResponse is the subset of the initiated reconcile submission we read
// back: its real ID and lifecycle status. We never assume the ID.
type submissionResponse struct {
	ID              uint   `json:"id"`
	ReconcileStatus string `json:"reconcile_status"`
}

// reconItemResponse is the subset of a created/updated count row we read back:
// its real ID (needed to UPDATE the row on the adjust path).
type reconItemResponse struct {
	ID uint `json:"id"`
}

// Run drives one reconciliation lifecycle on the single shared inventory. The
// variant alternates each call between the straight and the reopen+adjust path;
// both end at a terminal state so the next reconcile can initiate.
func (s *ReconciliationScenario) Run(ctx context.Context, env *Env) error {
	variant := s.iteration % 2
	s.iteration++

	ref := env.RefIDs
	if ref == nil || ref.InventoryID == 0 || len(ref.ProductIDs) == 0 || len(ref.SupplierIDs) == 0 || ref.UnitID == 0 {
		return fmt.Errorf("reconciliation: reference data not initialized")
	}
	inventoryID := ref.InventoryID

	// Ensure the shared inventory holds active items to count: add a fully-received
	// PO. This does not need the reconcile lock (PO receive is not exclusive) and
	// contributes to the single-inventory traffic.
	po, err := createPO(ctx, env, s.iteration, inventoryID)
	if err != nil {
		return err
	}
	if err := receivePO(ctx, env, po, true); err != nil {
		return err
	}

	// Take the write lock for the whole lifecycle: serializes reconciles AND
	// blocks PO receives so item quantities are stable from snapshot (initiate)
	// through apply (start-processing) — otherwise the snapshot-aware apply would
	// optimistic-lock-409 and leave the submission stuck pending.
	inventoryMu.Lock()
	defer inventoryMu.Unlock()

	items, err := listInventoryItems(ctx, env, inventoryID)
	if err != nil {
		return fmt.Errorf("list inventory items: %w", err)
	}
	if len(items) == 0 {
		return fmt.Errorf("reconciliation: inventory %d has no items to count", inventoryID)
	}

	// 1. Initiate. Body is empty; scope comes from the path. Read the submission
	// ID back from the response.
	var sub submissionResponse
	initiatePath := fmt.Sprintf("/inventories/%d/reconcile/initiate", inventoryID)
	if err := env.Client.Do(ctx, "POST", initiatePath, struct{}{}, &sub, "POST /inventories/:id/reconcile/initiate"); err != nil {
		return fmt.Errorf("initiate reconcile: %w", err)
	}
	if sub.ID == 0 {
		return fmt.Errorf("reconciliation: initiate returned no submission id")
	}
	env.Report.Created("reconciliation")

	// 2. File one labeled count row at countFraction of the baseline. Read its
	// item ID back so the adjust path can UPDATE it.
	itemID, err := s.createCountRow(ctx, env, sub.ID, items, "Morning — Zone A", "shelf-1", countFraction)
	if err != nil {
		return fmt.Errorf("count items: %w", err)
	}

	// 3. Close (locks staff out).
	closePath := fmt.Sprintf("/inventories/submissions/%d/close", sub.ID)
	if err := env.Client.Do(ctx, "POST", closePath, nil, nil, "POST /inventories/submissions/:id/close"); err != nil {
		return fmt.Errorf("close reconcile %d: %w", sub.ID, err)
	}
	env.Report.Created("reconciliation_closed")

	// variant 1: reopen -> adjust -> close again. The adjust UPDATES the existing
	// row in full (a new row label + a different count) rather than adding a
	// competing second row, so the per-item aggregate across live rows stays <=
	// the snapshot baseline (the service's aggregate guard).
	if variant == 1 {
		reopenPath := fmt.Sprintf("/inventories/submissions/%d/reopen", sub.ID)
		if err := env.Client.Do(ctx, "POST", reopenPath, nil, nil, "POST /inventories/submissions/:id/reopen"); err != nil {
			return fmt.Errorf("reopen reconcile %d: %w", sub.ID, err)
		}
		env.Report.Created("reconciliation_reopened")

		// Full-replace update: a distinct row label + a different (still <= baseline)
		// count. Because update replaces the row, the aggregate equals just this
		// row's counts — never the sum of two rows.
		if err := s.updateCountRow(ctx, env, sub.ID, itemID, items, "Afternoon — Zone B", "dock-2", adjustFraction); err != nil {
			return fmt.Errorf("adjust count items: %w", err)
		}
		env.Report.Created("reconciliation_adjusted")

		if err := env.Client.Do(ctx, "POST", closePath, nil, nil, "POST /inventories/submissions/:id/close"); err != nil {
			return fmt.Errorf("re-close reconcile %d: %w", sub.ID, err)
		}
	}

	// 4. Start processing (atomic apply). On a clean apply the submission becomes
	// `processed`; on drift the server returns 409 with a warning payload and
	// applies nothing — a routine outcome, not a failure. DoExpectingStatus treats
	// a 409 as expected: it records the call as a NON-failure (so an expected drift
	// never inflates total_failures) and returns the status so we can branch.
	startPath := fmt.Sprintf("/inventories/submissions/%d/start-processing", sub.ID)
	var result dto.StartProcessingResult
	status, err := env.Client.DoExpectingStatus(ctx, "POST", startPath, nil, &result,
		"POST /inventories/submissions/:id/start-processing", http.StatusConflict)
	if err != nil {
		return fmt.Errorf("start processing %d: %w", sub.ID, err)
	}
	// 409 (drift) or a 2xx body with DriftDetected: rolled back, nothing applied.
	// Recorded as its own non-failure drift metric rather than a failed call.
	if status == http.StatusConflict || result.DriftDetected {
		env.Report.Created("reconciliation_drift")
		return nil
	}
	env.Report.Created("reconciliation_processed")
	return nil
}

// createCountRow files one staff count row under sub at the given fraction of
// each item's snapshot baseline and returns the created row's ID. Every count in
// the row carries countLabel; the row carries rowLabel.
func (s *ReconciliationScenario) createCountRow(ctx context.Context, env *Env, subID uint, items []inventoryItem, rowLabel, countLabel string, fraction float64) (uint, error) {
	req := dto.CreateReconciliationItemRequest{
		SubmissionID: subID, // path-scoped server-side, but set for completeness
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

// updateCountRow full-replaces an existing count row (row label + entire counted
// payload) with a new fraction of the baseline. Replacing — rather than adding a
// second row — keeps the per-item aggregate across live rows within the baseline.
func (s *ReconciliationScenario) updateCountRow(ctx context.Context, env *Env, subID, itemID uint, items []inventoryItem, rowLabel, countLabel string, fraction float64) error {
	req := dto.UpdateReconciliationItemRequest{
		SubmissionID: subID, // path-scoped server-side
		ItemID:       itemID,
		Label:        rowLabel,
		Items:        countItems(items, countLabel, fraction),
	}
	path := fmt.Sprintf("/inventories/submissions/%d/reconciliation-items/%d", subID, itemID)
	return env.Client.Do(ctx, "PUT", path, req, nil, "PUT /inventories/submissions/:id/reconciliation-items/:item_id")
}

// countItems builds the count lines for a row: each item counted at the given
// fraction of its snapshot baseline (floored, clamped to the baseline), all
// carrying countLabel.
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

// countFraction is the share of the snapshot baseline the simulated count
// reports. It is 1.0 = count exactly what is on hand, so applying the reconcile
// is NON-DESTRUCTIVE — it never shrinks stock (the sim should leave healthy,
// growing stock rather than consume it). Counts can never exceed the baseline
// (snapshot invariant), so 1.0 is the maximum non-destructive value. Lower it if
// you want the sim to model count shrinkage/variance instead.
const countFraction = 1.0

// adjustFraction is the share used by the reopen+update adjust path. Also 1.0
// (non-destructive); the adjust is still a genuine full-replace update — it
// re-labels the count row — without depleting stock.
const adjustFraction = 1.0

// countedQuantity computes a simulated counted quantity for a baseline: a
// non-negative integer at fraction of the baseline, floored, and never exceeding
// the baseline (the S2 rule). A zero/negative baseline yields zero.
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

// listInventoryItems reads the active inventory items of an inventory back by
// real ID + quantity (the reconcile snapshot baseline).
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
