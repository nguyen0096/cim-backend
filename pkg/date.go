package pkg

import "time"

// GetTodayDate returns today's date with time set to midnight
func GetTodayDate() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}
