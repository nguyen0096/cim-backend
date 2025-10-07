package excel

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/xuri/excelize/v2"
)

const (
	MetadataHeaderPrefix    = "__"
	EndFileDividerCellColor = "7F8080"
)

type SheetInternalID string

type MultiLevelHeader []string

type MultiLeverHeaderStr string

func (mh MultiLevelHeader) String() MultiLeverHeaderStr {
	return MultiLeverHeaderStr(strings.Join(mh, "."))
}

func (mh MultiLeverHeaderStr) MultiLevelHeader() MultiLevelHeader {
	return MultiLevelHeader(strings.Split(string(mh), "."))
}

// FileConfig contains the configuration of Excel file.
type FileConfig struct {
	FilePath     string        `validate:"required"`
	SheetConfigs []SheetConfig `json:"sheets" validate:"required,min=1,dive"`
}

// SheetConfig contains the configuration for a sheet, used as anchor to let
// repository how to parse the sheet.
type SheetConfig struct {
	InternalID     SheetInternalID   `json:"internal_id" validate:"required"`
	NamePattern    SheetNamePattern  `json:"name_pattern" validate:"required"`
	NameParams     map[string]string `json:"name_params" validate:"-"`
	HeaderStartRow int               `json:"header_start_row" validate:"required,min=1"`
	HeaderStartCol int               `json:"header_start_col" validate:"required,min=1"`
	HeaderHeight   int               `json:"header_height" validate:"required,min=1"`
	FooterHeight   int               `json:"footer_height" validate:"-"`
	DataStartRow   int               `json:"data_start_row" validate:"-"`
	// IndexColumnNames are names of columns that will be indexed for lookup.
	IndexColumnNames []MultiLevelHeader `json:"index_columns" validate:"-"`
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

func (ht HeaderNode) Width() int {
	return ht.BottomRightCell.Col - ht.TopLeftCell.Col + 1
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

// trimFileConfigValues cleans the FileConfig by trimming the strings
func trimFileConfigValues(config FileConfig) FileConfig {
	config.FilePath = strings.TrimSpace(config.FilePath)

	// Trim NamePattern, InternalID, NameParams, IndexColumns if they exist
	for i, sheetConfig := range config.SheetConfigs {
		sheetConfig.NamePattern = SheetNamePattern(strings.TrimSpace(string(sheetConfig.NamePattern)))
		sheetConfig.InternalID = SheetInternalID(strings.TrimSpace(string(sheetConfig.InternalID)))
		if sheetConfig.NameParams != nil {
			for key, value := range sheetConfig.NameParams {
				sheetConfig.NameParams[key] = strings.TrimSpace(value)
			}
		}

		for i, column := range sheetConfig.IndexColumnNames {
			for j, columnName := range column {
				column[j] = strings.TrimSpace(columnName)
			}
			sheetConfig.IndexColumnNames[i] = column
		}

		config.SheetConfigs[i] = sheetConfig
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
