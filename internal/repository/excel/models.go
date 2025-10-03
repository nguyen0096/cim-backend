package excel

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/xuri/excelize/v2"
)

// FileConfig contains the configuration of Excel file.
type FileConfig struct {
	FilePath     string        `validate:"required"`
	SheetConfigs []SheetConfig `json:"sheets" validate:"required,min=1,dive"`

	// IndexColumns are names of columns that will be indexed for lookup.
	IndexColumns []string `json:"index_columns" validate:"-"`
}

type SheetInternalID string

// SheetConfig contains the configuration for a sheet, used as anchor to let
// repository how to parse the sheet.
type SheetConfig struct {
	InternalID     SheetInternalID   `json:"internal_id" validate:"required"`
	NamePattern    SheetNamePattern  `json:"name_pattern" validate:"required"`
	NameParams     map[string]string `json:"name_params" validate:"-"`
	HeaderStartRow int               `json:"header_start_row" validate:"required,min=1"`
	HeaderStartCol int               `json:"header_start_col" validate:"required,min=1"`
	HeaderHeight   int               `json:"header_height" validate:"required,min=1"`
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

	// HeaderRoot is the root node of the header tree. (Sheet metadata)
	HeaderRoot *HeaderNode
	// MergeCellLookup is a map of cell coordinates to the merge cell range. (Sheet metadata)
	MergeCellLookup map[int]map[int]string
}

type HeaderNode struct {
	TopLeftCell     Cell
	BottomRightCell Cell
	Value           string
	// IsEmpty represents if the node and all its children are empty
	IsEmpty    bool
	SubHeaders []*HeaderNode
	Lookup     map[string]*HeaderNode
}

func (hn *HeaderNode) SetLookup(value string, node *HeaderNode) error {
	if hn.Lookup == nil {
		hn.Lookup = make(map[string]*HeaderNode)
	}
	if _, ok := hn.Lookup[value]; ok {
		return fmt.Errorf("lookup value already exists")
	}
	hn.Lookup[value] = node
	return nil
}

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

func (ht HeaderNode) Width() int {
	return ht.BottomRightCell.Col - ht.TopLeftCell.Col + 1
}

type File struct {
	sync.RWMutex
	FileConfig
	Excel  *excelize.File
	Sheets map[SheetInternalID]*Sheet
}

// trimFileConfigValues cleans the FileConfig by trimming the strings
func trimFileConfigValues(config FileConfig) FileConfig {
	config.FilePath = strings.TrimSpace(config.FilePath)

	for i := range config.SheetConfigs {
		config.SheetConfigs[i].NamePattern = SheetNamePattern(strings.TrimSpace(string(config.SheetConfigs[i].NamePattern)))
		config.SheetConfigs[i].InternalID = SheetInternalID(strings.TrimSpace(string(config.SheetConfigs[i].InternalID)))
		if config.SheetConfigs[i].NameParams != nil {
			for key, value := range config.SheetConfigs[i].NameParams {
				config.SheetConfigs[i].NameParams[key] = strings.TrimSpace(value)
			}
		}
	}

	// Trim IndexColumns if they exist
	for i := range config.IndexColumns {
		config.IndexColumns[i] = strings.TrimSpace(config.IndexColumns[i])
	}

	return config
}

// validateFileConfig validates the FileConfig structure
func validateFileConfig(config FileConfig) error {
	validate := validator.New()
	if err := validate.Struct(config); err != nil {
		return fmt.Errorf("failed to validate file config: %w", err)
	}
	return nil
}

// NewFile creates a new File instance with validated configuration
func NewFile(config FileConfig) (*File, error) {
	config = trimFileConfigValues(config)
	if err := validateFileConfig(config); err != nil {
		return nil, fmt.Errorf("invalid file config: %w", err)
	}
	return &File{FileConfig: config}, nil
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

func (f *File) GetColByExactHeaders(sheetInternalID string, headers []string) (int, error) {
	if f.Sheets == nil {
		return -1, fmt.Errorf("sheets not found")
	}
	sheet, ok := f.Sheets[SheetInternalID(sheetInternalID)]
	if !ok {
		return -1, fmt.Errorf("sheet internal id [%s] not found", sheetInternalID)
	}

	headerNode := sheet.HeaderRoot
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
	for _, sheetCfg := range f.SheetConfigs {
		targetSheet := sheetCfg.NamePattern.Parse(sheetCfg.NameParams)
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

		if f.Sheets == nil {
			f.Sheets = make(map[SheetInternalID]*Sheet)
		}
		f.Sheets[sheetCfg.InternalID] = s
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
	s.HeaderRoot, err = s.parseHeaderRow()
	if err != nil {
		return fmt.Errorf("failed to parse header row: %w", err)
	}
	return nil
}

// parseHeaderRow recursively parses a header row and its children
func (s *Sheet) parseHeaderRow() (*HeaderNode, error) {
	if s.HeaderStartCol == 0 || s.HeaderStartRow == 0 || s.HeaderHeight == 0 {
		return nil, fmt.Errorf("header start col, row or height is not set")
	}

	rootNode := &HeaderNode{
		SubHeaders: []*HeaderNode{},
	}

	currentCol := s.HeaderStartCol
	for {
		node, err := s.recursivelyParseNode(1, Cell{Row: s.HeaderStartRow, Col: currentCol})
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

	// get header text from cell value and trim space
	cellValue, err := s.File.Excel.GetCellValue(s.SheetName, cellName)
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
