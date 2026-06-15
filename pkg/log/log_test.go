package log

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitSetsReportCaller(t *testing.T) {
	logger, err := Init(Config{LogLevel: logrus.InfoLevel, Environment: "development"})
	require.NoError(t, err)
	assert.True(t, logger.ReportCaller, "Init should enable ReportCaller")
}

func TestInitFormatterByEnvironment(t *testing.T) {
	t.Run("local uses text formatter", func(t *testing.T) {
		logger, err := Init(Config{LogLevel: logrus.InfoLevel, Environment: "development"})
		require.NoError(t, err)
		_, ok := logger.Formatter.(*logrus.TextFormatter)
		assert.True(t, ok, "development should use TextFormatter")
	})

	t.Run("production uses JSON formatter", func(t *testing.T) {
		logger, err := Init(Config{LogLevel: logrus.InfoLevel, Environment: "production"})
		require.NoError(t, err)
		_, ok := logger.Formatter.(*logrus.JSONFormatter)
		assert.True(t, ok, "production should use JSONFormatter")
	})
}

// TestStackTraceFieldNotGatedByEnvironment proves the format differs by env but
// the stack_trace field content is emitted in BOTH modes.
func TestStackTraceFieldNotGatedByEnvironment(t *testing.T) {
	for _, env := range []string{"development", "production"} {
		t.Run(env, func(t *testing.T) {
			logger, err := Init(Config{LogLevel: logrus.InfoLevel, Environment: env})
			require.NoError(t, err)

			var buf bytes.Buffer
			logger.SetOutput(&buf)
			logger.WithField("stack_trace", "goroutine 1 [running]:\nmain.foo()").Error("boom")

			out := buf.String()
			assert.Contains(t, out, "stack_trace", "stack_trace must be present in %s", env)
			assert.Contains(t, out, "main.foo", "stack content must be present in %s", env)

			// In production it should be valid JSON containing the field.
			if env == "production" {
				line := strings.TrimSpace(out)
				var obj map[string]interface{}
				require.NoError(t, json.Unmarshal([]byte(line), &obj))
				assert.Contains(t, obj, "stack_trace")
			}
		})
	}
}
