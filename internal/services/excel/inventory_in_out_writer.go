// inventory_in_out_writer.go renders ExportRows into an excelize file.
//
// Sheet structure:
//   rows 1-4 : audit metadata (4 lines)
//   row 5    : blank spacer
//   rows 6-7 : two-row grouped column headers
//   rows 8+  : data rows (one per PO item)
//   final    : footer SUMs
//
// Freeze panes are set on row 7 (rows 1-7 stay visible).
//
// Cells use Excel formulas wherever possible so users can edit values and
// have aggregations recalculate. The writer keeps no business logic — it
// renders the typed ExportRows produced by BuildExportRows.

package excel

import (
	"bytes"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

const inOutSheetName = "Sheet1"

// ExportContext bundles audit-metadata fields the writer renders into the
// header block. The shaper does not need these.
type ExportContext struct {
	InventoryName string
	GeneratedAt   time.Time
	GeneratedBy   string // user email or display name
}

// WriteInOutExportToBuffer renders the given ExportRows into a fresh
// excelize file and returns the encoded xlsx bytes.
func WriteInOutExportToBuffer(rows *ExportRows, ctx ExportContext) ([]byte, error) {
	f, err := WriteInOutExport(rows, ctx)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("excelize write: %w", err)
	}
	return buf.Bytes(), nil
}

// WriteInOutExport builds the excelize file and returns it open. Callers must
// Close() it. Useful for tests that want to inspect cells/formulas/merges.
func WriteInOutExport(rows *ExportRows, ctx ExportContext) (*excelize.File, error) {
	if rows == nil {
		return nil, fmt.Errorf("rows is nil")
	}
	f := excelize.NewFile()
	// excelize default sheet is "Sheet1" — keep it.

	if err := writeAuditMetadata(f, rows, ctx); err != nil {
		return nil, err
	}
	if err := writeHeaderRows(f, rows.DayCount, rows.StartDate); err != nil {
		return nil, err
	}
	dataStart := 8
	dataEnd, err := writeDataRows(f, rows, dataStart)
	if err != nil {
		return nil, err
	}
	if err := writeFooterRow(f, rows, dataStart, dataEnd); err != nil {
		return nil, err
	}
	if err := f.SetPanes(inOutSheetName, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      7,
		TopLeftCell: "A8",
		ActivePane:  "bottomLeft",
	}); err != nil {
		return nil, fmt.Errorf("freeze panes: %w", err)
	}

	return f, nil
}

// --- column layout --------------------------------------------------------
//
// Fixed prefix columns (1..N), then DayCount daily columns, then suffix
// columns. We build a small layout struct so the writer reads/writes
// consistent indices. Indices are 1-based to match excelize.
type colLayout struct {
	productName     int
	unit            int
	dailyStart      int // first daily-column index
	dailyEnd        int // last daily-column index (inclusive)
	purchasePrice   int
	sellingPrice    int
	beginningStock  int
	endingStock     int
	subtotalSold    int
	subtotalRevenue int
	totalPurchasedAmount int
	totalPurchasedTotal  int
	totalSold            int
	totalDisposedAmount  int
	totalDisposedTotal   int
	totalTransferredIn   int
	totalTransferredOut  int
	totalBeginningStock  int
	totalEndingStock     int
	totalRevenue         int
	last                 int
}

func buildLayout(dayCount int) colLayout {
	c := 1
	next := func(width int) int { v := c; c += width; return v }
	l := colLayout{}
	l.productName = next(1)
	l.unit = next(1)
	l.dailyStart = c
	c += dayCount
	l.dailyEnd = l.dailyStart + dayCount - 1
	l.purchasePrice = next(1)
	l.sellingPrice = next(1)
	l.beginningStock = next(1)
	l.endingStock = next(1)
	l.subtotalSold = next(1)
	l.subtotalRevenue = next(1)
	l.totalPurchasedAmount = next(1)
	l.totalPurchasedTotal = next(1)
	l.totalSold = next(1)
	l.totalDisposedAmount = next(1)
	l.totalDisposedTotal = next(1)
	l.totalTransferredIn = next(1)
	l.totalTransferredOut = next(1)
	l.totalBeginningStock = next(1)
	l.totalEndingStock = next(1)
	l.totalRevenue = next(1)
	l.last = c - 1
	return l
}

// --- audit metadata (rows 1-4) -------------------------------------------

func writeAuditMetadata(f *excelize.File, rows *ExportRows, ctx ExportContext) error {
	type pair struct{ row int; label, value string }
	pairs := []pair{
		{1, "Kho:", ctx.InventoryName},
		{2, "Khoảng thời gian:", fmt.Sprintf("%s – %s",
			rows.StartDate.Format("02/01/2006"), rows.EndDate.Format("02/01/2006"))},
		{3, "Thời điểm tạo:", ctx.GeneratedAt.Format("02/01/2006 15:04:05")},
		{4, "Người tạo:", ctx.GeneratedBy},
	}
	for _, p := range pairs {
		labelCell, _ := excelize.CoordinatesToCellName(1, p.row)
		valueCell, _ := excelize.CoordinatesToCellName(2, p.row)
		if err := f.SetCellValue(inOutSheetName, labelCell, p.label); err != nil {
			return fmt.Errorf("audit label: %w", err)
		}
		if err := f.SetCellValue(inOutSheetName, valueCell, p.value); err != nil {
			return fmt.Errorf("audit value: %w", err)
		}
	}
	return nil
}

// --- header rows (6-7) ----------------------------------------------------

func writeHeaderRows(f *excelize.File, dayCount int, startDate time.Time) error {
	l := buildLayout(dayCount)

	// Level-1 single columns get vertically merged across rows 6-7.
	mergeAndSetL1 := func(col int, label string) error {
		topCell, _ := excelize.CoordinatesToCellName(col, 6)
		bottomCell, _ := excelize.CoordinatesToCellName(col, 7)
		if err := f.SetCellValue(inOutSheetName, topCell, label); err != nil {
			return err
		}
		return f.MergeCell(inOutSheetName, topCell, bottomCell)
	}

	// Level-1 grouped headers span multiple columns horizontally on row 6.
	groupHeader := func(startCol, endCol int, label string) error {
		startCell, _ := excelize.CoordinatesToCellName(startCol, 6)
		endCell, _ := excelize.CoordinatesToCellName(endCol, 6)
		if err := f.SetCellValue(inOutSheetName, startCell, label); err != nil {
			return err
		}
		if startCol != endCol {
			return f.MergeCell(inOutSheetName, startCell, endCell)
		}
		return nil
	}

	subHeader := func(col int, label string) error {
		c, _ := excelize.CoordinatesToCellName(col, 7)
		return f.SetCellValue(inOutSheetName, c, label)
	}

	// Single-column level-1 headers
	if err := mergeAndSetL1(l.productName, "Sản phẩm"); err != nil {
		return err
	}
	if err := mergeAndSetL1(l.unit, "Đơn vị tính"); err != nil {
		return err
	}
	if err := mergeAndSetL1(l.purchasePrice, "Đơn giá nhập"); err != nil {
		return err
	}
	if err := mergeAndSetL1(l.sellingPrice, "Đơn giá bán"); err != nil {
		return err
	}
	if err := mergeAndSetL1(l.beginningStock, "Tồn đầu"); err != nil {
		return err
	}
	if err := mergeAndSetL1(l.endingStock, "Tồn cuối"); err != nil {
		return err
	}
	if err := mergeAndSetL1(l.subtotalSold, "Đã bán"); err != nil {
		return err
	}
	if err := mergeAndSetL1(l.subtotalRevenue, "Doanh thu"); err != nil {
		return err
	}
	if err := mergeAndSetL1(l.totalSold, "Tổng đã bán"); err != nil {
		return err
	}
	if err := mergeAndSetL1(l.totalTransferredIn, "Tổng nhập kho khác"); err != nil {
		return err
	}
	if err := mergeAndSetL1(l.totalTransferredOut, "Tổng xuất kho khác"); err != nil {
		return err
	}
	if err := mergeAndSetL1(l.totalBeginningStock, "Tổng tồn đầu"); err != nil {
		return err
	}
	if err := mergeAndSetL1(l.totalEndingStock, "Tổng tồn cuối"); err != nil {
		return err
	}
	if err := mergeAndSetL1(l.totalRevenue, "Tổng doanh thu"); err != nil {
		return err
	}

	// Grouped headers (level-1 spans, level-2 sub-headers on row 7)
	if err := groupHeader(l.dailyStart, l.dailyEnd, "Nhập trong kỳ"); err != nil {
		return err
	}
	for d := 0; d < dayCount; d++ {
		col := l.dailyStart + d
		date := startDate.AddDate(0, 0, d)
		if err := subHeader(col, date.Format("02/01")); err != nil {
			return err
		}
	}

	if err := groupHeader(l.totalPurchasedAmount, l.totalPurchasedTotal, "Tổng nhập"); err != nil {
		return err
	}
	if err := subHeader(l.totalPurchasedAmount, "SL"); err != nil {
		return err
	}
	if err := subHeader(l.totalPurchasedTotal, "TT"); err != nil {
		return err
	}

	if err := groupHeader(l.totalDisposedAmount, l.totalDisposedTotal, "Tổng hủy"); err != nil {
		return err
	}
	if err := subHeader(l.totalDisposedAmount, "SL"); err != nil {
		return err
	}
	if err := subHeader(l.totalDisposedTotal, "TT"); err != nil {
		return err
	}

	return nil
}

// --- data rows ------------------------------------------------------------

func writeDataRows(f *excelize.File, rows *ExportRows, startRow int) (int, error) {
	l := buildLayout(rows.DayCount)
	row := startRow

	// Group rows by product to apply level-1 merges (Sản phẩm, Đơn vị tính,
	// Tổng đã bán, Tổng tồn đầu, Tổng tồn cuối, Tổng doanh thu).
	productGroups := groupByProduct(rows.Rows)
	for _, g := range productGroups {
		groupStart := row
		for _, r := range g.rows {
			if err := writeOneRow(f, r, l, row); err != nil {
				return row, err
			}
			row++
		}
		groupEnd := row - 1
		if err := mergeProductGroup(f, l, g, groupStart, groupEnd); err != nil {
			return row, err
		}
	}
	return row - 1, nil
}

type productGroup struct {
	productID   uint
	productName string
	unitName    string
	rows        []*ExportRow
}

// groupByProduct walks rows assumed to be sorted with all entries for a given
// ProductID contiguous, and returns one group per distinct ProductID. Grouping
// keys on ProductID (not ProductName) because product names are not guaranteed
// unique — keying on the display name would silently merge distinct products
// that happen to share a name into a single section, corrupting grouped totals
// and merged-cell layout.
func groupByProduct(in []*ExportRow) []productGroup {
	var out []productGroup
	if len(in) == 0 {
		return out
	}
	cur := productGroup{productID: in[0].ProductID, productName: in[0].ProductName, unitName: in[0].UnitName, rows: []*ExportRow{in[0]}}
	for _, r := range in[1:] {
		if r.ProductID == cur.productID {
			cur.rows = append(cur.rows, r)
			continue
		}
		out = append(out, cur)
		cur = productGroup{productID: r.ProductID, productName: r.ProductName, unitName: r.UnitName, rows: []*ExportRow{r}}
	}
	out = append(out, cur)
	return out
}

func writeOneRow(f *excelize.File, r *ExportRow, l colLayout, row int) error {
	setStr := func(col int, v string) error {
		c, _ := excelize.CoordinatesToCellName(col, row)
		return f.SetCellValue(inOutSheetName, c, v)
	}
	setNum := func(col int, v decimal.Decimal) error {
		c, _ := excelize.CoordinatesToCellName(col, row)
		fl, _ := v.Float64()
		return f.SetCellValue(inOutSheetName, c, fl)
	}
	setFormula := func(col int, formula string) error {
		c, _ := excelize.CoordinatesToCellName(col, row)
		return f.SetCellFormula(inOutSheetName, c, formula)
	}
	colName := func(col int) string {
		n, _ := excelize.ColumnNumberToName(col)
		return n
	}

	if err := setStr(l.productName, r.ProductName); err != nil {
		return err
	}
	if err := setStr(l.unit, r.UnitName); err != nil {
		return err
	}
	for d, qty := range r.DailyPurchases {
		if qty.IsZero() {
			continue
		}
		if err := setNum(l.dailyStart+d, qty); err != nil {
			return err
		}
	}
	if err := setNum(l.purchasePrice, r.PurchasePrice); err != nil {
		return err
	}
	if err := setNum(l.sellingPrice, r.SellingPrice); err != nil {
		return err
	}
	if err := setNum(l.beginningStock, r.BeginningStock); err != nil {
		return err
	}
	if err := setNum(l.endingStock, r.EndingStock); err != nil {
		return err
	}
	if err := setNum(l.subtotalSold, r.SubtotalSold); err != nil {
		return err
	}
	// subtotal_revenue = selling_price × subtotal_sold
	if err := setFormula(l.subtotalRevenue, fmt.Sprintf("%s%d*%s%d",
		colName(l.sellingPrice), row, colName(l.subtotalSold), row)); err != nil {
		return err
	}
	if err := setNum(l.totalPurchasedAmount, r.TotalPurchasedAmount); err != nil {
		return err
	}
	// total_purchased.total = purchase_price × amount
	if err := setFormula(l.totalPurchasedTotal, fmt.Sprintf("%s%d*%s%d",
		colName(l.purchasePrice), row, colName(l.totalPurchasedAmount), row)); err != nil {
		return err
	}
	if err := setNum(l.totalDisposedAmount, r.TotalDisposedAmount); err != nil {
		return err
	}
	if err := setFormula(l.totalDisposedTotal, fmt.Sprintf("%s%d*%s%d",
		colName(l.purchasePrice), row, colName(l.totalDisposedAmount), row)); err != nil {
		return err
	}
	if err := setNum(l.totalTransferredIn, r.TotalTransferredIn); err != nil {
		return err
	}
	if err := setNum(l.totalTransferredOut, r.TotalTransferredOut); err != nil {
		return err
	}
	return nil
}

// mergeProductGroup applies vertical merges across the rows of a product
// group for the merged level-1 columns. Group-level cells get formulas that
// reference the per-row cells (=SUM(range)).
func mergeProductGroup(f *excelize.File, l colLayout, g productGroup, startRow, endRow int) error {
	if len(g.rows) == 0 {
		return nil
	}
	colName := func(col int) string {
		n, _ := excelize.ColumnNumberToName(col)
		return n
	}

	mergeRange := func(col int) error {
		if startRow == endRow {
			return nil
		}
		topCell, _ := excelize.CoordinatesToCellName(col, startRow)
		bottomCell, _ := excelize.CoordinatesToCellName(col, endRow)
		return f.MergeCell(inOutSheetName, topCell, bottomCell)
	}
	setSUMOnFirst := func(col int, sourceCol int) error {
		topCell, _ := excelize.CoordinatesToCellName(col, startRow)
		formula := fmt.Sprintf("SUM(%s%d:%s%d)", colName(sourceCol), startRow, colName(sourceCol), endRow)
		return f.SetCellFormula(inOutSheetName, topCell, formula)
	}

	if err := mergeRange(l.productName); err != nil {
		return err
	}
	if err := mergeRange(l.unit); err != nil {
		return err
	}

	// Merged aggregations
	if err := setSUMOnFirst(l.totalSold, l.subtotalSold); err != nil {
		return err
	}
	if err := mergeRange(l.totalSold); err != nil {
		return err
	}
	if err := setSUMOnFirst(l.totalBeginningStock, l.beginningStock); err != nil {
		return err
	}
	if err := mergeRange(l.totalBeginningStock); err != nil {
		return err
	}
	if err := setSUMOnFirst(l.totalEndingStock, l.endingStock); err != nil {
		return err
	}
	if err := mergeRange(l.totalEndingStock); err != nil {
		return err
	}
	if err := setSUMOnFirst(l.totalRevenue, l.subtotalRevenue); err != nil {
		return err
	}
	if err := mergeRange(l.totalRevenue); err != nil {
		return err
	}

	return nil
}

// --- footer ---------------------------------------------------------------

func writeFooterRow(f *excelize.File, rows *ExportRows, startRow, endRow int) error {
	if endRow < startRow {
		// no data rows — nothing to footer.
		return nil
	}
	l := buildLayout(rows.DayCount)
	row := endRow + 1
	colName := func(col int) string {
		n, _ := excelize.ColumnNumberToName(col)
		return n
	}
	setLabel := func(col int, label string) error {
		c, _ := excelize.CoordinatesToCellName(col, row)
		return f.SetCellValue(inOutSheetName, c, label)
	}
	setSum := func(col int) error {
		c, _ := excelize.CoordinatesToCellName(col, row)
		formula := fmt.Sprintf("SUM(%s%d:%s%d)", colName(col), startRow, colName(col), endRow)
		return f.SetCellFormula(inOutSheetName, c, formula)
	}

	if err := setLabel(l.productName, "Tổng cộng"); err != nil {
		return err
	}
	if err := setSum(l.totalPurchasedTotal); err != nil {
		return err
	}
	if err := setSum(l.totalDisposedTotal); err != nil {
		return err
	}
	if err := setSum(l.totalRevenue); err != nil {
		return err
	}
	return nil
}

// --- filename helper ------------------------------------------------------

// SanitizeFilenameSegment lowercases, replaces spaces with dashes and
// strips non-alphanumeric (except dash). Preserves the rule from the design
// comment: filename inventory segment is human-readable & URL-safe.
func SanitizeFilenameSegment(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		case r >= '0' && r <= '9':
			out = append(out, r)
		case r == ' ':
			out = append(out, '-')
		case r == '-':
			out = append(out, r)
		}
	}
	return string(out)
}
