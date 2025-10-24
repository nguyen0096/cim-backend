package pkg

import (
	"regexp"
	"strings"
)

// ExtractSpreadsheetID extracts the spreadsheet ID from a Google Sheets URL
// Supports formats:
// - https://docs.google.com/spreadsheets/d/{SPREADSHEET_ID}/edit
// - https://docs.google.com/spreadsheets/d/{SPREADSHEET_ID}/edit#gid=0
// - https://docs.google.com/spreadsheets/d/{SPREADSHEET_ID}
func ExtractSpreadsheetID(url string) (string, error) {
	if url == "" {
		return "", ErrValidation("URL cannot be empty", nil)
	}

	// If it's already just an ID (no slashes or dots), return it as-is
	if !strings.Contains(url, "/") && !strings.Contains(url, ".") {
		return url, nil
	}

	// Regular expression to match Google Sheets URL patterns
	re := regexp.MustCompile(`/spreadsheets/d/([a-zA-Z0-9-_]+)`)
	matches := re.FindStringSubmatch(url)

	if len(matches) < 2 {
		return "", ErrValidation("Invalid Google Sheets URL format", nil)
	}

	return matches[1], nil
}
