package spreadsheet

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

type ExcelFileProvider struct {
	file *excelize.File
}

func (e *ExcelFileProvider) Connect(filePath string) error {
	file, err := excelize.OpenFile(filePath)
	if err != nil {
		return fmt.Errorf("excel file provider: failed to open excel file: %w", err)
	}
	e.file = file
	return nil
}

func (e *ExcelFileProvider) Close() error {
	if e.file == nil {
		return fmt.Errorf("file not connected")
	}
	err := e.file.Close()
	if err != nil {
		return fmt.Errorf("excel file provider: failed to connect to excel file: %w", err)
	}
	e.file = nil
	return nil
}

func (e *ExcelFileProvider) GetSheetIndex(name string) (int, error) {
	index, err := e.file.GetSheetIndex(name)
	if err != nil {
		return -1, fmt.Errorf("excel file provider: failed to get sheet index: %w", err)
	}
	return index, nil
}

func (e *ExcelFileProvider) InsertRows(sheet string, row, n int) error {
	err := e.file.InsertRows(sheet, row, n)
	if err != nil {
		return fmt.Errorf("excel file provider: failed to insert rows: %w", err)
	}

	err = e.file.Save()
	if err != nil {
		return fmt.Errorf("excel file provider: failed to save excel file: %w", err)
	}

	return nil
}

func (e *ExcelFileProvider) SetCellValue(sheet, cell string, value interface{}) error {
	err := e.file.SetCellValue(sheet, cell, value)
	if err != nil {
		return fmt.Errorf("excel file provider: failed to set cell value: %w", err)
	}

	err = e.file.Save()
	if err != nil {
		return fmt.Errorf("excel file provider: failed to save excel file: %w", err)
	}
	return nil
}

func (e *ExcelFileProvider) GetCellValue(sheet, cell string) (string, error) {
	value, err := e.file.GetCellValue(sheet, cell)
	if err != nil {
		return "", fmt.Errorf("excel file provider: failed to get cell value: %w", err)
	}
	return value, nil
}

func (e *ExcelFileProvider) GetRows(sheet string) ([][]string, error) {
	rows, err := e.file.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("excel file provider: failed to get rows: %w", err)
	}
	return rows, nil
}

func (e *ExcelFileProvider) GetMergeCells(sheet string) ([]excelize.MergeCell, error) {
	mergeCells, err := e.file.GetMergeCells(sheet)
	if err != nil {
		return nil, fmt.Errorf("excel file provider: failed to get merge cells: %w", err)
	}
	return mergeCells, nil
}
