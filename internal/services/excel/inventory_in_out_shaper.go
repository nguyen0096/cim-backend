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
	HistoricalTxns []*models.InventoryTransaction              // txns strictly before StartDate
	PeriodTxns   []*repository.InventoryTransactionWithCounter // txns within [StartDate, EndDate+1d)

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

	// --- (3) historical txns → beginning stock per POI (FIFO already applied
	//     in DB via counter_transaction_id; we just sum deltas attributed
	//     to each POI).
	for _, t := range in.HistoricalTxns {
		poiID := historicalPOI(t)
		if poiID == 0 {
			continue
		}
		productID := lookupProductID(itemToProduct, t.InventoryItemID)
		if productID == 0 {
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
	out := make([]*ExportRow, 0, len(rowByPOItem))
	for _, r := range rowByPOItem {
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
		return a.POItemID < b.POItemID
	})

	return &ExportRows{
		StartDate: in.StartDate,
		EndDate:   in.EndDate,
		DayCount:  dayCount,
		Rows:      out,
	}
}

// historicalPOI returns the POI id this historical txn attributes to. For
// purchases / transfer-ins the txn carries its own purchase_order_item_id;
// for consumes the source POI is resolved via the counter purchase txn — the
// service layer is expected to expand HistoricalTxns to include that linkage,
// but in the simpler shaper we only count purchases / transfer-ins toward
// beginning stock and use the period query (which DOES join the counter) to
// model FIFO attribution within the window. This conservative approach
// matches the inventory_timeline_service algorithm.
//
// Net effect: BeginningStock here = sum of in-stock POI receipts before the
// window minus the consumes that historically depleted those POIs. That
// requires us to also process historical consumes; we do this by deferring
// to caller-supplied HistoricalTxns and using StockDelta on whichever POI
// the historical txn referenced (purchase: own POI; consume: counter POI
// not available without a join — so for consumes we expect the orchestrator
// to use the same join to resolve them, and pass them in here with the POI
// already resolved on PurchaseOrderItemID).
//
// To keep the shaper purely data-driven, we treat the txn's
// PurchaseOrderItemID as the resolved POI for ALL historical txns. The
// orchestrator must populate it correctly (purchase: own; consume: counter).
func historicalPOI(t *models.InventoryTransaction) uint {
	if t == nil || t.PurchaseOrderItemID == nil {
		return 0
	}
	return *t.PurchaseOrderItemID
}

// windowPOI returns the source POI for a period txn. Purchases use their
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
