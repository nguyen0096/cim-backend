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
// logged once via pkg/log with a "stack_trace" field; it writes the 500 itself and
// returns nil so Echo does not also log through HTTPErrorHandler.
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
	if c.Response().Committed {
		return
	}

	if handlerErr := HandleError(c, err); handlerErr != nil {
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
	if httpErr, ok := err.(*echo.HTTPError); ok {
		return httpErr
	}

	method := c.Request().Method
	path := c.Request().URL.Path

	// BatchError before AppError (BatchError embeds AppError).
	var batchErr *pkg.BatchError
	if errors.As(err, &batchErr) {
		// Log at Error so the stack is never suppressed by LOG_LEVEL=error. Do not
		// attach batchErr via WithError: BatchError.Error() expands every Locations
		// entry, so log only the bounded fields (locations count).
		log.WithFields(logrus.Fields{
			"error_code":  batchErr.Code.String(),
			"error":       batchErr.Message,
			"method":      method,
			"path":        path,
			"http_status": batchErr.HTTPStatus(),
			"locations":   len(batchErr.Locations),
			"stack_trace": pkg.StackTrace(batchErr),
		}).Error("batch error")

		return c.JSON(batchErr.HTTPStatus(), batchErr)
	}

	var appErr *pkg.AppError
	if errors.As(err, &appErr) {
		// Log at Error (see BatchError branch for why 4xx is not tiered to Warn).
		log.WithFields(logrus.Fields{
			"error_code":  appErr.Code.String(),
			"method":      method,
			"path":        path,
			"http_status": appErr.HTTPStatus(),
			"stack_trace": pkg.StackTrace(appErr),
		}).WithError(appErr).Error("app error")

		// Localize per request language; falls back to appErr.Message when no MessageKey.
		body := map[string]interface{}{
			"message": appErr.LocalizedMessage(c.Request().Context()),
			"code":    appErr.Code.String(),
		}
		// Expose the stable catalog key so the frontend can route the error without
		// parsing the localized message.
		if appErr.MessageKey != "" {
			body["key"] = appErr.MessageKey
		}
		return c.JSON(appErr.HTTPStatus(), body)
	}

	// Unknown errors -> 500, logged at Error with a stack.
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
