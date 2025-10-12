package pkg

import (
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
)

var Validator = validator.New()

// IsCSVFile checks if the filename has a .csv extension
func IsCSVFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".csv"
}

// IsExcelFile checks if the filename has a .xlsx extension
func IsExcelFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".xlsx"
}
