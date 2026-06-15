package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newEchoWithRecover wires the real RecoverMiddleware + CustomErrorHandler, the
// same way SetupServer does, for integration-style assertions.
func newEchoWithRecover() *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = CustomErrorHandler
	e.Use(RecoverMiddleware())
	return e
}

func TestPanicRecoveryLogsSingleStackTrace(t *testing.T) {
	buf := captureLogs(t)

	e := newEchoWithRecover()
	e.GET("/boom", func(c echo.Context) error {
		panic("something exploded")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	lines := nonEmptyLines(buf)
	require.Len(t, lines, 1, "panic must be logged exactly once (no double-log)")

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(lines[0], &entry))
	assert.Equal(t, "error", entry["level"])
	assert.Contains(t, entry, "stack_trace")
	assert.NotEmpty(t, entry["stack_trace"])

	// Client must not see the stack.
	assert.NotContains(t, rec.Body.String(), "stack")
	assert.Contains(t, rec.Body.String(), "Internal server error")
}

// TestStackTraceEmittedInBothModes is the mode-independence proof: panic and
// unknown-error paths emit a stack_trace under ENV=development AND
// ENV=production. Stacks are NOT gated by mode.
func TestStackTraceEmittedInBothModes(t *testing.T) {
	for _, env := range []string{"development", "production"} {
		t.Run("env="+env, func(t *testing.T) {
			t.Setenv("ENV", env)

			t.Run("panic", func(t *testing.T) {
				buf := captureLogs(t)
				e := newEchoWithRecover()
				e.GET("/boom", func(c echo.Context) error { panic("boom") })

				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

				assert.Equal(t, http.StatusInternalServerError, rec.Code)
				entry := lastLogEntry(t, buf)
				assert.Contains(t, entry, "stack_trace")
				assert.NotEmpty(t, entry["stack_trace"])
			})

			t.Run("unknown error", func(t *testing.T) {
				buf := captureLogs(t)
				c, rec := newContext()
				require.NoError(t, HandleError(c, errAny("kaboom")))

				assert.Equal(t, http.StatusInternalServerError, rec.Code)
				entry := lastLogEntry(t, buf)
				assert.Contains(t, entry, "stack_trace")
				assert.NotEmpty(t, entry["stack_trace"])
			})
		})
	}
}

type errAny string

func (e errAny) Error() string { return string(e) }

func nonEmptyLines(buf *bytes.Buffer) [][]byte {
	raw := bytes.TrimSpace(buf.Bytes())
	if len(raw) == 0 {
		return nil
	}
	return bytes.Split(raw, []byte("\n"))
}
