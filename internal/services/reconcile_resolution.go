package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// reconcileOneOffReason is stamped on submissions resolved by the #43 one-off.
// It covers both reconcile shrinkage sells and folded-in disposals.
const reconcileOneOffReason = "resolved by one-off data-fix #43 (FIFO backdated reconcile sells + disposals)"

// parsedReconcileSubmission is a submission already decoded into the fields the
// chaining algorithm needs. Kept DB-free so the drop math is unit-testable.
// It covers BOTH reconcile and dispose submissions; Type selects the per-item
// drop math (reconcile clamp vs direct remove-N) and the synthesized txn type.
type parsedReconcileSubmission struct {
	SubmissionID uint
	Type         models.SubmissionType
	CreatedAt    time.Time
	Items        []parsedReconcileItem
}

type parsedReconcileItem struct {
	InventoryItemID uint
	// PrevQuantity is the reconcile snapshot count; ignored for dispose items.
	PrevQuantity decimal.Decimal
	// ActualCount, for reconcile, is the actual counted quantity (an absolute
	// count, payload "quantity"). For dispose it is the direct remove-N amount.
	// Must be non-nil for every item; validated before computeReconcileDrops.
	ActualCount decimal.Decimal
}

// computeReconcileDrops is the pure (DB-free) core of the resolution.
//
// Given the submissions (chronological order is enforced here by created_at
// ASC) and each item's starting stock, it walks the chain maintaining
// consumedSoFar per item and computes, for each submission:
//
//	effective_prev = prev_quantity - consumedSoFar[item]
//	raw_delta      = effective_prev - actual_count
//	clamped_drop   = max(0, raw_delta)   // negative => count implies increase
//
// It returns one ItemResolution per item (in ascending item-id order for
// stable output) and, in submission-chain order, the per-submission
// itemConsumeQuantity maps (zero drops omitted) to feed consumeFIFO.
//
// chainOrder is the submission IDs in the exact order they were processed,
// aligned 1:1 with consumeQtyByChain and chainTypes.
//
// Reconcile and dispose submissions are interleaved strictly by created_at. For
// a dispose item the drop is the requested remove-N directly (PrevQuantity is
// ignored); it is pre-checked against the running stock (startStock minus the
// quantity already consumed by earlier submissions in the chain) and, if the
// removal would overdraw, the function HARD-FAILS with an error naming the
// dispose submission, item, and needed-vs-available — no clamping.
func computeReconcileDrops(
	subs []parsedReconcileSubmission,
	startStock map[uint]decimal.Decimal,
	productNames map[uint]string,
) (
	items []dto.ItemResolution,
	chainOrder []uint,
	chainTypes []models.SubmissionType,
	consumeQtyByChain []map[uint]decimal.Decimal,
	err error,
) {
	// Enforce chronological order (ASC by created_at; tie-break by id).
	ordered := make([]parsedReconcileSubmission, len(subs))
	copy(ordered, subs)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
			return ordered[i].SubmissionID < ordered[j].SubmissionID
		}
		return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
	})

	consumedSoFar := make(map[uint]decimal.Decimal)
	// itemRes accumulates per-item resolution; resByItem indexes into itemRes.
	itemRes := make(map[uint]*dto.ItemResolution)
	itemOrder := make([]uint, 0)

	ensureItem := func(id uint) *dto.ItemResolution {
		if r, ok := itemRes[id]; ok {
			return r
		}
		r := &dto.ItemResolution{
			InventoryItemID: id,
			ProductName:     productNames[id],
			StartStock:      startStock[id],
			FinalStock:      startStock[id],
		}
		itemRes[id] = r
		itemOrder = append(itemOrder, id)
		return r
	}

	isDispose := func(t models.SubmissionType) bool { return t == models.InventorySubmissionTypeDispose }

	for _, sub := range ordered {
		consumeQty := make(map[uint]decimal.Decimal)
		for _, it := range sub.Items {
			res := ensureItem(it.InventoryItemID)

			var drop decimal.Decimal
			var effectivePrev, rawDelta decimal.Decimal
			var clamped bool

			if isDispose(sub.Type) {
				// Dispose: remove-N directly. A NEGATIVE remove-N is invalid bad data
				// (it would imply adding stock and, since the drop>0 guard below skips
				// it, would otherwise be silently approved as a no-op). Hard-fail it
				// before any persist, like the insufficient-stock check below.
				if it.ActualCount.LessThan(decimal.Zero) {
					return nil, nil, nil, nil, pkg.NewAppError(pkg.ErrorCodeValidation,
						fmt.Sprintf("dispose submission %d has a negative quantity %s for inventory item %d: negative dispose quantities are not allowed",
							sub.SubmissionID, it.ActualCount.String(), it.InventoryItemID), nil)
				}
				// Pre-check against running stock so a removal can never overdraw the
				// on-hand quantity once earlier submissions in the chain are accounted
				// for.
				running := startStock[it.InventoryItemID].Sub(consumedSoFar[it.InventoryItemID])
				if it.ActualCount.GreaterThan(running) {
					return nil, nil, nil, nil, pkg.NewAppError(pkg.ErrorCodeValidation,
						fmt.Sprintf("dispose submission %d cannot remove %s of inventory item %d: only %s available (start %s minus %s already consumed)",
							sub.SubmissionID, it.ActualCount.String(), it.InventoryItemID,
							running.String(), startStock[it.InventoryItemID].String(),
							consumedSoFar[it.InventoryItemID].String()), nil)
				}
				drop = it.ActualCount
				// effectivePrev/rawDelta carry the running snapshot for visibility;
				// no clamp applies to disposals.
				effectivePrev = running
				rawDelta = drop
			} else {
				effectivePrev = it.PrevQuantity.Sub(consumedSoFar[it.InventoryItemID])
				rawDelta = effectivePrev.Sub(it.ActualCount)
				clamped = !rawDelta.GreaterThan(decimal.Zero) // rawDelta <= 0
				drop = rawDelta
				if clamped {
					drop = decimal.Zero
				}
			}

			res.Drops = append(res.Drops, dto.SubmissionDrop{
				SubmissionID:   sub.SubmissionID,
				SubmissionType: string(sub.Type),
				CreatedAt:      sub.CreatedAt,
				PrevQuantity:   it.PrevQuantity,
				ActualCount:    it.ActualCount,
				EffectivePrev:  effectivePrev,
				RawDelta:       rawDelta,
				ClampedDrop:    drop,
				Clamped:        clamped,
			})

			// Only positive drops affect stock. The guard applies to BOTH reconcile
			// (already clamped to >= 0 above) and dispose (a 0/negative remove-N must
			// not inject a phantom entry). Accumulate into the per-submission consume
			// map — a payload may list the SAME item id more than once, so the map
			// must hold the SUM of this submission's drops for that item to stay in
			// lockstep with TotalDrop/FinalStock/consumedSoFar (last-write-wins here
			// would make consumeFIFO consume less than the preview recorded).
			if drop.GreaterThan(decimal.Zero) {
				consumeQty[it.InventoryItemID] = consumeQty[it.InventoryItemID].Add(drop)
				res.TotalDrop = res.TotalDrop.Add(drop)
				if isDispose(sub.Type) {
					res.TotalDisposed = res.TotalDisposed.Add(drop)
				}
				res.FinalStock = res.FinalStock.Sub(drop)
				consumedSoFar[it.InventoryItemID] = consumedSoFar[it.InventoryItemID].Add(drop)
			}
		}
		chainOrder = append(chainOrder, sub.SubmissionID)
		chainTypes = append(chainTypes, sub.Type)
		consumeQtyByChain = append(consumeQtyByChain, consumeQty)
	}

	sort.Slice(itemOrder, func(i, j int) bool { return itemOrder[i] < itemOrder[j] })
	items = make([]dto.ItemResolution, 0, len(itemOrder))
	for _, id := range itemOrder {
		items = append(items, *itemRes[id])
	}
	return items, chainOrder, chainTypes, consumeQtyByChain, nil
}

// selectAndValidateSubmissions loads exactly the requested submission IDs
// (reconcile IDs ∪ dispose IDs) and asserts each exists, belongs to
// inventoryID, is approval_status=pending, and has the submission_type the
// caller assigned it (reconcile IDs must be reconciles; dispose IDs must be
// disposes). Returns them parsed (Type-tagged) — order here is requested-id
// order; computeReconcileDrops re-sorts the full set by created_at.
func (s *inventoryService) selectAndValidateSubmissions(
	ctx context.Context,
	inventoryID uint,
	reconcileIDs []uint,
	disposeIDs []uint,
) ([]parsedReconcileSubmission, []models.InventorySubmission, error) {
	if len(reconcileIDs) == 0 && len(disposeIDs) == 0 {
		return nil, nil, pkg.NewAppError(pkg.ErrorCodeValidation, "no submission IDs provided", nil)
	}

	// expectedType maps each requested id to the submission_type it must have;
	// reject any id requested as both reconcile and dispose.
	expectedType := make(map[uint]models.SubmissionType, len(reconcileIDs)+len(disposeIDs))
	orderedIDs := make([]uint, 0, len(reconcileIDs)+len(disposeIDs))
	assign := func(ids []uint, t models.SubmissionType) error {
		for _, id := range ids {
			if prev, ok := expectedType[id]; ok {
				if prev != t {
					return pkg.NewAppError(pkg.ErrorCodeValidation,
						fmt.Sprintf("submission %d requested as both reconcile and dispose", id), nil)
				}
				continue // duplicate within the same flag; keep one
			}
			expectedType[id] = t
			orderedIDs = append(orderedIDs, id)
		}
		return nil
	}
	if err := assign(reconcileIDs, models.InventorySubmissionTypeReconcile); err != nil {
		return nil, nil, err
	}
	if err := assign(disposeIDs, models.InventorySubmissionTypeDispose); err != nil {
		return nil, nil, err
	}

	var raw []models.InventorySubmission
	if err := s.db.WithContext(ctx).
		Where("id IN ?", orderedIDs).
		Order("created_at ASC").
		Find(&raw).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to load submissions: %w", err)
	}

	byID := make(map[uint]models.InventorySubmission, len(raw))
	for _, sub := range raw {
		byID[sub.ID] = sub
	}

	parsed := make([]parsedReconcileSubmission, 0, len(orderedIDs))
	selected := make([]models.InventorySubmission, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		sub, ok := byID[id]
		if !ok {
			return nil, nil, pkg.NewAppError(pkg.ErrorCodeNotFound,
				fmt.Sprintf("submission %d not found", id), nil)
		}
		if sub.InventoryID != inventoryID {
			return nil, nil, pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("submission %d belongs to inventory %d, expected %d", id, sub.InventoryID, inventoryID), nil)
		}
		if sub.ApprovalStatus != models.InventorySubmissionApprovalStatusPending {
			return nil, nil, pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("submission %d is not pending (approval_status=%s)", id, sub.ApprovalStatus), nil)
		}
		want := expectedType[id]
		if sub.SubmissionType != want {
			return nil, nil, pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("submission %d is a %s, expected %s", id, sub.SubmissionType, want), nil)
		}

		p, err := parseSubmissionPayload(sub)
		if err != nil {
			return nil, nil, err
		}
		parsed = append(parsed, p)
		selected = append(selected, sub)
	}

	return parsed, selected, nil
}

// parseSubmissionPayload decodes a reconcile OR dispose submission into the
// chaining-ready shape, asserting per-item quantities are present. For dispose
// payloads PrevQuantity is ignored and ActualCount holds the direct remove-N.
func parseSubmissionPayload(sub models.InventorySubmission) (parsedReconcileSubmission, error) {
	var req dto.ReconcileInventoryRequest // identical shape to DisposeInventoryRequest
	if err := json.Unmarshal(sub.Payload, &req); err != nil {
		return parsedReconcileSubmission{}, fmt.Errorf("failed to parse payload for submission %d: %w", sub.ID, err)
	}

	dispose := sub.SubmissionType == models.InventorySubmissionTypeDispose
	out := parsedReconcileSubmission{
		SubmissionID: sub.ID,
		Type:         sub.SubmissionType,
		CreatedAt:    sub.CreatedAt,
	}
	for _, it := range req.Items {
		if it.Quantity == nil {
			label := "actual quantity"
			if dispose {
				label = "dispose quantity"
			}
			return parsedReconcileSubmission{}, pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("submission %d item %d has nil %s", sub.ID, it.InventoryItemID, label), nil)
		}
		item := parsedReconcileItem{
			InventoryItemID: it.InventoryItemID,
			ActualCount:     *it.Quantity,
		}
		if !dispose {
			item.PrevQuantity = it.PrevQuantity // ignored for dispose
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

// collectItemIDs returns the unique inventory item IDs referenced across the
// submissions.
func collectItemIDs(subs []parsedReconcileSubmission) []uint {
	seen := make(map[uint]struct{})
	ids := make([]uint, 0)
	for _, sub := range subs {
		for _, it := range sub.Items {
			if _, ok := seen[it.InventoryItemID]; ok {
				continue
			}
			seen[it.InventoryItemID] = struct{}{}
			ids = append(ids, it.InventoryItemID)
		}
	}
	return ids
}

// ResolvePendingSubmissions implements the one-off #43 resolution. See interface
// doc. The same compute path runs for preview and apply; apply only gates
// persistence (wrapped in one transaction). Reconcile and dispose submissions
// are folded into the SAME chronological FIFO chain: reconciles synthesize
// backdated Sell txns, disposes synthesize backdated Disposal txns.
func (s *inventoryService) ResolvePendingSubmissions(
	ctx context.Context,
	inventoryID uint,
	reconcileIDs []uint,
	disposeIDs []uint,
	apply bool,
) (*dto.ResolutionPlan, error) {
	parsed, subModels, err := s.selectAndValidateSubmissions(ctx, inventoryID, reconcileIDs, disposeIDs)
	if err != nil {
		return nil, err
	}

	// submissionIDs is the combined set, used only for plan echo / persistence.
	submissionIDs := make([]uint, 0, len(subModels))
	for _, m := range subModels {
		submissionIDs = append(submissionIDs, m.ID)
	}

	itemIDs := collectItemIDs(parsed)

	// Load active items ONCE: consumeFIFO mutates item.Quantity /
	// txn.ConsumedQuantity / ConsumingTransactionID in place, so the same
	// in-memory slice chains FIFO across submissions. Re-loading mid-run would
	// corrupt the chain.
	activeItems, err := s.getActiveInventoryItems(ctx, inventoryID, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load active inventory items: %w", err)
	}
	itemMap := s.buildItemMap(activeItems)

	startStock := make(map[uint]decimal.Decimal, len(activeItems))
	productNames := make(map[uint]string, len(activeItems))
	for _, it := range activeItems {
		startStock[it.ID] = it.Quantity
		if it.Product != nil {
			productNames[it.ID] = it.Product.Name
		}
	}
	// Assert every referenced item resolved to an active item.
	for _, id := range itemIDs {
		if _, ok := itemMap[id]; !ok {
			return nil, pkg.NewAppError(pkg.ErrorCodeNotFound,
				fmt.Sprintf("inventory item %d referenced by submissions is not active", id), nil)
		}
	}

	itemResolutions, chainOrder, chainTypes, consumeQtyByChain, err := computeReconcileDrops(parsed, startStock, productNames)
	if err != nil {
		return nil, err
	}

	// Walk the chain, running each submission's drops through consumeFIFO with a
	// backdating handler whose txn type matches the submission type (reconcile =>
	// Sell, dispose => Disposal). Accumulate item changes + txns.
	ps := newProcessingState(s, nil)
	var allChanges []*models.InventoryItemChange
	var allTxns []*models.InventoryTransaction

	subByID := make(map[uint]models.InventorySubmission, len(subModels))
	for _, m := range subModels {
		subByID[m.ID] = m
	}

	for i, subID := range chainOrder {
		consumeQty := consumeQtyByChain[i]
		if len(consumeQty) == 0 {
			continue // entire submission clamped to no-op (reconcile only)
		}
		backdate := subByID[subID].CreatedAt

		// Pick the synthesized txn type by submission type. The handler is
		// otherwise identical to the production reconcile/dispose handlers PLUS
		// backdating CreatedAt to the submission's exact timestamp. COGS is
		// carried from the source purchase batch as-is (Price is float64).
		txnType := models.InventoryTransactionTypeSell
		if chainTypes[i] == models.InventorySubmissionTypeDispose {
			txnType = models.InventoryTransactionTypeDisposal
		}

		handler := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity decimal.Decimal) []*models.InventoryTransaction {
			return []*models.InventoryTransaction{
				{
					Base:                 models.Base{CreatedAt: backdate},
					InventoryItemID:      item.ID,
					TransactionType:      txnType,
					Price:                consumeTxn.Price,
					Quantity:             quantity,
					CounterTransactionID: &consumeTxn.ID,
				},
			}
		}

		changes, txns, err := s.consumeFIFO(ctx, ps, activeItems, consumeQty, handler)
		if err != nil {
			return nil, fmt.Errorf("failed to consume FIFO for submission %d: %w", subID, err)
		}
		allChanges = append(allChanges, changes...)
		allTxns = append(allTxns, txns...)
	}

	plan := buildResolutionPlan(inventoryID, submissionIDs, itemResolutions, allTxns, apply)

	if !apply {
		return plan, nil
	}

	// Coalesce before persisting. consumeFIFO emits one change per item PER
	// submission; for an item dropped in ≥2 selected submissions allChanges holds
	// multiple entries that all share the same item pointer (already drawn down to
	// its final quantity) but capture progressively smaller OriginalQuantity
	// snapshots. Since apply defers writes, the live DB row still equals the
	// EARLIEST snapshot, so validating any later entry would raise a false
	// optimistic-lock conflict and roll back the whole apply. Collapse to one
	// change per item (earliest OriginalQuantity = the true pre-fix DB baseline,
	// final shared-pointer quantity) before the lock-check/persist loop.
	persistChanges := coalesceItemChanges(allChanges)
	persistTxns := dedupeSourceTxns(allTxns)

	if err := s.persistResolution(ctx, persistChanges, persistTxns, subModels); err != nil {
		return nil, err
	}
	return plan, nil
}

// coalesceItemChanges collapses allChanges to ONE InventoryItemChange per
// inventory-item id. consumeFIFO appends entries in chain (chronological) order
// and reuses the same *InventoryItem pointer per item, so:
//   - the FIRST entry seen for an item carries the earliest OriginalQuantity,
//     which equals the locked DB row (the true pre-fix baseline);
//   - that entry's embedded *InventoryItem already holds the fully drawn-down
//     final quantity (mutated in place across every submission).
//
// Keeping the first entry therefore preserves both the correct lock baseline and
// the final quantity, and validates each item against the DB start exactly once.
// Output is ordered by item id for stable, deterministic persistence.
func coalesceItemChanges(changes []*models.InventoryItemChange) []*models.InventoryItemChange {
	if len(changes) == 0 {
		return changes
	}
	byID := make(map[uint]*models.InventoryItemChange, len(changes))
	order := make([]uint, 0, len(changes))
	for _, c := range changes {
		id := c.InventoryItem.ID
		if _, ok := byID[id]; ok {
			continue // keep the earliest entry's OriginalQuantity
		}
		byID[id] = c
		order = append(order, id)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	out := make([]*models.InventoryItemChange, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
}

// dedupeSourceTxns drops duplicate source purchase transactions (id != 0) from
// the save list. A source purchase consumed across multiple submissions is
// appended once per submission, yielding N identical UPDATEs for the same row
// (correct but wasteful). The shared pointer already holds the final accumulated
// consumed_quantity, so keeping the first occurrence per id suffices. New sells
// (id == 0) are distinct inserts and are NEVER deduped — all are preserved.
func dedupeSourceTxns(txns []*models.InventoryTransaction) []*models.InventoryTransaction {
	if len(txns) == 0 {
		return txns
	}
	seen := make(map[uint]struct{}, len(txns))
	out := make([]*models.InventoryTransaction, 0, len(txns))
	for _, t := range txns {
		if t.ID != 0 {
			if _, ok := seen[t.ID]; ok {
				continue
			}
			seen[t.ID] = struct{}{}
		}
		out = append(out, t)
	}
	return out
}

// persistResolution writes ALL changes atomically in a single transaction:
// inventory item quantity changes, sell inserts + source consumed_quantity
// bumps, and both status columns (+ reason) on every submission. These are not
// atomic across the existing repo methods, so the writes are inlined on tx.
func (s *inventoryService) persistResolution(
	ctx context.Context,
	changes []*models.InventoryItemChange,
	txns []*models.InventoryTransaction,
	subs []models.InventorySubmission,
) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Entry guard inside the tx: re-assert all subs are still pending
		// (light idempotency; no concurrent writes expected in the window).
		ids := make([]uint, len(subs))
		for i, sub := range subs {
			ids[i] = sub.ID
		}
		var current []models.InventorySubmission
		if err := tx.Where("id IN ?", ids).Find(&current).Error; err != nil {
			return fmt.Errorf("failed to re-read submissions: %w", err)
		}
		if len(current) != len(subs) {
			return pkg.NewAppError(pkg.ErrorCodeValidation, "submission set changed before apply", nil)
		}
		for _, sub := range current {
			if sub.ApprovalStatus != models.InventorySubmissionApprovalStatusPending {
				return pkg.NewAppError(pkg.ErrorCodeValidation,
					fmt.Sprintf("submission %d no longer pending (approval_status=%s); aborting apply", sub.ID, sub.ApprovalStatus), nil)
			}
		}

		// 1. Inventory item quantity changes (with optimistic-lock check
		//    mirroring updateInventoryItems).
		if err := saveItemChangesOnTx(ctx, tx, changes); err != nil {
			return err
		}

		// 2. Sell/disposal inserts + source purchase consumed_quantity bumps.
		//    txns contains two distinct identities that MUST be persisted
		//    differently or real purchase rows get corrupted:
		//      - NEW synthetic txns (ID==0): sells/disposals with a backdated
		//        CreatedAt. INSERT them via tx.Create so Base.BeforeCreate stamps
		//        created_by/updated_by = system actor and the preset CreatedAt
		//        survives (autoCreateTime only fills a ZERO CreatedAt).
		//      - EXISTING source purchase txns (ID!=0): consumeFIFO only mutated
		//        their consumed_quantity. They must NOT be routed through
		//        Save/upsert — that fires BeforeCreate and overwrites
		//        created_by/created_at on real rows. UPDATE only the mutated
		//        columns (consumed_quantity) plus the audit updated_* columns.
		var newTxns []*models.InventoryTransaction
		var srcTxns []*models.InventoryTransaction
		for _, txn := range txns {
			if txn.InventoryItem != nil && txn.InventoryItem.ID != 0 {
				txn.InventoryItemID = txn.InventoryItem.ID
			}
			if txn.ID == 0 {
				newTxns = append(newTxns, txn)
			} else {
				srcTxns = append(srcTxns, txn)
			}
		}

		if len(newTxns) > 0 {
			// Create (not Save/upsert): synthetic rows legitimately get
			// created_by=system@cim.local and keep their backdated CreatedAt.
			if err := tx.Create(newTxns).Error; err != nil {
				return fmt.Errorf("failed to insert synthetic transactions: %w", err)
			}
		}

		// Column-scoped UPDATE per source row. dedupeSourceTxns guarantees each
		// source id appears once, so each row is updated exactly once. Select
		// limits the write to consumed_quantity + updated_by/updated_at, so
		// created_by/created_at on real purchase rows are never touched and
		// BeforeCreate never fires for them.
		userEmail, err := pkg.GetUserEmailFromContext(ctx)
		if err != nil {
			return fmt.Errorf("failed to resolve actor for source txn update: %w", err)
		}
		now := time.Now()
		for _, src := range srcTxns {
			if err := tx.Model(&models.InventoryTransaction{}).
				Where("id = ?", src.ID).
				Select("consumed_quantity", "updated_by", "updated_at").
				Updates(map[string]interface{}{
					"consumed_quantity": src.ConsumedQuantity,
					"updated_by":        userEmail,
					"updated_at":        now,
				}).Error; err != nil {
				return fmt.Errorf("failed to update source transaction %d consumed_quantity: %w", src.ID, err)
			}
		}

		// 3. Submission status: approved + completed + reason, on all.
		updates, err := pkg.WithUpdateFields(ctx, map[string]interface{}{
			"approval_status":   models.InventorySubmissionApprovalStatusApproved,
			"processing_status": models.InventorySubmissionStatusCompleted,
			"reason":            reconcileOneOffReason,
		})
		if err != nil {
			return fmt.Errorf("failed to prepare submission update fields: %w", err)
		}
		if err := tx.Model(&models.InventorySubmission{}).
			Where("id IN ?", ids).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update submission statuses: %w", err)
		}

		return nil
	})
}

// saveItemChangesOnTx mirrors inventoryItemRepository.updateInventoryItems but
// runs on the caller-supplied tx so it shares the single apply transaction.
func saveItemChangesOnTx(ctx context.Context, tx *gorm.DB, changes []*models.InventoryItemChange) error {
	if len(changes) == 0 {
		return nil
	}

	var existing []*models.InventoryItem
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", models.GetIDs(changes)).
		Find(&existing).Error; err != nil {
		return fmt.Errorf("failed to fetch inventory items for update: %w", err)
	}

	existingMap := models.BuildIDMap(existing)
	for _, change := range changes {
		if change.InventoryItem.IsNew() {
			continue
		}
		ex, ok := existingMap[change.ID]
		if !ok {
			return fmt.Errorf("inventory item with ID %d not found", change.ID)
		}
		if !ex.Quantity.Equal(change.OriginalQuantity) {
			return pkg.ErrOptimisticLockConflict(ctx, "inventory item", change.ID, change.OriginalQuantity, ex.Quantity)
		}
	}

	items := make([]*models.InventoryItem, len(changes))
	for i, change := range changes {
		items[i] = change.InventoryItem
	}
	if err := tx.Save(items).Error; err != nil {
		return fmt.Errorf("failed to update inventory items: %w", err)
	}
	return nil
}

// buildResolutionPlan assembles the preview/apply plan from the per-item
// resolutions and the synthesized transactions.
func buildResolutionPlan(
	inventoryID uint,
	submissionIDs []uint,
	items []dto.ItemResolution,
	txns []*models.InventoryTransaction,
	applied bool,
) *dto.ResolutionPlan {
	plan := &dto.ResolutionPlan{
		InventoryID:   inventoryID,
		SubmissionIDs: submissionIDs,
		Applied:       applied,
		Items:         items,
	}

	// Flat sell + disposal lists and consumed_quantity deltas come from the new
	// synthesized txns. Both Sell and Disposal draw down source purchase batches,
	// so both contribute to ConsumedDeltas.
	consumed := make(map[uint]decimal.Decimal)
	for _, txn := range txns {
		var src uint
		if txn.CounterTransactionID != nil {
			src = *txn.CounterTransactionID
		}
		switch txn.TransactionType {
		case models.InventoryTransactionTypeSell:
			plan.Sells = append(plan.Sells, dto.SyntheticSell{
				InventoryItemID:     txn.InventoryItemID,
				Quantity:            txn.Quantity,
				SourcePurchaseTxnID: src,
				COGSPrice:           txn.Price,
				BackdatedDate:       txn.CreatedAt,
			})
			consumed[src] = consumed[src].Add(txn.Quantity)
		case models.InventoryTransactionTypeDisposal:
			plan.Disposals = append(plan.Disposals, dto.SyntheticDisposal{
				InventoryItemID:     txn.InventoryItemID,
				Quantity:            txn.Quantity,
				SourcePurchaseTxnID: src,
				COGSPrice:           txn.Price,
				BackdatedDate:       txn.CreatedAt,
			})
			consumed[src] = consumed[src].Add(txn.Quantity)
		}
	}
	plan.TotalSells = len(plan.Sells)
	plan.TotalDisposals = len(plan.Disposals)

	srcIDs := make([]uint, 0, len(consumed))
	for id := range consumed {
		srcIDs = append(srcIDs, id)
	}
	sort.Slice(srcIDs, func(i, j int) bool { return srcIDs[i] < srcIDs[j] })
	for _, id := range srcIDs {
		plan.ConsumedDeltas = append(plan.ConsumedDeltas, dto.ConsumedQuantityDelta{
			PurchaseTxnID: id,
			Delta:         consumed[id],
		})
	}

	// Clamped row summary.
	for _, item := range items {
		for _, d := range item.Drops {
			if d.Clamped {
				plan.ClampedRowCount++
				plan.TotalClampedRowList = append(plan.TotalClampedRowList, dto.ClampedRowRef{
					SubmissionID:    d.SubmissionID,
					InventoryItemID: item.InventoryItemID,
					RawDelta:        d.RawDelta,
				})
			}
		}
	}

	return plan
}
