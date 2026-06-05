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

// Styling constants. Font is Calibri (tabular figures align numbers cleanly);
// all content is size 14.
//
// Cells stay NUMERIC (so Excel formulas/SUMs recalc and there's no
// "Number Stored as Text" warning). Number-format codes are written in the
// canonical OOXML convention — ',' = thousands group, '.' = decimal — and
// Excel renders the separators per the viewer's regional settings. With
// Vietnamese regional settings (decimal ',', thousands '.') money shows as
// 1.234.567 and quantity as 1.234.567,89.
//   money: 0 decimals (VND has no minor unit).
//   qty:   up to 6 decimals, trailing zeros trimmed (108,9 not 108,900000).
const (
	contentFont = "Calibri"
	contentSize = 14.0
	moneyNumFmt = `#,##0`
	qtyNumFmt   = `#,##0.######`
	headerFill  = "305496" // deep blue, white bold text
	footerFill  = "DDEBF7" // soft light blue
	borderColor = "BFBFBF"
)

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
	if err := applyStyles(f, rows, dataStart, dataEnd); err != nil {
		return nil, err
	}
	// Uniform 25-point height for every row.
	rowHeight := 25.0
	customHeight := true
	if err := f.SetSheetProps(inOutSheetName, &excelize.SheetPropsOptions{
		DefaultRowHeight: &rowHeight,
		CustomHeight:     &customHeight,
	}); err != nil {
		return nil, fmt.Errorf("set default row height: %w", err)
	}
	// Freeze the header block (rows 1-7) and the Sản phẩm + Đơn vị tính columns
	// (A-B) so both stay visible while scrolling across the daily/total columns.
	if err := f.SetPanes(inOutSheetName, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      2,
		YSplit:      7,
		TopLeftCell: "C8",
		ActivePane:  "bottomRight",
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
	productName          int
	unit                 int
	dailyStart           int // first daily-column index
	dailyEnd             int // last daily-column index (inclusive)
	purchasePrice        int
	sellingPrice         int
	beginningStock       int
	endingStock          int
	subtotalSold         int
	subtotalRevenue      int
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
	type pair struct {
		row          int
		label, value string
	}
	pairs := []pair{
		{1, "Kho:", ctx.InventoryName},
		{2, "Khoảng thời gian:", fmt.Sprintf("%s – %s",
			rows.StartDate.Format("02/01/2006"), rows.EndDate.Format("02/01/2006"))},
		{3, "Thời điểm tạo:", ctx.GeneratedAt.Format("02/01/2006 15:04:05")},
		{4, "Người tạo:", ctx.GeneratedBy},
		{5, "Đơn vị tiền tệ:", "VND"},
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
	if err := mergeAndSetL1(l.purchasePrice, "Đơn giá nhập (VND)"); err != nil {
		return err
	}
	if err := mergeAndSetL1(l.sellingPrice, "Đơn giá bán (VND)"); err != nil {
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
	if err := groupHeader(l.dailyStart, l.dailyEnd, "Số lượng nhập trong kì"); err != nil {
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
	if err := subHeader(l.totalPurchasedTotal, "TT (VND)"); err != nil {
		return err
	}

	if err := groupHeader(l.totalDisposedAmount, l.totalDisposedTotal, "Tổng hủy"); err != nil {
		return err
	}
	if err := subHeader(l.totalDisposedAmount, "SL"); err != nil {
		return err
	}
	if err := subHeader(l.totalDisposedTotal, "TT (VND)"); err != nil {
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

// --- styling --------------------------------------------------------------

// applyStyles decorates the already-written sheet: Calibri 14 throughout,
// accounting money / 3-dp quantity number formats, blue header & footer
// emphasis fills, thin borders, alignment and column widths. It never touches
// cell values, formulas or merges.
func applyStyles(f *excelize.File, rows *ExportRows, dataStart, dataEnd int) error {
	l := buildLayout(rows.DayCount)
	colName := func(col int) string { n, _ := excelize.ColumnNumberToName(col); return n }
	money := moneyNumFmt
	qty := qtyNumFmt

	thin := []excelize.Border{
		{Type: "left", Color: borderColor, Style: 1},
		{Type: "right", Color: borderColor, Style: 1},
		{Type: "top", Color: borderColor, Style: 1},
		{Type: "bottom", Color: borderColor, Style: 1},
	}
	baseFont := excelize.Font{Family: contentFont, Size: contentSize}
	boldFont := excelize.Font{Family: contentFont, Size: contentSize, Bold: true}
	whiteBold := excelize.Font{Family: contentFont, Size: contentSize, Bold: true, Color: "FFFFFF"}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &whiteBold,
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{headerFill}},
		Border:    thin,
	})
	if err != nil {
		return fmt.Errorf("header style: %w", err)
	}
	auditLabelStyle, err := f.NewStyle(&excelize.Style{Font: &boldFont})
	if err != nil {
		return err
	}
	auditValueStyle, err := f.NewStyle(&excelize.Style{Font: &baseFont})
	if err != nil {
		return err
	}
	textStyle, err := f.NewStyle(&excelize.Style{
		Font:      &baseFont,
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: true},
		Border:    thin,
	})
	if err != nil {
		return err
	}
	// Product name is emphasized in bold (numeric cells below are bold too).
	productNameStyle, err := f.NewStyle(&excelize.Style{
		Font:      &boldFont,
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: true},
		Border:    thin,
	})
	if err != nil {
		return err
	}
	moneyStyle, err := f.NewStyle(&excelize.Style{
		Font:         &boldFont,
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:       thin,
		CustomNumFmt: &money,
	})
	if err != nil {
		return err
	}
	qtyStyle, err := f.NewStyle(&excelize.Style{
		Font:         &boldFont,
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:       thin,
		CustomNumFmt: &qty,
	})
	if err != nil {
		return err
	}
	footerLabelStyle, err := f.NewStyle(&excelize.Style{
		Font:      &boldFont,
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{footerFill}},
		Border:    thin,
	})
	if err != nil {
		return err
	}
	footerMoneyStyle, err := f.NewStyle(&excelize.Style{
		Font:         &boldFont,
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Fill:         excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{footerFill}},
		Border:       thin,
		CustomNumFmt: &money,
	})
	if err != nil {
		return err
	}

	setCol := func(col, style, fromRow, toRow int) error {
		tl, _ := excelize.CoordinatesToCellName(col, fromRow)
		br, _ := excelize.CoordinatesToCellName(col, toRow)
		return f.SetCellStyle(inOutSheetName, tl, br, style)
	}
	setCell := func(col, row, style int) error {
		c, _ := excelize.CoordinatesToCellName(col, row)
		return f.SetCellStyle(inOutSheetName, c, c, style)
	}

	// Audit block (rows 1-5): bold labels, normal values.
	for r := 1; r <= 5; r++ {
		if err := setCell(1, r, auditLabelStyle); err != nil {
			return err
		}
		if err := setCell(2, r, auditValueStyle); err != nil {
			return err
		}
	}

	// Header rows 6-7 across every column.
	htl, _ := excelize.CoordinatesToCellName(1, 6)
	hbr, _ := excelize.CoordinatesToCellName(l.last, 7)
	if err := f.SetCellStyle(inOutSheetName, htl, hbr, headerStyle); err != nil {
		return err
	}

	// Data rows, by column type.
	if dataEnd >= dataStart {
		moneyCols := []int{l.purchasePrice, l.sellingPrice, l.subtotalRevenue,
			l.totalPurchasedTotal, l.totalDisposedTotal, l.totalRevenue}
		qtyCols := []int{l.beginningStock, l.endingStock, l.subtotalSold,
			l.totalPurchasedAmount, l.totalDisposedAmount, l.totalSold,
			l.totalTransferredIn, l.totalTransferredOut,
			l.totalBeginningStock, l.totalEndingStock}

		if err := setCol(l.productName, productNameStyle, dataStart, dataEnd); err != nil {
			return err
		}
		if err := setCol(l.unit, textStyle, dataStart, dataEnd); err != nil {
			return err
		}
		for c := l.dailyStart; c <= l.dailyEnd; c++ {
			if err := setCol(c, qtyStyle, dataStart, dataEnd); err != nil {
				return err
			}
		}
		for _, c := range qtyCols {
			if err := setCol(c, qtyStyle, dataStart, dataEnd); err != nil {
				return err
			}
		}
		for _, c := range moneyCols {
			if err := setCol(c, moneyStyle, dataStart, dataEnd); err != nil {
				return err
			}
		}

		// Footer row: emphasis fill across the row, money format on the sums.
		footerRow := dataEnd + 1
		ftl, _ := excelize.CoordinatesToCellName(1, footerRow)
		fbr, _ := excelize.CoordinatesToCellName(l.last, footerRow)
		if err := f.SetCellStyle(inOutSheetName, ftl, fbr, footerLabelStyle); err != nil {
			return err
		}
		for _, c := range []int{l.totalPurchasedTotal, l.totalDisposedTotal, l.totalRevenue} {
			if err := setCell(c, footerRow, footerMoneyStyle); err != nil {
				return err
			}
		}
	}

	// Column widths: wide product, narrow daily, roomy value/total columns.
	setW := func(from, to int, w float64) error {
		return f.SetColWidth(inOutSheetName, colName(from), colName(to), w)
	}
	if err := setW(l.productName, l.productName, 50); err != nil {
		return err
	}
	if err := setW(l.unit, l.unit, 30); err != nil {
		return err
	}
	if err := setW(l.dailyStart, l.dailyEnd, 9); err != nil {
		return err
	}
	if err := setW(l.purchasePrice, l.last, 16); err != nil {
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
