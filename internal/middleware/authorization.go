package middleware

import (
	"fmt"
	"import-export-backend/internal/auth"
	"import-export-backend/internal/repository"
	"import-export-backend/pkg"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// AuthorizationMiddleware creates middleware for role-based access control
func AuthorizationMiddleware(casbinService *auth.CasbinService, userRepo *repository.UserRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get user ID from context (set by AuthMiddleware)
			userID, _ := c.Get(pkg.AuthContextKeyUserID).(string)
			if userID == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "User ID not found",
				})
			}

			// Check if user role already exists in context (skip database query if available)
			userRole, exists := c.Get("user_role").(string)
			if !exists || userRole == "" {
				// Fetch user role from database using Firebase UID
				user, err := userRepo.GetByUID(c.Request().Context(), userID)
				if err != nil {
					fmt.Printf("Error fetching user from database: %v\n", err)
					return c.JSON(http.StatusInternalServerError, map[string]string{
						"error": "Failed to fetch user information",
					})
				}

				// Set user role (default to staff if user not found)
				userRole = "staff"
				if user != nil {
					userRole = user.Role
				}
			}

			// Extract resource and action from the request
			resource, action := extractResourceAndAction(c)
			if resource == "" || action == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": "Unable to determine resource or action",
				})
			}

			// Check authorization using Casbin
			allowed, err := casbinService.Enforce(userRole, resource, action)
			if err != nil {
				fmt.Printf("Authorization error for user %s: %v\n", userID, err)
				return c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "Authorization check failed",
				})
			}

			if !allowed {
				fmt.Printf("Access denied for user %s (role: %s) to %s %s\n", userID, userRole, action, resource)
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": fmt.Sprintf("Access denied: %s role cannot %s %s", userRole, action, resource),
				})
			}

			// Set user role in context for handlers that might need it
			c.Set("user_role", userRole)

			// Authorization successful, continue to handler
			return next(c)
		}
	}
}

// extractResourceAndAction extracts the resource and action from the HTTP request
func extractResourceAndAction(c echo.Context) (string, string) {
	path := c.Request().URL.Path
	method := c.Request().Method

	// Remove API prefix
	path = strings.TrimPrefix(path, "/api/v1")

	// Map HTTP methods to actions
	action := methodToAction(method)
	if action == "" {
		return "", ""
	}

	// Extract resource from path
	resource := pathToResource(path)
	if resource == "" {
		return "", ""
	}

	return resource, action
}

// methodToAction maps HTTP methods to Casbin actions
func methodToAction(method string) string {
	switch method {
	case "GET":
		return "view"
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	default:
		return ""
	}
}

// pathToResource maps URL paths to Casbin resources
func pathToResource(path string) string {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	// Split path into segments
	segments := strings.Split(path, "/")
	if len(segments) == 0 {
		return ""
	}

	// Handle special cases for nested routes
	resource := segments[0]

	// Handle specific resource mappings
	switch resource {
	case "products":
		return "products"
	case "suppliers":
		return "suppliers"
	case "inventories", "inventory-items":
		return "inventories"
	case "purchase-orders":
		return "purchase_orders"
	case "excel":
		return "excel"
	case "settings":
		return "settings"
	case "reports":
		// Reports are read-only, map to prices for pricing reports
		if len(segments) > 1 && segments[1] == "purchase-summary" {
			return "prices"
		}
		return "prices"
	default:
		return resource
	}
}

// RequirePermission is a helper function to check permissions in handlers
func RequirePermission(casbinService *auth.CasbinService, userRole, resource, action string) error {
	allowed, err := casbinService.Enforce(userRole, resource, action)
	if err != nil {
		return fmt.Errorf("authorization check failed: %w", err)
	}
	if !allowed {
		return fmt.Errorf("access denied: %s role cannot %s %s", userRole, action, resource)
	}
	return nil
}
