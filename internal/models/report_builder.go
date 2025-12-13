package models

import (
	"cim-backend/pkg"
	"cim-backend/pkg/log"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type txnReportBuilder struct {
	isBuilt bool
	report  *TxnReportInventory

	// txns is the list of inventory transactions to process, that
	// belong to report duration.
	// @optimize: query from OLAP database for better performance.
	txns []*InventoryTransaction

	// historicalTxns is the list of inventory transactions before
	// the report start date, used to calculate starting quantities.
	// @optimize: query from OLAP database for better performance.
	historicalTxns []*InventoryTransaction

	// iiLookup is a lookup map of inventory items by their IDs.
	iiLookup map[uint]*InventoryItem

	// poItemLookup is a lookup map of purchase order items by their IDs.
	poItemLookup map[uint]*PurchaseOrderItem
}

func NewReportBuilder(
	inventory *Inventory,
	from, to time.Time,
) *txnReportBuilder {
	r := &TxnReportInventory{
		Report: Report{
			Title: fmt.Sprintf(
				ReportNameTmplMonthlyTransactionReport,
				from.Format("01/2006"), inventory.Name),
			Type:       ReportTypeTransaction,
			From:       pkg.Ptr(from),
			To:         pkg.Ptr(to),
			ExportFile: &ExportFile{},
		},
		Inventory: inventory,
	}
	return &txnReportBuilder{
		report: r,
	}
}

func (rb *txnReportBuilder) GetOutput() (*TxnReportInventory, error) {
	if !rb.isBuilt {
		return nil, fmt.Errorf("report not built yet")
	}
	return rb.report, nil
}

func (rb *txnReportBuilder) Txns(txns []*InventoryTransaction) *txnReportBuilder {
	rb.txns = txns
	return rb
}

func (rb *txnReportBuilder) HistoricalTxns(txns []*InventoryTransaction) *txnReportBuilder {
	rb.historicalTxns = txns
	return rb
}

func (rb *txnReportBuilder) InventoryItemLookup(lookup map[uint]*InventoryItem) *txnReportBuilder {
	rb.iiLookup = lookup
	return rb
}

func (rb *txnReportBuilder) PurchaseOrderItemLookup(lookup map[uint]*PurchaseOrderItem) *txnReportBuilder {
	rb.poItemLookup = lookup
	return rb
}

func (rb *txnReportBuilder) ready() error {
	if rb.report == nil {
		return fmt.Errorf("report is nil")
	}
	if rb.report.Inventory == nil {
		return fmt.Errorf("inventory is nil")
	}
	if len(rb.txns) == 0 {
		return fmt.Errorf("transactions are nil")
	}
	return nil
}

func (rb *txnReportBuilder) Build() (*txnReportBuilder, error) {
	if err := rb.ready(); err != nil {
		return nil, fmt.Errorf("report builder not ready: %w", err)
	}

	startQuantities := AggTxnQuantities(rb.historicalTxns)

	reportItems := make(map[uint]*TxnReportInventoryItem)

	for _, txn := range rb.txns {
		item, exists := rb.iiLookup[txn.InventoryItemID]
		if !exists {
			log.Warnf("inventory item %d not found in lookup for transaction %d", txn.InventoryItemID, txn.ID)
			continue
		}

		reportItem := rb.getOrCreateReportItem(reportItems, txn.InventoryItemID, item)
		reportItem.StartQuantity = startQuantities[txn.InventoryItemID]
		reportItem.Transactions = append(reportItem.Transactions, txn)

		day := txn.CreatedAt.Day()

		switch txn.TransactionType {
		case InventoryTransactionTypePurchase:
			reportItem.PurchaseQuantity = reportItem.PurchaseQuantity.Add(txn.Quantity)

			current, exists := reportItem.PurchaseQuantityByDay[day]
			if !exists {
				current = decimal.Zero
			}
			reportItem.PurchaseQuantityByDay[day] = current.Add(txn.Quantity)

			if txn.PurchaseOrderItemID != nil {
				rb.updatePOSummary(reportItem, rb.poItemLookup, *txn.PurchaseOrderItemID, day, txn.Quantity)
			}

		case InventoryTransactionTypeSell:
			reportItem.ReconcileQuantity = reportItem.ReconcileQuantity.Add(txn.Quantity)

		case InventoryTransactionTypeDisposal:
			reportItem.DisposeQuantity = reportItem.DisposeQuantity.Add(txn.Quantity)

		case InventoryTransactionTypeTransferIn:
			reportItem.TransferQuantity = reportItem.TransferQuantity.Add(txn.Quantity)

		case InventoryTransactionTypeTransferOut:
			reportItem.TransferQuantity = reportItem.TransferQuantity.Sub(txn.Quantity)
		}
	}

	items := make([]*TxnReportInventoryItem, 0, len(reportItems))
	for _, item := range reportItems {
		item.EndQuantity = item.StartQuantity.
			Add(item.PurchaseQuantity).
			Sub(item.ReconcileQuantity).
			Sub(item.DisposeQuantity).
			Add(item.TransferQuantity)
		items = append(items, item)
	}

	rb.report.Items = items
	rb.isBuilt = true
	return rb, nil
}

// getOrCreateReportItem retrieves or creates a report item for the given inventory item.
func (rb *txnReportBuilder) getOrCreateReportItem(
	reportItems map[uint]*TxnReportInventoryItem,
	itemID uint,
	inventoryItem *InventoryItem,
) *TxnReportInventoryItem {
	if item, exists := reportItems[itemID]; exists {
		return item
	}

	item := &TxnReportInventoryItem{
		InventoryItem:         inventoryItem,
		StartQuantity:         decimal.Zero,
		Transactions:          make([]*InventoryTransaction, 0),
		POMap:                 make(map[uint]*TxnReportPOSummary),
		PurchaseQuantity:      decimal.Zero,
		PurchaseQuantityByDay: make(map[int]decimal.Decimal),
		ReconcileQuantity:     decimal.Zero,
		TransferQuantity:      decimal.Zero,
		DisposeQuantity:       decimal.Zero,
		EndQuantity:           decimal.Zero,
	}

	reportItems[itemID] = item
	return item
}

// updatePOSummary updates the purchase order summary for a report item.
func (rb *txnReportBuilder) updatePOSummary(
	reportItem *TxnReportInventoryItem,
	poItemLookup map[uint]*PurchaseOrderItem,
	poItemID uint,
	day int,
	quantity decimal.Decimal,
) {
	poItem, exists := poItemLookup[poItemID]
	if !exists {
		return
	}

	// Check if PurchaseOrder is loaded
	if poItem.PurchaseOrder == nil {
		return
	}

	summary, exists := reportItem.POMap[poItemID]
	if !exists {
		summary = &TxnReportPOSummary{
			OrderNumber:           poItem.PurchaseOrder.OrderNumber,
			Status:                poItem.PurchaseOrder.Status,
			PurchaseQuantityByDay: make(map[int]decimal.Decimal),
		}
		reportItem.POMap[poItemID] = summary
	}

	current, exists := summary.PurchaseQuantityByDay[day]
	if !exists {
		current = decimal.Zero
	}
	summary.PurchaseQuantityByDay[day] = current.Add(quantity)
}
