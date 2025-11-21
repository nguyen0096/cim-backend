package pkg

import (
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/shopspring/decimal"
)

var Validator *validator.Validate

func init() {
	Validator = validator.New()
	Validator.RegisterCustomTypeFunc(ValidateDecimal, decimal.Decimal{})
}

// ValidateDecimal extracts the value from decimal.Decimal for validation
func ValidateDecimal(field reflect.Value) interface{} {
	if field.Type() == reflect.TypeOf(decimal.Decimal{}) {
		dec := field.Interface().(decimal.Decimal)
		return dec.String()
	}
	return nil
}

// IsCSVFile checks if the filename has a .csv extension
func IsCSVFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".csv"
}

// IsExcelFile checks if the filename has an Excel extension
func IsExcelFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".xlsx" || ext == ".xls"
}

// IsProductImportFile checks if the filename is a valid product import file (CSV or Excel)
func IsProductImportFile(filename string) bool {
	return IsCSVFile(filename) || IsExcelFile(filename)
}

// ValidateAndSetSortParams validates sort field and order, setting defaults if invalid
func ValidateAndSetSortParams(sortField string, order string, allowedSortFields []string, defaultSortField string, defaultOrder string) (string, string) {
	// Create a map for O(1) lookup instead of O(n) iteration
	allowedMap := make(map[string]bool, len(allowedSortFields))
	for _, field := range allowedSortFields {
		allowedMap[field] = true
	}

	// Validate sort field
	if !allowedMap[sortField] {
		sortField = defaultSortField
	}

	// Validate order
	if order != "asc" && order != "desc" {
		order = defaultOrder
	}

	return sortField, order
}
