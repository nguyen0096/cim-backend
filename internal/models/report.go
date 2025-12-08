package models

import "time"

type ReportType string

const (
	ReportTypeInventoryTransaction ReportType = "inventory_transactions_report"
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
