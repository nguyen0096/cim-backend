package pkg

import (
	"fmt"
	"strconv"
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
