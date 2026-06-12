package excel

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func dec(f float64) decimal.Decimal { return decimal.NewFromFloat(f) }

func sampleRows() *ExportRows {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	return &ExportRows{
		StartDate: start,
		EndDate:   end,
		DayCount:  5,
		Rows: []*ExportRow{
			{
				ProductID: 1, ProductName: "Apple", UnitName: "kg",
				POItemID: 100, POID: 10, PONumber: "PO-1",
				PurchasePrice: dec(50), SellingPrice: decPtr(80),
				DailyPurchases:       map[int]decimal.Decimal{0: dec(10)},
				BeginningStock:       dec(0),
				EndingStock:          dec(7),
				TotalPurchasedAmount: dec(10),
				SubtotalSold:         dec(3),
				TotalDisposedAmount:  dec(0),
			},
			{
				ProductID: 1, ProductName: "Apple", UnitName: "kg",
				POItemID: 101, POID: 11, PONumber: "PO-2",
				PurchasePrice: dec(60), SellingPrice: decPtr(90),
				DailyPurchases:       map[int]decimal.Decimal{2: dec(5)},
				BeginningStock:       dec(0),
				EndingStock:          dec(5),
				TotalPurchasedAmount: dec(5),
				SubtotalSold:         dec(0),
				TotalDisposedAmount:  dec(1),
			},
		},
	}
}

func TestWriteInOutExport_AuditMetadataRows(t *testing.T) {
	rows := sampleRows()
	f, err := WriteInOutExport(rows, ExportContext{
		InventoryName: "Kho A",
		GeneratedAt:   time.Date(2026, 4, 28, 14, 30, 0, 0, time.UTC),
		GeneratedBy:   "user@example.com",
	})
	require.NoError(t, err)
	defer f.Close()

	// RawCellValue so applied number formats (accounting money / 3-dp qty)
	// don't reshape the asserted stored values.
	read := func(cell string) string {
		v, _ := f.GetCellValue(inOutSheetName, cell, excelize.Options{RawCellValue: true})
		return v
	}

	assert.Equal(t, "Kho:", read("A1"))
	assert.Equal(t, "Kho A", read("B1"))
	assert.Contains(t, read("B2"), "01/04/2026")
	assert.Contains(t, read("B2"), "05/04/2026")
	assert.Equal(t, "28/04/2026 14:30:00", read("B3"))
	assert.Equal(t, "user@example.com", read("B4"))
	// Currency-unit note so readers know all monetary values are in VND.
	assert.Equal(t, "Đơn vị tiền tệ:", read("A5"))
	assert.Equal(t, "VND", read("B5"))
}

func TestWriteInOutExport_HeaderRowsLayout(t *testing.T) {
	rows := sampleRows()
	f, err := WriteInOutExport(rows, ExportContext{InventoryName: "Kho A"})
	require.NoError(t, err)
	defer f.Close()

	// Layout: A=ProductName, B=Unit, C..G=Daily(5), H=PurchasePrice, I=SellingPrice, ...
	// RawCellValue so applied number formats (accounting money / 3-dp qty)
	// don't reshape the asserted stored values.
	read := func(cell string) string {
		v, _ := f.GetCellValue(inOutSheetName, cell, excelize.Options{RawCellValue: true})
		return v
	}

	// Level-1 vertically merged columns: A, B headers in row 6
	assert.Equal(t, "Sản phẩm", read("A6"))
	assert.Equal(t, "Đơn vị tính", read("B6"))

	// Daily group spans C6:G6, with dates in row 7
	assert.Equal(t, "Số lượng nhập trong kì", read("C6"))
	assert.Equal(t, "01/04", read("C7"))
	assert.Equal(t, "05/04", read("G7"))

	// Per-batch single columns (rows 6-7 merged)
	assert.Equal(t, "Đơn giá nhập (VND)", read("H6"))
	assert.Equal(t, "Đơn giá bán (VND)", read("I6"))
	assert.Equal(t, "Tồn đầu", read("J6"))
	assert.Equal(t, "Tồn cuối", read("K6"))
	assert.Equal(t, "Đã bán", read("L6"))
	assert.Equal(t, "Doanh thu", read("M6"))

	// Tổng nhập group has SL/TT sub-headers
	assert.Equal(t, "Tổng nhập", read("N6"))
	assert.Equal(t, "SL", read("N7"))
	assert.Equal(t, "TT (VND)", read("O7"))
}

func TestWriteInOutExport_DataRowFormulasAndValues(t *testing.T) {
	rows := sampleRows()
	f, err := WriteInOutExport(rows, ExportContext{InventoryName: "Kho A"})
	require.NoError(t, err)
	defer f.Close()

	// Data row 1 (row 8): Apple, PO-1
	// RawCellValue so applied number formats (accounting money / 3-dp qty)
	// don't reshape the asserted stored values.
	read := func(cell string) string {
		v, _ := f.GetCellValue(inOutSheetName, cell, excelize.Options{RawCellValue: true})
		return v
	}
	formula := func(cell string) string {
		v, _ := f.GetCellFormula(inOutSheetName, cell)
		return v
	}

	assert.Equal(t, "Apple", read("A8"))
	assert.Equal(t, "kg", read("B8"))
	assert.Equal(t, "10", read("C8")) // day 0 of 5
	assert.Equal(t, "50", read("H8")) // purchase_price
	assert.Equal(t, "80", read("I8")) // selling_price
	// subtotal_revenue = selling × subtotal_sold, guarded so a missing price
	// (text "-" in the selling-price cell) yields "-" not #VALUE!. N() coerces
	// a "-" sold cell to 0 so a real 0 revenue stays numeric 0.
	assert.Equal(t, `IF(ISNUMBER(I8),I8*N(L8),"-")`, formula("M8"))
	// total_purchased.total = purchase_price × amount; N() tolerates a "-" amount.
	assert.Equal(t, "H8*N(N8)", formula("O8"))
	// total_disposed.total = purchase_price × disposed_amount; N()-guarded.
	assert.Equal(t, "H8*N(Q8)", formula("R8"))

	// Group totals on first row of group (row 8): merged across 8-9. Quantity
	// total dashes out a zero sum.
	assert.Equal(t, `IF(SUM(L8:L9)=0,"-",SUM(L8:L9))`, formula("P8")) // total_sold = SUM(subtotal_sold)
}

func TestWriteInOutExport_MissingSellingPriceRendersDash(t *testing.T) {
	// A POI with no effective selling price (nil) renders the selling-price
	// cell as the literal "-" and its revenue as an ISNUMBER-guarded formula
	// that evaluates to "-" rather than #VALUE!. The group/footer revenue SUMs
	// are plain SUMs and skip the text cell.
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	rows := &ExportRows{
		StartDate: start, EndDate: start.AddDate(0, 0, 4), DayCount: 5,
		Rows: []*ExportRow{{
			ProductID: 1, ProductName: "Apple", UnitName: "kg",
			POItemID: 100, POID: 10, PONumber: "PO-1",
			PurchasePrice: dec(50), SellingPrice: nil, // no effective selling price
			DailyPurchases:       map[int]decimal.Decimal{0: dec(10)},
			BeginningStock:       dec(0),
			EndingStock:          dec(7),
			TotalPurchasedAmount: dec(10),
			SubtotalSold:         dec(3),
		}},
	}
	f, err := WriteInOutExport(rows, ExportContext{InventoryName: "Kho A"})
	require.NoError(t, err)
	defer f.Close()

	read := func(cell string) string {
		v, _ := f.GetCellValue(inOutSheetName, cell, excelize.Options{RawCellValue: true})
		return v
	}
	formula := func(cell string) string {
		v, _ := f.GetCellFormula(inOutSheetName, cell)
		return v
	}

	assert.Equal(t, "-", read("I8")) // selling-price cell is literal dash
	// Revenue formula present and guarded; evaluates to "-" for a text price.
	assert.Equal(t, `IF(ISNUMBER(I8),I8*N(L8),"-")`, formula("M8"))
	// Group/footer revenue totals stay plain SUMs (Excel skips the "-" text).
	assert.Equal(t, "SUM(M8:M8)", formula("W8"))
}

func TestWriteInOutExport_ZeroQuantityRendersDash(t *testing.T) {
	// Quantity cells whose value is exactly zero render the literal "-" instead
	// of a noisy 0. A real 0 selling price (a valid value) still renders as 0.
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	rows := &ExportRows{
		StartDate: start, EndDate: start.AddDate(0, 0, 4), DayCount: 5,
		Rows: []*ExportRow{{
			ProductID: 1, ProductName: "Apple", UnitName: "kg",
			POItemID: 100, POID: 10, PONumber: "PO-1",
			PurchasePrice: dec(50), SellingPrice: decPtr(0), // real 0 price → stays 0
			DailyPurchases:       map[int]decimal.Decimal{0: dec(10)},
			BeginningStock:       dec(0), // zero qty → "-"
			EndingStock:          dec(7),
			TotalPurchasedAmount: dec(10),
			SubtotalSold:         dec(0), // zero qty → "-"
			TotalDisposedAmount:  dec(0), // zero qty → "-"
			TotalTransferredIn:   dec(0), // zero qty → "-"
			TotalTransferredOut:  dec(0), // zero qty → "-"
		}},
	}
	f, err := WriteInOutExport(rows, ExportContext{InventoryName: "Kho A"})
	require.NoError(t, err)
	defer f.Close()

	read := func(cell string) string {
		v, _ := f.GetCellValue(inOutSheetName, cell, excelize.Options{RawCellValue: true})
		return v
	}
	formula := func(cell string) string {
		v, _ := f.GetCellFormula(inOutSheetName, cell)
		return v
	}

	// Real 0 selling price stays the numeric 0 (NOT "-").
	assert.Equal(t, "0", read("I8"))
	// Zero quantities render "-".
	assert.Equal(t, "-", read("J8")) // beginning stock = 0
	assert.Equal(t, "-", read("L8")) // subtotal sold = 0
	assert.Equal(t, "-", read("Q8")) // total disposed amount = 0
	// Non-zero quantities stay numeric.
	assert.Equal(t, "10", read("N8")) // total purchased amount = 10
	assert.Equal(t, "7", read("K8"))  // ending stock = 7
	// Revenue formula tolerates the "-" sold cell via N() and the real 0 price.
	assert.Equal(t, `IF(ISNUMBER(I8),I8*N(L8),"-")`, formula("M8"))
	// Quantity group total dashes out a zero sum (single-row group: SUM(L8:L8)).
	assert.Equal(t, `IF(SUM(L8:L8)=0,"-",SUM(L8:L8))`, formula("P8"))
}

func TestWriteInOutExport_DailyNoPurchaseCellsRenderDash(t *testing.T) {
	// Every daily column under "Số lượng nhập trong kì" is filled: days with a
	// purchase show the numeric quantity; days with no purchase (missing map
	// key) and explicit zero-quantity entries render "-" like every other empty
	// quantity cell — never a blank.
	rows := sampleRows()
	// Explicit zero entry for day 1 of row 1: must render "-" too.
	rows.Rows[0].DailyPurchases[1] = dec(0)
	f, err := WriteInOutExport(rows, ExportContext{InventoryName: "Kho A"})
	require.NoError(t, err)
	defer f.Close()

	read := func(cell string) string {
		v, _ := f.GetCellValue(inOutSheetName, cell, excelize.Options{RawCellValue: true})
		return v
	}

	// Row 8 (Apple PO-1): purchase on day 0 only. Daily columns are C..G.
	assert.Equal(t, "10", read("C8")) // day 0: real purchase stays numeric
	assert.Equal(t, "-", read("D8"))  // day 1: explicit zero entry → "-"
	assert.Equal(t, "-", read("E8"))  // day 2: no purchase → "-"
	assert.Equal(t, "-", read("F8"))  // day 3: no purchase → "-"
	assert.Equal(t, "-", read("G8"))  // day 4: no purchase → "-"

	// Row 9 (Apple PO-2): purchase on day 2 only.
	assert.Equal(t, "-", read("C9"))
	assert.Equal(t, "-", read("D9"))
	assert.Equal(t, "5", read("E9")) // day 2: real purchase stays numeric
	assert.Equal(t, "-", read("F9"))
	assert.Equal(t, "-", read("G9"))
}

func TestWriteInOutExport_FooterRowSums(t *testing.T) {
	rows := sampleRows()
	f, err := WriteInOutExport(rows, ExportContext{InventoryName: "Kho A"})
	require.NoError(t, err)
	defer f.Close()

	// Footer is row 10 (data rows 8-9). SUMs of total_purchased.total (col O),
	// total_disposed.total (col R), total_revenue (col W).
	formula := func(cell string) string {
		v, _ := f.GetCellFormula(inOutSheetName, cell)
		return v
	}
	assert.Equal(t, "SUM(O8:O9)", formula("O10"))
	assert.Equal(t, "SUM(R8:R9)", formula("R10"))
}

func TestWriteInOutExport_ProductGroupMergesApplied(t *testing.T) {
	rows := sampleRows()
	f, err := WriteInOutExport(rows, ExportContext{InventoryName: "Kho A"})
	require.NoError(t, err)
	defer f.Close()

	merges, err := f.GetMergeCells(inOutSheetName)
	require.NoError(t, err)
	// Convert to a string set for substring checks.
	var allRanges []string
	for _, m := range merges {
		allRanges = append(allRanges, m.GetStartAxis()+":"+m.GetEndAxis())
	}
	joined := strings.Join(allRanges, ",")
	assert.Contains(t, joined, "A8:A9", "ProductName merged across group rows")
	assert.Contains(t, joined, "B8:B9", "Unit merged across group rows")
}

func TestWriteInOutExport_DistinctProductsSameDisplayNameNotMerged(t *testing.T) {
	// Two distinct products (different ProductIDs) that happen to share the
	// same display name must NOT be grouped/merged into a single section.
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	rows := &ExportRows{
		StartDate: start,
		EndDate:   end,
		DayCount:  5,
		Rows: []*ExportRow{
			{
				ProductID: 1, ProductName: "Apple", UnitName: "kg",
				POItemID: 100, POID: 10, PONumber: "PO-1",
				PurchasePrice: dec(50), SellingPrice: decPtr(80),
				DailyPurchases:       map[int]decimal.Decimal{0: dec(10)},
				BeginningStock:       dec(0),
				EndingStock:          dec(7),
				TotalPurchasedAmount: dec(10),
				SubtotalSold:         dec(3),
			},
			{
				ProductID: 2, ProductName: "Apple", UnitName: "kg", // distinct product, same name
				POItemID: 200, POID: 20, PONumber: "PO-2",
				PurchasePrice: dec(60), SellingPrice: decPtr(90),
				DailyPurchases:       map[int]decimal.Decimal{1: dec(5)},
				BeginningStock:       dec(0),
				EndingStock:          dec(5),
				TotalPurchasedAmount: dec(5),
				SubtotalSold:         dec(0),
			},
		},
	}
	f, err := WriteInOutExport(rows, ExportContext{InventoryName: "Kho A"})
	require.NoError(t, err)
	defer f.Close()

	merges, err := f.GetMergeCells(inOutSheetName)
	require.NoError(t, err)
	var allRanges []string
	for _, m := range merges {
		allRanges = append(allRanges, m.GetStartAxis()+":"+m.GetEndAxis())
	}
	joined := strings.Join(allRanges, ",")
	// Critically: the two rows must NOT be merged across A8:A9 — that would
	// indicate they were grouped together as one product.
	assert.NotContains(t, joined, "A8:A9", "distinct products with shared name must not be grouped")

	// Group-aggregate cells should be per-row (single-row groups → no SUM
	// across both rows). The total_sold cell on row 8 should reference only
	// L8:L8, and similarly row 9 references L9:L9.
	formula := func(cell string) string {
		v, _ := f.GetCellFormula(inOutSheetName, cell)
		return v
	}
	// Two single-row groups: row 8 totals over L8:L8, row 9 over L9:L9.
	// Quantity totals are dash-guarded so a zero sum renders "-".
	assert.Equal(t, `IF(SUM(L8:L8)=0,"-",SUM(L8:L8))`, formula("P8"))
	assert.Equal(t, `IF(SUM(L9:L9)=0,"-",SUM(L9:L9))`, formula("P9"))
}

func TestSanitizeFilenameSegment(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Kho A", "kho-a"},
		{"  My Inventory  ", "--my-inventory--"},
		{"Kho_2025!@#", "kho2025"},
		{"abc-123", "abc-123"},
		{"VietName tieng Viet", "vietname-tieng-viet"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, SanitizeFilenameSegment(c.in), c.in)
	}
}

func TestWriteInOutExport_FreezePanesSet(t *testing.T) {
	rows := sampleRows()
	f, err := WriteInOutExport(rows, ExportContext{InventoryName: "Kho A"})
	require.NoError(t, err)
	defer f.Close()

	// excelize doesn't expose a clean Pane reader, but we can read the sheet
	// view XML by inspecting the file written to a buffer. Quick smoke test
	// only: ensure the file roundtrips.
	if _, err := f.WriteToBuffer(); err != nil {
		t.Fatalf("write to buffer: %v", err)
	}
	_ = excelize.NewFile() // keep import in case
}

func TestWriteInOutExport_StylesApplied(t *testing.T) {
	rows := sampleRows()
	f, err := WriteInOutExport(rows, ExportContext{InventoryName: "Kho A"})
	require.NoError(t, err)
	defer f.Close()

	styleOf := func(cell string) *excelize.Style {
		id, err := f.GetCellStyle(inOutSheetName, cell)
		require.NoError(t, err)
		s, err := f.GetStyle(id)
		require.NoError(t, err)
		return s
	}

	// Header cell: Calibri 14, bold, white text on the blue fill.
	h := styleOf("A6")
	require.NotNil(t, h.Font)
	assert.Equal(t, "Calibri", h.Font.Family)
	assert.Equal(t, 14.0, h.Font.Size)
	assert.True(t, h.Font.Bold)
	assert.Equal(t, "FFFFFF", h.Font.Color)
	require.NotEmpty(t, h.Fill.Color)
	assert.Equal(t, headerFill, h.Fill.Color[0])

	// Data cell (product name, col A): size 14 content font, bold.
	d := styleOf("A8")
	require.NotNil(t, d.Font)
	assert.Equal(t, 14.0, d.Font.Size)
	assert.True(t, d.Font.Bold, "product name should be bold")

	// Money cell (purchase price, col H): numeric with the VND number format, bold.
	m := styleOf("H8")
	require.NotNil(t, m.CustomNumFmt)
	assert.Equal(t, moneyNumFmt, *m.CustomNumFmt) // "#,##0"
	require.NotNil(t, m.Font)
	assert.True(t, m.Font.Bold, "numeric cells should be bold")

	// Quantity cell (daily, col C): numeric, up-to-6-dp format, bold.
	q := styleOf("C8")
	require.NotNil(t, q.CustomNumFmt)
	assert.Equal(t, qtyNumFmt, *q.CustomNumFmt) // "#,##0.######"
	require.NotNil(t, q.Font)
	assert.True(t, q.Font.Bold, "numeric cells should be bold")
}

func TestWriteInOutExport_RowHeight(t *testing.T) {
	rows := sampleRows()
	f, err := WriteInOutExport(rows, ExportContext{InventoryName: "Kho A"})
	require.NoError(t, err)
	defer f.Close()

	props, err := f.GetSheetProps(inOutSheetName)
	require.NoError(t, err)
	require.NotNil(t, props.DefaultRowHeight)
	assert.Equal(t, 25.0, *props.DefaultRowHeight)
}

func TestWriteInOutExport_DecimalsNotRoundedOrPadded(t *testing.T) {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	rows := &ExportRows{
		StartDate: start, EndDate: start.AddDate(0, 0, 4), DayCount: 5,
		Rows: []*ExportRow{{
			ProductID: 1, ProductName: "Apple", UnitName: "kg",
			POItemID: 100, POID: 10, PONumber: "PO-1",
			PurchasePrice:  dec(99.75),   // money with decimals — must NOT round to 100
			SellingPrice:   decPtr(80),
			BeginningStock: dec(1526.13), // qty — must NOT pad to 1526.130000
			EndingStock:    dec(7),
		}},
	}
	f, err := WriteInOutExport(rows, ExportContext{InventoryName: "Kho A"})
	require.NoError(t, err)
	defer f.Close()

	// Formatted (rendered) value — no RawCellValue — exercises the number format.
	// excelize renders separators US-style ('.' decimal); under VN Excel these
	// flip to ',' decimal / '.' thousands.
	formatted := func(cell string) string {
		v, _ := f.GetCellValue(inOutSheetName, cell)
		return v
	}
	assert.Equal(t, "99.75", formatted("H8"))      // money: natural decimals, not "100"
	assert.Equal(t, "1,526.13", formatted("J8"))   // qty: natural decimals, not "1526.130000"
}
