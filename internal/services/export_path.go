package services

import (
	"fmt"
	"strings"
	"time"

	excelpkg "cim-backend/internal/services/excel"

	"github.com/google/uuid"
)

// defaultExportPrefix is used when no prefix is configured. It mirrors the
// config default so the helper is safe to call even with an empty prefix.
const defaultExportPrefix = "exports"

// emptyNameFallback is the object-name segment used when the sanitized
// inventory/report name is empty (e.g. a name with no URL-safe characters).
const emptyNameFallback = "inventory"

// sanitizeExportPrefix normalizes a prefix into a safe object-key prefix:
// whitespace is trimmed, both "/" and "\" are treated as separators, and empty,
// ".", and ".." segments are dropped so a misconfigured/path-like value can
// never regress generated keys toward—or above—the bucket root. Falls back to
// the default when nothing safe remains. Mirrors config.normalizeExportPrefix
// as defense-in-depth for callers that pass a prefix directly.
func sanitizeExportPrefix(prefix string) string {
	prefix = strings.ReplaceAll(strings.TrimSpace(prefix), "\\", "/")
	segments := strings.Split(prefix, "/")
	safe := make([]string, 0, len(segments))
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" || seg == "." || seg == ".." {
			continue
		}
		safe = append(safe, seg)
	}
	if len(safe) == 0 {
		return defaultExportPrefix
	}
	return strings.Join(safe, "/")
}

// BuildExportObjectKey produces a server-derived, deterministic object key for
// an export file. The shape is:
//
//	<prefix>/inventory/<inventoryID>/<YYYYMMDD-HHMMSS>-<name>-<from>-<to>-<uniq>.xlsx
//
// e.g. exports/inventory/10/20260415-143005-kho-a-20260401-20260430-1a2b3c4d.xlsx
//
// The key is segregated per inventory, carries a human-meaningful object name
// reusing excelpkg.SanitizeFilenameSegment, and appends a full uuid component
// for collision safety. It never returns a bare-root key: the prefix is
// sanitized (slashes/traversal dropped) and defaults to "exports", and an empty
// name falls back to "inventory".
func BuildExportObjectKey(prefix string, inventoryID uint, generatedAt time.Time, nameSegment, from, to string) string {
	prefix = sanitizeExportPrefix(prefix)

	name := excelpkg.SanitizeFilenameSegment(nameSegment)
	if name == "" {
		name = emptyNameFallback
	}

	// Full uuid uniqueness component so concurrent exports of the same inventory
	// in the same second can't collide and silently overwrite each other on
	// S3/R2 PutObject.
	uniq := uuid.New().String()

	return fmt.Sprintf("%s/inventory/%d/%s-%s-%s-%s-%s.xlsx",
		prefix,
		inventoryID,
		generatedAt.Format("20060102-150405"),
		name,
		from,
		to,
		uniq,
	)
}

// buildExportFallbackKey produces a config-prefixed, date-bucketed export key
// for callers that lack inventory/period identity (e.g. the legacy
// PopulateExportURL path). Shape:
//
//	<prefix>/inventory/<YYYY/MM/DD>/<uuid>.xlsx
//
// It still honours the configured prefix and never lands at the bucket root.
func buildExportFallbackKey(prefix string, generatedAt time.Time) string {
	prefix = sanitizeExportPrefix(prefix)
	return fmt.Sprintf("%s/inventory/%04d/%02d/%02d/%s.xlsx",
		prefix,
		generatedAt.Year(), generatedAt.Month(), generatedAt.Day(),
		uuid.New().String(),
	)
}
