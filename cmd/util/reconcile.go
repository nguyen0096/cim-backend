package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"cim-backend/database"
	"cim-backend/internal/config"
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
)

// reconcileOneOffReason is stamped on resolved submissions and recreated clones.
const reconcileOneOffReason = "resolved by one-off data-fix #46 (correct-earlier-sub reconcile + backdated FIFO sells/disposals)"

// reconcileActor is the system actor stamped on synthetic txns and recreated subs.
const reconcileActor = "system@cim.local"

var (
	reconcileInventoryID  uint
	reconcileSubmissionID []uint
	disposeSubmissionID   []uint
	reconcileApply        bool
	reconcileOut          string
	reconcileDBURL        string
	reconcileProdConfirm  bool
)

var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "One-off reconcile data-fix commands (#46)",
	Long:  "Resolve pending reconcile/dispose submissions by correcting the earlier count and synthesizing backdated FIFO sells/disposals.",
}

var reconcileResolveCmd = &cobra.Command{
	Use:           "resolve",
	Short:         "Resolve pending reconcile + dispose submissions (preview by default)",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: "One-time #46 data-fix. For each adjacent reconcile pair (sub1 earlier, " +
		"sub2 later) it checks the rule\n\n" +
		"    sub2_qty <= sub1_qty + range_purchases - range_disposes\n\n" +
		"and, when violated, CORRECTS the earlier count to\n\n" +
		"    sub1_qty := sub2_qty - range_purchases + range_disposes\n\n" +
		"so the between-period shows zero usage and the net shrinkage lands on the " +
		"earlier sub's date. Corrections propagate backward (latest->earliest). " +
		"Disposes are applied as real backdated Disposal txns at their own dates " +
		"AND accounted in the rule's -range_disposes term (not double-counted).\n\n" +
		"PREVIEW (default) persists NOTHING: it prints a summary and writes the plan " +
		"(+ xlsx) to --out. --apply persists the IDENTICAL plan in one transaction. " +
		"Always review the --out plan first.",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReconcileDB()
		if err != nil {
			return err
		}

		ctx := pkg.WithUserEmail(context.Background(), reconcileActor)

		mode := "PREVIEW (persisting nothing)"
		if reconcileApply {
			mode = "APPLY (persisting in one transaction)"
		}
		fmt.Printf("Reconcile resolve (#46) — inventory=%d reconcile-submissions=%v dispose-submissions=%v mode=%s\n",
			reconcileInventoryID, reconcileSubmissionID, disposeSubmissionID, mode)

		if !reconcileApply {
			printPendingDisposeSubmissions(db, reconcileInventoryID)
		}

		plan, err := computeResolution(ctx, db, reconcileInventoryID, reconcileSubmissionID, disposeSubmissionID)
		if err != nil {
			return err
		}

		printPlanSummary(plan)

		if reconcileOut != "" {
			if err := writePlanJSON(plan, reconcileOut); err != nil {
				return err
			}
			xlsxPath := strings.TrimSuffix(reconcileOut, ".json") + ".xlsx"
			if err := writePlanXLSX(plan, xlsxPath); err != nil {
				return err
			}
			fmt.Printf("\nWrote plan JSON to %s and xlsx to %s\n", reconcileOut, xlsxPath)
		}

		if reconcileApply {
			if err := applyResolution(ctx, db, plan); err != nil {
				return err
			}
			fmt.Println("\nApplied. Corrected submissions soft-deleted + recloned; backdated txns inserted in one transaction.")
		} else {
			fmt.Println("\nPreview only — nothing persisted. Re-run with --apply to persist.")
		}
		return nil
	},
}

// openReconcileDB opens gorm from the global config, or from --db-url directly
// without mutating global config. A non-localhost --db-url requires --prod-confirm
// before --apply.
func openReconcileDB() (*gorm.DB, error) {
	if reconcileDBURL == "" {
		return database.Initialize(config.App.Database)
	}

	host, err := dsnHost(reconcileDBURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse --db-url: %w", err)
	}
	if !isLocalHost(host) && reconcileApply && !reconcileProdConfirm {
		return nil, fmt.Errorf(
			"refusing to --apply against non-localhost host %q without --prod-confirm; "+
				"re-run with --prod-confirm once you have a verified backup and an approved window", host)
	}

	db, err := gorm.Open(postgres.Open(reconcileDBURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to --db-url: %w", err)
	}
	fmt.Printf("Connected via --db-url (host=%s, prod-confirm=%t)\n", host, reconcileProdConfirm)
	return db, nil
}

// dsnHost extracts the host from a Postgres DSN in either URL form
// (postgres://user:pass@host:port/db) or keyword form (host=... port=...).
func dsnHost(dsn string) (string, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", err
		}
		return u.Hostname(), nil
	}
	for _, field := range strings.Fields(dsn) {
		if strings.HasPrefix(field, "host=") {
			return strings.TrimPrefix(field, "host="), nil
		}
	}
	return "", fmt.Errorf("could not determine host from DSN")
}

// isLocalHost reports whether host is a recognized loopback/local host. An empty
// host is treated as NON-local so a hostless DSN cannot bypass the apply guard.
func isLocalHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1", "host.docker.internal":
		return true
	}
	return false
}

// ResolutionPlan is the full preview of what --apply will persist. The same
// computeResolution builds it for both preview and apply.
type ResolutionPlan struct {
	InventoryID  uint   `json:"inventory_id"`
	ReconcileIDs []uint `json:"reconcile_submission_ids"`
	DisposeIDs   []uint `json:"dispose_submission_ids"`

	Items []ItemPlan `json:"items"`

	// Corrections: reconcile submissions whose count was corrected; apply
	// soft-deletes the original and recreates a clone with the corrected payload.
	Corrections []SubmissionCorrection `json:"corrections"`

	// Submissions applied as-is; apply moves their status to approved+completed.
	AppliedAsIs []uint `json:"applied_as_is_submission_ids"`

	// Synthetic backdated txns to insert, ordered by item then date.
	Txns []SyntheticTxn `json:"synthetic_txns"`

	// Source purchase consumed_quantity deltas (informational).
	ConsumedDeltas []ConsumedQuantityDelta `json:"consumed_quantity_deltas"`

	// internal apply payload (not serialized): the exact rows to write.
	srcTxnUpdates []srcConsumedUpdate `json:"-"`
	itemUpdates   []itemQtyUpdate     `json:"-"`
}

// ItemPlan summarizes one inventory item: start stock, the corrected chain, and
// the resulting final stock.
type ItemPlan struct {
	InventoryItemID uint            `json:"inventory_item_id"`
	ProductName     string          `json:"product_name"`
	StartStock      decimal.Decimal `json:"start_stock"`
	Steps           []ItemStep      `json:"steps"`
	FinalStock      decimal.Decimal `json:"final_stock"`
	// CurrentQuantity is the item's live quantity at compute time (the optimistic-lock baseline).
	CurrentQuantity decimal.Decimal `json:"current_quantity"`
	// AppliedQuantity is the quantity apply writes: current_quantity minus synthesized outbound.
	AppliedQuantity decimal.Decimal `json:"applied_quantity"`
}

// ItemStep is one submission's effect on the item, in chronological order.
type ItemStep struct {
	SubmissionID   uint            `json:"submission_id"`
	SubmissionType string          `json:"submission_type"`
	CreatedAt      time.Time       `json:"created_at"`
	OrigQuantity   decimal.Decimal `json:"orig_quantity"`
	// CorrectedQuantity == OrigQuantity for consistent reconciles and disposes.
	CorrectedQuantity decimal.Decimal `json:"corrected_quantity"`
	// PrevQuantity is the chain-consistent stock immediately before this sub.
	PrevQuantity decimal.Decimal `json:"prev_quantity"`
	// Drop is the net shrinkage consumed at this step's date.
	Drop      decimal.Decimal `json:"drop"`
	Corrected bool            `json:"corrected"`
}

// SubmissionCorrection records a reconcile submission whose count was corrected.
type SubmissionCorrection struct {
	SubmissionID    uint            `json:"submission_id"`
	InventoryItemID uint            `json:"inventory_item_id"`
	OrigQuantity    decimal.Decimal `json:"orig_quantity"`
	NewQuantity     decimal.Decimal `json:"new_quantity"`
	OrigPrev        decimal.Decimal `json:"orig_prev_quantity"`
	NewPrev         decimal.Decimal `json:"new_prev_quantity"`
}

// SyntheticTxn is one backdated sell or disposal to insert.
type SyntheticTxn struct {
	InventoryItemID     uint            `json:"inventory_item_id"`
	TransactionType     string          `json:"transaction_type"`
	Quantity            decimal.Decimal `json:"quantity"`
	SourcePurchaseTxnID uint            `json:"source_purchase_txn_id"`
	COGSPrice           float64         `json:"cogs_price"`
	BackdatedDate       time.Time       `json:"backdated_date"`
}

// ConsumedQuantityDelta records the increase in consumed_quantity for a source row.
type ConsumedQuantityDelta struct {
	PurchaseTxnID uint            `json:"purchase_txn_id"`
	Delta         decimal.Decimal `json:"delta"`
}

// srcConsumedUpdate carries the final absolute consumed_quantity for a source row.
type srcConsumedUpdate struct {
	txnID            uint
	consumedQuantity decimal.Decimal
}

// itemQtyUpdate carries the optimistic-lock baseline and the final quantity for
// an inventory item.
type itemQtyUpdate struct {
	itemID      uint
	originalQty decimal.Decimal
	newQty      decimal.Decimal
}

// parsedSub is a submission decoded into the fields the algorithm needs.
type parsedSub struct {
	models.InventorySubmission
	items []parsedItem
}

type parsedItem struct {
	itemID uint
	// prev is the reconcile snapshot count (ignored for dispose).
	prev decimal.Decimal
	// qty is the reconcile counted quantity, or the dispose remove-N.
	qty decimal.Decimal
}

func (p parsedSub) isReconcile() bool {
	return p.SubmissionType == models.InventorySubmissionTypeReconcile
}

// loadSubmissions loads exactly the requested IDs, asserts each exists, belongs
// to inventoryID, is pending, and has the expected type. Returns them parsed and
// sorted chronologically (created_at ASC, tie-break by id).
func loadSubmissions(ctx context.Context, db *gorm.DB, inventoryID uint, reconcileIDs, disposeIDs []uint) ([]parsedSub, error) {
	if len(reconcileIDs) == 0 {
		return nil, fmt.Errorf("at least one reconcile submission id is required")
	}

	expected := make(map[uint]models.SubmissionType)
	ordered := make([]uint, 0, len(reconcileIDs)+len(disposeIDs))
	assign := func(ids []uint, t models.SubmissionType) error {
		for _, id := range ids {
			if prev, ok := expected[id]; ok {
				if prev != t {
					return fmt.Errorf("submission %d requested as both reconcile and dispose", id)
				}
				continue
			}
			expected[id] = t
			ordered = append(ordered, id)
		}
		return nil
	}
	if err := assign(reconcileIDs, models.InventorySubmissionTypeReconcile); err != nil {
		return nil, err
	}
	if err := assign(disposeIDs, models.InventorySubmissionTypeDispose); err != nil {
		return nil, err
	}

	var raw []models.InventorySubmission
	if err := db.WithContext(ctx).Where("id IN ?", ordered).Find(&raw).Error; err != nil {
		return nil, fmt.Errorf("failed to load submissions: %w", err)
	}
	byID := make(map[uint]models.InventorySubmission, len(raw))
	for _, s := range raw {
		byID[s.ID] = s
	}

	out := make([]parsedSub, 0, len(ordered))
	for _, id := range ordered {
		sub, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("submission %d not found", id)
		}
		if sub.InventoryID != inventoryID {
			return nil, fmt.Errorf("submission %d belongs to inventory %d, expected %d", id, sub.InventoryID, inventoryID)
		}
		if sub.ApprovalStatus != models.InventorySubmissionApprovalStatusPending {
			return nil, fmt.Errorf("submission %d is not pending (approval_status=%s)", id, sub.ApprovalStatus)
		}
		if sub.SubmissionType != expected[id] {
			return nil, fmt.Errorf("submission %d is a %s, expected %s", id, sub.SubmissionType, expected[id])
		}
		p, err := parseSub(sub)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func parseSub(sub models.InventorySubmission) (parsedSub, error) {
	var req dto.ReconcileInventoryRequest // identical shape to DisposeInventoryRequest
	if err := json.Unmarshal(sub.Payload, &req); err != nil {
		return parsedSub{}, fmt.Errorf("failed to parse payload for submission %d: %w", sub.ID, err)
	}
	dispose := sub.SubmissionType == models.InventorySubmissionTypeDispose
	out := parsedSub{InventorySubmission: sub}
	for _, it := range req.Items {
		if it.Quantity == nil {
			return parsedSub{}, fmt.Errorf("submission %d item %d has nil quantity", sub.ID, it.InventoryItemID)
		}
		pi := parsedItem{itemID: it.InventoryItemID, qty: *it.Quantity}
		if !dispose {
			pi.prev = it.PrevQuantity
		}
		out.items = append(out.items, pi)
	}
	return out, nil
}

// loadActiveItems loads the active inventory items (with FIFO-ordered consumable
// purchase txns) for the unique item ids referenced across the submissions.
func loadActiveItems(ctx context.Context, db *gorm.DB, inventoryID uint, itemIDs []uint) (map[uint]*models.InventoryItem, error) {
	var items []*models.InventoryItem
	err := db.WithContext(ctx).
		Preload("Product").
		Where("inventory_id = ? AND id IN ? AND status = ?", inventoryID, itemIDs, models.InventoryItemStatusActive).
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("failed to load active inventory items: %w", err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no active inventory items found for inventory %d", inventoryID)
	}

	// Consumable (purchase/transfer-in) txns in FIFO order; unfiltered so the
	// algorithm can walk the full purchase ledger.
	var txns []*models.InventoryTransaction
	if err := db.WithContext(ctx).
		Where("inventory_item_id IN ? AND transaction_type IN ?", itemIDs, models.GetConsumableTransactionTypes()).
		Order("created_at ASC, id ASC").
		Find(&txns).Error; err != nil {
		return nil, fmt.Errorf("failed to load consumable transactions: %w", err)
	}

	byID := make(map[uint]*models.InventoryItem, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}
	for _, t := range txns {
		if it, ok := byID[t.InventoryItemID]; ok {
			it.ConsumableTransactions = append(it.ConsumableTransactions, t)
		}
	}
	return byID, nil
}

// itemInput is the per-item, DB-free input to the algorithm.
type itemInput struct {
	itemID      uint
	productName string
	// reconcileSubs holds the item's reconcile steps in chronological order.
	reconcileSubs []reconcileStep
	// disposeSubs holds the item's dispose steps in chronological order.
	disposeSubs []disposeStep
	// purchases holds the item's purchase/transfer-in txns in chronological order.
	purchases []purchaseTxn
}

type reconcileStep struct {
	subID     uint
	createdAt time.Time
	prev      decimal.Decimal
	qty       decimal.Decimal
}

type disposeStep struct {
	subID     uint
	createdAt time.Time
	qty       decimal.Decimal
}

type purchaseTxn struct {
	txnID            uint
	createdAt        time.Time
	quantity         decimal.Decimal
	consumedQuantity decimal.Decimal
	price            float64
}

// correctedStep is the per-item algorithm output for one reconcile sub.
type correctedStep struct {
	subID     uint
	createdAt time.Time
	origQty   decimal.Decimal
	newQty    decimal.Decimal
	prevQty   decimal.Decimal // chain-consistent stock just before this sub
	drop      decimal.Decimal // net shrinkage (sell) consumed at this sub's date
	corrected bool
}

// resolveItem runs the rule + backward propagation for ONE item. It is the
// DB-free core of the data-fix.
//
// Window convention: range r between adjacent reconciles (sub1 earlier, sub2
// later) is half-open (sub1.createdAt, sub2.createdAt]; a row exactly on sub1
// belongs to the prior range (or start-stock). start_stock sums purchases created
// at or before the first reconcile.
func resolveItem(in itemInput) (startStock decimal.Decimal, steps []correctedStep, sells []SyntheticTxn, disposals []SyntheticTxn, consumedAfter map[uint]decimal.Decimal, ignoredTrailingDisposeSubIDs []uint, err error) {
	recs := in.reconcileSubs

	// Hard-fail on a negative dispose quantity; a zero quantity is a benign no-op.
	for _, dsp := range in.disposeSubs {
		if dsp.qty.LessThan(decimal.Zero) {
			return decimal.Zero, nil, nil, nil, nil, nil, fmt.Errorf(
				"item %d dispose submission %d (inventory_item_id %d) has negative quantity %s: aborting, no partial writes",
				in.itemID, dsp.subID, in.itemID, dsp.qty.String())
		}
	}

	// Reconcile-dependent math, only meaningful with at least one reconcile pair.
	// A dispose-only item skips it and FIFO-consumes its disposals from the full
	// purchase ledger below.
	if len(recs) > 0 {
		first := recs[0]
		last := recs[len(recs)-1]

		// Ignore purchases dated strictly after the last in-scope reconcile: they fall
		// outside every window and do not affect the shrinkage being corrected.
		if len(in.purchases) > 0 {
			kept := make([]purchaseTxn, 0, len(in.purchases))
			for _, p := range in.purchases {
				if isTrailingPurchase(p.createdAt, last.createdAt) {
					continue
				}
				kept = append(kept, p)
			}
			in.purchases = kept
		}

		// Ignore disposes dated strictly after the last reconcile, reporting their
		// sub-ids so the caller can leave them pending.
		if len(in.disposeSubs) > 0 {
			keptD := make([]disposeStep, 0, len(in.disposeSubs))
			for _, dsp := range in.disposeSubs {
				if dsp.createdAt.After(last.createdAt) {
					ignoredTrailingDisposeSubIDs = append(ignoredTrailingDisposeSubIDs, dsp.subID)
					continue
				}
				keptD = append(keptD, dsp)
			}
			in.disposeSubs = keptD
		}

		// start_stock: purchases created at or before the first reconcile.
		startStock = decimal.Zero
		for _, p := range in.purchases {
			if !p.createdAt.After(first.createdAt) {
				startStock = startStock.Add(p.quantity)
			}
		}

		rangePurchases := make([]decimal.Decimal, len(recs)) // rangePurchases[k] = purchases in (recs[k-1], recs[k]]
		for i := range rangePurchases {
			rangePurchases[i] = decimal.Zero
		}
		for _, p := range in.purchases {
			if !p.createdAt.After(first.createdAt) {
				continue // start-stock
			}
			k := rangeIndexFor(recs, p.createdAt)
			if k < 0 {
				return decimal.Zero, nil, nil, nil, nil, nil, fmt.Errorf(
					"item %d purchase txn %d (created_at %s) did not map to any range", in.itemID, p.txnID, p.createdAt.Format(time.RFC3339))
			}
			rangePurchases[k] = rangePurchases[k].Add(p.quantity)
		}

		// range_disposes[k] = pending dispose qty in (recs[k-1], recs[k]]. A dispose
		// before the first reconcile is an error (out of scope).
		rangeDisposes := make([]decimal.Decimal, len(recs))
		for i := range rangeDisposes {
			rangeDisposes[i] = decimal.Zero
		}
		for _, dsp := range in.disposeSubs {
			if !dsp.createdAt.After(first.createdAt) {
				return decimal.Zero, nil, nil, nil, nil, nil, fmt.Errorf(
					"item %d dispose submission %d (created_at %s) is not after the first reconcile %d (%s): out of scope",
					in.itemID, dsp.subID, dsp.createdAt.Format(time.RFC3339), first.subID, first.createdAt.Format(time.RFC3339))
			}
			k := rangeIndexFor(recs, dsp.createdAt)
			if k <= 0 {
				return decimal.Zero, nil, nil, nil, nil, nil, fmt.Errorf(
					"item %d dispose submission %d did not map to an inter-reconcile range", in.itemID, dsp.subID)
			}
			rangeDisposes[k] = rangeDisposes[k].Add(dsp.qty)
		}

		corrected := make([]decimal.Decimal, len(recs))
		wasCorrected := make([]bool, len(recs))
		for i, r := range recs {
			corrected[i] = r.qty
		}

		// Backward propagation (latest -> earliest):
		//   rule: corrected[k] <= corrected[k-1] + range_purchases[k] - range_disposes[k]
		//   else: corrected[k-1] := corrected[k] - range_purchases[k] + range_disposes[k]
		for k := len(recs) - 1; k >= 1; k-- {
			implied := corrected[k-1].Add(rangePurchases[k]).Sub(rangeDisposes[k])
			if corrected[k].GreaterThan(implied) {
				corrected[k-1] = corrected[k].Sub(rangePurchases[k]).Add(rangeDisposes[k])
				wasCorrected[k-1] = true
			}
		}

		// After propagation the first sub's corrected stock must not exceed start-stock.
		if corrected[0].GreaterThan(startStock) {
			return decimal.Zero, nil, nil, nil, nil, nil, fmt.Errorf(
				"item %d FATAL: corrected first-reconcile stock %s > start stock %s (sub %d): aborting, no partial writes",
				in.itemID, corrected[0].String(), startStock.String(), first.subID)
		}

		// Per-step prev_quantity and net shrinkage (drop):
		//   prev[0]  = start_stock
		//   prev[k]  = corrected[k-1] + range_purchases[k] - range_disposes[k]
		//   drop[k]  = prev[k] - corrected[k]
		steps = make([]correctedStep, len(recs))
		for k, r := range recs {
			prev := startStock
			if k >= 1 {
				prev = corrected[k-1].Add(rangePurchases[k]).Sub(rangeDisposes[k])
			}
			drop := prev.Sub(corrected[k])
			if drop.IsNegative() {
				return decimal.Zero, nil, nil, nil, nil, nil, fmt.Errorf(
					"item %d sub %d computed negative drop %s (prev %s, corrected %s)",
					in.itemID, r.subID, drop.String(), prev.String(), corrected[k].String())
			}
			steps[k] = correctedStep{
				subID:     r.subID,
				createdAt: r.createdAt,
				origQty:   r.qty,
				newQty:    corrected[k],
				prevQty:   prev,
				drop:      drop,
				corrected: wasCorrected[k],
			}
		}
	}

	// Chronological consumption events (reconcile sells + disposes), FIFO-consumed
	// against the purchase ledger.
	type consumeEvent struct {
		at     time.Time
		qty    decimal.Decimal
		isSell bool // true => sell, false => disposal
	}
	events := make([]consumeEvent, 0, len(steps)+len(in.disposeSubs))
	for _, s := range steps {
		if s.drop.GreaterThan(decimal.Zero) {
			events = append(events, consumeEvent{at: s.createdAt, qty: s.drop, isSell: true})
		}
	}
	for _, dsp := range in.disposeSubs {
		if dsp.qty.GreaterThan(decimal.Zero) {
			events = append(events, consumeEvent{at: dsp.createdAt, qty: dsp.qty, isSell: false})
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].at.Before(events[j].at) })

	// FIFO consume against the purchase ledger, oldest first, regardless of event
	// time. Exhausting the ledger while an event still needs stock is a hard-fail.
	consumedAfter = make(map[uint]decimal.Decimal, len(in.purchases))
	remaining := make([]decimal.Decimal, len(in.purchases))
	for i, p := range in.purchases {
		consumedAfter[p.txnID] = p.consumedQuantity
		remaining[i] = p.quantity.Sub(p.consumedQuantity)
	}
	idx := 0 // oldest purchase row not yet fully consumed (FIFO cursor)
	for _, ev := range events {
		toConsume := ev.qty
		for toConsume.GreaterThan(decimal.Zero) {
			for idx < len(in.purchases) && !remaining[idx].GreaterThan(decimal.Zero) {
				idx++
			}
			if idx >= len(in.purchases) {
				evName := "sell (reconcile shrinkage)"
				if !ev.isSell {
					evName = "disposal"
				}
				return decimal.Zero, nil, nil, nil, nil, nil, fmt.Errorf(
					"item %d FATAL: insufficient stock at %s — %s event needs %s more, but the purchase ledger is exhausted; aborting, no partial writes",
					in.itemID, ev.at.Format(time.RFC3339), evName, toConsume.String())
			}
			take := toConsume
			if remaining[idx].LessThan(take) {
				take = remaining[idx]
			}
			src := in.purchases[idx]
			txn := SyntheticTxn{
				InventoryItemID:     in.itemID,
				Quantity:            take,
				SourcePurchaseTxnID: src.txnID,
				COGSPrice:           src.price,
				BackdatedDate:       ev.at,
			}
			if ev.isSell {
				txn.TransactionType = string(models.InventoryTransactionTypeSell)
				sells = append(sells, txn)
			} else {
				txn.TransactionType = string(models.InventoryTransactionTypeDisposal)
				disposals = append(disposals, txn)
			}
			remaining[idx] = remaining[idx].Sub(take)
			consumedAfter[src.txnID] = consumedAfter[src.txnID].Add(take)
			toConsume = toConsume.Sub(take)
		}
	}

	return startStock, steps, sells, disposals, consumedAfter, ignoredTrailingDisposeSubIDs, nil
}

// rangeIndexFor returns the range index k (1..len-1) such that t is in the
// half-open window (recs[k-1].createdAt, recs[k].createdAt]; returns -1 if t is
// not strictly after recs[0] and within the last reconcile.
func rangeIndexFor(recs []reconcileStep, t time.Time) int {
	for k := 1; k < len(recs); k++ {
		if t.After(recs[k-1].createdAt) && !t.After(recs[k].createdAt) {
			return k
		}
	}
	return -1
}

// isTrailingPurchase reports whether purchaseAt is strictly after lastReconcileAt,
// i.e. out of scope. Single source of truth for both call sites.
func isTrailingPurchase(purchaseAt, lastReconcileAt time.Time) bool {
	return purchaseAt.After(lastReconcileAt)
}

// computeResolution loads inputs, runs the core, and builds the shared
// preview/apply plan.
func computeResolution(ctx context.Context, db *gorm.DB, inventoryID uint, reconcileIDs, disposeIDs []uint) (*ResolutionPlan, error) {
	subs, err := loadSubmissions(ctx, db, inventoryID, reconcileIDs, disposeIDs)
	if err != nil {
		return nil, err
	}

	// Need >= 2 reconciles to form a range.
	reconcileCount := 0
	for _, s := range subs {
		if s.isReconcile() {
			reconcileCount++
		}
	}
	if reconcileCount < 2 {
		return nil, fmt.Errorf("need at least 2 reconcile submissions to form a range; got %d", reconcileCount)
	}

	seen := make(map[uint]struct{})
	itemIDs := make([]uint, 0)
	for _, s := range subs {
		for _, it := range s.items {
			if _, ok := seen[it.itemID]; !ok {
				seen[it.itemID] = struct{}{}
				itemIDs = append(itemIDs, it.itemID)
			}
		}
	}

	items, err := loadActiveItems(ctx, db, inventoryID, itemIDs)
	for _, id := range itemIDs {
		if _, ok := items[id]; !ok {
			return nil, fmt.Errorf("inventory item %d referenced by submissions is not active", id)
		}
	}
	if err != nil {
		return nil, err
	}

	inputs := make(map[uint]*itemInput)
	getInput := func(id uint) *itemInput {
		if in, ok := inputs[id]; ok {
			return in
		}
		in := &itemInput{itemID: id}
		if it := items[id]; it != nil && it.Product != nil {
			in.productName = it.Product.Name
		}
		inputs[id] = in
		return in
	}
	for _, s := range subs {
		for _, it := range s.items {
			in := getInput(it.itemID)
			if s.isReconcile() {
				in.reconcileSubs = append(in.reconcileSubs, reconcileStep{
					subID: s.ID, createdAt: s.CreatedAt, prev: it.prev, qty: it.qty,
				})
			} else {
				in.disposeSubs = append(in.disposeSubs, disposeStep{
					subID: s.ID, createdAt: s.CreatedAt, qty: it.qty,
				})
			}
		}
	}
	for id, in := range inputs {
		for _, txn := range items[id].ConsumableTransactions {
			in.purchases = append(in.purchases, purchaseTxn{
				txnID:            txn.ID,
				createdAt:        txn.CreatedAt,
				quantity:         txn.Quantity,
				consumedQuantity: txn.ConsumedQuantity,
				price:            txn.Price,
			})
		}
		sort.SliceStable(in.reconcileSubs, func(i, j int) bool { return in.reconcileSubs[i].createdAt.Before(in.reconcileSubs[j].createdAt) })
		sort.SliceStable(in.disposeSubs, func(i, j int) bool { return in.disposeSubs[i].createdAt.Before(in.disposeSubs[j].createdAt) })
	}

	// Assert no in-scope purchase has been consumed yet. Trailing purchases (via
	// isTrailingPurchase, matching resolveItem) are exempt; a dispose-only item has
	// no boundary so all its purchases are in scope.
	for _, id := range itemIDs {
		in := inputs[id]
		if in == nil {
			continue
		}
		hasReconcile := len(in.reconcileSubs) > 0
		var lastReconcileAt time.Time
		if hasReconcile {
			lastReconcileAt = in.reconcileSubs[len(in.reconcileSubs)-1].createdAt
		}
		for _, p := range in.purchases {
			if hasReconcile && isTrailingPurchase(p.createdAt, lastReconcileAt) {
				continue // out of scope
			}
			if p.consumedQuantity.GreaterThan(decimal.Zero) {
				return nil, fmt.Errorf(
					"assumption failed: item %d purchase txn %d already has consumed_quantity %s > 0; this one-off assumes an unconsumed ledger",
					id, p.txnID, p.consumedQuantity.String())
			}
		}
	}

	plan := &ResolutionPlan{
		InventoryID:  inventoryID,
		ReconcileIDs: reconcileIDs,
		DisposeIDs:   disposeIDs,
	}

	// Run the core per item in ascending item-id order for stable output.
	orderedItemIDs := make([]uint, 0, len(inputs))
	for id := range inputs {
		orderedItemIDs = append(orderedItemIDs, id)
	}
	sort.Slice(orderedItemIDs, func(i, j int) bool { return orderedItemIDs[i] < orderedItemIDs[j] })

	correctedSubItems := make(map[uint]bool) // submission ids that were corrected
	consumedDeltaByTxn := make(map[uint]decimal.Decimal)

	// Per-dispose-submission tracking: a single dispose may be in-scope for one item
	// and trailing for another, so record both to decide AppliedAsIs membership.
	disposeAppliedItems := make(map[uint][]uint)         // dispose subID -> item ids it was applied for (in scope)
	disposeIgnoredTrailingItems := make(map[uint][]uint) // dispose subID -> item ids it was ignored-as-trailing for

	for _, id := range orderedItemIDs {
		in := inputs[id]
		startStock, steps, sells, disposals, consumedAfter, ignoredTrailingDisposeSubIDs, err := resolveItem(*in)
		if err != nil {
			return nil, err
		}

		// Trailing disposes ignored for this item: excluded from preview steps and
		// kept out of AppliedAsIs (left pending).
		ignoredTrailing := make(map[uint]bool, len(ignoredTrailingDisposeSubIDs))
		for _, subID := range ignoredTrailingDisposeSubIDs {
			ignoredTrailing[subID] = true
			disposeIgnoredTrailingItems[subID] = append(disposeIgnoredTrailingItems[subID], id)
		}
		for _, dsp := range in.disposeSubs {
			if !ignoredTrailing[dsp.subID] {
				disposeAppliedItems[dsp.subID] = append(disposeAppliedItems[dsp.subID], id)
			}
		}

		// Total outbound synthesized for this item; apply decrements live quantity by it.
		outbound := decimal.Zero
		for _, s := range sells {
			outbound = outbound.Add(s.Quantity)
		}
		for _, d := range disposals {
			outbound = outbound.Add(d.Quantity)
		}
		currentQty := items[id].Quantity
		appliedQty := currentQty.Sub(outbound)

		// final == last reconcile corrected count; a dispose-only item has none, so
		// its final stock is appliedQty.
		finalStock := appliedQty
		if len(steps) > 0 {
			finalStock = steps[len(steps)-1].newQty
		}

		itemPlan := ItemPlan{
			InventoryItemID: id,
			ProductName:     in.productName,
			StartStock:      startStock,
			FinalStock:      finalStock,
			CurrentQuantity: currentQty,
			AppliedQuantity: appliedQty,
		}
		for _, s := range steps {
			itemPlan.Steps = append(itemPlan.Steps, ItemStep{
				SubmissionID:      s.subID,
				SubmissionType:    string(models.InventorySubmissionTypeReconcile),
				CreatedAt:         s.createdAt,
				OrigQuantity:      s.origQty,
				CorrectedQuantity: s.newQty,
				PrevQuantity:      s.prevQty,
				Drop:              s.drop,
				Corrected:         s.corrected,
			})
			if s.corrected {
				correctedSubItems[s.subID] = true
				plan.Corrections = append(plan.Corrections, SubmissionCorrection{
					SubmissionID:    s.subID,
					InventoryItemID: id,
					OrigQuantity:    s.origQty,
					NewQuantity:     s.newQty,
					OrigPrev:        recStepPrev(in, s.subID),
					NewPrev:         s.prevQty,
				})
			}
		}
		// Informational dispose steps. in.disposeSubs is the original unfiltered set
		// (resolveItem mutated only its by-value copy), so filter via ignoredTrailing.
		for _, dsp := range in.disposeSubs {
			if ignoredTrailing[dsp.subID] {
				continue
			}
			itemPlan.Steps = append(itemPlan.Steps, ItemStep{
				SubmissionID:      dsp.subID,
				SubmissionType:    string(models.InventorySubmissionTypeDispose),
				CreatedAt:         dsp.createdAt,
				OrigQuantity:      dsp.qty,
				CorrectedQuantity: dsp.qty,
				Drop:              dsp.qty,
			})
		}
		sort.SliceStable(itemPlan.Steps, func(i, j int) bool { return itemPlan.Steps[i].CreatedAt.Before(itemPlan.Steps[j].CreatedAt) })
		plan.Items = append(plan.Items, itemPlan)

		plan.Txns = append(plan.Txns, sells...)
		plan.Txns = append(plan.Txns, disposals...)

		// Source consumed updates + deltas.
		for _, p := range in.purchases {
			after := consumedAfter[p.txnID]
			delta := after.Sub(p.consumedQuantity)
			if delta.GreaterThan(decimal.Zero) {
				plan.srcTxnUpdates = append(plan.srcTxnUpdates, srcConsumedUpdate{txnID: p.txnID, consumedQuantity: after})
				consumedDeltaByTxn[p.txnID] = delta
			}
		}

		// Item quantity update: decrement live quantity by synthesized outbound
		// (new_quantity = current_live - Σ outbound) rather than overwriting with
		// FinalStock, which would discard net post-reconcile activity.
		plan.itemUpdates = append(plan.itemUpdates, itemQtyUpdate{
			itemID:      id,
			originalQty: currentQty,
			newQty:      appliedQty,
		})
	}

	// Stable txn ordering: item, then date, then type.
	sort.SliceStable(plan.Txns, func(i, j int) bool {
		if plan.Txns[i].InventoryItemID != plan.Txns[j].InventoryItemID {
			return plan.Txns[i].InventoryItemID < plan.Txns[j].InventoryItemID
		}
		if !plan.Txns[i].BackdatedDate.Equal(plan.Txns[j].BackdatedDate) {
			return plan.Txns[i].BackdatedDate.Before(plan.Txns[j].BackdatedDate)
		}
		return plan.Txns[i].TransactionType < plan.Txns[j].TransactionType
	})

	deltaIDs := make([]uint, 0, len(consumedDeltaByTxn))
	for id := range consumedDeltaByTxn {
		deltaIDs = append(deltaIDs, id)
	}
	sort.Slice(deltaIDs, func(i, j int) bool { return deltaIDs[i] < deltaIDs[j] })
	for _, id := range deltaIDs {
		plan.ConsumedDeltas = append(plan.ConsumedDeltas, ConsumedQuantityDelta{PurchaseTxnID: id, Delta: consumedDeltaByTxn[id]})
	}

	// Applied-as-is membership: every uncorrected reconcile, and every dispose that
	// was in-scope for all the items it touches.
	for _, s := range subs {
		if s.isReconcile() {
			if correctedSubItems[s.ID] {
				continue
			}
			plan.AppliedAsIs = append(plan.AppliedAsIs, s.ID)
			continue
		}

		// Dispose submission.
		applied := disposeAppliedItems[s.ID]
		ignored := disposeIgnoredTrailingItems[s.ID]

		// A single dispose in-scope for one item but trailing for another cannot be
		// resolved consistently.
		if len(applied) > 0 && len(ignored) > 0 {
			return nil, fmt.Errorf(
				"item-level conflict: dispose submission %d is trailing (after last reconcile) for item %d but in-scope for item %d; cannot resolve consistently",
				s.ID, ignored[0], applied[0])
		}

		// Trailing for every item -> leave pending.
		if len(applied) == 0 {
			continue
		}
		plan.AppliedAsIs = append(plan.AppliedAsIs, s.ID)
	}

	return plan, nil
}

// recStepPrev returns the original recorded prev_quantity for a reconcile sub id.
func recStepPrev(in *itemInput, subID uint) decimal.Decimal {
	for _, r := range in.reconcileSubs {
		if r.subID == subID {
			return r.prev
		}
	}
	return decimal.Zero
}

// applyResolution persists the plan in one transaction.
func applyResolution(ctx context.Context, db *gorm.DB, plan *ResolutionPlan) error {
	allSubIDs := make([]uint, 0, len(plan.ReconcileIDs)+len(plan.DisposeIDs))
	allSubIDs = append(allSubIDs, plan.ReconcileIDs...)
	allSubIDs = append(allSubIDs, plan.DisposeIDs...)

	correctionByID := make(map[uint][]SubmissionCorrection)
	for _, c := range plan.Corrections {
		correctionByID[c.SubmissionID] = append(correctionByID[c.SubmissionID], c)
	}

	now := time.Now()

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Re-check all target submissions are still pending.
		var current []models.InventorySubmission
		if err := tx.Where("id IN ?", allSubIDs).Find(&current).Error; err != nil {
			return fmt.Errorf("failed to re-read submissions: %w", err)
		}
		if len(current) != len(allSubIDs) {
			return fmt.Errorf("submission set changed before apply (expected %d, found %d)", len(allSubIDs), len(current))
		}
		currentByID := make(map[uint]models.InventorySubmission, len(current))
		for _, s := range current {
			if s.ApprovalStatus != models.InventorySubmissionApprovalStatusPending {
				return fmt.Errorf("submission %d no longer pending (approval_status=%s); aborting apply", s.ID, s.ApprovalStatus)
			}
			currentByID[s.ID] = s
		}

		// 1. Optimistic-lock + update inventory item quantities.
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

		// 2. Insert backdated synthetic txns. Preset CreatedAt survives autoCreateTime.
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
				return fmt.Errorf("failed to insert synthetic %s txn: %w", t.TransactionType, err)
			}
		}

		// 3. Column-scoped UPDATE of source purchase consumed_quantity. Writes the
		//    preview-time value without re-locking (single off-peak maintenance run).
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

		// 4. Corrected reconcile submissions: soft-delete original, recreate a clone
		//    with the same metadata and a corrected payload.
		for _, subID := range plan.ReconcileIDs {
			corrs, corrected := correctionByID[subID]
			if !corrected {
				continue
			}
			orig := currentByID[subID]

			newPayload, err := correctedPayload(orig.Payload, corrs)
			if err != nil {
				return fmt.Errorf("submission %d: %w", subID, err)
			}

			if err := tx.Delete(&models.InventorySubmission{}, subID).Error; err != nil {
				return fmt.Errorf("failed to soft-delete submission %d: %w", subID, err)
			}

			clone := models.InventorySubmission{
				Base: models.Base{
					CreatedAt: orig.CreatedAt,
					CreatedBy: orig.CreatedBy,
				},
				InventoryID:      orig.InventoryID,
				SubmissionType:   orig.SubmissionType,
				ProcessingStatus: models.InventorySubmissionStatusCompleted,
				ApprovalStatus:   models.InventorySubmissionApprovalStatusApproved,
				Payload:          newPayload,
				Reason:           reconcileOneOffReason,
			}
			if err := tx.Create(&clone).Error; err != nil {
				return fmt.Errorf("failed to create corrected clone of submission %d: %w", subID, err)
			}

			// BeforeCreate stamps created_by with the system actor; restore the
			// original submitter via a hook-free UpdateColumn. updated_by stays the
			// system actor.
			if err := tx.Model(&clone).UpdateColumn("created_by", orig.CreatedBy).Error; err != nil {
				return fmt.Errorf("failed to restore created_by on corrected clone of submission %d: %w", subID, err)
			}
		}

		// 5. Applied-as-is submissions: move to approved + completed + reason.
		if len(plan.AppliedAsIs) > 0 {
			updates, err := pkg.WithUpdateFields(ctx, map[string]interface{}{
				"approval_status":   models.InventorySubmissionApprovalStatusApproved,
				"processing_status": models.InventorySubmissionStatusCompleted,
				"reason":            reconcileOneOffReason,
			})
			if err != nil {
				return fmt.Errorf("failed to prepare submission update fields: %w", err)
			}
			if err := tx.Model(&models.InventorySubmission{}).
				Where("id IN ?", plan.AppliedAsIs).
				Updates(updates).Error; err != nil {
				return fmt.Errorf("failed to update applied-as-is submission statuses: %w", err)
			}
		}

		return nil
	})
}

// correctedPayload rewrites the per-item quantity and prev_quantity for the
// corrected items, leaving others untouched.
func correctedPayload(orig json.RawMessage, corrs []SubmissionCorrection) (json.RawMessage, error) {
	var req dto.ReconcileInventoryRequest
	if err := json.Unmarshal(orig, &req); err != nil {
		return nil, fmt.Errorf("failed to parse original payload: %w", err)
	}
	byItem := make(map[uint]SubmissionCorrection, len(corrs))
	for _, c := range corrs {
		byItem[c.InventoryItemID] = c
	}
	for i := range req.Items {
		if c, ok := byItem[req.Items[i].InventoryItemID]; ok {
			q := c.NewQuantity
			req.Items[i].Quantity = &q
			req.Items[i].PrevQuantity = c.NewPrev
		}
	}
	out, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal corrected payload: %w", err)
	}
	return out, nil
}

func printPendingDisposeSubmissions(db *gorm.DB, inventoryID uint) {
	var subs []models.InventorySubmission
	if err := db.
		Where("inventory_id = ? AND submission_type = ? AND approval_status = ?",
			inventoryID, models.InventorySubmissionTypeDispose, models.InventorySubmissionApprovalStatusPending).
		Order("created_at ASC").
		Find(&subs).Error; err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to list pending dispose submissions: %v\n", err)
		return
	}
	fmt.Printf("\nPending dispose submissions for inventory %d (%d):\n", inventoryID, len(subs))
	if len(subs) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, sub := range subs {
		fmt.Printf("  id=%d created_at=%s\n", sub.ID, sub.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Println("  Pass the chosen ids via --dispose-submissions to fold them in.")
}

func printPlanSummary(plan *ResolutionPlan) {
	fmt.Println("\n=== Resolution plan summary (#46) ===")
	fmt.Printf("Inventory: %d   Reconcile: %v   Dispose: %v\n", plan.InventoryID, plan.ReconcileIDs, plan.DisposeIDs)
	fmt.Printf("Items: %d   Synthetic txns: %d   Corrections: %d\n", len(plan.Items), len(plan.Txns), len(plan.Corrections))

	fmt.Println("\nPer-item:")
	for _, item := range plan.Items {
		fmt.Printf("  item %d (%s): start=%s final=%s current=%s applied=%s\n",
			item.InventoryItemID, item.ProductName, item.StartStock.String(), item.FinalStock.String(),
			item.CurrentQuantity.String(), item.AppliedQuantity.String())
		for _, s := range item.Steps {
			tag := ""
			if s.Corrected {
				tag = "  [CORRECTED]"
			}
			fmt.Printf("    %s sub %d (%s): orig=%s corrected=%s prev=%s drop=%s%s\n",
				s.CreatedAt.Format("2006-01-02"), s.SubmissionID, s.SubmissionType,
				s.OrigQuantity.String(), s.CorrectedQuantity.String(), s.PrevQuantity.String(), s.Drop.String(), tag)
		}
	}

	fmt.Printf("\nBackdated synthetic txns (%d):\n", len(plan.Txns))
	for _, t := range plan.Txns {
		fmt.Printf("  item %d %s qty=%s srcPurchaseTxn=%d cogs=%.2f date=%s\n",
			t.InventoryItemID, t.TransactionType, t.Quantity.String(), t.SourcePurchaseTxnID, t.COGSPrice, t.BackdatedDate.Format("2006-01-02 15:04:05"))
	}

	if len(plan.Corrections) > 0 {
		fmt.Printf("\nCorrections (%d) — original soft-deleted, clone recreated:\n", len(plan.Corrections))
		for _, c := range plan.Corrections {
			fmt.Printf("  sub %d item %d: qty %s->%s  prev %s->%s\n",
				c.SubmissionID, c.InventoryItemID, c.OrigQuantity.String(), c.NewQuantity.String(), c.OrigPrev.String(), c.NewPrev.String())
		}
	}
}

func writePlanJSON(plan *ResolutionPlan, path string) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write plan to %s: %w", path, err)
	}
	return nil
}

// writePlanXLSX writes one row per item: start stock, each reconcile sub's
// corrected count, then the final state.
func writePlanXLSX(plan *ResolutionPlan, path string) error {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := "Reconcile"
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return fmt.Errorf("failed to create sheet: %w", err)
	}
	f.SetActiveSheet(idx)
	_ = f.DeleteSheet("Sheet1")

	// Collect reconcile sub ids in chronological order (same across items).
	type col struct {
		subID uint
		at    time.Time
	}
	var cols []col
	if len(plan.Items) > 0 {
		for _, s := range plan.Items[0].Steps {
			if s.SubmissionType == string(models.InventorySubmissionTypeReconcile) {
				cols = append(cols, col{subID: s.SubmissionID, at: s.CreatedAt})
			}
		}
	}
	headers := []string{"Item ID", "Product", "Start Stock"}
	for _, c := range cols {
		headers = append(headers, fmt.Sprintf("Sub %d (%s)", c.subID, c.at.Format("2006-01-02")))
	}
	headers = append(headers, "Final Stock", "Current Stock", "Applied Stock")
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}

	for r, item := range plan.Items {
		row := r + 2
		colIdx := 1
		set := func(v interface{}) {
			cell, _ := excelize.CoordinatesToCellName(colIdx, row)
			_ = f.SetCellValue(sheet, cell, v)
			colIdx++
		}
		set(item.InventoryItemID)
		set(item.ProductName)
		set(item.StartStock.String())
		stepByID := make(map[uint]ItemStep)
		for _, s := range item.Steps {
			stepByID[s.SubmissionID] = s
		}
		for _, c := range cols {
			if s, ok := stepByID[c.subID]; ok {
				set(s.CorrectedQuantity.String())
			} else {
				set("")
			}
		}
		set(item.FinalStock.String())
		set(item.CurrentQuantity.String())
		set(item.AppliedQuantity.String())
	}

	if err := f.SaveAs(path); err != nil {
		return fmt.Errorf("failed to save xlsx to %s: %w", path, err)
	}
	return nil
}

func init() {
	reconcileResolveCmd.Flags().UintVar(&reconcileInventoryID, "inventory", 1, "inventory ID to resolve")
	reconcileResolveCmd.Flags().UintSliceVar(&reconcileSubmissionID, "submissions", []uint{1, 2, 4, 6}, "reconcile submission IDs to resolve")
	reconcileResolveCmd.Flags().UintSliceVar(&disposeSubmissionID, "dispose-submissions", nil, "dispose submission IDs to fold in (comma-separated; default none)")
	reconcileResolveCmd.Flags().BoolVar(&reconcileApply, "apply", false, "persist the plan (default: preview only)")
	reconcileResolveCmd.Flags().StringVar(&reconcileOut, "out", "", "write the resolution plan as JSON (and a sibling .xlsx) to this path")
	reconcileResolveCmd.Flags().StringVar(&reconcileDBURL, "db-url", "", "full Postgres DSN to run against directly (overrides global config; recommend sslmode + quiet)")
	reconcileResolveCmd.Flags().BoolVar(&reconcileProdConfirm, "prod-confirm", false, "required to --apply when --db-url host is non-localhost")

	reconcileCmd.AddCommand(reconcileResolveCmd)
	rootCmd.AddCommand(reconcileCmd)
}
