package spreadsheet

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

type SheetInternalID string

type SheetNamePattern string

func (s SheetNamePattern) Parse(params map[string]string) string {
	result := string(s)
	for key, value := range params {
		result = strings.ReplaceAll(result, key, value)
	}
	return result
}

// SheetConfig contains the configuration for a sheet, used as anchor to let
// repository how to parse the sheet.
type SheetConfig struct {
	InternalID SheetInternalID `json:"internal_id" validate:"required"`

	// Naming

	NamePattern SheetNamePattern  `json:"name_pattern" validate:"required"`
	NameParams  map[string]string `json:"name_params" validate:"-"`

	// Header
	HeaderStartRow int `json:"header_start_row" validate:"required,min=1"`
	HeaderStartCol int `json:"header_start_col" validate:"required,min=1"`
	HeaderHeight   int `json:"header_height" validate:"required,min=1"`

	// Footer

	FooterHeight int `json:"footer_height" validate:"-"`

	// Data
	DataStartRow int `json:"data_start_row" validate:"-"`

	// IndexColumnNames are names of columns that will be indexed for lookup.
	IndexColumnNames []TreeHeader `json:"index_columns" validate:"-"`
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

	if sc.NameParams == nil {
		sc.NameParams = make(map[string]string)
	}
	for k, v := range timeParams {
		sc.NameParams[k] = v
	}
}

type Sheet struct {
	SheetConfig
	File      *File
	SheetName string
	Index     int

	// HeaderRoot is the root node of the header tree. (Sheet metadata)
	HeaderRoot *HeaderNode
	// MergeCellLookup is a map of cell coordinates to the merge cell range. (Sheet metadata)
	MergeCellLookup map[int]map[int]string

	// ColumnIndices is a map of column number to column index map. (Runtime data)
	ColumnIndices map[int]map[string]int
}

// AppendRow appends a new row to the sheet.
func (s *Sheet) InsertRow(
	ctx context.Context,
	rowNumber int,
	rowData map[TreeHeaderStr]interface{},
) error {
	if rowNumber < s.DataStartRow {
		return fmt.Errorf("row number is less than data start row")
	}
	err := s.File.Provider.InsertRows(ctx, s.SheetName, rowNumber, 1)
	if err != nil {
		return fmt.Errorf("failed to insert rows: %w", err)
	}
	return s.UpdateRow(ctx, rowNumber, rowData)
}

// UpdateRow updates the row data by headers.
func (s *Sheet) UpdateRow(
	ctx context.Context,
	rowNumber int,
	rowData map[TreeHeaderStr]interface{},
) error {
	for header, value := range rowData {
		col, err := s.GetColByExactHeaders(s.InternalID, header.TreeHeader())
		if err != nil {
			return fmt.Errorf("failed to get column index: %w", err)
		}
		cell := Cell{Row: rowNumber, Col: col}
		if err := s.File.Provider.SetCellValue(ctx, s.SheetName, cell.String(), value); err != nil {
			return fmt.Errorf("failed to set cell value: %w", err)
		}
	}
	return nil
}

func (s *Sheet) GetColByExactHeaders(sheetInternalID SheetInternalID, headers []string) (int, error) {
	headerNode := s.HeaderRoot

	// if defined header doens't have enough levels, fill up the the low levels with empty string
	for i := len(headers); i < s.HeaderHeight; i++ {
		headers = append(headers, "")
	}

	for _, header := range headers {
		if headerNode.Lookup == nil {
			return -1, fmt.Errorf("header lookup not found")
		}
		node, ok := headerNode.Lookup[header]
		if !ok {
			return -1, fmt.Errorf("header [%s] not found", header)
		}
		headerNode = node
	}

	if headerNode.TopLeftCell.Col != headerNode.BottomRightCell.Col {
		return -1, fmt.Errorf("header node is not a single column")
	}
	return headerNode.TopLeftCell.Col, nil
}

// recursively parse a node and its children.
func (s *Sheet) recursivelyParseNode(ctx context.Context, currentDepth int, topLeftCell Cell) (*HeaderNode, error) {
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

	// get header text from cell value and trim space
	cellValue, err := s.File.Provider.GetCellValue(ctx, s.SheetName, cellName)
	if err != nil {
		return nil, fmt.Errorf("failed to get cell value: %w", err)
	}
	cellValue = strings.TrimSpace(cellValue)

	if currentDepth == s.HeaderHeight {
		// header is the lowest level, return the node
		return &HeaderNode{
			Value:           cellValue,
			TopLeftCell:     topLeftCell,
			BottomRightCell: bottomRightCell,
			SubHeaders:      []*HeaderNode{},
			IsEmpty:         cellValue == "",
		}, nil
	}

	node := &HeaderNode{
		Value:           cellValue,
		TopLeftCell:     topLeftCell,
		BottomRightCell: bottomRightCell,
		SubHeaders:      []*HeaderNode{},
		IsEmpty:         cellValue == "",
	}

	currentCol := topLeftCell.Col
	for currentCol <= bottomRightCell.Col {
		child, err := s.recursivelyParseNode(
			ctx,
			currentDepth+1,
			Cell{Col: currentCol, Row: node.BottomRightCell.Row + 1})
		if err != nil {
			return nil, fmt.Errorf("failed to parse child node: %w", err)
		}
		// update parent node
		node.IsEmpty = node.IsEmpty && child.IsEmpty
		node.SubHeaders = append(node.SubHeaders, child)
		if err := node.SetLookup(child.Value, child); err != nil {
			return nil, fmt.Errorf("failed to set lookup value [%s] cell [%s] - [%s]",
				child.Value, node.TopLeftCell.String(), node.BottomRightCell.String())
		}

		// move to next column
		currentCol += child.BottomRightCell.Col - child.TopLeftCell.Col + 1
	}

	return node, nil
}

// buildColumnIndices builds the column indices for the sheet.
func (s *Sheet) buildColumnIndices(ctx context.Context) error {
	for _, mlHeader := range s.IndexColumnNames {
		col, err := s.GetColByExactHeaders(s.InternalID, mlHeader)
		if err != nil {
			return fmt.Errorf("failed to get column index: %w", err)
		}

		rows, err := s.File.Provider.GetRows(ctx, s.SheetName)
		if err != nil {
			return fmt.Errorf("failed to get Excel rows: %w", err)
		}
		bottomRow := len(rows)

		indexMap := make(map[string]int)
		// read cell value from DataStartRow to either divider cell or end of sheet
		for row := s.DataStartRow; row <= bottomRow; row++ {
			cellName, err := excelize.CoordinatesToCellName(col, row)
			if err != nil {
				return fmt.Errorf("failed to convert coordinates to cell name")
			}
			cellValue, err := s.File.Provider.GetCellValue(ctx, s.SheetName, cellName)
			if err != nil {
				return fmt.Errorf("failed to get cell value: %w", err)
			}
			cellValue = strings.TrimSpace(cellValue)
			indexMap[cellValue] = row
		}

		if s.ColumnIndices == nil {
			s.ColumnIndices = make(map[int]map[string]int)
		}
		s.ColumnIndices[col] = indexMap
	}
	return nil
}

// parseHeader parses the header structure of an Excel sheet into a tree representation.
// It analyzes merged cells and creates HeaderTree structures that represent the hierarchical
// organization of headers across multiple rows.
func (s *Sheet) parseHeader(ctx context.Context) error {
	mergeCells, err := s.File.Provider.GetMergeCells(ctx, s.SheetName)
	if err != nil {
		return fmt.Errorf("failed to get merged cells: %w", err)
	}
	s.MergeCellLookup = buildMergeCellLookup(mergeCells)
	s.HeaderRoot, err = s.parseHeaderRow(ctx)
	if err != nil {
		return fmt.Errorf("failed to parse header row: %w", err)
	}
	return nil
}

// parseHeaderRow recursively parses a header row and its children
func (s *Sheet) parseHeaderRow(ctx context.Context) (*HeaderNode, error) {
	if s.HeaderStartCol == 0 || s.HeaderStartRow == 0 || s.HeaderHeight == 0 {
		return nil, fmt.Errorf("header start col, row or height is not set")
	}

	rootNode := &HeaderNode{
		SubHeaders: []*HeaderNode{},
	}

	currentCol := s.HeaderStartCol
	for {
		node, err := s.recursivelyParseNode(ctx, 1, Cell{Row: s.HeaderStartRow, Col: currentCol})
		if err != nil {
			return nil, fmt.Errorf("failed to parse node: %w", err)
		}
		currentCol += node.BottomRightCell.Col - node.TopLeftCell.Col + 1

		if node.IsEmpty {
			// stop the loop when we found an empty header tree
			break
		}

		// capture new node and set lookup
		rootNode.SubHeaders = append(rootNode.SubHeaders, node)
		if err := rootNode.SetLookup(node.Value, node); err != nil {
			return nil, fmt.Errorf("failed to set lookup value [%s] cell [%s] - [%s]",
				node.Value, node.TopLeftCell.String(), node.BottomRightCell.String())
		}
	}
	return rootNode, nil
}
