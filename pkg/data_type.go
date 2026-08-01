package pkg

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

func Ptr[T any](v T) *T {
	return &v
}

// ToString converts various types to string.
func ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		// JSON numbers are decoded as float64
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// DecimalPlaces returns the number of significant decimal places in d
// (e.g. "100.11" -> 2, "100.00" -> 0). decimal.String() drops trailing zeros.
func DecimalPlaces(d decimal.Decimal) int {
	parts := strings.Split(d.String(), ".")
	if len(parts) == 1 {
		return 0
	}
	return len(parts[1])
}
