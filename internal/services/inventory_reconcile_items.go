package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// maxReconItemLabelLength is the max reconciliation label length, measured in runes.
const maxReconItemLabelLength = 255

// loadActiveReconcileParent loads the parent submission for a child-item op and
// confirms it is an initiated reconcile (has snapshot rows), under a row-level
// write lock so a concurrent terminal transition is serialized against it.
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

	// The parent must still be in flight (active reconcile: pending processing,
	// reconcile_status open/closed) — not a terminal canceled/processed reconcile.
	if !submission.IsActiveReconcile() {
		return nil, pkg.ErrReconParentNotInFlight(ctx, submissionID, string(submission.ReconcileStatus))
	}
	return submission, nil
}

// guardParentEditable locks out staff once a reconcile is closed or beyond; only
// recon_manage holders may edit while closed, and everyone may edit while open.
func (s *inventoryService) guardParentEditable(ctx context.Context, parent *models.InventorySubmission) error {
	if parent.ReconcileStatus == models.ReconcileLifecycleStatusOpen {
		return nil
	}
	if pkg.HasPermission(ctx, pkg.RBACResourceInventorySubmissions, pkg.RBACActionReconManage) {
		if parent.ReconcileStatus == models.ReconcileLifecycleStatusClosed {
			return nil
		}
	}
	return pkg.ErrReconSubmissionClosed(ctx, parent.ID, string(parent.ReconcileStatus))
}

// loadItemInParent loads a child item and enforces that it belongs to the path-scoped parent.
func (s *inventoryService) loadItemInParent(ctx context.Context, submissionID, itemID uint) (*models.ReconciliationRequestItem, error) {
	item, err := s.reconItemRepo.GetByID(ctx, itemID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.ErrReconItemNotFound(ctx, itemID)
		}
		return nil, fmt.Errorf("failed to load reconciliation item: %w", err)
	}

	if item.SubmissionID != submissionID {
		return nil, pkg.ErrReconItemNotInParent(ctx, itemID, submissionID)
	}
	return item, nil
}

// guardOwnership enforces that the caller created the row; recon_manage holders bypass it.
func (s *inventoryService) guardOwnership(ctx context.Context, item *models.ReconciliationRequestItem) error {
	if pkg.HasPermission(ctx, pkg.RBACResourceInventorySubmissions, pkg.RBACActionReconManage) {
		return nil
	}
	userEmail, err := pkg.GetUserEmailFromContext(ctx)
	if err != nil {
		return pkg.ErrUnauthorized("user not authenticated", err)
	}
	if item.CreatedBy != userEmail {
		return pkg.ErrReconItemNotOwned(ctx)
	}
	return nil
}

// validateCountsAgainstSnapshot validates a child payload (non-negative quantities,
// per-row count-label rule, snapshot baseline present) and returns the normalized
// payload bytes. Counts may exceed the baseline; the overage is surfaced as a review
// anomaly and applied as a stock-up at process time.
func (s *inventoryService) validateCountsAgainstSnapshot(ctx context.Context, submissionID uint, items []dto.ReconciliationCountItem) (json.RawMessage, error) {
	baselines, err := s.snapshotRepo.GetPrevQuantitiesBySubmission(ctx, submissionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load snapshot baselines: %w", err)
	}

	for _, item := range items {
		// A nil Quantity means the field was omitted; an explicit 0 is a valid count.
		if item.Quantity == nil {
			return nil, pkg.ErrReconItemMissingQuantity(ctx, item.InventoryItemID)
		}
		quantity := *item.Quantity

		if quantity.IsNegative() {
			return nil, pkg.ErrReconItemNegativeQuantity(ctx, item.InventoryItemID)
		}

		if utf8.RuneCountInString(strings.TrimSpace(item.Label)) > maxReconItemLabelLength {
			return nil, pkg.ErrReconItemLabelTooLong(ctx, item.InventoryItemID, maxReconItemLabelLength)
		}

		if _, ok := baselines[item.InventoryItemID]; !ok {
			return nil, pkg.ErrReconItemNoSnapshotBaseline(ctx, s.resolveProductName(ctx, item.InventoryItemID))
		}
	}

	if err := validateCountLabelDistinctness(ctx, items); err != nil {
		return nil, err
	}

	payload := buildReconItemPayload(items)
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reconciliation item payload: %w", err)
	}
	return bytes, nil
}

// resolveProductName resolves an inventory item's product display name for a
// rejection message, returning "" on repo error, missing item, or soft-deleted product.
func (s *inventoryService) resolveProductName(ctx context.Context, itemID uint) string {
	items, err := s.inventoryItemRepo.GetByIDs(ctx, []uint{itemID})
	if err != nil {
		return ""
	}
	if it, ok := s.buildItemMap(items)[itemID]; ok && it.Product != nil {
		return it.Product.Name
	}
	return ""
}

// resolveProductNames resolves product display names for inventory item ids,
// omitting any that error, are missing, or have a soft-deleted product.
func (s *inventoryService) resolveProductNames(ctx context.Context, itemIDs []uint) map[uint]string {
	names := make(map[uint]string, len(itemIDs))
	if len(itemIDs) == 0 {
		return names
	}
	items, err := s.inventoryItemRepo.GetByIDs(ctx, itemIDs)
	if err != nil {
		return names
	}
	for id, it := range s.buildItemMap(items) {
		if it.Product != nil && it.Product.Name != "" {
			names[id] = it.Product.Name
		}
	}
	return names
}

// validateCountLabelDistinctness requires each item's counts to have distinct trimmed
// labels with at most one blank, within the payload only.
func validateCountLabelDistinctness(ctx context.Context, items []dto.ReconciliationCountItem) error {
	blanks := make(map[uint]int)
	nonBlank := make(map[uint]map[string]struct{})
	for _, item := range items {
		label := strings.TrimSpace(item.Label)
		if label == "" {
			blanks[item.InventoryItemID]++
			if blanks[item.InventoryItemID] > 1 {
				return pkg.ErrReconItemLabelRequiredForDuplicate(ctx, item.InventoryItemID)
			}
			continue
		}
		set, ok := nonBlank[item.InventoryItemID]
		if !ok {
			set = make(map[string]struct{})
			nonBlank[item.InventoryItemID] = set
		}
		if _, clash := set[label]; clash {
			return pkg.ErrReconItemLabelConflict(ctx, item.InventoryItemID, label)
		}
		set[label] = struct{}{}
	}
	return nil
}

// validateRowLabel validates a count-session row label for ownerEmail: trimmed,
// within the rune cap, and distinct among the owner's OTHER live rows. A blank label
// is allowed as the owner's single unlabelled session; the current row (excludeItemID)
// is excluded. Returns the trimmed label to persist.
func validateRowLabel(ctx context.Context, ownerEmail, rawLabel string, excludeItemID uint, rows []models.ReconciliationRequestItem) (string, error) {
	label := strings.TrimSpace(rawLabel)
	if utf8.RuneCountInString(label) > maxReconItemLabelLength {
		return "", pkg.ErrReconRowLabelTooLong(ctx, maxReconItemLabelLength)
	}

	otherLabels := make(map[string]struct{})
	for _, row := range rows {
		if row.ID == excludeItemID || row.CreatedBy != ownerEmail {
			continue
		}
		otherLabels[strings.TrimSpace(row.Label)] = struct{}{}
	}

	if _, clash := otherLabels[label]; clash {
		if label == "" {
			return "", pkg.ErrReconRowLabelRequired(ctx)
		}
		return "", pkg.ErrReconRowLabelConflict(ctx, label)
	}
	return label, nil
}

// reconItemPayload is the on-row JSON shape for a child item: counts only.
type reconItemPayload struct {
	Items []reconItemPayloadLine `json:"items"`
}

type reconItemPayloadLine struct {
	InventoryItemID uint            `json:"inventory_item_id"`
	Quantity        decimal.Decimal `json:"quantity"`
	// Label is the optional free-text count identifier.
	Label string `json:"label,omitempty"`
}

// buildReconItemPayload builds the on-row payload, normalizing a nil Quantity to zero.
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
			Label:           strings.TrimSpace(item.Label),
		})
	}
	return reconItemPayload{Items: lines}
}

// CreateReconciliationItem files a new in_progress child item owned by the caller under a parent initiated reconcile.
func (s *inventoryService) CreateReconciliationItem(ctx context.Context, req dto.CreateReconciliationItemRequest) (*dto.ReconciliationItemResponse, error) {
	var created *models.ReconciliationRequestItem
	err := s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		parent, err := s.loadActiveReconcileParent(txCtx, req.SubmissionID)
		if err != nil {
			return err
		}
		if err := s.guardParentEditable(txCtx, parent); err != nil {
			return err
		}

		siblingRows, err := s.reconItemRepo.ListBySubmission(txCtx, req.SubmissionID)
		if err != nil {
			return fmt.Errorf("failed to load reconciliation rows: %w", err)
		}

		callerEmail, err := pkg.GetUserEmailFromContext(txCtx)
		if err != nil {
			return pkg.ErrUnauthorized("user not authenticated", err)
		}
		rowLabel, err := validateRowLabel(txCtx, callerEmail, req.Label, 0, siblingRows)
		if err != nil {
			return err
		}

		payloadBytes, err := s.validateCountsAgainstSnapshot(txCtx, req.SubmissionID, req.Items)
		if err != nil {
			return err
		}

		item := &models.ReconciliationRequestItem{
			SubmissionID: req.SubmissionID,
			Label:        rowLabel,
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
	names, err := s.resolveReconItemProductNames(ctx, created)
	if err != nil {
		return nil, err
	}
	resp, err := toReconciliationItemResponse(created, names)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateReconciliationItem replaces the counted payload of a child item.
func (s *inventoryService) UpdateReconciliationItem(ctx context.Context, req dto.UpdateReconciliationItemRequest) (*dto.ReconciliationItemResponse, error) {
	var updated *models.ReconciliationRequestItem
	err := s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		parent, err := s.loadActiveReconcileParent(txCtx, req.SubmissionID)
		if err != nil {
			return err
		}
		if err := s.guardParentEditable(txCtx, parent); err != nil {
			return err
		}
		item, err := s.loadItemInParent(txCtx, req.SubmissionID, req.ItemID)
		if err != nil {
			return err
		}
		if err := s.guardOwnership(txCtx, item); err != nil {
			return err
		}

		siblingRows, err := s.reconItemRepo.ListBySubmission(txCtx, req.SubmissionID)
		if err != nil {
			return fmt.Errorf("failed to load reconciliation rows: %w", err)
		}

		rowLabel, err := validateRowLabel(txCtx, item.CreatedBy, req.Label, item.ID, siblingRows)
		if err != nil {
			return err
		}

		payloadBytes, err := s.validateCountsAgainstSnapshot(txCtx, req.SubmissionID, req.Items)
		if err != nil {
			return err
		}

		if err := s.reconItemRepo.UpdateLabelPayloadAndStatus(txCtx, item.ID, rowLabel, payloadBytes, models.ReconciliationRequestItemStatusInProgress); err != nil {
			return fmt.Errorf("failed to update reconciliation item: %w", err)
		}
		item.Label = rowLabel
		item.Payload = payloadBytes
		item.Status = models.ReconciliationRequestItemStatusInProgress
		updated = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	names, err := s.resolveReconItemProductNames(ctx, updated)
	if err != nil {
		return nil, err
	}
	resp, err := toReconciliationItemResponse(updated, names)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// SetReconciliationItemReadiness toggles a staff count session between in_progress and ready_for_review.
func (s *inventoryService) SetReconciliationItemReadiness(ctx context.Context, req dto.SetReconciliationItemReadinessRequest) (*dto.ReconciliationItemResponse, error) {
	status := models.ReconciliationRequestItemStatus(req.Status)
	if !status.IsValid() {
		return nil, pkg.ErrValidation("invalid reconciliation item status", nil)
	}

	var updated *models.ReconciliationRequestItem
	err := s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		parent, err := s.loadActiveReconcileParent(txCtx, req.SubmissionID)
		if err != nil {
			return err
		}
		if parent.ReconcileStatus != models.ReconcileLifecycleStatusOpen {
			return pkg.ErrReconSubmissionClosed(txCtx, parent.ID, string(parent.ReconcileStatus))
		}
		item, err := s.loadItemInParent(txCtx, req.SubmissionID, req.ItemID)
		if err != nil {
			return err
		}
		// Staff-only: recon_manage holders are rejected even for a row they own.
		if pkg.HasPermission(txCtx, pkg.RBACResourceInventorySubmissions, pkg.RBACActionReconManage) {
			return pkg.ErrForbidden("readiness toggle is restricted to staff", nil)
		}
		callerEmail, err := pkg.GetUserEmailFromContext(txCtx)
		if err != nil {
			return pkg.ErrUnauthorized("user not authenticated", err)
		}
		if item.CreatedBy != callerEmail {
			return pkg.ErrReconItemNotOwned(txCtx)
		}

		if err := s.reconItemRepo.UpdateStatus(txCtx, item.ID, status); err != nil {
			return fmt.Errorf("failed to update reconciliation item readiness: %w", err)
		}
		item.Status = status
		updated = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	names, err := s.resolveReconItemProductNames(ctx, updated)
	if err != nil {
		return nil, err
	}
	resp, err := toReconciliationItemResponse(updated, names)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteReconciliationItem soft-deletes a child item.
func (s *inventoryService) DeleteReconciliationItem(ctx context.Context, req dto.DeleteReconciliationItemRequest) error {
	return s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		parent, err := s.loadActiveReconcileParent(txCtx, req.SubmissionID)
		if err != nil {
			return err
		}
		if err := s.guardParentEditable(txCtx, parent); err != nil {
			return err
		}
		item, err := s.loadItemInParent(txCtx, req.SubmissionID, req.ItemID)
		if err != nil {
			return err
		}
		if err := s.guardOwnership(txCtx, item); err != nil {
			return err
		}

		if err := s.reconItemRepo.SoftDelete(txCtx, item.ID); err != nil {
			return fmt.Errorf("failed to soft-delete reconciliation item: %w", err)
		}
		return nil
	})
}

// ListReconciliationItems returns the live count-session rows of an initiated reconcile.
// recon_manage holders see all rows; staff see only their own, id-ascending.
func (s *inventoryService) ListReconciliationItems(ctx context.Context, submissionID uint) ([]dto.ReconciliationItemResponse, error) {
	submission, err := s.inventorySubmissionRepo.GetByID(ctx, submissionID)
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

	var rows []models.ReconciliationRequestItem
	if pkg.HasPermission(ctx, pkg.RBACResourceInventorySubmissions, pkg.RBACActionReconManage) {
		rows, err = s.reconItemRepo.ListBySubmission(ctx, submissionID)
		if err != nil {
			return nil, fmt.Errorf("failed to list reconciliation items: %w", err)
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	} else {
		callerEmail, emailErr := pkg.GetUserEmailFromContext(ctx)
		if emailErr != nil {
			return nil, pkg.ErrUnauthorized("user not authenticated", emailErr)
		}
		rows, err = s.reconItemRepo.ListBySubmissionAndCreator(ctx, submissionID, callerEmail)
		if err != nil {
			return nil, fmt.Errorf("failed to list reconciliation items: %w", err)
		}
	}

	ptrs := make([]*models.ReconciliationRequestItem, len(rows))
	for i := range rows {
		ptrs[i] = &rows[i]
	}
	names, err := s.resolveReconItemProductNames(ctx, ptrs...)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.ReconciliationItemResponse, 0, len(rows))
	for i := range rows {
		resp, mapErr := toReconciliationItemResponse(&rows[i], names)
		if mapErr != nil {
			return nil, mapErr
		}
		responses = append(responses, resp)
	}
	return responses, nil
}

// resolveReconItemProductNames resolves product display names for every distinct
// inventory_item across the rows' payloads via the by-id resolver, so
// removed/discontinued items (absent from the inventory-items list) still resolve.
func (s *inventoryService) resolveReconItemProductNames(ctx context.Context, rows ...*models.ReconciliationRequestItem) (map[uint]string, error) {
	seen := make(map[uint]struct{})
	ids := make([]uint, 0)
	for _, row := range rows {
		if row == nil || len(row.Payload) == 0 {
			continue
		}
		var parsed reconItemPayload
		if err := json.Unmarshal(row.Payload, &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse reconciliation item %d payload: %w", row.ID, err)
		}
		for _, line := range parsed.Items {
			if _, ok := seen[line.InventoryItemID]; !ok {
				seen[line.InventoryItemID] = struct{}{}
				ids = append(ids, line.InventoryItemID)
			}
		}
	}
	return s.resolveProductNames(ctx, ids), nil
}

// toReconciliationItemResponse maps a persisted row to the FE row response shape,
// stamping each line's product name from productNames.
func toReconciliationItemResponse(row *models.ReconciliationRequestItem, productNames map[uint]string) (dto.ReconciliationItemResponse, error) {
	lines := make([]dto.ReconciliationItemLine, 0)
	if len(row.Payload) > 0 {
		var parsed reconItemPayload
		if err := json.Unmarshal(row.Payload, &parsed); err != nil {
			return dto.ReconciliationItemResponse{}, fmt.Errorf("failed to parse reconciliation item %d payload: %w", row.ID, err)
		}
		lines = make([]dto.ReconciliationItemLine, 0, len(parsed.Items))
		for _, line := range parsed.Items {
			lines = append(lines, dto.ReconciliationItemLine{
				InventoryItemID: line.InventoryItemID,
				Quantity:        line.Quantity,
				Label:           line.Label,
				ProductName:     productNames[line.InventoryItemID],
			})
		}
	}
	return dto.ReconciliationItemResponse{
		ID:           row.ID,
		SubmissionID: row.SubmissionID,
		Label:        row.Label,
		Status:       string(row.Status),
		Items:        lines,
		CreatedBy:    row.CreatedBy,
		CreatedAt:    row.CreatedAt.Format(pkg.DateTimeFormat),
		UpdatedBy:    row.UpdatedBy,
		UpdatedAt:    row.UpdatedAt.Format(pkg.DateTimeFormat),
	}, nil
}
