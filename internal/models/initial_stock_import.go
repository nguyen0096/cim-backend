package models

import (
	"encoding/json"
	"time"
)

// InitialStockImport records an applied initial-stock load by idempotency key,
// scoped per inventory, together with the response payload replayed on a repeat
// submit. Deliberately carries no models.Base: the table has no updated_by /
// updated_at / deleted_at columns, so CreatedBy is set explicitly.
type InitialStockImport struct {
	ID             uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	IdempotencyKey string          `json:"idempotency_key" gorm:"column:idempotency_key;not null;uniqueIndex:idx_initial_stock_imports_inventory_key,priority:2"`
	InventoryID    uint            `json:"inventory_id" gorm:"column:inventory_id;not null;uniqueIndex:idx_initial_stock_imports_inventory_key,priority:1"`
	SheetName      string          `json:"sheet_name" gorm:"column:sheet_name;not null"`
	FileName       string          `json:"file_name" gorm:"column:file_name;not null"`
	FileSHA256     string          `json:"file_sha256" gorm:"column:file_sha256;not null"`
	RowCount       int             `json:"row_count" gorm:"column:row_count;not null"`
	ResultSummary  json.RawMessage `json:"result_summary" gorm:"column:result_summary;type:jsonb;not null"`
	CreatedBy      string          `json:"created_by" gorm:"column:created_by;not null"`
	CreatedAt      time.Time       `json:"created_at"`
}

func (InitialStockImport) TableName() string {
	return "initial_stock_imports"
}
