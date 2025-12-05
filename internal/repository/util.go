package repository

import "strings"

// isDuplicateError checks if the error is a duplicate/unique constraint violation
func isDuplicateError(err error, constraintName *string) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "duplicate") &&
		strings.Contains(errStr, "unique constraint") &&
		(constraintName == nil || strings.Contains(errStr, *constraintName))
}
