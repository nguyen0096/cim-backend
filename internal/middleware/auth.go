package middleware

import (
	"import-export-backend/internal/config"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// AuthMiddleware validates JWT tokens
func AuthMiddleware() echo.MiddlewareFunc {
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

			// Get JWT secret from config
			cfg := config.Load()
			secret := cfg.JWT.Secret

			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				// Validate signing method
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, echo.NewHTTPError(http.StatusUnauthorized, "Invalid signing method")
				}
				return []byte(secret), nil
			})

			if err != nil || !token.Valid {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
			}

			// Extract claims
			if claims, ok := token.Claims.(jwt.MapClaims); ok {
				c.Set("user_id", claims["user_id"])
				c.Set("user_role", claims["role"])
				c.Set("user_email", claims["email"])
			}

			return next(c)
		}
	}
}

// AdminOnlyMiddleware restricts access to admin users only
func AdminOnlyMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role := c.Get("user_role")
			if role != "admin" {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "Admin access required"})
			}
			return next(c)
		}
	}
}
