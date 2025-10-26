package pkg

import "testing"

// CleanUp is used as a defer function to clean up test data, even if test panics
// or fails.
func CleanUp(t *testing.T, fnc func() error) {
	if err := recover(); err != nil {
		t.Errorf("Test panic: %v", err)
	}

	if err := fnc(); err != nil {
		t.Errorf("Clean up failed: %v", err)
	}
}
