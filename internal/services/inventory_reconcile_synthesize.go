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

// SynthesizeSubmissionPayload sums the live child rows of an initiated reconcile
// into a ReconcileInventoryRequest payload with the snapshot baseline per item and
// the review label. Read-only.
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

// synthesizeReconcile is the pure core of SynthesizeSubmissionPayload.
func synthesizeReconcile(
	inventoryID uint,
	rows []models.ReconciliationRequestItem,
	baselines map[uint]decimal.Decimal,
	managerOwned map[uint]bool,
) (*dto.SynthesizedReconcile, error) {
	totals := make(map[uint]decimal.Decimal)
	// breakdown holds review-only per-(item, label) contributions; labelOrder keeps
	// first-seen label order. Neither affects totals.
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

	// Deterministic order (ascending inventory_item_id).
	itemIDs := make([]uint, 0, len(totals))
	for id := range totals {
		itemIDs = append(itemIDs, id)
	}
	sort.Slice(itemIDs, func(i, j int) bool { return itemIDs[i] < itemIDs[j] })

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

		// Anomaly: counted item with no snapshot row. Surface it and fall back to zero baseline.
		if !hasBaseline {
			anomalies = append(anomalies, fmt.Sprintf(
				"sản phẩm (inventory_item_id=%d) không có số lượng nền (snapshot) — cần kiểm tra lại", id))
			baseline = decimal.Zero
		}

		// Anomaly: counted exceeds baseline. Surface it and cap emitted at baseline so
		// a downstream consume (snapshot - counted) can never go negative.
		emitted := counted
		if counted.GreaterThan(baseline) {
			if hasBaseline {
				anomalies = append(anomalies, fmt.Sprintf(
					"tổng số lượng kiểm đếm của sản phẩm (inventory_item_id=%d) là %s vượt quá số lượng nền %s — cần kiểm tra lại",
					id, counted.String(), baseline.String()))
			}
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

// aggregateReviewLabel derives the submission-level review label from staff session
// readiness; manager-owned sessions are excluded and no staff sessions stays in_progress.
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
