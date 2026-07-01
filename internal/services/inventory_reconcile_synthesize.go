package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/shopspring/decimal"
)

// This file implements SynthesizeSubmissionPayload (epic #38, Part 5): folding the
// live staff child rows of an initiated reconcile into the legacy
// ReconcileInventoryRequest-shaped payload the apply path (reconcileInventory /
// consumeFIFO / SaveInventoryItemChanges) expects, plus the derived review label.
//
// It is PURE / READ-ONLY: it performs no writes. The parent inventory_submissions
// row keeps an empty payload until the Part-7 apply step persists the finalized
// synthesized payload; until then list/detail must render items by synthesizing
// over the child rows rather than reading the empty Payload (S4).
//
// Locked rules honored here:
//   - Baseline = the parent snapshot captured at initiate (sole source of truth
//     for prev_quantity, B2). The synthesized PrevQuantity is the snapshot value,
//     NOT live stock; live is only read by the warnings layer to show drift.
//   - Counts are summed by inventory_item_id across ALL live (non-soft-deleted)
//     child rows (soft-deleted rows are already excluded by ListBySubmission).
//   - The Part-4 write-time aggregate guard already enforces sum(counted) <=
//     snapshot per item; synthesis relies on that invariant but does NOT trust it
//     blindly — if a stored aggregate somehow exceeds the snapshot it is surfaced
//     as an anomaly (and the line is still emitted at the baseline-capped value so
//     a downstream consume can never be negative) rather than silently corrupting.

// SynthesizeSubmissionPayload sums the live child rows of an initiated reconcile
// into the legacy ReconcileInventoryRequest payload, attaches the snapshot
// baseline per item, and computes the review label. Read-only; uses DB(ctx) via
// the repositories so it can run inside or outside a transaction.
func (s *inventoryService) SynthesizeSubmissionPayload(ctx context.Context, submissionID uint) (*dto.SynthesizedReconcile, error) {
	submission, err := s.inventorySubmissionRepo.GetByID(ctx, submissionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load submission %d: %w", submissionID, err)
	}

	rows, err := s.reconItemRepo.ListBySubmission(ctx, submissionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list reconciliation items for submission %d: %w", submissionID, err)
	}

	baselines, err := s.snapshotRepo.GetPrevQuantitiesBySubmission(ctx, submissionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load snapshot baselines for submission %d: %w", submissionID, err)
	}

	managerOwned, err := s.managerOwnedSessions(ctx, rows)
	if err != nil {
		return nil, err
	}

	return synthesizeReconcile(submission.InventoryID, rows, baselines, managerOwned)
}

// synthesizeReconcile is the pure, repository-free core of
// SynthesizeSubmissionPayload.
func synthesizeReconcile(
	inventoryID uint,
	rows []models.ReconciliationRequestItem,
	baselines map[uint]decimal.Decimal,
	managerOwned map[uint]bool,
) (*dto.SynthesizedReconcile, error) {
	totals := make(map[uint]decimal.Decimal)
	// breakdown accumulates the per-(inventory_item, label) contributions behind each
	// summed total (issue #73), review-only. labelOrder preserves first-seen label
	// order per item so the rendered breakdown is stable. The label is
	// representation-only: it never affects totals (the apply math).
	breakdown := make(map[uint]map[string]decimal.Decimal)
	labelOrder := make(map[uint][]string)
	for _, row := range rows {
		if len(row.Payload) == 0 {
			continue
		}
		var parsed reconItemPayload
		if err := json.Unmarshal(row.Payload, &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse reconciliation item %d payload: %w", row.ID, err)
		}
		for _, line := range parsed.Items {
			cur, ok := totals[line.InventoryItemID]
			if !ok {
				cur = decimal.Zero
			}
			totals[line.InventoryItemID] = cur.Add(line.Quantity)

			byLabel, ok := breakdown[line.InventoryItemID]
			if !ok {
				byLabel = make(map[string]decimal.Decimal)
				breakdown[line.InventoryItemID] = byLabel
			}
			if _, seen := byLabel[line.Label]; !seen {
				labelOrder[line.InventoryItemID] = append(labelOrder[line.InventoryItemID], line.Label)
			}
			byLabel[line.Label] = byLabel[line.Label].Add(line.Quantity)
		}
	}

	// Emit items in a deterministic order (ascending inventory_item_id) so the
	// synthesized payload and the rendered list are stable across calls.
	itemIDs := make([]uint, 0, len(totals))
	for id := range totals {
		itemIDs = append(itemIDs, id)
	}
	sort.Slice(itemIDs, func(i, j int) bool { return itemIDs[i] < itemIDs[j] })

	// Emit the per-(item, label) breakdown in the same deterministic item order as
	// the summed lines, preserving first-seen label order within each item.
	var breakdownLines []dto.ReconcileItemBreakdown
	for _, id := range itemIDs {
		for _, label := range labelOrder[id] {
			qty := breakdown[id][label]
			breakdownLines = append(breakdownLines, dto.ReconcileItemBreakdown{
				InventoryItemID: id,
				Label:           label,
				Quantity:        qty,
			})
		}
	}

	var anomalies []string
	items := make([]dto.QuantityItem, 0, len(itemIDs))
	for _, id := range itemIDs {
		counted := totals[id]
		baseline, hasBaseline := baselines[id]

		// Anomaly: a counted item with no snapshot row. The Part-4 write guard
		// rejects this at write time (ErrReconItemNoSnapshotBaseline), so it should
		// be impossible; if it ever occurs we surface it and fall back to a zero
		// baseline so the line is still visible for review rather than dropped.
		if !hasBaseline {
			anomalies = append(anomalies, fmt.Sprintf(
				"sản phẩm (inventory_item_id=%d) không có số lượng nền (snapshot) — cần kiểm tra lại", id))
			baseline = decimal.Zero
		}

		// Anomaly: counted exceeds the snapshot baseline. The Part-4 write-time
		// aggregate guard makes this impossible under normal operation; if a stored
		// aggregate somehow exceeds the snapshot — OR the snapshot is missing
		// entirely (baseline falls back to zero above) and a positive count was
		// recorded — we surface it AND cap the emitted counted at the baseline so a
		// downstream consume (snapshot - counted) can never go negative / corrupt
		// FIFO. The missing-baseline case must clamp too: emitting a positive count
		// against a zero baseline would otherwise reintroduce the exact negative
		// consume this cap exists to prevent.
		emitted := counted
		if counted.GreaterThan(baseline) {
			if hasBaseline {
				anomalies = append(anomalies, fmt.Sprintf(
					"tổng số lượng kiểm đếm của sản phẩm (inventory_item_id=%d) là %s vượt quá số lượng nền %s — cần kiểm tra lại",
					id, counted.String(), baseline.String()))
			}
			// (missing-baseline already surfaced its own warning above)
			emitted = baseline
		}

		quantity := emitted
		items = append(items, dto.QuantityItem{
			InventoryItemID: id,
			Quantity:        &quantity,
			PrevQuantity:    baseline,
		})
	}

	return &dto.SynthesizedReconcile{
		Request: dto.ReconcileInventoryRequest{
			InventoryID: inventoryID,
			Items:       items,
		},
		Label:     aggregateReviewLabel(rows, managerOwned),
		Anomalies: anomalies,
		Breakdown: breakdownLines,
	}, nil
}

// aggregateReviewLabel derives the submission-level review label from the
// per-session readiness of the live STAFF child rows. Manager-owned sessions
// (managerOwned[row.ID]) are excluded: readiness is a staff concept and managers
// cannot toggle, so a manager session must never hold the submission in_progress.
// With no staff sessions the label stays in_progress (nothing to review).
func aggregateReviewLabel(rows []models.ReconciliationRequestItem, managerOwned map[uint]bool) dto.ReconcileReviewLabel {
	staffSessions := 0
	for _, row := range rows {
		if managerOwned[row.ID] {
			continue
		}
		staffSessions++
		if row.Status != models.ReconciliationRequestItemStatusReadyForReview {
			return dto.ReconcileReviewLabelInProgress
		}
	}
	if staffSessions == 0 {
		return dto.ReconcileReviewLabelInProgress
	}
	return dto.ReconcileReviewLabelReadyForReview
}
