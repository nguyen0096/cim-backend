package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
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

	return s.synthesizeFromRows(ctx, submission.InventoryID, submissionID, rows)
}

// synthesizeFromRows folds the given child rows into the synthesized payload. The
// caller chooses the row scope (all rows, or one creator's own sessions), so the
// same core drives both the manager-wide and staff-own-only views.
func (s *inventoryService) synthesizeFromRows(ctx context.Context, inventoryID, submissionID uint, rows []models.ReconciliationRequestItem) (*dto.SynthesizedReconcile, error) {
	baselines, err := s.snapshotRepo.GetPrevQuantitiesBySubmission(ctx, submissionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load snapshot baselines for submission %d: %w", submissionID, err)
	}

	managerOwned, err := s.managerOwnedSessions(ctx, rows)
	if err != nil {
		return nil, err
	}

	// Product names for anomaly messages (be-94: name, not raw id). Snapshot items
	// cover the overage case, which only fires when a baseline exists.
	baselineIDs := make([]uint, 0, len(baselines))
	for id := range baselines {
		baselineIDs = append(baselineIDs, id)
	}
	productNames := s.resolveProductNames(ctx, baselineIDs)

	return synthesizeReconcile(inventoryID, rows, baselines, managerOwned, productNames)
}

// synthesizeReconcile is the pure core of SynthesizeSubmissionPayload.
func synthesizeReconcile(
	inventoryID uint,
	rows []models.ReconciliationRequestItem,
	baselines map[uint]decimal.Decimal,
	managerOwned map[uint]bool,
	productNames map[uint]string,
) (*dto.SynthesizedReconcile, error) {
	totals := make(map[uint]decimal.Decimal)
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
		}
	}

	// Deterministic order (ascending inventory_item_id).
	itemIDs := make([]uint, 0, len(totals))
	for id := range totals {
		itemIDs = append(itemIDs, id)
	}
	sort.Slice(itemIDs, func(i, j int) bool { return itemIDs[i] < itemIDs[j] })

	breakdownLines, err := sessionBreakdown(rows)
	if err != nil {
		return nil, err
	}

	// productLabel names the product (be-94) when known, else falls back to the id.
	productLabel := func(id uint) string {
		if name, ok := productNames[id]; ok && name != "" {
			return fmt.Sprintf("«%s»", name)
		}
		return fmt.Sprintf("(inventory_item_id=%d)", id)
	}

	var anomalies []string
	var itemAnomalies []dto.SubmissionItemWarning
	items := make([]dto.QuantityItem, 0, len(itemIDs))
	for _, id := range itemIDs {
		counted := totals[id]
		baseline, hasBaseline := baselines[id]

		// Anomaly: counted item with no snapshot row. Surface it and fall back to zero baseline.
		if !hasBaseline {
			msg := fmt.Sprintf(
				"sản phẩm %s không có số lượng nền (snapshot) — cần kiểm tra lại", productLabel(id))
			anomalies = append(anomalies, msg)
			itemAnomalies = append(itemAnomalies, dto.SubmissionItemWarning{
				InventoryItemID: id,
				Code:            dto.SubmissionItemWarningNoBaseline,
				Message:         msg,
			})
			baseline = decimal.Zero
		}

		// Anomaly: counted exceeds baseline. Surface it; the true counted flows through
		// so the overage is applied as a stock-up at process time.
		if hasBaseline && counted.GreaterThan(baseline) {
			msg := fmt.Sprintf(
				"tổng số lượng kiểm đếm của sản phẩm %s là %s vượt quá số lượng nền %s — cần kiểm tra lại",
				productLabel(id), counted.String(), baseline.String())
			anomalies = append(anomalies, msg)
			itemAnomalies = append(itemAnomalies, dto.SubmissionItemWarning{
				InventoryItemID: id,
				Code:            dto.SubmissionItemWarningOverage,
				Message:         msg,
			})
		}

		// No snapshot row (defensive; the write guard blocks this): emit the zero
		// baseline so apply neither consumes nor stocks up.
		quantity := counted
		if !hasBaseline {
			quantity = baseline
		}
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
		Label:         aggregateReviewLabel(rows, managerOwned),
		Anomalies:     anomalies,
		ItemAnomalies: itemAnomalies,
		Breakdown:     breakdownLines,
	}, nil
}

// sessionBreakdown emits one review-only entry per (inventory_item, creator,
// session-label, count-label) contribution, each carrying the session's
// label/creator/timestamp so the UI can group contributions by session.
// (creator + session-label) uniquely identifies a session, so sessions sharing a
// count-label stay distinct. Entries are ordered by inventory_item_id, then
// first-seen; quantities for a repeated key are summed.
func sessionBreakdown(rows []models.ReconciliationRequestItem) ([]dto.ReconcileItemBreakdown, error) {
	type key struct {
		itemID       uint
		createdBy    string
		sessionLabel string
		countLabel   string
	}
	type entry struct {
		breakdown *dto.ReconcileItemBreakdown
		seq       int
	}
	agg := make(map[key]*entry)
	order := make([]key, 0)

	sorted := make([]models.ReconciliationRequestItem, len(rows))
	copy(sorted, rows)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	for _, row := range sorted {
		if len(row.Payload) == 0 {
			continue
		}
		var parsed reconItemPayload
		if err := json.Unmarshal(row.Payload, &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse reconciliation item %d payload: %w", row.ID, err)
		}
		createdAt := ""
		if !row.CreatedAt.IsZero() {
			createdAt = row.CreatedAt.Format(pkg.DateTimeFormat)
		}
		for _, line := range parsed.Items {
			k := key{itemID: line.InventoryItemID, createdBy: row.CreatedBy, sessionLabel: row.Label, countLabel: line.Label}
			e, ok := agg[k]
			if !ok {
				e = &entry{
					breakdown: &dto.ReconcileItemBreakdown{
						InventoryItemID: line.InventoryItemID,
						Label:           line.Label,
						Quantity:        decimal.Zero,
						SessionLabel:    row.Label,
						CreatedBy:       row.CreatedBy,
						CreatedAt:       createdAt,
					},
					seq: len(order),
				}
				agg[k] = e
				order = append(order, k)
			}
			e.breakdown.Quantity = e.breakdown.Quantity.Add(line.Quantity)
		}
	}

	sort.SliceStable(order, func(i, j int) bool {
		a, b := agg[order[i]], agg[order[j]]
		if a.breakdown.InventoryItemID != b.breakdown.InventoryItemID {
			return a.breakdown.InventoryItemID < b.breakdown.InventoryItemID
		}
		return a.seq < b.seq
	})

	var lines []dto.ReconcileItemBreakdown
	for _, k := range order {
		lines = append(lines, *agg[k].breakdown)
	}
	return lines, nil
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
