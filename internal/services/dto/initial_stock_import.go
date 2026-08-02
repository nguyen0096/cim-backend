package dto

// Planned actions reported per sheet row. Closed enum, pinned by the frontend contract.
const (
	InitialStockActionCreateProduct     = "create_product"
	InitialStockActionMatchProduct      = "match_product"
	InitialStockActionCreateItem        = "create_item"
	InitialStockActionCreateTransaction = "create_transaction"
	InitialStockActionSkipZeroQuantity  = "skip_zero_quantity"
)

// InitialStockSheetInfo describes one worksheet and whether it can be loaded.
type InitialStockSheetInfo struct {
	Name              string `json:"name"`
	HasExpectedHeader bool   `json:"has_expected_header"`
	DataRowCount      int    `json:"data_row_count"`
	Reason            string `json:"reason"`
}

// InitialStockSheetsResponse is the response of the sheet-listing step.
type InitialStockSheetsResponse struct {
	Sheets []InitialStockSheetInfo `json:"sheets"`
}

// InitialStockInventoryOption is one entry of the tool's own inventory picker.
type InitialStockInventoryOption struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Location string `json:"location"`
	Status   string `json:"status"`
}

// InitialStockInventoriesResponse carries the picker list. An empty array is a
// valid success, which the client renders as an explicit empty state.
type InitialStockInventoriesResponse struct {
	Data []InitialStockInventoryOption `json:"data"`
}

// InitialStockImportRequest is the parsed multipart form of the import endpoint.
type InitialStockImportRequest struct {
	InventoryID    uint   `json:"inventory_id"`
	SheetName      string `json:"sheet_name"`
	DryRun         bool   `json:"dry_run"`
	IdempotencyKey string `json:"-"`
	FileName       string `json:"-"`
	FileSHA256     string `json:"-"`
}

// InitialStockImportRow is the per-row plan. Quantities are decimal strings, never floats.
type InitialStockImportRow struct {
	Row      int    `json:"row"`
	Name     string `json:"name"`
	Unit     string `json:"unit"`
	Quantity string `json:"quantity"`
	// ProductType is the type that will be in effect after the load, not the sheet's
	// value: a matched product keeps its own, and Warnings names the ignored one.
	ProductType string   `json:"product_type"`
	ProductID   uint     `json:"product_id"`
	Actions     []string `json:"actions"`
	// Warnings are rendered Vietnamese strings for conditions that do not stop the row
	// from importing. Always an array, empty when the row has none. Distinct from
	// InitialStockImportResponse.Errors, whose rows are dropped.
	Warnings          []string `json:"warnings"`
	CurrentQuantity   string   `json:"current_quantity"`
	ResultingQuantity string   `json:"resulting_quantity"`
	UnitDecimalPlaces int      `json:"unit_decimal_places"`
}

// InitialStockImportError is one rejected row: the shared row-error fields plus the
// sheet values it was rejected on, so the client can render it in the same table as a
// plan row instead of an empty-celled stub. Row, Location and Message are the shape
// pkg.NewRowLocation produces; the values are additive.
type InitialStockImportError struct {
	Row      int    `json:"row"`
	Location string `json:"location"`
	Message  string `json:"message"`
	// Name, Unit and Quantity are the cells as the parser read them: trimmed, never
	// normalized or coerced, so a row rejected for an unparseable quantity still shows
	// the text that has to be fixed. A blank cell is an empty string, never null.
	Name     string `json:"name"`
	Unit     string `json:"unit"`
	Quantity string `json:"quantity"`
}

// InitialStockBlocking is a non-row condition that would refuse an apply. A dry-run
// reports it here instead of failing, so the operator sees it before committing.
type InitialStockBlocking struct {
	Key     string `json:"key"`
	Message string `json:"message"`
}

// InitialStockImportResponse is returned with 200 in both dry-run and apply mode.
// One flat surface: the six counters the client reads sit alongside the five it does
// not, rather than being duplicated into a nested summary.
type InitialStockImportResponse struct {
	DryRun      bool                      `json:"dry_run"`
	InventoryID uint                      `json:"inventory_id"`
	SheetName   string                    `json:"sheet_name"`
	Blocking    []InitialStockBlocking    `json:"blocking"`
	Rows        []InitialStockImportRow   `json:"rows"`
	Errors      []InitialStockImportError `json:"errors"`

	// Read by the client.
	RowsProcessed       int `json:"rows_processed"`
	ProductsCreated     int `json:"products_created"`
	ProductsMatched     int `json:"products_matched"`
	ItemsCreated        int `json:"items_created"`
	TransactionsCreated int `json:"transactions_created"`
	RowsSkipped         int `json:"rows_skipped"`

	// Operator-facing detail the client does not render.
	// RowsOnItemsWithExistingStock is the additive-load guard: a non-zero count on a
	// sheet the operator believes to be a first load means the on-hand is already
	// there and the load would double it.
	RowsOK                       int    `json:"rows_ok"`
	RowsFailed                   int    `json:"rows_failed"`
	UnitsCreated                 int    `json:"units_created"`
	TotalQuantity                string `json:"total_quantity"`
	RowsOnItemsWithExistingStock int    `json:"rows_on_items_with_existing_stock"`
}
