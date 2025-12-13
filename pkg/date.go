package pkg

import (
	"time"
)

// GetTodayDate returns today's date with time set to midnight
func GetTodayDate() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// GetMonthStart returns the first day of the current month with the given offset.
// Offset = 0 returns the current month, -1 returns the previous month, +1 returns the next month.
func GetMonthStart(offset int) time.Time {
	date := time.Now()
	return time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, offset, 0)
}
