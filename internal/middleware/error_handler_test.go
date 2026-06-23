package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"cim-backend/pkg"
	"cim-backend/pkg/log"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLogs swaps the package logger output to a buffer for the duration of
// the test and restores it afterwards.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := log.Logger
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetFormatter(&logrus.JSONFormatter{})
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	log.Logger = logger
	t.Cleanup(func() { log.Logger = prev })
	return &buf
}

func newContext() (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/things", nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestHandleErrorLogsStackTrace(t *testing.T) {
	t.Run("AppError (5xx) logs stack_trace at error", func(t *testing.T) {
		buf := captureLogs(t)
		c, rec := newContext()

		err := pkg.NewAppError(pkg.ErrorCodeInternal, "kaboom", errors.New("root"))
		require.NoError(t, HandleError(c, err))

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		entry := lastLogEntry(t, buf)
		assert.Equal(t, "error", entry["level"])
		assert.Contains(t, entry, "stack_trace")
		assert.NotEmpty(t, entry["stack_trace"])
		// Client body must not leak the stack.
		assert.NotContains(t, rec.Body.String(), "stack")
	})

	// Regression guard for the "handled 4xx errors must always emit a stack"
	// contract: the default LOG_LEVEL is "error" (see internal/config), so a 4xx
	// logged at Warn would be silently dropped in prod. Pin the logger to Error
	// level and assert the record is still emitted with a stack at error level.
	t.Run("AppError (4xx) still logs stack_trace at default error level", func(t *testing.T) {
		buf := captureLogs(t)
		log.Logger.SetLevel(logrus.ErrorLevel)
		c, rec := newContext()

		err := pkg.NewAppError(pkg.ErrorCodeValidation, "bad input", nil)
		require.NoError(t, HandleError(c, err))

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		entry := lastLogEntry(t, buf)
		assert.Equal(t, "error", entry["level"])
		assert.Contains(t, entry, "stack_trace")
		assert.NotEmpty(t, entry["stack_trace"])
	})

	t.Run("AppError with MessageKey is localized per request language", func(t *testing.T) {
		newLangContext := func(lang string) (echo.Context, *httptest.ResponseRecorder) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/inventories/7/reconcile/initiate", nil)
			req = req.WithContext(pkg.WithLanguage(req.Context(), lang))
			rec := httptest.NewRecorder()
			return e.NewContext(req, rec), rec
		}

		readBody := func(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
			var body map[string]interface{}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			return body
		}

		// The domain conflict carries a MessageKey (not a fixed string); the
		// handler resolves it to the request language at write time.
		cEN, recEN := newLangContext(pkg.LangEN)
		require.NoError(t, HandleError(cEN, pkg.ErrActivePendingReconcileConflict(7, nil)))
		assert.Equal(t, http.StatusConflict, recEN.Code)
		bodyEN := readBody(t, recEN)
		assert.Equal(t, pkg.ErrorCodeActivePendingReconcileConflict.String(), bodyEN["code"])
		assert.Equal(t,
			fmt.Sprintf(pkg.GetErrorMessageByLang(pkg.ErrKeyActivePendingReconcileConflict, pkg.LangEN), 7),
			bodyEN["message"])

		cVI, recVI := newLangContext(pkg.LangVI)
		require.NoError(t, HandleError(cVI, pkg.ErrActivePendingReconcileConflict(7, nil)))
		bodyVI := readBody(t, recVI)
		assert.NotEqual(t, bodyEN["message"], bodyVI["message"], "VI response must differ from EN")
		assert.Contains(t, bodyVI["message"], "7")
	})

	t.Run("BatchError logs the captured creation stack at error", func(t *testing.T) {
		buf := captureLogs(t)
		log.Logger.SetLevel(logrus.ErrorLevel)
		c, _ := newContext()

		batchErr := pkg.NewBatchError(pkg.ErrorCodeValidation, "batch failed", nil)
		batchErr.AddLocation("row 1", "bad")
		require.NotEmpty(t, batchErr.Stack)
		require.NoError(t, HandleError(c, batchErr))

		entry := lastLogEntry(t, buf)
		assert.Equal(t, "error", entry["level"])
		assert.Contains(t, entry, "stack_trace")
		// Must be the stack captured at NewBatchError (origin), not a
		// debug.Stack() taken inside the handler. Guards the embedded-AppError
		// errors.As regression.
		assert.Equal(t, batchErr.Stack, entry["stack_trace"])
	})

	t.Run("BatchError log is bounded and does not expand per-row Locations", func(t *testing.T) {
		buf := captureLogs(t)
		log.Logger.SetLevel(logrus.ErrorLevel)
		c, _ := newContext()

		batchErr := pkg.NewBatchError(pkg.ErrorCodeValidation, "batch failed", nil)
		// Many locations with a recognizable, sensitive per-row marker.
		const rowMarker = "SENSITIVE_ROW_DETAIL"
		const numLocations = 500
		for i := 0; i < numLocations; i++ {
			batchErr.AddLocation(
				fmt.Sprintf("row %d", i),
				fmt.Sprintf("%s value=%d", rowMarker, i),
			)
		}
		require.NoError(t, HandleError(c, batchErr))

		entry := lastLogEntry(t, buf)
		assert.Equal(t, "error", entry["level"])

		// Structured fields from #57 must still be present.
		assert.Equal(t, pkg.ErrorCodeValidation.String(), entry["error_code"])
		assert.Equal(t, float64(http.StatusBadRequest), entry["http_status"])
		assert.Equal(t, float64(numLocations), entry["locations"])
		assert.Contains(t, entry, "stack_trace")
		assert.NotEmpty(t, entry["stack_trace"])
		assert.Equal(t, batchErr.Stack, entry["stack_trace"])

		// The per-row expansion produced by BatchError.Error() must NOT leak into
		// the log: not the row markers, not the "Locations:" header that Error()
		// emits. The whole captured output is checked (not just the structured
		// fields) to catch any field that serializes the full error string.
		raw := buf.String()
		assert.NotContains(t, raw, rowMarker,
			"per-row Locations detail leaked into the bounded batch-error log")
		assert.NotContains(t, raw, "Locations:",
			"BatchError.Error() per-row expansion leaked into the log")
	})

	t.Run("unknown error logs stack_trace at error", func(t *testing.T) {
		buf := captureLogs(t)
		c, rec := newContext()

		require.NoError(t, HandleError(c, errors.New("some raw error")))

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		entry := lastLogEntry(t, buf)
		assert.Equal(t, "error", entry["level"])
		assert.Contains(t, entry, "stack_trace")
		assert.NotEmpty(t, entry["stack_trace"])
	})
}

func lastLogEntry(t *testing.T, buf *bytes.Buffer) map[string]interface{} {
	t.Helper()
	out := bytes.TrimSpace(buf.Bytes())
	require.NotEmpty(t, out, "expected at least one log line")
	lines := bytes.Split(out, []byte("\n"))
	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(lines[len(lines)-1], &entry))
	return entry
}
