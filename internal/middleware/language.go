package middleware

import (
	"cim-backend/pkg"

	"github.com/labstack/echo/v4"
)

// LanguageMiddleware extracts the language from Accept-Language header
// and stores it in the request context for later retrieval.
func LanguageMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get language from Accept-Language header
			lang := pkg.GetLanguage(c.Request())

			// Store language in request context
			reqCtx := c.Request().Context()
			reqCtx = pkg.WithLanguage(reqCtx, lang)
			c.SetRequest(c.Request().WithContext(reqCtx))

			return next(c)
		}
	}
}
