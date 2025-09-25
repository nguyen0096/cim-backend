package middleware

import (
	"errors"
	"import-export-backend/pkg"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// CustomErrorHandler is Echo's custom error handler function
func CustomErrorHandler(err error, c echo.Context) {
	// Check if response has already been sent
	if c.Response().Committed {
		return
	}

	// Use our existing HandleError function
	if handlerErr := HandleError(c, err); handlerErr != nil {
		// If HandleError itself returns an error, fall back to basic error handling
		log.Printf("Error in error handler: %v", handlerErr)
		c.JSON(http.StatusInternalServerError, map[string]string{
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

	// Check if it's our custom AppError
	var appErr *pkg.AppError
	if errors.As(err, &appErr) {
		// Log the actual error with context
		log.Printf("AppError [%s] in %s %s: %v",
			appErr.Code.String(),
			c.Request().Method,
			c.Request().URL.Path,
			appErr.Error())

		// Return the display message to client
		return c.JSON(appErr.HTTPStatus(), map[string]interface{}{
			"error": appErr.Message,
			"code":  appErr.Code.String(),
		})
	}

	// Handle other errors as internal server errors
	log.Printf("Internal error in %s %s: %v",
		c.Request().Method,
		c.Request().URL.Path,
		err)

	return c.JSON(http.StatusInternalServerError, map[string]string{
		"error": "Internal server error",
	})
}

// WrapError wraps a regular error into an AppError
func WrapError(code pkg.ErrorCode, message string, cause error) *pkg.AppError {
	return pkg.NewAppError(code, message, cause)
}
