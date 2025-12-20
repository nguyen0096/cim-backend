package middleware

import (
	"cim-backend/internal/auth"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// AuthorizationMiddleware creates middleware for role-based access control
func AuthorizationMiddleware(casbinService *auth.CasbinService, userService *services.UserService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get user ID from context (set by AuthMiddleware)
			userUID, _ := c.Get(pkg.AuthContextKeyUserID.String()).(string)
			if userUID == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "User ID not found",
				})
			}

			// Check if user role already exists in context (skip database query if available)
			userRole, exists := c.Get(pkg.AuthContextKeyUserRole.String()).(string)
			if !exists || userRole == "" {
				// Get user email from context
				userEmail, exists := c.Get(pkg.AuthContextKeyUserEmail.String()).(string)
				if !exists || userEmail == "" {
					return c.JSON(http.StatusUnauthorized, map[string]string{
						"error": "User email not found in token",
					})
				}

				// Fetch user role from database using email
				user, err := userService.GetUserByEmail(c.Request().Context(), userEmail)
				if err != nil {
					fmt.Printf("Error fetching user from database: %v\n", err)
					return c.JSON(http.StatusForbidden, map[string]string{
						"error":   "Failed to fetch user information",
						"details": err.Error(),
					})
				}

				switch user.Status {
				case "inactive":
					return c.JSON(http.StatusForbidden, map[string]string{
						"error": "User is inactive",
					})
				case "pending":
					return c.JSON(http.StatusForbidden, map[string]string{
						"error": "User is pending",
					})
				default:
					userRole = string(user.Role)
				}

				if user.UID != userUID {
					// Update UID and handle Casbin policy updates
					user.UID = userUID
					if err := userService.UpdateUser(c.Request().Context(), user.ID.String(), userUID, user.Name, string(user.Role), user.Status, true); err != nil {
						fmt.Printf("Error updating user UID: %v\n", err)
						// Continue anyway, this is not critical for authorization
					}
				}

				if roles, err := casbinService.GetRolesForUser(userUID); err == nil && len(roles) == 0 {
					if err := casbinService.AddRoleForUser(userUID, string(user.Role)); err != nil {
						fmt.Printf("Error adding role to user: %v\n", err)
						// Continue anyway, this is not critical for authorization
					}
				} else if err != nil {
					fmt.Printf("Error getting roles for user: %v\n", err)
					// Continue anyway, this is not critical for authorization
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
				fmt.Printf("Authorization error for user %s: %v\n", userUID, err)
				return c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "Authorization check failed",
				})
			}

			if !allowed {
				fmt.Printf("Access denied for user %s (role: %s) to %s %s\n", userUID, userRole, action, resource)
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": fmt.Sprintf("Access denied: %s role cannot %s %s", userRole, action, resource),
				})
			}

			reqCtx := c.Request().Context()
			reqCtx = context.WithValue(reqCtx, pkg.AuthContextKeyUserRole, userRole)

			// Get and set user permissions in context
			permissions, err := getUserPermissions(casbinService, userRole)
			if err != nil {
				fmt.Printf("Error getting user permissions: %v\n", err)
				// Continue anyway, permissions are optional for handlers
			} else {
				reqCtx = context.WithValue(reqCtx, pkg.AuthContextKeyUserPermissions, permissions)
			}

			c.SetRequest(c.Request().WithContext(reqCtx))

			// Authorization successful, continue to handler
			return next(c)
		}
	}
}

// RouteMapping defines custom resource and action mappings for specific routes
type RouteMapping struct {
	Method      string // HTTP method (GET, POST, etc.). Use "*" to match any method
	PathPattern string // The path pattern to match (supports * as wildcard for path segments)
	Resource    string // The resource name for RBAC
	Action      string // Optional: custom action (if empty, use methodToAction)
}

// customRouteMappings defines special routing rules that override default behavior
// Add new mappings here to customize resource/action extraction
//
// Example usage:
//
//	{
//	    Method:      "POST",                     // Specific HTTP method (or "*" for any)
//	    PathPattern: "/inventories/*/reconcile", // Path with * wildcards
//	    Resource:    "inventory-submissions",    // RBAC resource name
//	    Action:      "create",                   // Custom action (or "" to use HTTP method mapping)
//	}
var customRouteMappings = []RouteMapping{
	// Purchase Order Import
	{
		Method:      "POST",
		PathPattern: "/purchase-orders/upload",
		Resource:    "purchase-orders",
		Action:      "import",
	},
	{
		Method:      "POST",
		PathPattern: "/purchase-orders/upload-files/*/process",
		Resource:    "purchase-orders",
		Action:      "import",
	},
	// Payment receipt form submissions
	{
		Method:      "POST",
		PathPattern: "/payment-receipt-forms/*/submit",
		Resource:    "payment-receipt-forms",
		Action:      "submit",
	},
	{
		Method:      "GET",
		PathPattern: "/payment-receipt-forms/pending",
		Resource:    "payment-receipt-forms",
		Action:      "view-pending",
	},
	{
		Method:      "*",
		PathPattern: "/inventories/*/reconcile",
		Resource:    "inventory-submissions",
		Action:      "",
	},
	{
		Method:      "*",
		PathPattern: "/inventories/*/dispose",
		Resource:    "inventory-submissions",
		Action:      "",
	},
	{
		Method:      "*",
		PathPattern: "/inventories/transfer",
		Resource:    "inventory-submissions",
		Action:      "",
	},
	{
		Method:      "PUT",
		PathPattern: "/purchase-orders/*/receive",
		Resource:    "purchase-orders",
		Action:      "confirm",
	},
	{
		Method:      "POST",
		PathPattern: "/purchase-orders/*/revenue-expense/retry",
		Resource:    "purchase-orders",
		Action:      "retry_excel",
	},
	// Sale Order Status Update
	{
		Method:      "PUT",
		PathPattern: "/sale-orders/*/status",
		Resource:    "sale-orders",
		Action:      "update_status",
	},
}

// matchPathPattern checks if a path matches a pattern (supports * as wildcard for path segments)
func matchPathPattern(path, pattern string) bool {
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")

	if len(pathParts) != len(patternParts) {
		return false
	}

	for i := range patternParts {
		if patternParts[i] != "*" && patternParts[i] != pathParts[i] {
			return false
		}
	}

	return true
}

// matchRouteMapping checks if a request matches a route mapping
func matchRouteMapping(method, path string, mapping RouteMapping) bool {
	// Check method match ("*" matches any method)
	if mapping.Method != "*" && mapping.Method != method {
		return false
	}

	// Check path pattern match
	return matchPathPattern(path, mapping.PathPattern)
}

// extractResourceAndAction extracts the resource and action from the HTTP request
func extractResourceAndAction(c echo.Context) (string, string) {
	path := c.Request().URL.Path
	method := c.Request().Method

	// Remove API prefix
	path = strings.TrimPrefix(path, "/api/v1")

	// Check custom route mappings first
	for _, mapping := range customRouteMappings {
		if matchRouteMapping(method, path, mapping) {
			action := mapping.Action
			if action == "" {
				action = methodToAction(method)
			}
			return mapping.Resource, action
		}
	}

	// Fall back to default resource extraction
	resource := pathToResource(path)
	if resource == "" {
		return "", ""
	}

	return resource, methodToAction(method)
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

	switch {
	case resource == "inventories" && len(segments) >= 3:
		return "inventory-items"
	case resource == "users" && len(segments) > 1 && segments[1] == "permissions":
		return "permissions"
	default:
		return resource
	}
}

// getUserPermissions retrieves all permissions for a given role
func getUserPermissions(casbinService *auth.CasbinService, userRole string) (map[pkg.UserPermission]struct{}, error) {
	// Get all policies for the role from Casbin
	enforcer := casbinService.GetEnforcer()
	if enforcer == nil {
		return nil, fmt.Errorf("casbin enforcer not available")
	}

	// Get all policies where the subject is the user role
	policies, err := enforcer.GetFilteredPolicy(0, userRole)
	if err != nil {
		return nil, fmt.Errorf("error getting filtered policies: %w", err)
	}

	permissions := make(map[pkg.UserPermission]struct{})
	for _, policy := range policies {
		if len(policy) >= 3 {
			// policy format: [role, resource, action]
			permissions[pkg.UserPermission{
				Resource: policy[1],
				Action:   policy[2],
			}] = struct{}{}
		}
	}

	return permissions, nil
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
