package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
)

// TestCORSPreflightAllowsIdempotencyKey guards against regressing the browser PO
// receive preflight (#132): the CORS allow-list must include Idempotency-Key.
func TestCORSPreflightAllowsIdempotencyKey(t *testing.T) {
	const origin = "https://app.example.com"

	e := echo.New()
	e.Use(echoMiddleware.CORSWithConfig(echoMiddleware.CORSConfig{
		AllowOrigins: []string{origin},
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
		AllowHeaders: corsAllowHeaders,
	}))
	e.PUT("/api/v1/purchase-orders/:id/receive", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/purchase-orders/1/receive", nil)
	req.Header.Set(echo.HeaderOrigin, origin)
	req.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodPut)
	req.Header.Set(echo.HeaderAccessControlRequestHeaders, "idempotency-key")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Contains(t, rec.Header().Get(echo.HeaderAccessControlAllowHeaders), "Idempotency-Key")
}
