package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type ReportType string

const (
	ReportTypeTransaction ReportType = "transactions_report"
)

const (
	ReportNameTmplMonthlyTransactionReport = "Xuất nhập tồn tháng %s - %s"
)

// Report represents a generated report file,
// contains metadata about the report.
type Report struct {
	*ExportFile
	Title string     `json:"name"`
	From  time.Time  `json:"from_date"`
	To    time.Time  `json:"to_date"`
	Type  ReportType `json:"type"`
}

type TxnReportPOSummary struct {
	OrderNumber           string                  `json:"order_number"`
	Status                PurchaseOrderStatus     `json:"status"`
	PurchaseQuantityByDay map[int]decimal.Decimal `json:"quantity_by_day"`
}

type TxnReportInventoryItem struct {
	Report
	*InventoryItem

	StartQuantity decimal.Decimal         `json:"start_quantity"`
	Transactions  []*InventoryTransaction `json:"transactions,omitempty"`

	// computed fields

	POMap                 map[uint]*TxnReportPOSummary `json:"-"`
	PurchaseQuantity      decimal.Decimal              `json:"total_purchase"`
	PurchaseQuantityByDay map[int]decimal.Decimal      `json:"purchase_quantity_by_day"`
	ReconcileQuantity     decimal.Decimal              `json:"total_reconcile"`
	TransferQuantity      decimal.Decimal              `json:"total_transfer"`
	DisposeQuantity       decimal.Decimal              `json:"total_dispose"`
	EndQuantity           decimal.Decimal              `json:"end_quantity"`
}

type TxnReportInventory struct {
	Report
	*Inventory

	Items []*TxnReportInventoryItem `json:"items,omitempty"`
}
