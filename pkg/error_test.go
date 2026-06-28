package pkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBatchError(t *testing.T) {
	t.Run("should create BatchError with code and message when no cause", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "validation failed", nil)

		assert.NotNil(t, batchErr)
		assert.Equal(t, ErrorCodeValidation, batchErr.Code)
		assert.Equal(t, "validation failed", batchErr.Message)
		assert.Nil(t, batchErr.Cause)
		assert.Empty(t, batchErr.Locations)
	})

	t.Run("should create BatchError with cause when provided", func(t *testing.T) {
		cause := errors.New("underlying error")
		batchErr := NewBatchError(ErrorCodeInternal, "batch processing failed", cause)

		assert.NotNil(t, batchErr)
		assert.Equal(t, ErrorCodeInternal, batchErr.Code)
		assert.Equal(t, "batch processing failed", batchErr.Message)
		assert.Equal(t, cause, batchErr.Cause)
		assert.Empty(t, batchErr.Locations)
	})

	t.Run("should initialize empty locations slice", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "test", nil)

		assert.NotNil(t, batchErr.Locations)
		assert.Len(t, batchErr.Locations, 0)
	})
}

func TestBatchError_AddLocation(t *testing.T) {
	t.Run("should add single location to BatchError", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "validation failed", nil)
		batchErr.AddLocation("row 1", "invalid product code")

		assert.Len(t, batchErr.Locations, 1)
		assert.Equal(t, "row 1", batchErr.Locations[0].Location)
		assert.Equal(t, "invalid product code", batchErr.Locations[0].Message)
	})

	t.Run("should add multiple locations to BatchError", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "validation failed", nil)
		batchErr.AddLocation("row 1", "invalid product code")
		batchErr.AddLocation("row 3", "missing quantity")
		batchErr.AddLocation("row 5", "negative price")

		assert.Len(t, batchErr.Locations, 3)
		assert.Equal(t, "row 1", batchErr.Locations[0].Location)
		assert.Equal(t, "invalid product code", batchErr.Locations[0].Message)
		assert.Equal(t, "row 3", batchErr.Locations[1].Location)
		assert.Equal(t, "missing quantity", batchErr.Locations[1].Message)
		assert.Equal(t, "row 5", batchErr.Locations[2].Location)
		assert.Equal(t, "negative price", batchErr.Locations[2].Message)
	})

	t.Run("should preserve order of added locations", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "test", nil)

		for i := 1; i <= 10; i++ {
			batchErr.AddLocation("location "+string(rune('A'+i-1)), "error message")
		}

		assert.Len(t, batchErr.Locations, 10)
		for i := 0; i < 10; i++ {
			assert.Equal(t, "location "+string(rune('A'+i)), batchErr.Locations[i].Location)
		}
	})
}

func TestBatchError_Error(t *testing.T) {
	t.Run("should return message only when no cause and no locations", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "validation failed", nil)

		result := batchErr.Error()

		assert.Equal(t, "validation failed", result)
	})

	t.Run("should include cause in error message when provided", func(t *testing.T) {
		cause := errors.New("database error")
		batchErr := NewBatchError(ErrorCodeInternal, "batch processing failed", cause)

		result := batchErr.Error()

		assert.Contains(t, result, "batch processing failed")
		assert.Contains(t, result, "database error")
		assert.Equal(t, "batch processing failed: database error", result)
	})

	t.Run("should include locations in error message when present", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "validation failed", nil)
		batchErr.AddLocation("row 1", "invalid product")
		batchErr.AddLocation("row 3", "missing data")

		result := batchErr.Error()

		assert.Contains(t, result, "validation failed")
		assert.Contains(t, result, "Locations:")
		assert.Contains(t, result, "- row 1: invalid product")
		assert.Contains(t, result, "- row 3: missing data")
	})

	t.Run("should include both cause and locations when both present", func(t *testing.T) {
		cause := errors.New("internal error")
		batchErr := NewBatchError(ErrorCodeInternal, "batch failed", cause)
		batchErr.AddLocation("row 1", "error 1")
		batchErr.AddLocation("row 2", "error 2")

		result := batchErr.Error()

		assert.Contains(t, result, "batch failed")
		assert.Contains(t, result, "internal error")
		assert.Contains(t, result, "Locations:")
		assert.Contains(t, result, "- row 1: error 1")
		assert.Contains(t, result, "- row 2: error 2")
	})

	t.Run("should format multiple locations correctly", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "test", nil)
		batchErr.AddLocation("line 1", "error A")
		batchErr.AddLocation("line 2", "error B")
		batchErr.AddLocation("line 3", "error C")

		result := batchErr.Error()

		expected := "test\nLocations:\n\n- line 1: error A\n- line 2: error B\n- line 3: error C"
		assert.Equal(t, expected, result)
	})
}

func TestBatchError_MarshalJSON(t *testing.T) {
	t.Run("should marshal BatchError with code and message only", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "validation failed", nil)

		jsonData, err := json.Marshal(batchErr)

		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonData, &result)
		require.NoError(t, err)

		assert.Equal(t, "validation", result["code"])
		assert.Equal(t, "validation failed", result["message"])
		assert.NotContains(t, result, "cause")
		assert.NotContains(t, result, "locations")
	})

	t.Run("should marshal BatchError with cause when provided", func(t *testing.T) {
		cause := errors.New("underlying error")
		batchErr := NewBatchError(ErrorCodeInternal, "batch failed", cause)

		jsonData, err := json.Marshal(batchErr)

		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonData, &result)
		require.NoError(t, err)

		assert.Equal(t, "internal", result["code"])
		assert.Equal(t, "batch failed", result["message"])
		assert.Equal(t, "underlying error", result["cause"])
		assert.NotContains(t, result, "locations")
	})

	t.Run("should marshal BatchError with locations when present", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "validation failed", nil)
		batchErr.AddLocation("row 1", "invalid product")
		batchErr.AddLocation("row 3", "missing data")

		jsonData, err := json.Marshal(batchErr)

		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonData, &result)
		require.NoError(t, err)

		assert.Equal(t, "validation", result["code"])
		assert.Equal(t, "validation failed", result["message"])
		assert.NotContains(t, result, "cause")

		locations, ok := result["locations"].([]interface{})
		require.True(t, ok)
		require.Len(t, locations, 2)

		loc1 := locations[0].(map[string]interface{})
		assert.Equal(t, "row 1", loc1["location"])
		assert.Equal(t, "invalid product", loc1["message"])

		loc2 := locations[1].(map[string]interface{})
		assert.Equal(t, "row 3", loc2["location"])
		assert.Equal(t, "missing data", loc2["message"])
	})

	t.Run("should marshal BatchError with both cause and locations", func(t *testing.T) {
		cause := errors.New("database error")
		batchErr := NewBatchError(ErrorCodeInternal, "batch failed", cause)
		batchErr.AddLocation("row 1", "error 1")
		batchErr.AddLocation("row 2", "error 2")

		jsonData, err := json.Marshal(batchErr)

		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonData, &result)
		require.NoError(t, err)

		assert.Equal(t, "internal", result["code"])
		assert.Equal(t, "batch failed", result["message"])
		assert.Equal(t, "database error", result["cause"])

		locations, ok := result["locations"].([]interface{})
		require.True(t, ok)
		assert.Len(t, locations, 2)
	})

	t.Run("should produce valid JSON structure", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "test error", nil)
		batchErr.AddLocation("cell A1", "value required")

		jsonData, err := batchErr.MarshalJSON()

		require.NoError(t, err)
		assert.True(t, json.Valid(jsonData))
	})

	t.Run("should handle empty locations array correctly", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "test", nil)

		jsonData, err := json.Marshal(batchErr)

		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonData, &result)
		require.NoError(t, err)

		// Empty locations should not be included in JSON
		assert.NotContains(t, result, "locations")
	})

	t.Run("should handle special characters in location messages", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "test", nil)
		batchErr.AddLocation("row \"1\"", "value contains: \n\t special chars")

		jsonData, err := json.Marshal(batchErr)

		require.NoError(t, err)
		assert.True(t, json.Valid(jsonData))

		var result map[string]interface{}
		err = json.Unmarshal(jsonData, &result)
		require.NoError(t, err)

		locations := result["locations"].([]interface{})
		loc1 := locations[0].(map[string]interface{})
		assert.Equal(t, "row \"1\"", loc1["location"])
		assert.Equal(t, "value contains: \n\t special chars", loc1["message"])
	})
}

func TestBatchErrorLocation(t *testing.T) {
	t.Run("should create BatchErrorLocation with location and message", func(t *testing.T) {
		location := BatchErrorLocation{
			Location: "row 1",
			Message:  "test message",
		}

		assert.Equal(t, "row 1", location.Location)
		assert.Equal(t, "test message", location.Message)
	})
}

func TestBatchError_Integration(t *testing.T) {
	t.Run("should work with error interface", func(t *testing.T) {
		var err error
		batchErr := NewBatchError(ErrorCodeValidation, "validation failed", nil)
		batchErr.AddLocation("row 1", "invalid data")

		err = batchErr

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "validation failed")
		assert.Contains(t, err.Error(), "row 1")
	})

	t.Run("should be type-assertable from error interface", func(t *testing.T) {
		var err error = NewBatchError(ErrorCodeValidation, "test", nil)

		batchErr, ok := err.(*BatchError)

		require.True(t, ok)
		assert.Equal(t, ErrorCodeValidation, batchErr.Code)
		assert.Equal(t, "test", batchErr.Message)
	})

	t.Run("should maintain AppError functionality", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "test", nil)

		// Should have embedded AppError fields
		assert.NotNil(t, batchErr.AppError)
		assert.Equal(t, ErrorCodeValidation, batchErr.AppError.Code)
		assert.Equal(t, "test", batchErr.AppError.Message)
	})
}

func TestNewAppErrorCapturesStack(t *testing.T) {
	appErr := NewAppError(ErrorCodeInternal, "boom", nil)

	require.NotNil(t, appErr)
	assert.NotEmpty(t, appErr.Stack, "NewAppError should capture a stack trace")
	// Stack should reference this test function (the construction call site),
	// confirming constructor frames were skipped.
	assert.Contains(t, appErr.Stack, "TestNewAppErrorCapturesStack")
}

func TestNewBatchErrorCapturesStack(t *testing.T) {
	batchErr := NewBatchError(ErrorCodeValidation, "boom", nil)

	require.NotNil(t, batchErr)
	assert.NotEmpty(t, batchErr.Stack, "NewBatchError should capture a stack trace")
	assert.Contains(t, batchErr.Stack, "TestNewBatchErrorCapturesStack")
}

func TestStackNeverInMarshalJSON(t *testing.T) {
	t.Run("AppError MarshalJSON omits stack", func(t *testing.T) {
		appErr := NewAppError(ErrorCodeInternal, "boom", errors.New("cause"))
		require.NotEmpty(t, appErr.Stack)

		data, err := json.Marshal(appErr)
		require.NoError(t, err)

		assert.NotContains(t, string(data), "stack")
		assert.NotContains(t, string(data), appErr.Stack)

		var obj map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &obj))
		_, hasStack := obj["Stack"]
		assert.False(t, hasStack)
		_, hasStackTrace := obj["stack_trace"]
		assert.False(t, hasStackTrace)
	})

	t.Run("BatchError MarshalJSON omits stack", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "boom", errors.New("cause"))
		batchErr.AddLocation("row 1", "bad")
		require.NotEmpty(t, batchErr.Stack)

		data, err := json.Marshal(batchErr)
		require.NoError(t, err)

		assert.NotContains(t, string(data), "stack")
		assert.NotContains(t, string(data), batchErr.Stack)
	})
}

// TestMarshalJSONKey covers the issue #42 additive "key" field: it is emitted
// when MessageKey is set and omitted otherwise, for both AppError and BatchError.
func TestMarshalJSONKey(t *testing.T) {
	t.Run("AppError emits key when MessageKey set", func(t *testing.T) {
		appErr := NewAppError(ErrorCodeValidation, "msg", nil)
		appErr.MessageKey = ErrKeyReconRowLabelRequired

		data, err := json.Marshal(appErr)
		require.NoError(t, err)
		var obj map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &obj))
		assert.Equal(t, ErrKeyReconRowLabelRequired, obj["key"])
	})

	t.Run("AppError omits key when MessageKey empty", func(t *testing.T) {
		appErr := NewAppError(ErrorCodeValidation, "msg", nil)

		data, err := json.Marshal(appErr)
		require.NoError(t, err)
		var obj map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &obj))
		_, hasKey := obj["key"]
		assert.False(t, hasKey)
	})

	t.Run("BatchError emits key when MessageKey set", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "msg", nil)
		batchErr.MessageKey = ErrKeyReconItemLabelConflict

		data, err := batchErr.MarshalJSON()
		require.NoError(t, err)
		var obj map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &obj))
		assert.Equal(t, ErrKeyReconItemLabelConflict, obj["key"])
	})
}

func TestStackTraceHelper(t *testing.T) {
	t.Run("returns AppError stack directly", func(t *testing.T) {
		appErr := NewAppError(ErrorCodeInternal, "boom", nil)
		got := StackTrace(appErr)
		assert.Equal(t, appErr.Stack, got)
		assert.Contains(t, got, "TestStackTraceHelper")
	})

	t.Run("unwraps %w-wrapped AppError", func(t *testing.T) {
		appErr := NewAppError(ErrorCodeInternal, "boom", nil)
		wrapped := fmt.Errorf("context: %w", appErr)
		got := StackTrace(wrapped)
		assert.Equal(t, appErr.Stack, got)
	})

	t.Run("returns BatchError's embedded creation stack", func(t *testing.T) {
		// errors.As(&appErr) does not reach the AppError embedded in
		// *BatchError; StackTrace must still return the captured creation stack
		// (not a fresh debug.Stack), via the stackCapturer interface.
		batchErr := NewBatchError(ErrorCodeValidation, "boom", nil)
		got := StackTrace(batchErr)
		assert.Equal(t, batchErr.Stack, got)
		assert.Contains(t, got, "TestStackTraceHelper")
	})

	t.Run("unwraps %w-wrapped BatchError", func(t *testing.T) {
		batchErr := NewBatchError(ErrorCodeValidation, "boom", nil)
		wrapped := fmt.Errorf("context: %w", batchErr)
		got := StackTrace(wrapped)
		assert.Equal(t, batchErr.Stack, got)
	})

	t.Run("falls back to debug.Stack for raw errors", func(t *testing.T) {
		raw := errors.New("raw error")
		got := StackTrace(raw)
		assert.NotEmpty(t, got)
		// debug.Stack output for the current goroutine references this test.
		assert.Contains(t, got, "TestStackTraceHelper")
	})
}
