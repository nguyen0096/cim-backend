package excel

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/xuri/excelize/v2"
)

// FileConfig contains the configuration of Excel file.
type FileConfig struct {
	FilePath string
	Sheets   []SheetConfig
}

// SheetConfig contains the configuration for a sheet, used as anchor to let
// repository how to parse the sheet.
type SheetConfig struct {
	SheetNamePattern SheetNamePattern
	SheetNameParams  map[string]string
	HeaderStartRow   int
	HeaderStartCol   int
	HeaderHeight     int
}

func (sc *SheetConfig) SetSheetNameTimeParams(t time.Time) {
	timeParams := map[string]string{
		"{MM}":   t.Format("01"),
		"{M}":    t.Format("1"),
		"{DD}":   t.Format("02"),
		"{D}":    t.Format("2"),
		"{YYYY}": t.Format("2006"),
		"{YY}":   t.Format("06"),
	}

	if sc.SheetNameParams == nil {
		sc.SheetNameParams = make(map[string]string)
	}
	for k, v := range timeParams {
		sc.SheetNameParams[k] = v
	}
}

type SheetNamePattern string

func (s SheetNamePattern) Parse(params map[string]string) string {
	result := string(s)
	for key, value := range params {
		result = strings.ReplaceAll(result, key, value)
	}
	return result
}

type Sheet struct {
	SheetConfig
	File      *File
	SheetName string
	Index     int

	// Header metadata

	HeaderTrees     []HeaderNode
	MergeCellLookup map[int]map[int]string
}

type HeaderNode struct {
	TopLeftCell     Cell
	BottomRightCell Cell
	Value           string
	// IsEmpty represents if the node and all its children are empty
	IsEmpty    bool
	SubHeaders []HeaderNode
}

type Cell struct {
	Row int
	Col int
}

func NewCellFromName(name string) (Cell, error) {
	col, row, err := excelize.CellNameToCoordinates(name)
	if err != nil {
		return Cell{}, fmt.Errorf("failed to convert cell name to coordinates: %w", err)
	}
	return Cell{Row: row, Col: col}, nil
}

func (ht HeaderNode) Width() int {
	return ht.BottomRightCell.Col - ht.TopLeftCell.Col + 1
}

type File struct {
	sync.RWMutex
	FileConfig
	Excel  *excelize.File
	sheets []*Sheet
}

func NewFile(config FileConfig) (*File, error) {
	return &File{
		FileConfig: config,
	}, nil
}

func (f *File) Close() error {
	f.Lock()
	defer f.Unlock()

	if f.Excel == nil {
		return fmt.Errorf("file not opened")
	}

	err := f.Excel.Close()
	f.Excel = nil
	return err
}

// Load reads an Excel file and creates a in-memory excelize.File instance.
func (f *File) Load() (err error) {
	defer func() {
		if err := recover(); err != nil {
			f.Excel = nil
			err = fmt.Errorf("failed to load file: %v", err)
		}
	}()
	f.Lock()
	defer f.Unlock()

	if f.Excel != nil {
		return fmt.Errorf("file already opened")
	}

	if _, err := os.Stat(f.FilePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", f.FilePath)
	}

	f.Excel, err = excelize.OpenFile(f.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open excel file: %w", err)
	}

	if err := f.parseSheets(); err != nil {
		return fmt.Errorf("failed to parse sheets: %w", err)
	}

	return nil
}

func (f *File) parseSheets() error {
	for _, sheetCfg := range f.Sheets {
		targetSheet := sheetCfg.SheetNamePattern.Parse(sheetCfg.SheetNameParams)
		idx, err := f.Excel.GetSheetIndex(targetSheet)
		if err != nil {
			continue
		}
		s := &Sheet{
			File:        f,
			SheetConfig: sheetCfg,
			SheetName:   targetSheet,
			Index:       idx,
		}

		if err := s.parseHeader(); err != nil {
			return fmt.Errorf("failed to parse header: %w", err)
		}

		if f.sheets == nil {
			f.sheets = make([]*Sheet, 0, len(f.Sheets))
		}
		f.sheets = append(f.sheets, s)
	}
	return nil
}

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

// parseHeader parses the header structure of an Excel sheet into a tree representation.
// It analyzes merged cells and creates HeaderTree structures that represent the hierarchical
// organization of headers across multiple rows.
func (s *Sheet) parseHeader() error {
	mergeCells, err := s.File.Excel.GetMergeCells(s.SheetName)
	if err != nil {
		return fmt.Errorf("failed to get merged cells: %w", err)
	}
	s.MergeCellLookup = buildMergeCellLookup(mergeCells)
	s.HeaderTrees, err = s.parseHeaderRow()
	if err != nil {
		return fmt.Errorf("failed to parse header row: %w", err)
	}
	return nil
}

// parseHeaderRow recursively parses a header row and its children
func (s *Sheet) parseHeaderRow() ([]HeaderNode, error) {
	if s.HeaderStartCol == 0 || s.HeaderStartRow == 0 || s.HeaderHeight == 0 {
		return nil, fmt.Errorf("header start col, row or height is not set")
	}

	var foundEmptyTree bool
	currentCol := s.HeaderStartCol
	for !foundEmptyTree {
		node, err := s.recursivelyParseNode(1, Cell{Row: s.HeaderStartRow, Col: currentCol})
		if err != nil {
			return nil, fmt.Errorf("failed to parse node: %w", err)
		}
		foundEmptyTree = node.IsEmpty
		currentCol += node.BottomRightCell.Col - node.TopLeftCell.Col + 1
	}
	return s.HeaderTrees, nil
}

// recursively parse a node and its children.
func (s *Sheet) recursivelyParseNode(currentDepth int, topLeftCell Cell) (*HeaderNode, error) {
	if currentDepth == 0 {
		return nil, fmt.Errorf("current depth cannot be 0")
	}

	bottomRightCell := topLeftCell
	ok, mergeRange := getMergeCellRange(s.MergeCellLookup, topLeftCell)
	if ok {
		tl, br, err := getTLBRCells(mergeRange)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLBR cells: %w", err)
		}

		if tl.Row != topLeftCell.Row || tl.Col != topLeftCell.Col {
			return nil, fmt.Errorf("root cell is expected to be top left cell of the merge range")
		}
		bottomRightCell = br
		currentDepth = currentDepth + (br.Row - tl.Row)
	}

	cellName, err := excelize.CoordinatesToCellName(topLeftCell.Col, topLeftCell.Row)
	if err != nil {
		return nil, fmt.Errorf("failed to convert coordinates to cell name: %w", err)
	}

	if currentDepth > s.HeaderHeight {
		return nil, fmt.Errorf("header of column %d is out of bounds", topLeftCell.Col)
	}

	cellValue, err := s.File.Excel.GetCellValue(s.SheetName, cellName)
	if err != nil {
		return nil, fmt.Errorf("failed to get cell value: %w", err)
	}

	if currentDepth == s.HeaderHeight {
		// header is the lowest level, return the node
		return &HeaderNode{
			Value:           cellValue,
			TopLeftCell:     topLeftCell,
			BottomRightCell: bottomRightCell,
			SubHeaders:      []HeaderNode{},
			IsEmpty:         cellValue == "",
		}, nil
	}

	node := &HeaderNode{
		Value:           cellValue,
		TopLeftCell:     topLeftCell,
		BottomRightCell: bottomRightCell,
		SubHeaders:      []HeaderNode{},
		IsEmpty:         cellValue == "",
	}

	currentCol := topLeftCell.Col
	for currentCol <= bottomRightCell.Col {
		child, err := s.recursivelyParseNode(
			currentDepth+1,
			Cell{Col: currentCol, Row: node.BottomRightCell.Row + 1})
		if err != nil {
			return nil, fmt.Errorf("failed to parse child node: %w", err)
		}
		// update parent node
		node.IsEmpty = node.IsEmpty && child.IsEmpty
		node.SubHeaders = append(node.SubHeaders, *child)

		// move to next column
		currentCol += child.BottomRightCell.Col - child.TopLeftCell.Col + 1
	}

	return node, nil
}

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
