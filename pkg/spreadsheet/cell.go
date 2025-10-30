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

// buildMergeCellLookup builds a merge cell lookup from a list of merge cells.
func buildMergeCellLookup(mergeCells []excelize.MergeCell) map[int]map[int]string {
	// Initialize the sheet's spatial index (sparse map)
	spatialIndex := make(map[int]map[int]string)

	// Populate the spatial index - only store cells that are actually merged
	for _, mergeCell := range mergeCells {
		rangeStr := mergeCell[0]
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

		// Use 1-based coordinates directly for spatial index
		// Populate all cells in the merged range
		for row := startRow; row <= endRow; row++ {
			// Initialize row map if it doesn't exist
			if spatialIndex[row] == nil {
				spatialIndex[row] = make(map[int]string)
			}

			for col := startCol; col <= endCol; col++ {
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
