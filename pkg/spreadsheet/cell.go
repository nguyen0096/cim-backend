package spreadsheet

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

type Cell struct {
	Row int
	Col int
}

func (c Cell) String() string {
	cellName, err := excelize.CoordinatesToCellName(c.Col, c.Row)
	if err != nil {
		return fmt.Sprintf("Cell{Row: %d, Col: %d}", c.Row, c.Col)
	}
	return cellName
}

func NewCellFromName(name string) (Cell, error) {
	col, row, err := excelize.CellNameToCoordinates(name)
	if err != nil {
		return Cell{}, fmt.Errorf("failed to convert cell name to coordinates: %w", err)
	}
	return Cell{Row: row, Col: col}, nil
}

// getTLBRCells gets the top left and bottom right cells of
// a merged range string.
func getTLBRCells(rangeStr string) (Cell, Cell, error) {
	parts := strings.Split(rangeStr, ":")
	if len(parts) != 2 {
		return Cell{}, Cell{}, fmt.Errorf("invalid merge range")
	}
	tlc, tlr, err := excelize.CellNameToCoordinates(parts[0])
	if err != nil {
		return Cell{}, Cell{}, fmt.Errorf("failed to get cell coordinates: %w", err)
	}
	brc, brr, err := excelize.CellNameToCoordinates(parts[1])
	if err != nil {
		return Cell{}, Cell{}, fmt.Errorf("failed to get cell coordinates: %w", err)
	}
	return Cell{tlr, tlc}, Cell{brr, brc}, nil
}

// ConvertExcelizeMergeCells converts excelize MergeCells to provider-agnostic MergeCells
func ConvertExcelizeMergeCells(excelizeMergeCells []excelize.MergeCell) []MergeCell {
	result := make([]MergeCell, 0, len(excelizeMergeCells))
	for _, em := range excelizeMergeCells {
		rangeStr := em[0]
		parts := strings.Split(rangeStr, ":")
		if len(parts) != 2 {
			continue
		}

		startCol, startRow, err := excelize.CellNameToCoordinates(parts[0])
		if err != nil {
			continue
		}
		endCol, endRow, err := excelize.CellNameToCoordinates(parts[1])
		if err != nil {
			continue
		}

		result = append(result, MergeCell{
			StartRow: startRow,
			StartCol: startCol,
			EndRow:   endRow,
			EndCol:   endCol,
		})
	}
	return result
}

// buildMergeCellLookup builds a merge cell lookup from a list of merge cells.
func buildMergeCellLookup(mergeCells []MergeCell) map[int]map[int]string {
	// Initialize the sheet's spatial index (sparse map)
	spatialIndex := make(map[int]map[int]string)

	// Populate the spatial index - only store cells that are actually merged
	for _, mergeCell := range mergeCells {
		rangeStr := mergeCell.GetRange()

		// Use 1-based coordinates directly for spatial index
		// Populate all cells in the merged range
		for row := mergeCell.StartRow; row <= mergeCell.EndRow; row++ {
			// Initialize row map if it doesn't exist
			if spatialIndex[row] == nil {
				spatialIndex[row] = make(map[int]string)
			}

			for col := mergeCell.StartCol; col <= mergeCell.EndCol; col++ {
				spatialIndex[row][col] = rangeStr
			}
		}
	}

	return spatialIndex
}

// getMergeCellRange checks if a cell is part of a merged range using the spatial index
func getMergeCellRange(mergeCellLookup map[int]map[int]string, cell Cell) (bool, string) {
	rangeStr, rowExists := mergeCellLookup[cell.Row][cell.Col]
	if !rowExists {
		return false, ""
	}
	return true, rangeStr
}

// MergeCell represents a merged cell range in a provider-agnostic way
type MergeCell struct {
	StartRow int
	StartCol int
	EndRow   int
	EndCol   int
}

// GetRange returns the range string in Excel format (e.g., "A1:B2")
func (m MergeCell) GetRange() string {
	startCell, _ := excelize.CoordinatesToCellName(m.StartCol, m.StartRow)
	endCell, _ := excelize.CoordinatesToCellName(m.EndCol, m.EndRow)
	return fmt.Sprintf("%s:%s", startCell, endCell)
}
