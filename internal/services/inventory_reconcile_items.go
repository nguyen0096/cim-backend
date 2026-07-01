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

// maxReconItemLabelLength is the app-validated max length of a reconciliation label
// (issue #73), measured in RUNES (not bytes) so multibyte Vietnamese labels are not
// rejected below the documented 255-character cap. It applies to BOTH the per-COUNT
// labels in the JSONB payload and the ROW-level count-session label column. The
// count labels live in schemaless JSONB (no DB constraint); the row label column is
// VARCHAR(255), but it is also rune-checked here so a multibyte label up to 255
// runes is accepted and the error is the localized domain error, not a DB error.
const maxReconItemLabelLength = 255

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
	// stays at approval_status=pending through the open/closed phases until the
	// Start-Processing apply, but it can already be REJECTED via ProcessSubmission
	// (which sets approval_status=rejected + processing_status=canceled while
	// leaving the snapshot rows in place). Without this guard, callers could keep
	// creating/editing/deleting child items under a terminal parent. Any
	// non-pending approval status (rejected, or post-apply approved) closes the
	// window. Evaluated while holding the FOR UPDATE lock acquired above, in the
	// same tx as the child write, so a concurrent terminal transition is serialized
	// against it rather than slipping through a stale read.
	if submission.ApprovalStatus != models.InventorySubmissionApprovalStatusPending {
		return nil, pkg.ErrReconParentNotInFlight(ctx, submissionID, string(submission.ApprovalStatus))
	}
	return submission, nil
}

// guardParentEditable enforces the locked staff-immutability rule (epic #38,
// Part 6): once a reconcile is `closed` (or beyond — processing/processed), staff
// can no longer mutate child rows; only admin/accountant (the recon_manage
// holders) may still edit while closed. While `open`, everyone with child-item
// permission may edit.
//
// It runs under the parent FOR UPDATE lock held by loadActiveReconcileParent and
// in the same tx as the child write, so a concurrent `close` is serialized
// against an in-flight staff edit: either the close commits first and this guard
// (re-reading the locked parent row in this tx) sees `closed` and rejects the
// staff write, or the staff write holds the lock and the close blocks until it
// commits. No staff edit can interleave past a committed close.
func (s *inventoryService) guardParentEditable(ctx context.Context, parent *models.InventorySubmission) error {
	if parent.ReconcileStatus == models.ReconcileLifecycleStatusOpen {
		return nil
	}
	// Not open => closed/processing/processed. Admin/accountant (recon_manage) may
	// still edit while closed; everyone else (staff) is locked out.
	if pkg.HasPermission(ctx, pkg.RBACResourceInventorySubmissions, pkg.RBACActionReconManage) {
		// Admin/accountant may edit while `closed`, but never once the apply has
		// begun (processing) or completed (processed) — those are terminal/in-flight.
		if parent.ReconcileStatus == models.ReconcileLifecycleStatusClosed {
			return nil
		}
	}
	return pkg.ErrReconSubmissionClosed(ctx, parent.ID, string(parent.ReconcileStatus))
}

// loadItemInParent loads a child item and enforces that it belongs to the
// path-scoped parent. Ownership is enforced separately by the caller (staff are
// ownership-scoped; admin/accountant editing a closed submission are not — they
// may edit any staff row, epic #38 Part 6). There is no longer an applied/
// immutable per-row guard: row immutability is enforced at the submission level
// (guardParentEditable) by the parent's reconcile_status.
func (s *inventoryService) loadItemInParent(ctx context.Context, submissionID, itemID uint) (*models.ReconciliationRequestItem, error) {
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
	return item, nil
}

// guardOwnership enforces that the caller created the row. Ownership is keyed on
// models.Base.CreatedBy (the contributor's email), matching how every other
// table records its actor. Admin/accountant (recon_manage holders) bypass this:
// while a submission is `closed` they review and may edit ANY staff row (epic
// #38, Part 6); staff remain restricted to their own rows.
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

// validateCountsAgainstSnapshot enforces the per-item validation for a child
// payload: non-negative quantities, the issue-#73 PER-ROW count-label rule (within
// THIS row's payload, counts of the same inventory_item_id must have DISTINCT
// labels with AT MOST ONE blank; label > 255 RUNES is rejected), every counted
// item has a snapshot baseline, and — the data-correctness guard — the AGGREGATE
// counted quantity per item across ALL live (non-deleted) sibling child rows of the
// same parent plus this payload must not exceed the snapshot baseline (S2: counted
// > snapshot is rejected; there is no positive-adjustment mechanism in this epic).
// The snapshot baseline is read from the parent's snapshot rows (the sole source of
// truth for prev_quantity).
//
// Label scope (issue #73 re-scope, FE contract cim-ui #42): the count-label
// distinctness rule is PER ROW — validated only against the row being written, NOT
// across sibling rows. The FE edits one row at a time and the ROW-level label (on
// the row itself) distinguishes rows, so two different rows MAY reuse the same
// count label (or both be blank). The aggregate QUANTITY guard below stays
// per-submission (summed across siblings) — only the label comparison narrowed.
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
// siblingRows is the full set of live child rows for the parent (loaded ONCE by the
// caller under the FOR UPDATE lock and shared with validateRowLabel); the row with
// id excludeItemID is filtered out of the aggregate here.
//
// Returns the normalized counted payload bytes to persist.
func (s *inventoryService) validateCountsAgainstSnapshot(ctx context.Context, submissionID uint, items []dto.ReconciliationCountItem, excludeItemID uint, siblingRows []models.ReconciliationRequestItem) (json.RawMessage, error) {
	baselines, err := s.snapshotRepo.GetPrevQuantitiesBySubmission(ctx, submissionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load snapshot baselines: %w", err)
	}

	// Seed the per-item running counted totals from all OTHER live child rows of this
	// parent (excluding the row being updated). The rows were read under the parent
	// FOR UPDATE lock in this tx; siblings are stable for the duration. We then fold
	// THIS payload's quantities into the running totals so the aggregate quantity
	// guard is enforced over the union (this payload + the other live rows).
	totals, err := sumLiveSiblingCounts(siblingRows, excludeItemID)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
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

		// Rune-length is per-line and order-independent (Count RUNES, not bytes — the
		// 255-CHARACTER cap must not reject multibyte Vietnamese labels).
		if utf8.RuneCountInString(strings.TrimSpace(item.Label)) > maxReconItemLabelLength {
			return nil, pkg.ErrReconItemLabelTooLong(ctx, item.InventoryItemID, maxReconItemLabelLength)
		}

		baseline, ok := baselines[item.InventoryItemID]
		if !ok {
			// Resolve the product NAME lazily, ONLY on this rejecting branch — the
			// happy path issues no extra query. Fail-fast guarantees at most one
			// such lookup per request (see resolveProductName).
			return nil, pkg.ErrReconItemNoSnapshotBaseline(ctx, s.resolveProductName(ctx, item.InventoryItemID))
		}
		// Per-row guard first (clearer single-row message when this row alone exceeds).
		if quantity.GreaterThan(baseline) {
			return nil, pkg.ErrReconItemCountExceedsBaseline(ctx, s.resolveProductName(ctx, item.InventoryItemID), quantity, baseline)
		}
		// Aggregate guard: this line + every prior line of this payload for the same
		// item + every other live sibling row's count for it.
		total := totals[item.InventoryItemID].Add(quantity)
		if total.GreaterThan(baseline) {
			return nil, pkg.ErrReconItemAggregateExceedsBaseline(ctx, s.resolveProductName(ctx, item.InventoryItemID), total, baseline)
		}
		totals[item.InventoryItemID] = total
	}

	// Count-label rule (issue #73, PER ROW) — evaluated on each item's label SET so the
	// result is ORDER-INSENSITIVE (a payload and any permutation of it validate the
	// same; Codex P2). Within THIS payload, counts for the same inventory_item_id must
	// have DISTINCT labels with AT MOST ONE blank: ≥2 blanks for an item is the
	// duplicate-needs-label error; any repeated non-blank label is the conflict error.
	// There is no positional "first count may be blank" notion — a lone blank is fine,
	// a second blank is not, regardless of position. Labels are NOT compared across
	// sibling rows (the re-scope): two different rows may reuse a count label / both be
	// blank — the row-level label distinguishes them.
	if err := validateCountLabelDistinctness(ctx, items); err != nil {
		return nil, err
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

// resolveProductName resolves a single inventory item's product display name for
// a count-baseline rejection message. It is called ONLY on a rejecting branch of
// validateCountsAgainstSnapshot, which is fail-fast — so a request rejects on at
// most one row and this issues at most one GetByIDs round-trip; the happy path
// never queries here.
//
// GetByIDs is Unscoped() but Preload("Product") is scoped, so a soft-deleted
// product comes back with Product == nil even though the item itself resolves.
// We guard explicitly (mirroring inventory_service.go), returning "" on repo
// error, item-missing, or nil/soft-deleted product — never a nil deref. An empty
// name renders inside guillemets («») so the message stays legible.
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

// validateCountLabelDistinctness enforces the issue-#73 PER-ROW count-label rule on
// each inventory_item_id's label SET, independently of array position (Codex P2):
// within the payload, an item's counts must have DISTINCT labels with AT MOST ONE
// blank. For any permutation of the same payload the verdict is identical — it
// inspects the whole group's labels, not "the first one seen". A second blank for an
// item -> ErrReconItemLabelRequiredForDuplicate; a repeated non-blank label ->
// ErrReconItemLabelConflict. Labels are trimmed (matching how they persist) and
// compared case-sensitively (free text; no normalization).
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

// validateRowLabel enforces the issue-#73 ROW-level label rule for a Create/Update
// of a count session, scoped per (submission, ownerEmail):
//
//   - trim + reject label > maxReconItemLabelLength RUNES (utf8.RuneCountInString,
//     not bytes — Vietnamese is multibyte);
//   - REQUIRED once the owner already has >= 1 OTHER live row in this submission
//     (the first/only row may be blank; it is not retro-labeled);
//   - DISTINCT among the owner's live rows (no two of the owner's rows share a label;
//     blanks can never collide because a blank is only allowed when it is the owner's
//     only row).
//
// ownerEmail is the row's owner: the caller on Create (Base.BeforeCreate stamps
// created_by = caller) and the existing item.CreatedBy on Update (ownership is not
// reassigned). excludeItemID is the row being updated (0 on Create) so the row's own
// current label is not compared against itself.
//
// rows is the full set of live child rows for the parent (loaded ONCE by the caller
// under the FOR UPDATE lock and shared with validateCountsAgainstSnapshot), so the
// owner's other live rows are stable against concurrent writes for the same parent.
// Returns the trimmed label to persist.
func validateRowLabel(ctx context.Context, ownerEmail, rawLabel string, excludeItemID uint, rows []models.ReconciliationRequestItem) (string, error) {
	label := strings.TrimSpace(rawLabel)
	if utf8.RuneCountInString(label) > maxReconItemLabelLength {
		return "", pkg.ErrReconRowLabelTooLong(ctx, maxReconItemLabelLength)
	}

	otherLabels := make(map[string]struct{})
	ownerOtherRows := 0
	for _, row := range rows {
		if row.ID == excludeItemID {
			continue
		}
		if row.CreatedBy != ownerEmail {
			continue
		}
		ownerOtherRows++
		otherLabels[strings.TrimSpace(row.Label)] = struct{}{}
	}

	// Required once the owner already has another live row: a 2nd+ session must be
	// labelled so it can be told apart from the first.
	if ownerOtherRows > 0 && label == "" {
		return "", pkg.ErrReconRowLabelRequired(ctx)
	}
	// Distinct among the owner's live rows.
	if label != "" {
		if _, clash := otherLabels[label]; clash {
			return "", pkg.ErrReconRowLabelConflict(ctx, label)
		}
	}
	return label, nil
}

// sumLiveSiblingCounts returns, per inventory_item_id, the total counted quantity
// across the supplied live (non-soft-deleted) child rows, EXCLUDING the row with id
// excludeItemID (0 = exclude nothing). The rows are loaded once by the caller under
// the parent FOR UPDATE lock (so the aggregate check is race-safe against concurrent
// staff submissions for the same parent).
//
// Note (issue #73 re-scope): this returns QUANTITY totals only. The count-label
// distinctness rule is now PER ROW (validated within the payload being written, not
// across siblings — see validateCountsAgainstSnapshot), because the FE edits one row
// at a time and the row-level label distinguishes rows; two different rows may
// legitimately reuse the same count label. The aggregate quantity-vs-snapshot guard
// stays per-submission (summed across siblings), which is why the totals are still
// gathered here.
func sumLiveSiblingCounts(rows []models.ReconciliationRequestItem, excludeItemID uint) (map[uint]decimal.Decimal, error) {
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
	// Label is the optional free-text count identifier (issue #73). Persisted as-is
	// (already validated/trimmed by validateCountsAgainstSnapshot); omitempty so a
	// blank label keeps the stored shape backward-compatible with pre-#73 rows.
	Label string `json:"label,omitempty"`
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
			Label:           strings.TrimSpace(item.Label),
		})
	}
	return reconItemPayload{Items: lines}
}

// CreateReconciliationItem files a new staff child item under a parent initiated
// reconcile. The new row is in_progress and is owned by the caller. Counts are
// validated against the snapshot baseline. Parent load + create run in one tx so
// a parent that disappears (or loses its snapshots) mid-create can't leave an
// orphan row. Rejected once the parent is closed (staff) — see guardParentEditable.
func (s *inventoryService) CreateReconciliationItem(ctx context.Context, req dto.CreateReconciliationItemRequest) (*dto.ReconciliationItemResponse, error) {
	var created *models.ReconciliationRequestItem
	err := s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		parent, err := s.loadActiveReconcileParent(txCtx, req.SubmissionID)
		if err != nil {
			return err
		}
		// Staff-immutability: once the submission is closed, only admin/accountant
		// may file/edit rows (under the parent FOR UPDATE lock).
		if err := s.guardParentEditable(txCtx, parent); err != nil {
			return err
		}

		// Load the parent's live child rows ONCE under the FOR UPDATE lock; both the
		// row-label rule and the aggregate-vs-snapshot guard read from this set.
		siblingRows, err := s.reconItemRepo.ListBySubmission(txCtx, req.SubmissionID)
		if err != nil {
			return fmt.Errorf("failed to load reconciliation rows: %w", err)
		}

		// Row-level label rule (issue #73), scoped to the caller's own live rows. The
		// new row's owner is the caller (Base.BeforeCreate stamps created_by).
		callerEmail, err := pkg.GetUserEmailFromContext(txCtx)
		if err != nil {
			return pkg.ErrUnauthorized("user not authenticated", err)
		}
		rowLabel, err := validateRowLabel(txCtx, callerEmail, req.Label, 0, siblingRows)
		if err != nil {
			return err
		}

		// excludeItemID = 0 on create: there is no existing row to exclude from the
		// sibling aggregate; this new row's counts are the payload being validated.
		payloadBytes, err := s.validateCountsAgainstSnapshot(txCtx, req.SubmissionID, req.Items, 0, siblingRows)
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
	resp, err := toReconciliationItemResponse(created)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateReconciliationItem replaces the counted payload of a child item. Staff
// may edit only their own rows while the parent is `open`; admin/accountant may
// edit any row while the parent is `closed` (the review edit). The row stays
// in_progress (the only status). The whole op is one tx (load + guard + validate
// + write) under the parent FOR UPDATE lock.
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

		// Load the parent's live child rows ONCE under the FOR UPDATE lock; both the
		// row-label rule and the aggregate-vs-snapshot guard read from this set.
		siblingRows, err := s.reconItemRepo.ListBySubmission(txCtx, req.SubmissionID)
		if err != nil {
			return fmt.Errorf("failed to load reconciliation rows: %w", err)
		}

		// Row-level label rule (issue #73), scoped to the row OWNER's live rows (not
		// the caller's — ownership is not reassigned on an admin/accountant review
		// edit). Exclude this row so its own current label is not compared to itself.
		rowLabel, err := validateRowLabel(txCtx, item.CreatedBy, req.Label, item.ID, siblingRows)
		if err != nil {
			return err
		}

		// Exclude this row from the sibling aggregate: its current persisted counts
		// are being REPLACED by req.Items, so the new payload — not the old one — is
		// what counts toward the per-item baseline.
		payloadBytes, err := s.validateCountsAgainstSnapshot(txCtx, req.SubmissionID, req.Items, item.ID, siblingRows)
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
	resp, err := toReconciliationItemResponse(updated)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// SetReconciliationItemReadiness toggles a staff count session in_progress <->
// ready_for_review. Staff-only and self-scoped: recon_manage holders are rejected
// outright, and a staff caller may toggle only their own row while the parent is
// open. Runs in one tx under the parent FOR UPDATE lock so it serializes against close.
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
		// Staff-only: recon_manage holders (admin/accountant) are rejected even
		// for a row they own, so manager-owned sessions never drive review_label.
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
	resp, err := toReconciliationItemResponse(updated)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteReconciliationItem soft-deletes a child item. Staff may delete only their
// own rows while the parent is `open`; admin/accountant may delete any row while
// the parent is `closed`. The whole op is one tx under the parent FOR UPDATE lock.
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

// ListReconciliationItems returns the live (non-soft-deleted) count-session rows of
// an initiated reconcile, each with its row Label and its flattened count lines
// (issue #73 / FE contract cim-ui #42). Read-only.
//
// RBAC scoping: holders of recon_manage (admin/accountant) see ALL rows under the
// submission; everyone else (staff) sees only their OWN rows (created_by == caller).
// Rows are returned in deterministic id-ascending order.
//
// Unlike the write paths this does not take the parent FOR UPDATE lock or apply the
// editability guard — it is a pure read that must work while the submission is open
// OR closed (the admin reviews rows after close). It still confirms the parent is an
// initiated reconcile (a reconcile with snapshot rows) so callers cannot read rows
// across an unrelated submission.
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
		// Admin/accountant: all rows under the submission, ordered by id for stability.
		rows, err = s.reconItemRepo.ListBySubmission(ctx, submissionID)
		if err != nil {
			return nil, fmt.Errorf("failed to list reconciliation items: %w", err)
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	} else {
		// Staff: only their own rows (created_by == caller), already id-ascending.
		callerEmail, emailErr := pkg.GetUserEmailFromContext(ctx)
		if emailErr != nil {
			return nil, pkg.ErrUnauthorized("user not authenticated", emailErr)
		}
		rows, err = s.reconItemRepo.ListBySubmissionAndCreator(ctx, submissionID, callerEmail)
		if err != nil {
			return nil, fmt.Errorf("failed to list reconciliation items: %w", err)
		}
	}

	responses := make([]dto.ReconciliationItemResponse, 0, len(rows))
	for i := range rows {
		resp, mapErr := toReconciliationItemResponse(&rows[i])
		if mapErr != nil {
			return nil, mapErr
		}
		responses = append(responses, resp)
	}
	return responses, nil
}

// toReconciliationItemResponse maps a persisted row to the FE row response shape,
// flattening the JSONB payload into the lean {inventory_item_id, quantity, label}
// lines and formatting timestamps with pkg.DateTimeFormat (matching the other
// submission responses). Used by Create / Update / List so all three return the
// contract row shape rather than the raw model.
func toReconciliationItemResponse(row *models.ReconciliationRequestItem) (dto.ReconciliationItemResponse, error) {
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
