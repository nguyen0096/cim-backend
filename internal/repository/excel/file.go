package excel

import (
	"fmt"
	"os"
	"sync"

	"github.com/xuri/excelize/v2"
)

type File struct {
	sync.RWMutex
	FileConfig
	Excel  *excelize.File
	Sheets map[SheetInternalID]*Sheet
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

func (f *File) UpsertRow(
	sheetInternalID SheetInternalID,
	indexColHeaderStr MultiLeverHeaderStr,
	indexValue string,
	rowData map[MultiLeverHeaderStr]interface{},
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

	row, ok, err := f.findRowByIndex(sheet.InternalID, indexColHeaderStr.MultiLevelHeader(), indexValue)
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

	err = f.Excel.Save()
	if err != nil {
		return fmt.Errorf("failed to save excel file: %w", err)
	}
	return nil
}

func (f *File) findRowByIndex(
	sheetInternalID SheetInternalID,
	indexColHeader MultiLevelHeader,
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
