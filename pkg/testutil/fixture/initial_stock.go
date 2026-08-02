package fixture

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/xuri/excelize/v2"
)

// InitialStockExpectedHeader is the row-3 header the initial-stock tool requires.
var InitialStockExpectedHeader = []string{"STT", "TÊN", "ĐVT", "SỐ LƯỢNG"}

// InitialStockSheetSpec describes one worksheet of a generated initial-stock workbook.
type InitialStockSheetSpec struct {
	Name string
	// Header overrides the row-3 header. Nil means InitialStockExpectedHeader.
	Header []string
	// HeaderRow places the header on a different 1-based row, modelling the
	// report-style sheet whose headers sit below row 3.
	HeaderRow int
	// Rows are the data rows: name, unit, quantity, category.
	Rows []InitialStockRowSpec
}

// InitialStockRowSpec is one data row. Quantity is written verbatim so a test can
// inject text or a fractional value.
type InitialStockRowSpec struct {
	Name     string
	Unit     string
	Quantity string
	Category string
	// Blank leaves the whole row, STT included, unwritten: the stray empty line a real
	// sheet carries in the middle of its data.
	Blank bool
}

// InitialStockRows builds n rows with deterministic names and the given unit.
func InitialStockRows(prefix, unit string, quantities []string, category string) []InitialStockRowSpec {
	rows := make([]InitialStockRowSpec, 0, len(quantities))
	for i, q := range quantities {
		rows = append(rows, InitialStockRowSpec{
			Name:     fmt.Sprintf("%s %02d", prefix, i+1),
			Unit:     unit,
			Quantity: q,
			Category: category,
		})
	}
	return rows
}

// CreateInitialStockWorkbook renders the sheets to xlsx bytes, mirroring the real
// file's layout: a title row, a merged label row, the header, the data rows, then a
// trailing empty row.
func CreateInitialStockWorkbook(sheets []InitialStockSheetSpec) []byte {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	for i, spec := range sheets {
		name := spec.Name
		if i == 0 {
			if err := f.SetSheetName("Sheet1", name); err != nil {
				panic(fmt.Sprintf("failed to rename first sheet: %v", err))
			}
		} else if _, err := f.NewSheet(name); err != nil {
			panic(fmt.Sprintf("failed to add sheet %s: %v", name, err))
		}

		headerRow := spec.HeaderRow
		if headerRow == 0 {
			headerRow = 3
		}
		header := spec.Header
		if header == nil {
			header = InitialStockExpectedHeader
		}

		set := func(col, row int, value interface{}) {
			cell, err := excelize.CoordinatesToCellName(col, row)
			if err != nil {
				panic(fmt.Sprintf("failed to build cell name: %v", err))
			}
			if err := f.SetCellValue(name, cell, value); err != nil {
				panic(fmt.Sprintf("failed to set %s!%s: %v", name, cell, err))
			}
		}

		set(1, 1, "TỒN KHO THÁNG 11/2025")
		set(1, 2, "KHÔNG CÓ CỘT ĐIỀN")
		if err := f.MergeCell(name, "A2", "D2"); err != nil {
			panic(fmt.Sprintf("failed to merge title row: %v", err))
		}

		for c, h := range header {
			set(c+1, headerRow, h)
		}
		for r, row := range spec.Rows {
			at := headerRow + 1 + r
			if row.Blank {
				continue
			}
			set(1, at, r+1)
			set(2, at, row.Name)
			set(3, at, row.Unit)
			if row.Quantity != "" {
				// SetCellValue on a string writes a text cell, which is exactly what a
				// "text in SỐ LƯỢNG" test needs; numeric strings are re-typed below.
				if num, err := parseNumericCell(row.Quantity); err == nil {
					set(4, at, num)
				} else {
					set(4, at, row.Quantity)
				}
			}
			set(5, at, row.Category)
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		panic(fmt.Sprintf("failed to render workbook: %v", err))
	}
	return bytes.NewBuffer(buf.Bytes()).Bytes()
}

// parseNumericCell reports whether the raw value is a number. ParseFloat requires
// the whole string, so "12abc" stays a text cell.
func parseNumericCell(raw string) (float64, error) {
	return strconv.ParseFloat(raw, 64)
}
