package pkg

import (
	"time"
)

// GetTodayDate returns today's date with time set to midnight
func GetTodayDate() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// GetCurrentMonthStart returns the first day of the month for the given date.
func GetCurrentMonthStart() time.Time {
	date := time.Now()
	return time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// GetCurrentMonthEnd returns the last day of the month for the given date.
func GetCurrentMonthEnd() time.Time {
	date := time.Now()
	return time.Date(date.Year(), date.Month()+1, 0, 0, 0, 0, 0, time.UTC)
}
