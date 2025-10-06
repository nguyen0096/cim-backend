package middleware

import (
	"context"
	"fmt"
	"import-export-backend/internal/auth"
	"import-export-backend/pkg"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// AuthMiddleware validates Firebase ID tokens
func AuthMiddleware(firebaseAuth *auth.FirebaseAuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authorization header required"})
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Bearer token required"})
			}

			// Verify Firebase ID token
			ctx := context.Background()
			token, err := firebaseAuth.VerifyToken(ctx, tokenString)
			if err != nil {
				fmt.Println("Error verifying token:", err)
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
			}

			// Extract user information from Firebase token and set in both echo and request context
			c.Set(pkg.AuthContextKeyUserID, token.UID)
			c.Set(pkg.AuthContextKeyUserEmail, token.Claims["email"])

			// Add user information to the request context for GORM hooks and other services
			reqCtx := c.Request().Context()
			reqCtx = context.WithValue(reqCtx, pkg.AuthContextKeyUserID, token.UID)
			if email, ok := token.Claims["email"].(string); ok {
				reqCtx = pkg.WithUserEmail(reqCtx, email)
			}

			// Set additional Firebase claims
			if name, ok := token.Claims["name"].(string); ok {
				c.Set("user_name", name)
				reqCtx = context.WithValue(reqCtx, pkg.AuthContextKeyUserName, name)
			}

			c.SetRequest(c.Request().WithContext(reqCtx))

			return next(c)
		}
	}
}
