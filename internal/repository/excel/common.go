package excel

import (
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// Common date formats for parsing Excel dates
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

// DefaultCellStyle provides a common Excel cell style configuration
var DefaultCellStyle = &excelize.Style{
	Font: &excelize.Font{
		Family: "Times New Roman",
		Bold:   true,
	},
	Border: []excelize.Border{
		{Type: "left", Color: "000000", Style: 1},
		{Type: "top", Color: "000000", Style: 1},
		{Type: "bottom", Color: "000000", Style: 1},
		{Type: "right", Color: "000000", Style: 1},
	},
}

// CreateCellStyle creates a new Excel cell style with optional date format
func CreateCellStyle(file *excelize.File, dateFormat string) (int, error) {
	style := *DefaultCellStyle // Copy the default style
	if dateFormat != "" {
		style.CustomNumFmt = &dateFormat
	}

	styleID, err := file.NewStyle(&style)
	if err != nil {
		return 0, fmt.Errorf("failed to create style: %w", err)
	}
	return styleID, nil
}

// DetectExcelDateFormat converts Go time format to Excel date format
func DetectExcelDateFormat(goDateFormat string) string {
	switch goDateFormat {
	case "1/02/2006", "01/02/2006":
		return "mm/dd/yyyy"
	case "02/1/2006", "02/01/2006":
		return "dd/mm/yyyy"
	case "1-02-2006", "01-02-2006":
		return "mm-dd-yyyy"
	case "02-01-2006", "02-1-2006":
		return "dd-mm-yyyy"
	case "2006-1-02", "2006-01-02":
		return "yyyy-mm-dd"
	case "2006/1/02", "2006/01/02":
		return "yyyy/mm/dd"
	default:
		return "mm/dd/yyyy" // default fallback
	}
}

// FindLastTransactionDateInfo finds the last transaction date and its format
func FindLastTransactionDateInfo(rows [][]string, headerRow int, today time.Time) (bool, string) {
	detectedDateFormat := CommonDateFormats[0] // default format
	// Scan rows from bottom up, starting after the header
	for i := len(rows) - 1; i >= headerRow+1; i-- {
		if len(rows[i]) == 0 {
			continue
		}

		dateValue := strings.TrimSpace(rows[i][0])
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
func HasNonEmptyData(row []string) bool {
	for _, cell := range row {
		if len(cell) > 0 && strings.TrimSpace(cell) != "" {
			return true
		}
	}
	return false
}

// FindHeaderRow finds the row that contains column headers
func FindHeaderRow(rows [][]string) int {
	headerRow := -1
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}

		// Check if this row looks like a header row
		headerCount := 0
		for _, cell := range row {
			if cell != "" && !IsCommentOrEmpty(cell) {
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

// GetTodayDate returns today's date with time set to midnight
func GetTodayDate() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// ValidateData validates data before adding it to Excel
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
func ParseDateFromRow(row []string) (time.Time, error) {
	if len(row) == 0 {
		return time.Time{}, fmt.Errorf("empty row")
	}

	dateValue := strings.TrimSpace(row[0])
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
