package services

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"cim-backend/pkg"
)

// errInitialStockHeaderMismatch marks a sheet whose row-3 header is not the
// expected one. Callers turn it into a per-sheet verdict or a request error.
var errInitialStockHeaderMismatch = fmt.Errorf("initial stock: unexpected sheet header")

// openInitialStockWorkbook opens the upload with explicit unzip limits and converts
// any failure, including a panic inside excelize on malformed third-party input,
// into a keyed 400 rather than letting the recovery middleware collapse it to a 500.
func openInitialStockWorkbook(data []byte) (f *excelize.File, err error) {
	defer func() {
		if r := recover(); r != nil {
			f = nil
			err = pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockParseFailed)
		}
	}()

	if len(data) == 0 {
		return nil, pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockEmptyFile)
	}

	opened, openErr := excelize.OpenReader(bytes.NewReader(data), excelize.Options{
		UnzipSizeLimit:    initialStockUnzipSizeLimit,
		UnzipXMLSizeLimit: initialStockUnzipXMLSizeLimit,
	})
	if openErr != nil {
		return nil, pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockParseFailed)
	}
	return opened, nil
}

// parseInitialStockSheet reads the data rows of a sheet: header fixed at row 3,
// data from row 4 to the last row of the sheet. A fully empty row carries nothing to
// lose and is skipped, never taken as the end of the data: ending there would drop
// everything below it without any count or error reflecting the loss. Returns
// errInitialStockHeaderMismatch when the header does not match.
func parseInitialStockSheet(f *excelize.File, sheetName string) (rows []sheetRow, err error) {
	defer func() {
		if r := recover(); r != nil {
			rows = nil
			err = pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockParseFailed)
		}
	}()

	// RawCellValue keeps quantities as their stored decimal string, so a display
	// number format cannot alter the parsed value.
	raw, rowsErr := f.GetRows(sheetName, excelize.Options{RawCellValue: true})
	if rowsErr != nil {
		return nil, pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockParseFailed)
	}

	if len(raw) < initialStockHeaderRow {
		return nil, errInitialStockHeaderMismatch
	}
	if !matchesInitialStockHeader(raw[initialStockHeaderRow-1]) {
		return nil, errInitialStockHeaderMismatch
	}

	out := make([]sheetRow, 0, len(raw))
	for i := initialStockHeaderRow; i < len(raw); i++ {
		record := raw[i]
		if isEmptyRecord(record) {
			continue
		}
		out = append(out, sheetRow{
			SheetRow:    i + 1,
			Name:        strings.TrimSpace(cellAt(record, 1)),
			Unit:        strings.TrimSpace(cellAt(record, 2)),
			RawQuantity: strings.TrimSpace(cellAt(record, 3)),
			ProductType: strings.TrimSpace(cellAt(record, 4)),
		})
	}
	return out, nil
}

func matchesInitialStockHeader(record []string) bool {
	for i, want := range initialStockExpectedHeader {
		if strings.ToUpper(strings.TrimSpace(cellAt(record, i))) != want {
			return false
		}
	}
	return true
}

func cellAt(record []string, idx int) string {
	if idx < 0 || idx >= len(record) {
		return ""
	}
	return record[idx]
}

func isEmptyRecord(record []string) bool {
	for _, cell := range record {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}
