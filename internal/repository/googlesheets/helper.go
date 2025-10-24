package googlesheets

import (
	"fmt"
	"strings"
	"time"
)

// Common date formats for parsing dates
var CommonDateFormats = []string{
	"1/02/2006",
	"01/02/2006",
	"02/1/2006",
	"02/01/2006",
	"1-02-2006",
	"01-02-2006",
	"02-01-2006",
	"02-1-2006",
	"2006-1-02",
	"2006-01-02",
	"2006/1/02",
	"2006/01/02",
}

// FindLastTransactionDateInfo finds the last transaction date and its format
func FindLastTransactionDateInfo(rows [][]interface{}, headerRow int, today time.Time) (bool, string) {
	detectedDateFormat := CommonDateFormats[0] // default format
	// Scan rows from bottom up, starting after the header
	for i := len(rows) - 1; i >= headerRow+1; i-- {
		if len(rows[i]) == 0 {
			continue
		}

		dateValue := getCellValueAsString(rows[i][0])
		if dateValue == "" {
			continue
		}

		for _, dateFormat := range CommonDateFormats {
			// try parse datestr
			parsedDate, err := time.Parse(dateFormat, dateValue)
			if err != nil {
				continue
			}

			detectedDateFormat = dateFormat

			if parsedDate.Format(dateFormat) == today.Format(dateFormat) {
				return true, detectedDateFormat
			}
		}

		break
	}

	return false, detectedDateFormat
}

// IsCommentOrEmpty checks if a cell value is a comment or empty
func IsCommentOrEmpty(value string) bool {
	// Fast path for empty strings
	if len(value) == 0 {
		return true
	}

	// Fast path for strings that are just whitespace
	trimmed := strings.TrimSpace(value)
	if len(trimmed) == 0 {
		return true
	}

	// Check for comments
	return strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//")
}

// HasNonEmptyData checks if a row has any non-empty data (optimized)
func HasNonEmptyData(row []interface{}) bool {
	for _, cell := range row {
		cellStr := getCellValueAsString(cell)
		if len(cellStr) > 0 && strings.TrimSpace(cellStr) != "" {
			return true
		}
	}
	return false
}

// FindHeaderRow finds the row that contains column headers
func FindHeaderRow(rows [][]interface{}) int {
	headerRow := -1
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}

		// Check if this row looks like a header row
		headerCount := 0
		for _, cell := range row {
			cellStr := getCellValueAsString(cell)
			if cellStr != "" && !IsCommentOrEmpty(cellStr) {
				headerCount++
			}
		}

		// If we have at least 3 non-empty cells, consider it a header row
		if headerCount >= 3 {
			headerRow = i
			break
		}
	}

	return headerRow
}

// ValidateData validates data before adding it to Google Sheets
func ValidateData(data map[string]interface{}) error {
	if data == nil {
		return fmt.Errorf("data cannot be nil")
	}

	if len(data) == 0 {
		return fmt.Errorf("data cannot be empty")
	}

	// Check if at least one field has a non-empty value
	hasValidData := false
	for _, value := range data {
		if value == nil {
			continue
		}
		valueStr := fmt.Sprintf("%v", value)
		if strings.TrimSpace(valueStr) != "" && valueStr != "<nil>" {
			hasValidData = true
			break
		}
	}

	if !hasValidData {
		return fmt.Errorf("data must contain at least one non-empty value")
	}

	return nil
}

// ParseDateFromRow tries to parse a date from the first column of a row
func ParseDateFromRow(row []interface{}) (time.Time, error) {
	if len(row) == 0 {
		return time.Time{}, fmt.Errorf("empty row")
	}

	dateValue := getCellValueAsString(row[0])
	if dateValue == "" {
		return time.Time{}, fmt.Errorf("empty date value")
	}

	// Try to parse the date using various formats
	for _, dateFormat := range CommonDateFormats {
		if parsedDate, err := time.Parse(dateFormat, dateValue); err == nil {
			return parsedDate, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid date format: %s", dateValue)
}

// getCellValueAsString converts a cell value from interface{} to string
func getCellValueAsString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}
