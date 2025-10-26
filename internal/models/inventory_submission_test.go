package models

import (
	"cim-backend/pkg"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalErrors(t *testing.T) {
	errors := []error{
		errors.New("error 1"),
		fmt.Errorf("error 2: %w", errors.New("error 3")),
		pkg.NewAppError(pkg.ErrorCodeValidation, "error 4", errors.New("error 5")),
	}
	errorsJSON, err := MarshalErrors(errors)
	require.NoError(t, err)

	// Unmarshal to compare structure instead of string to avoid key order issues
	var actual []map[string]interface{}
	err = json.Unmarshal(errorsJSON, &actual)
	require.NoError(t, err)

	// Verify first error (simple error)
	assert.Equal(t, map[string]interface{}{
		"message": "error 1",
	}, actual[0])

	// Verify second error (wrapped error)
	assert.Equal(t, map[string]interface{}{
		"message": errors[1].Error(),
	}, actual[1])

	// Verify third error (AppError with code, message, and cause)
	assert.Equal(t, map[string]interface{}{
		"code":    "validation",
		"message": "error 4",
		"cause":   "error 5",
	}, actual[2])
}
