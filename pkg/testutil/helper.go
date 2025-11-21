package testutil

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
)

func Failf(format string, args ...any) {
	Fail(fmt.Sprintf(format, args...), 1)
}
