package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ExcelColumnMetadata represents a column in the Excel schema
type ExcelColumnMetadata struct {
	ColumnIndex int    `json:"column_index"`
	ColumnName  string `json:"column_name"`
	DataType    string `json:"data_type"` // string, number, date
	Required    bool   `json:"required"`
}

// ExcelSheetMetadata represents metadata for an Excel sheet
type ExcelSheetMetadata struct {
	ID        *uuid.UUID            `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	SheetName string                `json:"sheet_name"`
	Headers   []ExcelColumnMetadata `json:"headers"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
}

// FileMetadata contains template schema of excel file and file path
type FileMetadata struct {
	ID        *uuid.UUID           `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	FileType  string               `json:"file_type"`
	FilePath  string               `json:"file_path"`
	Sheets    []ExcelSheetMetadata `json:"sheets"`
	Version   string               `json:"version"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// Parser interface for parsing Excel files from metadata store
type Parser interface {
	Parse() ([]map[string]interface{}, error)
	ParseSheet(sheetName string) ([]map[string]interface{}, error)
}

// Writer interface for appending new row to Excel file
type Writer interface {
	AppendRow(rowData map[string]interface{}) error
	AppendRowToSheet(sheetName string, rowData map[string]interface{}) error
	Save() error
}

func (m *FileMetadata) BeforeCreate(tx *gorm.DB) error {
	if m.ID == nil {
		id := uuid.New()
		m.ID = &id
	}
	return nil
}

func (m *ExcelSheetMetadata) BeforeCreate(tx *gorm.DB) error {
	if m.ID == nil {
		id := uuid.New()
		m.ID = &id
	}
	return nil
}
