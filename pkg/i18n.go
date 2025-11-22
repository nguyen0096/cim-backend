package pkg

import (
	"context"
	"net/http"
	"strings"
)

// Language codes
const (
	LangEN = "en"
	LangVI = "vi"
)

// languageContextKeyType is a custom type for context key to avoid collisions
type languageContextKeyType string

// LanguageContextKey is the context key for storing language preference
const LanguageContextKey languageContextKeyType = "language"

// GetLanguage parses the Accept-Language header and returns the preferred language code.
// Returns "en" (default) or "vi". Supports full locale codes like "vi-VN" -> "vi".
func GetLanguage(r *http.Request) string {
	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang == "" {
		return LangEN
	}

	// Parse Accept-Language header (e.g., "en-US,en;q=0.9,vi;q=0.8")
	// Split by comma and process each language preference
	languages := strings.Split(acceptLang, ",")

	for _, lang := range languages {
		// Remove quality values (e.g., ";q=0.9")
		lang = strings.TrimSpace(strings.Split(lang, ";")[0])
		// Extract base language code (e.g., "vi-VN" -> "vi")
		lang = strings.ToLower(strings.Split(lang, "-")[0])

		if lang == LangVI {
			return LangVI
		}
		if lang == LangEN {
			return LangEN
		}
	}

	// Default to English if no supported language found
	return LangEN
}

// GetLanguageFromContext retrieves the language from context.
// Returns the language if found, otherwise returns default language (LangEN).
func GetLanguageFromContext(ctx context.Context) string {
	if lang, ok := ctx.Value(LanguageContextKey).(string); ok && lang != "" {
		return lang
	}
	return LangEN
}

// WithLanguage stores the language in the context and returns a new context.
func WithLanguage(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, LanguageContextKey, lang)
}
