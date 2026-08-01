// Package excel provides Excel formatting utilities for inventory reports.
//
// inventory_in_out_shaper.go implements the pure data shaper that turns raw
// inventory-transaction + PO metadata into typed ExportRows for the
// inventory in/out export. No I/O — this layer is the unit of testability for
// FIFO consumption, beginning/ending-stock derivation, and row inclusion
// rules.
package excel

import (
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
)

// ShaperInput is the input bundle for BuildExportRows. The orchestrator
// service collects this from existing repositories.
type ShaperInput struct {
	StartDate    time.Time // inclusive, midnight UTC
	EndDate      time.Time // inclusive (whole day); the writer/queries use EndDate+1d for exclusive cutoffs

	InventoryID  uint
	Items        []*ItemInfo // inventory items in scope (one per product+inventory)
	HistoricalTxns []*repository.InventoryTransactionWithCounter // txns strictly before StartDate
	PeriodTxns   []*repository.InventoryTransactionWithCounter   // txns within [StartDate, EndDate+1d)

	// POInfo: keyed by purchase_order_item_id. Includes POIs referenced by
	// any in-window txn AND any POI with remaining stock at any point in
	// the window (i.e. any POI relevant to a row). Lookups must NOT be
	// filtered by date — FIFO source POIs may live outside the window.
	POInfo map[uint]*repository.POItemSellingPriceInfo
}

// ItemInfo represents an inventory_item row enriched with product/unit names.
type ItemInfo struct {
	ItemID      uint
	ProductID   uint
	ProductName string
	UnitName    string
}

// ExportRow is a single PO-item row in the spreadsheet. Daily purchase-amount
// columns are sparse: only the row's own purchase day is populated, summing
// any same-day purchase txns onto that POI.
type ExportRow struct {
	ProductID    uint
	ProductName  string
	UnitName     string

	POItemID     uint
	POID         uint
	PONumber     string

	// AdjustmentSourceTxnID is the reconcile_stock_up txn id keying a zero-cost
	// adjustment (found-stock) row family. 0 for normal PO-item rows.
	AdjustmentSourceTxnID uint

	PurchasePrice decimal.Decimal
	// SellingPrice is nil when the POI has no effective price; the writer then
	// renders the cell (and its revenue) as "-". A real 0 is a valid price and
	// renders as 0.
	SellingPrice *decimal.Decimal

	// DailyPurchases: amounts indexed by day-of-window (0-based inclusive).
	// Sparse: only days where this POI received a purchase have non-zero entries.
	DailyPurchases map[int]decimal.Decimal

	BeginningStock decimal.Decimal
	EndingStock    decimal.Decimal

	TotalPurchasedAmount decimal.Decimal
	TotalDisposedAmount  decimal.Decimal
	SubtotalSold         decimal.Decimal
	TotalTransferredIn   decimal.Decimal
	TotalTransferredOut  decimal.Decimal
}

// ExportRows is the typed output of the shaper. The writer consumes this.
type ExportRows struct {
	StartDate time.Time
	EndDate   time.Time
	DayCount  int // EndDate - StartDate + 1
	Rows      []*ExportRow
}

// BuildExportRows is the pure shaper.
//
// Algorithm overview:
//  1. For each PO item with activity OR carry-over inventory at any point in
//     the window, emit one row.
//  2. Beginning stock per POI: sum stock-deltas of all historical txns whose
//     source POI is this POI (purchases/transfer-ins add; FIFO-attributed
//     consumes subtract).
//  3. Apply window txns chronologically: purchases add to row's day cell;
//     consumes (sell/disposal/transfer-out) attribute to the source POI via
//     CounterPOIID and decrement its stock.
//  4. Ending stock = beginning + window in - window out.
//  5. Exclude rows where beginning_stock + total_in == 0 AND no window
//     activity (i.e. fully depleted before window start with no in-window
//     purchases).
func BuildExportRows(in ShaperInput) *ExportRows {
	dayCount := daysBetweenInclusive(in.StartDate, in.EndDate)

	// --- (1) build product map by item id (so we can flow txns→product info)
	itemToProduct := make(map[uint]*ItemInfo, len(in.Items))
	for _, it := range in.Items {
		itemToProduct[it.ItemID] = it
	}

	// --- (2) seed rows. Any POI that appears in POInfo and matches an item
	//     in this inventory is a candidate. We fold beginning stock from
	//     historical txns and window stock from period txns onto these.
	rowByPOItem := make(map[uint]*ExportRow)
	getOrCreateRow := func(poItemID uint, productID uint) *ExportRow {
		if r, ok := rowByPOItem[poItemID]; ok {
			return r
		}
		info := in.POInfo[poItemID]
		if info == nil {
			return nil
		}
		// Resolve product/unit from any item with this product. Same product
		// may live in multiple items; we pick whichever item the txn referenced.
		var productName, unitName string
		for _, it := range in.Items {
			if it.ProductID == productID {
				productName = it.ProductName
				unitName = it.UnitName
				break
			}
		}
		// Seed purchase price from POI metadata (purchase_order_items.unit_price)
		// so rows with no in-window purchase txn (e.g. carry-over from
		// historical purchases) still have the correct unit cost. The in-window
		// purchase-txn price below is a fallback for legacy POIs whose
		// unit_price wasn't populated.
		purchasePrice := info.PurchasePrice
		r := &ExportRow{
			ProductID:      productID,
			ProductName:    productName,
			UnitName:       unitName,
			POItemID:       poItemID,
			POID:           info.POID,
			PONumber:       info.PONumber,
			PurchasePrice:  purchasePrice,
			SellingPrice:   info.EffectivePrice, // nil → rendered "-" by the writer
			DailyPurchases: make(map[int]decimal.Decimal),
		}
		rowByPOItem[poItemID] = r
		return r
	}

	// Zero-cost adjustment (found-stock) rows, keyed by reconcile_stock_up txn id.
	rowByAdjustment := make(map[uint]*ExportRow)
	getOrCreateAdjustmentRow := func(txnID uint, productID uint, label string) *ExportRow {
		if r, ok := rowByAdjustment[txnID]; ok {
			return r
		}
		var productName, unitName string
		for _, it := range in.Items {
			if it.ProductID == productID {
				productName = it.ProductName
				unitName = it.UnitName
				break
			}
		}
		r := &ExportRow{
			ProductID:             productID,
			ProductName:           productName,
			UnitName:              unitName,
			PONumber:              label,
			PurchasePrice:         decimal.Zero,
			SellingPrice:          nil,
			DailyPurchases:        make(map[int]decimal.Decimal),
			AdjustmentSourceTxnID: txnID,
		}
		rowByAdjustment[txnID] = r
		return r
	}

	// Found-stock layer txn ids: reconcile_stock_ups and transfer-ins flagged
	// IsAdjustment (set at transfer time, so found provenance carries across any
	// number of transfer hops). A consume of found stock references one of these
	// via counter_transaction_id, so membership routes both the receipt layer and
	// its consumes onto the same adjustment row — including a re-sale of units
	// transferred (once or many times) into another inventory.
	// Value is the row label. A layer is labelled by its own movement: opening stock
	// and a counting correction are both PO-less zero-cost layers keyed the same way,
	// but they are not the same event. A transferred layer inherits only
	// adjustment-ness through IsAdjustment, which does not record origin, so it shows
	// under the generic adjustment label at the destination — the same behaviour
	// transferred reconcile_stock_up has always had.
	foundLayerIDs := make(map[uint]string)
	markFound := func(txns []*repository.InventoryTransactionWithCounter) {
		for _, t := range txns {
			if t == nil || t.InventoryTransaction == nil {
				continue
			}
			if t.IsAdjustment || t.TransactionType == models.InventoryTransactionTypeReconcileStockUp {
				label := models.AdjustmentCategoryLabel
				if t.TransactionType == models.InventoryTransactionTypeInitial {
					label = models.OpeningStockCategoryLabel
				}
				foundLayerIDs[t.ID] = label
			}
		}
	}
	markFound(in.HistoricalTxns)
	markFound(in.PeriodTxns)

	// adjustmentKey returns the adjustment row key and its label for a txn: a found
	// receipt layer keys on its own id; a consume of found stock keys on its counter
	// (that layer), so both land on the same row under that layer's label.
	adjustmentKey := func(t *repository.InventoryTransactionWithCounter) (uint, string, bool) {
		if t == nil || t.InventoryTransaction == nil {
			return 0, "", false
		}
		switch t.TransactionType {
		case models.InventoryTransactionTypeReconcileStockUp,
			models.InventoryTransactionTypeInitial,
			models.InventoryTransactionTypeTransferIn:
			if label, ok := foundLayerIDs[t.ID]; ok {
				return t.ID, label, true
			}
		case models.InventoryTransactionTypeSell, models.InventoryTransactionTypeDisposal, models.InventoryTransactionTypeTransferOut:
			if t.CounterTransactionID == nil {
				return 0, "", false
			}
			if label, ok := foundLayerIDs[*t.CounterTransactionID]; ok {
				return *t.CounterTransactionID, label, true
			}
		}
		return 0, "", false
	}

	// --- (3) historical txns → beginning stock per POI (FIFO already applied
	//     in DB via counter_transaction_id; we just sum deltas attributed
	//     to each POI). Found-stock (reconcile_stock_up and its consumes) go to
	//     the adjustment row family instead.
	for _, t := range in.HistoricalTxns {
		if t == nil || t.InventoryTransaction == nil {
			continue
		}
		productID := lookupProductID(itemToProduct, t.InventoryItemID)
		if productID == 0 {
			continue
		}
		if adjID, label, ok := adjustmentKey(t); ok {
			row := getOrCreateAdjustmentRow(adjID, productID, label)
			row.BeginningStock = row.BeginningStock.Add(t.TransactionType.StockDelta(t.Quantity))
			continue
		}
		poiID := windowPOI(t)
		if poiID == 0 {
			continue
		}
		row := getOrCreateRow(poiID, productID)
		if row == nil {
			continue
		}
		row.BeginningStock = row.BeginningStock.Add(t.TransactionType.StockDelta(t.Quantity))
	}

	// --- (4) period txns → daily purchases & per-POI movement.
	// Sort period txns chronologically so daily attribution is deterministic.
	period := append([]*repository.InventoryTransactionWithCounter{}, in.PeriodTxns...)
	sort.SliceStable(period, func(i, j int) bool {
		return period[i].CreatedAt.Before(period[j].CreatedAt)
	})

	for _, t := range period {
		productID := lookupProductID(itemToProduct, t.InventoryItemID)
		if productID == 0 {
			continue
		}

		// Found-stock: the stock-up is the window "in"; sells/disposals/transfers
		// of found units are the window "out". Attributed to the adjustment row.
		if adjID, label, ok := adjustmentKey(t); ok {
			row := getOrCreateAdjustmentRow(adjID, productID, label)
			qty := t.Quantity
			addDaily := func() {
				if day := dayIndex(in.StartDate, t.CreatedAt); day >= 0 && day < dayCount {
					row.DailyPurchases[day] = row.DailyPurchases[day].Add(qty)
				}
			}
			switch t.TransactionType {
			// Both are the window "in" for their row. Omitting `initial` here would
			// leave windowIn at 0 while its consumes still land in windowOut, so the
			// row would foot negative.
			case models.InventoryTransactionTypeReconcileStockUp,
				models.InventoryTransactionTypeInitial:
				row.TotalPurchasedAmount = row.TotalPurchasedAmount.Add(qty)
				addDaily()
			case models.InventoryTransactionTypeTransferIn:
				row.TotalTransferredIn = row.TotalTransferredIn.Add(qty)
				addDaily()
			case models.InventoryTransactionTypeSell:
				row.SubtotalSold = row.SubtotalSold.Add(qty)
			case models.InventoryTransactionTypeDisposal:
				row.TotalDisposedAmount = row.TotalDisposedAmount.Add(qty)
			case models.InventoryTransactionTypeTransferOut:
				row.TotalTransferredOut = row.TotalTransferredOut.Add(qty)
			}
			continue
		}

		poiID := windowPOI(t)
		if poiID == 0 {
			continue
		}
		row := getOrCreateRow(poiID, productID)
		if row == nil {
			continue
		}

		qty := t.Quantity
		switch t.TransactionType {
		case models.InventoryTransactionTypePurchase:
			day := dayIndex(in.StartDate, t.CreatedAt)
			if day >= 0 && day < dayCount {
				row.DailyPurchases[day] = row.DailyPurchases[day].Add(qty)
			}
			row.TotalPurchasedAmount = row.TotalPurchasedAmount.Add(qty)
			// Capture purchase price from the purchase txn (PO item's unit cost).
			if row.PurchasePrice.IsZero() {
				row.PurchasePrice = decimal.NewFromFloat(t.Price)
			}
		case models.InventoryTransactionTypeTransferIn:
			// Transfer-in into this inventory attributes to source POI of the
			// originating transfer-out (resolved via CounterPOIID upstream).
			day := dayIndex(in.StartDate, t.CreatedAt)
			if day >= 0 && day < dayCount {
				row.DailyPurchases[day] = row.DailyPurchases[day].Add(qty)
			}
			row.TotalTransferredIn = row.TotalTransferredIn.Add(qty)
			if row.PurchasePrice.IsZero() {
				row.PurchasePrice = decimal.NewFromFloat(t.Price)
			}
		case models.InventoryTransactionTypeSell:
			row.SubtotalSold = row.SubtotalSold.Add(qty)
		case models.InventoryTransactionTypeDisposal:
			row.TotalDisposedAmount = row.TotalDisposedAmount.Add(qty)
		case models.InventoryTransactionTypeTransferOut:
			row.TotalTransferredOut = row.TotalTransferredOut.Add(qty)
		}
	}

	// --- (5) compute ending stock and assemble final list.
	//   Inclusion rule: drop rows where beginning_stock == 0 AND no in-window
	//   activity (i.e. POI fully depleted before window start with no
	//   in-window movement).
	allRows := make([]*ExportRow, 0, len(rowByPOItem)+len(rowByAdjustment))
	for _, r := range rowByPOItem {
		allRows = append(allRows, r)
	}
	for _, r := range rowByAdjustment {
		allRows = append(allRows, r)
	}
	out := make([]*ExportRow, 0, len(allRows))
	for _, r := range allRows {
		windowIn := r.TotalPurchasedAmount.Add(r.TotalTransferredIn)
		windowOut := r.SubtotalSold.Add(r.TotalDisposedAmount).Add(r.TotalTransferredOut)
		r.EndingStock = r.BeginningStock.Add(windowIn).Sub(windowOut)

		hasActivity := !windowIn.IsZero() || !windowOut.IsZero()
		hasCarryOver := !r.BeginningStock.IsZero()
		if !hasActivity && !hasCarryOver {
			continue
		}
		out = append(out, r)
	}

	// Stable order: by product name, then by product ID (so distinct products
	// sharing a display name remain in adjacent but separate groups), then by
	// PO number, then by POI ID.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.ProductName != b.ProductName {
			return a.ProductName < b.ProductName
		}
		if a.ProductID != b.ProductID {
			return a.ProductID < b.ProductID
		}
		if a.PONumber != b.PONumber {
			return a.PONumber < b.PONumber
		}
		if a.POItemID != b.POItemID {
			return a.POItemID < b.POItemID
		}
		return a.AdjustmentSourceTxnID < b.AdjustmentSourceTxnID
	})

	return &ExportRows{
		StartDate: in.StartDate,
		EndDate:   in.EndDate,
		DayCount:  dayCount,
		Rows:      out,
	}
}

// windowPOI returns the source POI for a txn. Purchases use their
// own PurchaseOrderItemID; consumes use the counter purchase's POI.
func windowPOI(t *repository.InventoryTransactionWithCounter) uint {
	if t == nil || t.InventoryTransaction == nil {
		return 0
	}
	switch t.TransactionType {
	case models.InventoryTransactionTypePurchase:
		if t.PurchaseOrderItemID != nil {
			return *t.PurchaseOrderItemID
		}
	default:
		if t.CounterPOIID != nil {
			return *t.CounterPOIID
		}
	}
	return 0
}

func lookupProductID(m map[uint]*ItemInfo, itemID uint) uint {
	if it, ok := m[itemID]; ok {
		return it.ProductID
	}
	return 0
}

func daysBetweenInclusive(start, end time.Time) int {
	if end.Before(start) {
		return 0
	}
	d := int(end.Sub(start).Hours() / 24)
	return d + 1
}

func dayIndex(start, t time.Time) int {
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	tDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, start.Location())
	return int(tDay.Sub(startDay).Hours() / 24)
}
