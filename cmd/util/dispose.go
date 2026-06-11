package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
)

// disposeSubmissionToApprove is the single pending dispose submission id this
// one-off approves. Registered as the --submission flag (no default).
var disposeSubmissionToApprove uint

// reconcileCmd's `dispose` subcommand: approve ONE standalone pending dispose
// submission, replicating the app's dispose-approval (FIFO consume oldest-first,
// per-source COGS disposal txns, source consumed_quantity bumps, item quantity
// decrement) but BACKDATING each disposal txn's CreatedAt to the submission's own
// created_at. Needed because `reconcile resolve` requires >= 2 reconcile
// submissions and cannot process a lone dispose.
var disposeApproveCmd = &cobra.Command{
	Use:   "dispose",
	Short: "Approve ONE pending dispose submission with backdated disposal txns (preview by default)",
	// Runtime (RunE) errors print only the error, not the usage block; flag-parse
	// errors still show usage (cobra applies SilenceUsage only after parsing).
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: "One-off: approve a SINGLE standalone pending DISPOSE submission, replicating " +
		"the app's dispose-approval logic (per item, FIFO-consume the requested quantity " +
		"from the oldest purchases; for each source purchase create a Disposal txn at the " +
		"source's COGS price, bump the source's consumed_quantity, decrement the item's " +
		"quantity) but BACKDATING each disposal txn's CreatedAt to the submission's own " +
		"created_at.\n\n" +
		"PREVIEW (default) persists NOTHING: it prints a summary and (with --out) writes the " +
		"plan as JSON. --apply persists the IDENTICAL plan in one transaction. Always review first.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if disposeSubmissionToApprove == 0 {
			return fmt.Errorf("--submission is required (the pending dispose submission id)")
		}

		db, err := openReconcileDB()
		if err != nil {
			return err
		}

		// System context: synthetic txns + status updates are stamped with the system
		// actor via pkg.WithUserEmail (used by Base.BeforeCreate / WithUpdateFields).
		ctx := pkg.WithUserEmail(context.Background(), reconcileActor)

		mode := "PREVIEW (persisting nothing)"
		if reconcileApply {
			mode = "APPLY (persisting in one transaction)"
		}
		fmt.Printf("Dispose approve (one-off) — inventory=%d submission=%d mode=%s\n",
			reconcileInventoryID, disposeSubmissionToApprove, mode)

		plan, err := computeDisposePlan(ctx, db, reconcileInventoryID, disposeSubmissionToApprove)
		if err != nil {
			return err
		}

		printDisposePlanSummary(plan)

		if reconcileOut != "" {
			if err := writeDisposePlanJSON(plan, reconcileOut); err != nil {
				return err
			}
			fmt.Printf("\nWrote dispose plan JSON to %s\n", reconcileOut)
		}

		if reconcileApply {
			if err := applyDisposePlan(ctx, db, plan); err != nil {
				return err
			}
			fmt.Println("\nApplied. Backdated disposal txns inserted, source consumed_quantity bumped, item quantity decremented, submission approved+completed — all in one transaction.")
		} else {
			fmt.Println("\nPreview only — nothing persisted. Re-run with --apply to persist.")
		}
		return nil
	},
}

// ---------------------------------------------------------------------------
// Dispose plan types (one-off; same flavor as ResolutionPlan)
// ---------------------------------------------------------------------------

// DisposePlan is the full, faithful preview of what --apply will persist for a
// single dispose submission. The same computeDisposePlan builds it for preview
// and apply, so preview always equals apply.
type DisposePlan struct {
	InventoryID  uint      `json:"inventory_id"`
	SubmissionID uint      `json:"submission_id"`
	SubmissionAt time.Time `json:"submission_created_at"`

	Items []DisposeItemPlan `json:"items"`

	// Disposal txns to insert (backdated to the submission's created_at).
	Txns []SyntheticTxn `json:"disposal_txns"`

	// Source purchase consumed_quantity deltas (informational).
	ConsumedDeltas []ConsumedQuantityDelta `json:"consumed_quantity_deltas"`

	// internal apply payload (not serialized): the exact rows to write.
	srcTxnUpdates []srcConsumedUpdate `json:"-"`
	itemUpdates   []itemQtyUpdate     `json:"-"`
}

// DisposeItemPlan summarizes one item's dispose: requested qty, current live
// quantity (optimistic-lock baseline), and resulting quantity.
type DisposeItemPlan struct {
	InventoryItemID uint            `json:"inventory_item_id"`
	ProductName     string          `json:"product_name"`
	RequestedQty    decimal.Decimal `json:"requested_quantity"`
	CurrentQuantity decimal.Decimal `json:"current_quantity"`
	ResultingQty    decimal.Decimal `json:"resulting_quantity"`
}

// ---------------------------------------------------------------------------
// computeDisposePlan: shared preview/apply compute path (loads, FIFO walk, plan)
// ---------------------------------------------------------------------------

func computeDisposePlan(ctx context.Context, db *gorm.DB, inventoryID, submissionID uint) (*DisposePlan, error) {
	// 1. Load + validate the submission.
	var sub models.InventorySubmission
	if err := db.WithContext(ctx).Where("id = ?", submissionID).First(&sub).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("dispose submission %d not found (or soft-deleted)", submissionID)
		}
		return nil, fmt.Errorf("failed to load submission %d: %w", submissionID, err)
	}
	if sub.SubmissionType != models.InventorySubmissionTypeDispose {
		return nil, fmt.Errorf("submission %d is a %s, expected dispose", submissionID, sub.SubmissionType)
	}
	if sub.ApprovalStatus != models.InventorySubmissionApprovalStatusPending {
		return nil, fmt.Errorf("submission %d is not pending (approval_status=%s)", submissionID, sub.ApprovalStatus)
	}
	if sub.InventoryID != inventoryID {
		return nil, fmt.Errorf("submission %d belongs to inventory %d, expected %d", submissionID, sub.InventoryID, inventoryID)
	}

	// 2. Unmarshal the dispose payload; collect itemID -> requested qty.
	var req dto.DisposeInventoryRequest
	if err := json.Unmarshal(sub.Payload, &req); err != nil {
		return nil, fmt.Errorf("failed to parse payload for submission %d: %w", submissionID, err)
	}
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("submission %d has no items", submissionID)
	}
	requestedByItem := make(map[uint]decimal.Decimal, len(req.Items))
	itemIDs := make([]uint, 0, len(req.Items))
	for _, it := range req.Items {
		if it.Quantity == nil {
			return nil, fmt.Errorf("submission %d item %d has nil quantity", submissionID, it.InventoryItemID)
		}
		if it.Quantity.LessThan(decimal.Zero) {
			return nil, fmt.Errorf("submission %d item %d has negative quantity %s", submissionID, it.InventoryItemID, it.Quantity.String())
		}
		if _, dup := requestedByItem[it.InventoryItemID]; dup {
			return nil, fmt.Errorf("submission %d lists item %d more than once", submissionID, it.InventoryItemID)
		}
		requestedByItem[it.InventoryItemID] = *it.Quantity
		itemIDs = append(itemIDs, it.InventoryItemID)
	}

	// 3. Load active items (with FIFO-ordered purchases); assert each is active.
	items, err := loadActiveItems(ctx, db, inventoryID, itemIDs)
	if err != nil {
		return nil, err
	}
	for _, id := range itemIDs {
		if _, ok := items[id]; !ok {
			return nil, fmt.Errorf("inventory item %d referenced by submission %d is not active", id, submissionID)
		}
	}

	plan := &DisposePlan{
		InventoryID:  inventoryID,
		SubmissionID: submissionID,
		SubmissionAt: sub.CreatedAt,
	}

	consumedDeltaByTxn := make(map[uint]decimal.Decimal)

	// 4. Compute the plan in memory (NO writes), per item in ascending id order for
	//    stable output. Replicate consumeFIFO's up-front total-stock validation, then
	//    the FIFO walk (oldest purchase first). Each take => one disposal txn at the
	//    source's COGS price, backdated to the submission's created_at.
	sort.Slice(itemIDs, func(i, j int) bool { return itemIDs[i] < itemIDs[j] })
	for _, id := range itemIDs {
		item := items[id]
		requested := requestedByItem[id]

		// consumeFIFO validates up front: requested must not exceed available stock.
		// We replicate against the live item quantity (same as the app, which checks
		// consumeQty > item.Quantity). No partial writes if any item is insufficient.
		if requested.GreaterThan(item.Quantity) {
			return nil, fmt.Errorf(
				"item %d: dispose quantity %s exceeds available quantity %s; aborting, no writes",
				id, requested.String(), item.Quantity.String())
		}

		productName := ""
		if item.Product != nil {
			productName = item.Product.Name
		}

		itemPlan := DisposeItemPlan{
			InventoryItemID: id,
			ProductName:     productName,
			RequestedQty:    requested,
			CurrentQuantity: item.Quantity,
			ResultingQty:    item.Quantity.Sub(requested),
		}
		plan.Items = append(plan.Items, itemPlan)

		// Item quantity decrement (optimistic-lock baseline = current live quantity).
		plan.itemUpdates = append(plan.itemUpdates, itemQtyUpdate{
			itemID:      id,
			originalQty: item.Quantity,
			newQty:      item.Quantity.Sub(requested),
		})

		if !requested.GreaterThan(decimal.Zero) {
			continue // zero-quantity item: benign no-op, no disposal txns
		}

		// FIFO walk over the source purchases (already ordered created_at ASC, id ASC).
		toConsume := requested
		for _, src := range item.ConsumableTransactions {
			if !toConsume.GreaterThan(decimal.Zero) {
				break
			}
			remaining := src.Quantity.Sub(src.ConsumedQuantity)
			if !remaining.GreaterThan(decimal.Zero) {
				continue
			}
			take := toConsume
			if remaining.LessThan(take) {
				take = remaining
			}
			plan.Txns = append(plan.Txns, SyntheticTxn{
				InventoryItemID:     id,
				TransactionType:     string(models.InventoryTransactionTypeDisposal),
				Quantity:            take,
				SourcePurchaseTxnID: src.ID,
				COGSPrice:           src.Price,
				BackdatedDate:       sub.CreatedAt,
			})
			newConsumed := src.ConsumedQuantity.Add(take)
			plan.srcTxnUpdates = append(plan.srcTxnUpdates, srcConsumedUpdate{txnID: src.ID, consumedQuantity: newConsumed})
			consumedDeltaByTxn[src.ID] = consumedDeltaByTxn[src.ID].Add(take)
			toConsume = toConsume.Sub(take)
		}

		// Defensive: the up-front check guarantees enough stock, but the per-purchase
		// remaining sum can be < item.Quantity if the ledger is inconsistent. Hard-fail.
		if toConsume.GreaterThan(decimal.Zero) {
			return nil, fmt.Errorf(
				"item %d FATAL: purchase ledger exhausted with %s still to dispose (quantity %s but unconsumed purchases sum less); aborting, no writes",
				id, toConsume.String(), requested.String())
		}
	}

	// Stable txn ordering: by item, then date, then source purchase id.
	sort.SliceStable(plan.Txns, func(i, j int) bool {
		if plan.Txns[i].InventoryItemID != plan.Txns[j].InventoryItemID {
			return plan.Txns[i].InventoryItemID < plan.Txns[j].InventoryItemID
		}
		if !plan.Txns[i].BackdatedDate.Equal(plan.Txns[j].BackdatedDate) {
			return plan.Txns[i].BackdatedDate.Before(plan.Txns[j].BackdatedDate)
		}
		return plan.Txns[i].SourcePurchaseTxnID < plan.Txns[j].SourcePurchaseTxnID
	})

	// Consumed deltas (sorted by txn id).
	deltaIDs := make([]uint, 0, len(consumedDeltaByTxn))
	for id := range consumedDeltaByTxn {
		deltaIDs = append(deltaIDs, id)
	}
	sort.Slice(deltaIDs, func(i, j int) bool { return deltaIDs[i] < deltaIDs[j] })
	for _, id := range deltaIDs {
		plan.ConsumedDeltas = append(plan.ConsumedDeltas, ConsumedQuantityDelta{PurchaseTxnID: id, Delta: consumedDeltaByTxn[id]})
	}

	return plan, nil
}

// ---------------------------------------------------------------------------
// applyDisposePlan: one bounded transaction
// ---------------------------------------------------------------------------

func applyDisposePlan(ctx context.Context, db *gorm.DB, plan *DisposePlan) error {
	now := time.Now()

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Re-read the submission inside the tx; abort if no longer pending (idempotency).
		var sub models.InventorySubmission
		if err := tx.Where("id = ?", plan.SubmissionID).First(&sub).Error; err != nil {
			return fmt.Errorf("failed to re-read submission %d: %w", plan.SubmissionID, err)
		}
		if sub.ApprovalStatus != models.InventorySubmissionApprovalStatusPending {
			return fmt.Errorf("submission %d no longer pending (approval_status=%s); aborting apply", plan.SubmissionID, sub.ApprovalStatus)
		}

		// 1. Lock + optimistic-check + decrement inventory item quantities.
		itemIDs := make([]uint, 0, len(plan.itemUpdates))
		for _, u := range plan.itemUpdates {
			itemIDs = append(itemIDs, u.itemID)
		}
		var lockedItems []*models.InventoryItem
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id IN ?", itemIDs).Find(&lockedItems).Error; err != nil {
			return fmt.Errorf("failed to lock inventory items: %w", err)
		}
		lockedByID := make(map[uint]*models.InventoryItem, len(lockedItems))
		for _, it := range lockedItems {
			lockedByID[it.ID] = it
		}
		for _, u := range plan.itemUpdates {
			ex, ok := lockedByID[u.itemID]
			if !ok {
				return fmt.Errorf("inventory item %d not found at apply", u.itemID)
			}
			if !ex.Quantity.Equal(u.originalQty) {
				return pkg.ErrOptimisticLockConflict(ctx, "inventory item", u.itemID, u.originalQty, ex.Quantity)
			}
			if err := tx.Model(&models.InventoryItem{}).
				Where("id = ?", u.itemID).
				Select("quantity", "updated_by", "updated_at").
				Updates(map[string]interface{}{
					"quantity":   u.newQty,
					"updated_by": reconcileActor,
					"updated_at": now,
				}).Error; err != nil {
				return fmt.Errorf("failed to update inventory item %d quantity: %w", u.itemID, err)
			}
		}

		// 2. Insert backdated disposal txns (Create => BeforeCreate stamps actor;
		//    preset CreatedAt survives autoCreateTime which only fills a ZERO time).
		for i := range plan.Txns {
			t := plan.Txns[i]
			src := t.SourcePurchaseTxnID
			row := &models.InventoryTransaction{
				Base:                 models.Base{CreatedAt: t.BackdatedDate},
				InventoryItemID:      t.InventoryItemID,
				TransactionType:      models.InventoryTransactionType(t.TransactionType),
				Price:                t.COGSPrice,
				Quantity:             t.Quantity,
				CounterTransactionID: &src,
			}
			if err := tx.Create(row).Error; err != nil {
				return fmt.Errorf("failed to insert backdated %s txn: %w", t.TransactionType, err)
			}
		}

		// 3. Column-scoped UPDATE of source purchase consumed_quantity. Never touch
		//    created_by/created_at on real purchase rows.
		for _, u := range plan.srcTxnUpdates {
			if err := tx.Model(&models.InventoryTransaction{}).
				Where("id = ?", u.txnID).
				Select("consumed_quantity", "updated_by", "updated_at").
				Updates(map[string]interface{}{
					"consumed_quantity": u.consumedQuantity,
					"updated_by":        reconcileActor,
					"updated_at":        now,
				}).Error; err != nil {
				return fmt.Errorf("failed to update source txn %d consumed_quantity: %w", u.txnID, err)
			}
		}

		// 4. Flip the submission to approved + completed + reason.
		updates, err := pkg.WithUpdateFields(ctx, map[string]interface{}{
			"approval_status":   models.InventorySubmissionApprovalStatusApproved,
			"processing_status": models.InventorySubmissionStatusCompleted,
			"reason":            reconcileOneOffReason,
		})
		if err != nil {
			return fmt.Errorf("failed to prepare submission update fields: %w", err)
		}
		if err := tx.Model(&models.InventorySubmission{}).
			Where("id = ?", plan.SubmissionID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update submission %d status: %w", plan.SubmissionID, err)
		}

		return nil
	})
}

// ---------------------------------------------------------------------------
// Output: summary + JSON
// ---------------------------------------------------------------------------

func printDisposePlanSummary(plan *DisposePlan) {
	fmt.Println("\n=== Dispose plan summary (one-off) ===")
	fmt.Printf("Inventory: %d   Submission: %d   Backdate (created_at): %s\n",
		plan.InventoryID, plan.SubmissionID, plan.SubmissionAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Items: %d   Disposal txns: %d\n", len(plan.Items), len(plan.Txns))

	fmt.Println("\nPer-item:")
	for _, item := range plan.Items {
		fmt.Printf("  item %d (%s): requested=%s current=%s resulting=%s\n",
			item.InventoryItemID, item.ProductName, item.RequestedQty.String(),
			item.CurrentQuantity.String(), item.ResultingQty.String())
	}

	fmt.Printf("\nBackdated disposal txns (%d):\n", len(plan.Txns))
	for _, t := range plan.Txns {
		fmt.Printf("  item %d %s qty=%s srcPurchaseTxn=%d cogs=%.2f date=%s\n",
			t.InventoryItemID, t.TransactionType, t.Quantity.String(), t.SourcePurchaseTxnID,
			t.COGSPrice, t.BackdatedDate.Format("2006-01-02 15:04:05"))
	}
}

func writeDisposePlanJSON(plan *DisposePlan, path string) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal dispose plan: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write dispose plan to %s: %w", path, err)
	}
	return nil
}

func init() {
	disposeApproveCmd.Flags().UintVar(&reconcileInventoryID, "inventory", 1, "inventory ID the dispose submission belongs to")
	disposeApproveCmd.Flags().UintVar(&disposeSubmissionToApprove, "submission", 0, "the pending dispose submission id to approve (required)")
	disposeApproveCmd.Flags().BoolVar(&reconcileApply, "apply", false, "persist the plan (default: preview only)")
	disposeApproveCmd.Flags().StringVar(&reconcileOut, "out", "", "write the dispose plan as JSON to this path")
	disposeApproveCmd.Flags().StringVar(&reconcileDBURL, "db-url", "", "full Postgres DSN to run against directly (overrides global config; recommend sslmode + quiet)")
	disposeApproveCmd.Flags().BoolVar(&reconcileProdConfirm, "prod-confirm", false, "required to --apply when --db-url host is non-localhost")

	reconcileCmd.AddCommand(disposeApproveCmd)
}
