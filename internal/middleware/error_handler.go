package middleware

import (
	"cim-backend/pkg"
	"cim-backend/pkg/log"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
)

// RecoverMiddleware returns the panic-recovery middleware. Recovered panics are
// logged exactly once through logrus (pkg/log) with a "stack_trace" field in
// every environment. The LogErrorFunc returns nil so Echo does NOT also invoke
// the centralized HTTPErrorHandler, avoiding a duplicate log line, and writes
// the 500 response itself.
func RecoverMiddleware() echo.MiddlewareFunc {
	return echoMiddleware.RecoverWithConfig(echoMiddleware.RecoverConfig{
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			log.WithFields(logrus.Fields{
				"method":      c.Request().Method,
				"path":        c.Request().URL.Path,
				"stack_trace": string(stack),
			}).WithError(err).Error("panic recovered")

			if !c.Response().Committed {
				_ = c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "Internal server error",
				})
			}
			return nil
		},
	})
}

// CustomErrorHandler is Echo's custom error handler function
func CustomErrorHandler(err error, c echo.Context) {
	// Check if response has already been sent
	if c.Response().Committed {
		return
	}

	// Use our existing HandleError function
	if handlerErr := HandleError(c, err); handlerErr != nil {
		// If HandleError itself returns an error, fall back to basic error handling
		log.WithFields(logrus.Fields{
			"stack_trace": pkg.StackTrace(handlerErr),
		}).WithError(handlerErr).Error("error in error handler")
		_ = c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Internal server error",
		})
	}
}

// HandleError handles different types of errors and returns appropriate responses
func HandleError(c echo.Context, err error) error {
	// Check if it's already an HTTP error (already handled)
	if httpErr, ok := err.(*echo.HTTPError); ok {
		return httpErr
	}

	method := c.Request().Method
	path := c.Request().URL.Path

	// Check if it's a BatchError first (before AppError, since BatchError embeds AppError)
	var batchErr *pkg.BatchError
	if errors.As(err, &batchErr) {
		// Log the batch error with context and stack at Error level. Stacks are
		// emitted in every mode (not dev-gated). We log at Error (rather than
		// tiering 4xx down to Warn) so the stack is never suppressed by the
		// default LOG_LEVEL=error threshold -- handled errors must ALWAYS emit a
		// stack per the logging contract; only the output FORMAT is env-gated.
		//
		// We deliberately do NOT attach the error via .WithError(batchErr):
		// BatchError.Error() expands every Locations entry, so a batch with many
		// invalid rows would produce an unbounded error-level log line that leaks
		// per-row validation details and pressures log ingestion. The structured
		// fields below already carry the bounded summary (code, message, status,
		// locations COUNT, stack), so the record stays bounded regardless of
		// len(Locations).
		log.WithFields(logrus.Fields{
			"error_code":  batchErr.Code.String(),
			"error":       batchErr.Message,
			"method":      method,
			"path":        path,
			"http_status": batchErr.HTTPStatus(),
			"locations":   len(batchErr.Locations),
			"stack_trace": pkg.StackTrace(batchErr),
		}).Error("batch error")

		// Return structured JSON response with locations
		return c.JSON(batchErr.HTTPStatus(), batchErr)
	}

	// Check if it's our custom AppError
	var appErr *pkg.AppError
	if errors.As(err, &appErr) {
		// Log the actual error with context and stack at Error level (see the
		// BatchError branch above for why we do not tier 4xx down to Warn).
		log.WithFields(logrus.Fields{
			"error_code":  appErr.Code.String(),
			"method":      method,
			"path":        path,
			"http_status": appErr.HTTPStatus(),
			"stack_trace": pkg.StackTrace(appErr),
		}).WithError(appErr).Error("app error")

		// Return the display message to client
		return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
			"message": appErr.Message,
			"code":    appErr.Code.String(),
		})
	}

	// Handle other (unknown) errors as internal server errors. Always logged at
	// Error with a stack, in every mode.
	log.WithFields(logrus.Fields{
		"method":      method,
		"path":        path,
		"stack_trace": pkg.StackTrace(err),
	}).WithError(err).Error("internal error")

	return c.JSON(http.StatusInternalServerError, map[string]string{
		"error": "Internal server error",
	})
}

// WrapError wraps a regular error into an AppError
func WrapError(code pkg.ErrorCode, message string, cause error) *pkg.AppError {
	return pkg.NewAppError(code, message, cause)
}
