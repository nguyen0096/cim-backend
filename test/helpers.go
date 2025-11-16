package apptest

import (
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
)

// ToString converts various types to string
// Fails the test if value is nil or unsupported type (fail-fast for tests)
func ToString(v interface{}) string {
	if v == nil {
		Fail("ToString: value is nil - field may be missing from response")
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
		Fail(fmt.Sprintf("ToString: unsupported type %T", v))
		return ""
	}
}

func Failf(format string, args ...any) {
	Fail(fmt.Sprintf(format, args...), 1)
}
