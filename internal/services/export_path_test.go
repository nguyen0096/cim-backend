package services

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildExportObjectKey_PrefixAppliedAndSegregated(t *testing.T) {
	at := time.Date(2026, 4, 15, 14, 30, 5, 0, time.UTC)
	key := BuildExportObjectKey("exports", 10, at, "Kho A", "20260401", "20260430")

	assert.True(t, strings.HasPrefix(key, "exports/inventory/10/"),
		"key %q must carry prefix + per-inventory path", key)
	assert.Contains(t, key, "20260415-143005-")
	assert.Contains(t, key, "-kho-a-")
	assert.Contains(t, key, "-20260401-20260430-")
	assert.True(t, strings.HasSuffix(key, ".xlsx"))
}

func TestBuildExportObjectKey_PrefixIsConfigurable(t *testing.T) {
	at := time.Date(2026, 4, 15, 14, 30, 5, 0, time.UTC)
	key := BuildExportObjectKey("reports/cim", 7, at, "Warehouse", "20260101", "20260131")
	assert.True(t, strings.HasPrefix(key, "reports/cim/inventory/7/"), key)
}

func TestBuildExportObjectKey_TrimsSlashesAndDefaults(t *testing.T) {
	at := time.Date(2026, 4, 15, 14, 30, 5, 0, time.UTC)

	// Surrounding slashes are trimmed so the key has no double-slash / bare root.
	key := BuildExportObjectKey("/exports/", 1, at, "X", "a", "b")
	assert.True(t, strings.HasPrefix(key, "exports/inventory/1/"), key)
	assert.False(t, strings.Contains(key, "//"), "no double slashes: %q", key)

	// Empty / whitespace prefix falls back to the default; never bare root.
	for _, p := range []string{"", "   ", "/", "///"} {
		k := BuildExportObjectKey(p, 1, at, "X", "a", "b")
		assert.True(t, strings.HasPrefix(k, "exports/inventory/1/"), "prefix %q -> %q", p, k)
		assert.NotEqual(t, byte('/'), k[0], "must never start at bucket root: %q", k)
	}
}

func TestBuildExportObjectKey_RejectsTraversalPrefix(t *testing.T) {
	at := time.Date(2026, 4, 15, 14, 30, 5, 0, time.UTC)

	// A path-traversal / relative prefix must never survive into the key.
	for _, p := range []string{"../exports", "..", "../..", "\\exports\\", "reports/../cim"} {
		k := BuildExportObjectKey(p, 1, at, "X", "a", "b")
		assert.False(t, strings.Contains(k, ".."), "no traversal for prefix %q -> %q", p, k)
		assert.False(t, strings.Contains(k, "\\"), "no backslash for prefix %q -> %q", p, k)
		assert.False(t, strings.Contains(k, "//"), "no double slash for prefix %q -> %q", p, k)
		assert.NotEqual(t, byte('/'), k[0], "must never start at bucket root: %q", k)
	}

	// reports/../cim collapses to reports/cim.
	k := BuildExportObjectKey("reports/../cim", 1, at, "X", "a", "b")
	assert.True(t, strings.HasPrefix(k, "reports/cim/inventory/1/"), k)
}

func TestBuildExportObjectKey_SanitizesName(t *testing.T) {
	at := time.Date(2026, 4, 15, 14, 30, 5, 0, time.UTC)

	// Spaces -> dashes, path separators / traversal / unicode stripped.
	key := BuildExportObjectKey("exports", 3, at, "../../etc passwd \\ Kho Ä", "f", "t")
	// No path-injection survives into the name segment.
	assert.False(t, strings.Contains(key, ".."), "no traversal: %q", key)
	assert.False(t, strings.Contains(key, "\\"), "no backslash: %q", key)
	// The name still begins under the inventory path (no extra "/" injected).
	assert.True(t, strings.HasPrefix(key, "exports/inventory/3/"), key)
	// Only one segment for inventory id (no slashes from the name leaked in).
	assert.Equal(t, 4, len(strings.Split(key, "/")), "unexpected path depth: %q", key)
}

func TestBuildExportObjectKey_EmptyNameFallback(t *testing.T) {
	at := time.Date(2026, 4, 15, 14, 30, 5, 0, time.UTC)
	// A name with no URL-safe characters sanitizes to "" → falls back.
	key := BuildExportObjectKey("exports", 9, at, "***", "20260401", "20260430")
	assert.Contains(t, key, "-"+emptyNameFallback+"-", "expected fallback name in %q", key)
}

func TestBuildExportObjectKey_UniquePerCall(t *testing.T) {
	at := time.Date(2026, 4, 15, 14, 30, 5, 0, time.UTC)
	k1 := BuildExportObjectKey("exports", 1, at, "X", "a", "b")
	k2 := BuildExportObjectKey("exports", 1, at, "X", "a", "b")
	assert.NotEqual(t, k1, k2, "uuid component must make keys unique")
}

func TestBuildExportFallbackKey(t *testing.T) {
	at := time.Date(2026, 4, 15, 14, 30, 5, 0, time.UTC)

	key := buildExportFallbackKey("exports", at)
	assert.True(t, strings.HasPrefix(key, "exports/inventory/2026/04/15/"), key)
	assert.True(t, strings.HasSuffix(key, ".xlsx"))

	// Default + trim behaviour, never bare root.
	for _, p := range []string{"", "/", "/exports/"} {
		k := buildExportFallbackKey(p, at)
		require.NotEmpty(t, k)
		assert.NotEqual(t, byte('/'), k[0], "prefix %q -> %q", p, k)
		assert.False(t, strings.Contains(k, "//"), "no double slash: %q", k)
	}
}
