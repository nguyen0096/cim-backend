package scenario

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"cim-backend/internal/services/dto"

	"github.com/shopspring/decimal"
)

// reconNonceSeq makes each reconciliation scenario instance's nonce unique even
// when many instances are constructed in the same nanosecond (e.g. a load-mode
// worker pool spinning up). Combined with the wall-clock base it keeps the
// per-iteration inventory names unique within AND across runs.
var reconNonceSeq atomic.Int64

// Reconciliation lifecycle driver (epic #38, Part 6 + issue #73 labels).
//
// Endpoints, in order:
//
//	(create dedicated inventory + received PO so it holds active items)
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
// Each iteration runs on its OWN inventory. Otherwise a parked (open/closed but
// still pending) reconcile would trip the one-active-pending guard the moment
// the next iteration initiates on the same inventory. Isolating inventories lets
// parked open/closed states coexist for broad-state coverage.
//
// Across iterations the variant cycles to spread submissions over the reachable
// lifecycle states so a mock run leaves a broad mix in the DB:
//
//	variant 0: initiate -> count -> close -> start-processing                         (processed)
//	variant 1: initiate -> count -> close -> reopen -> adjust(update) -> close -> start-processing (processed via reopen+update path)
//	variant 2: initiate -> count -> close                                             (parked: closed)
//	variant 3: initiate -> count                                                      (parked: open)
type ReconciliationScenario struct {
	iteration int
	// nonce uniquely tags the per-iteration inventories created by this scenario
	// instance so names never collide within or across runs. Captured lazily on
	// the first Run.
	nonce int64
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

// Run drives one reconciliation lifecycle on its own dedicated inventory. The
// variant cycles each call so a multi-volume run covers open/closed/processed
// plus a reopen+update path.
func (s *ReconciliationScenario) Run(ctx context.Context, env *Env) error {
	if s.nonce == 0 {
		// Combine wall clock with a process-global sequence so concurrently
		// constructed instances never share a nonce (and thus never collide on
		// inventory names / the one-active-pending guard).
		s.nonce = time.Now().UnixNano() + reconNonceSeq.Add(1)
	}
	variant := s.iteration % 4
	s.iteration++

	ref := env.RefIDs
	if ref == nil || len(ref.ProductIDs) == 0 || len(ref.SupplierIDs) == 0 || ref.UnitID == 0 {
		return fmt.Errorf("reconciliation: reference data not initialized")
	}

	// Each iteration owns its inventory so parked reconciles never collide on the
	// one-active-pending guard. Seed it with a fully-received PO so it holds
	// active items (initiate snapshots one row per active item).
	invName := reconInventoryName(s.nonce, s.iteration)
	inventoryID, err := createDedicatedInventory(ctx, env, invName)
	if err != nil {
		return fmt.Errorf("create reconcile inventory: %w", err)
	}
	po, err := createPO(ctx, env, s.iteration, inventoryID)
	if err != nil {
		return err
	}
	if err := receivePO(ctx, env, po, true); err != nil {
		return err
	}

	items, err := listInventoryItems(ctx, env, inventoryID)
	if err != nil {
		return fmt.Errorf("list inventory items: %w", err)
	}
	if len(items) == 0 {
		return fmt.Errorf("reconciliation: inventory %d has no items to count after PO receive", inventoryID)
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

	// variant 3: leave it parked at OPEN for broad-state coverage.
	if variant == 3 {
		env.Report.Created("reconciliation_open")
		return nil
	}

	// 3. Close (locks staff out).
	closePath := fmt.Sprintf("/inventories/submissions/%d/close", sub.ID)
	if err := env.Client.Do(ctx, "POST", closePath, nil, nil, "POST /inventories/submissions/:id/close"); err != nil {
		return fmt.Errorf("close reconcile %d: %w", sub.ID, err)
	}
	env.Report.Created("reconciliation_closed")

	// variant 2: leave it parked at CLOSED.
	if variant == 2 {
		return nil
	}

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

// reconInventoryName builds a unique name for an iteration's dedicated
// reconciliation inventory. The run nonce isolates this scenario instance and
// the iteration isolates each lifecycle, so no two iterations ever target the
// same inventory (which would trip the one-active-pending guard).
func reconInventoryName(nonce int64, iteration int) string {
	return fmt.Sprintf("SIM Recon Inv %d-%d", nonce, iteration)
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

// countFraction is the share of the snapshot baseline the initial simulated
// count reports — a realistic shrinkage strictly within the baseline.
const countFraction = 0.6

// adjustFraction is the share used by the reopen+update adjust path. A different
// value from countFraction makes the update a genuine change, and being <=
// countFraction keeps the replaced row's counts well within the baseline.
const adjustFraction = 0.5

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
