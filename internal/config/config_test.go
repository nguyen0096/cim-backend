package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeExportPrefix(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"exports", "exports"},
		{"/exports", "exports"},
		{"exports/", "exports"},
		{"/exports/", "exports"},
		{"  reports/cim  ", "reports/cim"},
		{"", "exports"},    // empty -> default
		{"   ", "exports"}, // whitespace -> default
		{"/", "exports"},   // only slashes -> default
		{"///", "exports"}, // only slashes -> default
		{"a/b/c", "a/b/c"}, // nested preserved
		// Path-traversal / relative segments are dropped so keys can never
		// regress toward or above the bucket root.
		{"../exports", "exports"},
		{"..", "exports"},
		{"../..", "exports"},
		{"\\exports\\", "exports"},
		{"reports/../cim", "reports/cim"},
		{"reports/./cim", "reports/cim"},
		{"a//b", "a/b"}, // empty interior segment dropped
	}
	for _, c := range cases {
		assert.Equal(t, c.want, normalizeExportPrefix(c.in), "input %q", c.in)
	}
}

func TestLoad_ExportPrefixDefault(t *testing.T) {
	t.Setenv("R2_EXPORT_PREFIX", "")
	cfg := Load()
	assert.Equal(t, "exports", cfg.R2.ExportPrefix)
}

func TestLoad_ExportPrefixOverrideAndTrim(t *testing.T) {
	t.Setenv("R2_EXPORT_PREFIX", "/custom/exports/")
	cfg := Load()
	assert.Equal(t, "custom/exports", cfg.R2.ExportPrefix)
}
