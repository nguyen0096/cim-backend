package spreadsheet

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/xuri/excelize/v2"
)

// FileConfig contains the configuration of Excel file.
type FileConfig struct {
	FilePath     string        `validate:"required"`
	SheetConfigs []SheetConfig `json:"sheets" validate:"required,min=1,dive"`
}

// FileProvider abstracts the underlying spreadsheet implementation (Excel, Google Sheets, etc.)
type FileProvider interface {
	// Connect opens a connection to the file
	Connect(filePath string) error
	// Close closes the connection to the file
	Close() error
	// GetSheetIndex returns the index of a sheet by name
	GetSheetIndex(name string) (int, error)
	// InsertRows inserts rows at the specified position
	InsertRows(sheet string, row, n int) error
	// SetCellValue sets the value of a cell
	SetCellValue(sheet, cell string, value interface{}) error
	// GetCellValue gets the value of a cell
	GetCellValue(sheet, cell string) (string, error)
	// GetRows gets all rows in a sheet
	GetRows(sheet string) ([][]string, error)
	// GetMergeCells gets all merged cells in a sheet
	GetMergeCells(sheet string) ([]excelize.MergeCell, error)
}

type File struct {
	sync.RWMutex
	FileConfig
	Provider FileProvider
	Sheets   map[SheetInternalID]*Sheet
}

func (f *File) UpsertRow(
	sheetInternalID SheetInternalID,
	indexColHeaderStr TreeHeaderStr,
	indexValue string,
	rowData map[TreeHeaderStr]interface{},
) error {
	f.Lock()
	defer f.Unlock()

	if f.Sheets == nil {
		return fmt.Errorf("sheets not found")
	}
	sheet, ok := f.Sheets[sheetInternalID]
	if !ok {
		return fmt.Errorf("sheet internal id [%s] not found", sheetInternalID)
	}

	row, ok, err := f.findRowByIndex(sheet.InternalID, indexColHeaderStr.TreeHeader(), indexValue)
	if err != nil {
		return fmt.Errorf("failed to find row by index: %w", err)
	}

	if ok {
		err = sheet.UpdateRow(row, rowData)
		if err != nil {
			return fmt.Errorf("failed to update row: %w", err)
		}
	} else {
		err = sheet.InsertRow(sheet.DataStartRow, rowData)
		if err != nil {
			return fmt.Errorf("failed to append row: %w", err)
		}
	}
	return nil
}

func (f *File) findRowByIndex(
	sheetInternalID SheetInternalID,
	indexColHeader TreeHeader,
	indexValue string,
) (int, bool, error) {
	if f.Sheets == nil {
		return -1, false, fmt.Errorf("sheets not found")
	}
	sheet, ok := f.Sheets[sheetInternalID]
	if !ok {
		return -1, false, fmt.Errorf("sheet internal id [%s] not found", sheetInternalID)
	}

	indexCol, err := sheet.GetColByExactHeaders(sheet.InternalID, indexColHeader)
	if err != nil {
		return -1, false, fmt.Errorf("failed to get column index: %w", err)
	}

	index, ok := sheet.ColumnIndices[indexCol]
	if !ok {
		return -1, false, fmt.Errorf("index column doesn't have index %d", indexCol)
	}
	row, ok := index[indexValue]
	return row, ok, nil
}

func (f *File) Close() error {
	f.Lock()
	defer f.Unlock()

	if f.Provider == nil {
		return fmt.Errorf("file not opened")
	}

	err := f.Provider.Close()
	f.Provider = nil
	return err
}

// Connect connects to the file and parses the sheets.
func (f *File) Connect() (err error) {
	defer func() {
		if err := recover(); err != nil {
			f.Provider = nil
			err = fmt.Errorf("failed to load file: %v", err)
		}
	}()
	f.Lock()
	defer f.Unlock()

	if f.Provider != nil {
		return fmt.Errorf("file already opened")
	}

	if _, err := os.Stat(f.FilePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", f.FilePath)
	}

	err = f.Provider.Connect(f.FilePath)
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
		idx, err := f.Provider.GetSheetIndex(targetSheet)
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
		// store parsed sheet by internal ID
		if f.Sheets == nil {
			f.Sheets = make(map[SheetInternalID]*Sheet)
		}
		f.Sheets[sheetCfg.InternalID] = s

		// build runtime data
		s.ColumnIndices = make(map[int]map[string]int)
		if err := s.buildColumnIndices(); err != nil {
			return fmt.Errorf("failed to build column indices: %w", err)
		}
	}
	return nil
}

// NewFile creates a new File instance with validated configuration
func NewFile(config FileConfig) (*File, error) {
	config = trimFileConfigValues(config)
	if err := validateFileConfig(config); err != nil {
		return nil, fmt.Errorf("invalid file config: %w", err)
	}

	// set default data start row to header start row + header height if not set
	for i, sheetConfig := range config.SheetConfigs {
		if sheetConfig.DataStartRow == 0 {
			sheetConfig.DataStartRow = sheetConfig.HeaderStartRow + sheetConfig.HeaderHeight
			config.SheetConfigs[i] = sheetConfig
		}
	}

	return &File{FileConfig: config}, nil
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
