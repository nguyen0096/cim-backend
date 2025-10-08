package auth

import (
	"fmt"
	"path/filepath"

	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

// CasbinService handles authorization using Casbin
type CasbinService struct {
	enforcer *casbin.Enforcer
}

// NewCasbinService creates a new Casbin service with PostgreSQL adapter
func NewCasbinService(db *gorm.DB) (*CasbinService, error) {
	// Initialize GORM adapter for Casbin
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize casbin adapter: %w", err)
	}

	// Get the path to the model file
	modelPath := filepath.Join("internal", "auth", "rbac_model.conf")

	// Initialize Casbin enforcer
	enforcer, err := casbin.NewEnforcer(modelPath, adapter)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize casbin enforcer: %w", err)
	}

	// Load policy from database
	err = enforcer.LoadPolicy()
	if err != nil {
		return nil, fmt.Errorf("failed to load casbin policy: %w", err)
	}

	return &CasbinService{
		enforcer: enforcer,
	}, nil
}

// Enforce checks if the user has permission to perform the action
func (c *CasbinService) Enforce(subject, object, action string) (bool, error) {
	allowed, err := c.enforcer.Enforce(subject, object, action)
	if err != nil {
		return false, fmt.Errorf("failed to enforce authorization: %w", err)
	}
	return allowed, nil
}

// AddRoleForUser adds a role for a user
func (c *CasbinService) AddRoleForUser(user, role string) error {
	_, err := c.enforcer.AddRoleForUser(user, role)
	if err != nil {
		return fmt.Errorf("failed to add role for user: %w", err)
	}
	return nil
}

// DeleteRoleForUser removes a role from a user
func (c *CasbinService) DeleteRoleForUser(user, role string) error {
	_, err := c.enforcer.DeleteRoleForUser(user, role)
	if err != nil {
		return fmt.Errorf("failed to delete role for user: %w", err)
	}
	return nil
}

// GetRolesForUser returns all roles for a user
func (c *CasbinService) GetRolesForUser(user string) ([]string, error) {
	roles, err := c.enforcer.GetRolesForUser(user)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles for user: %w", err)
	}
	return roles, nil
}

// GetUsersForRole returns all users with a specific role
func (c *CasbinService) GetUsersForRole(role string) ([]string, error) {
	users, err := c.enforcer.GetUsersForRole(role)
	if err != nil {
		return nil, fmt.Errorf("failed to get users for role: %w", err)
	}
	return users, nil
}

// AddPolicy adds a new policy rule
func (c *CasbinService) AddPolicy(subject, object, action string) error {
	added, err := c.enforcer.AddPolicy(subject, object, action)
	if err != nil {
		return fmt.Errorf("failed to add policy: %w", err)
	}
	if !added {
		return fmt.Errorf("policy already exists")
	}
	return nil
}

// RemovePolicy removes a policy rule
func (c *CasbinService) RemovePolicy(subject, object, action string) error {
	removed, err := c.enforcer.RemovePolicy(subject, object, action)
	if err != nil {
		return fmt.Errorf("failed to remove policy: %w", err)
	}
	if !removed {
		return fmt.Errorf("policy does not exist")
	}
	return nil
}

// InitializeDefaultPolicies initializes the default RBAC policies
func (c *CasbinService) InitializeDefaultPolicies() error {
	// Define default policies with effects (allow/deny)
	policies := [][]string{
		// Admin policies
		{"admin", "products", "view", "allow"},
		{"admin", "products", "create", "allow"},
		{"admin", "products", "update", "allow"},
		{"admin", "products", "delete", "allow"},
		{"admin", "suppliers", "view", "allow"},
		{"admin", "suppliers", "create", "allow"},
		{"admin", "suppliers", "update", "allow"},
		{"admin", "suppliers", "delete", "allow"},
		{"admin", "inventories", "view", "allow"},
		{"admin", "inventories", "create", "allow"},
		{"admin", "inventories", "update", "allow"},
		{"admin", "inventories", "delete", "allow"},
		{"admin", "purchase-orders", "view", "allow"},
		{"admin", "purchase-orders", "create", "allow"},
		{"admin", "purchase-orders", "update", "allow"},
		{"admin", "purchase-orders", "delete", "allow"},
		{"admin", "purchase-order-items", "view", "allow"},
		{"admin", "purchase-order-items", "create", "allow"},
		{"admin", "purchase-order-items", "update", "allow"},
		{"admin", "purchase-order-items", "delete", "allow"},
		{"admin", "excel", "view", "allow"},
		{"admin", "excel", "create", "allow"},
		{"admin", "excel", "update", "allow"},
		{"admin", "excel", "delete", "allow"},
		{"admin", "settings", "view", "allow"},
		{"admin", "settings", "create", "allow"},
		{"admin", "settings", "update", "allow"},
		{"admin", "settings", "delete", "allow"},
		{"admin", "prices", "view", "allow"},
		{"admin", "users", "view", "allow"},

		// Accountant policies
		{"accountant", "products", "view", "allow"},
		{"accountant", "suppliers", "view", "allow"},
		{"accountant", "inventories", "view", "allow"},
		{"accountant", "purchase-orders", "view", "allow"},
		{"accountant", "purchase-orders", "create", "allow"},
		{"accountant", "purchase-orders", "update", "allow"},
		{"accountant", "purchase-orders", "delete", "allow"},
		{"accountant", "purchase-order-items", "view", "allow"},
		{"accountant", "purchase-order-items", "create", "allow"},
		{"accountant", "purchase-order-items", "update", "allow"},
		{"accountant", "purchase-order-items", "delete", "allow"},
		{"accountant", "purchase-orders", "complete", "allow"},
		{"accountant", "excel", "view", "allow"},
		{"accountant", "settings", "view", "allow"},
		{"accountant", "settings", "create", "allow"},
		{"accountant", "settings", "update", "allow"},
		{"accountant", "prices", "view", "allow"},
		{"accountant", "users", "view", "allow"},

		// Staff policies
		{"staff", "purchase-orders", "view", "allow"},
		{"staff", "purchase-orders", "complete", "deny"}, // Explicitly deny complete action
		{"staff", "purchase-order-items", "view", "allow"},
		{"staff", "purchase-order-items", "create", "allow"},
		{"staff", "purchase-order-items", "update", "allow"},
		{"staff", "purchase-order-items", "delete", "allow"},
		{"staff", "users", "view", "allow"},
	}

	// Add policies to Casbin
	for _, policy := range policies {
		_, err := c.enforcer.AddPolicy(policy[0], policy[1], policy[2], policy[3])
		if err != nil {
			return fmt.Errorf("failed to add policy %v: %w", policy, err)
		}
	}

	// Save policies to database
	err := c.enforcer.SavePolicy()
	if err != nil {
		return fmt.Errorf("failed to save policies: %w", err)
	}

	return nil
}

// GetUserPermissions returns all allowed permissions for a user based on their roles, excluding denied permissions
func (c *CasbinService) GetUserPermissions(user string) ([]string, error) {
	// Get all roles for the user
	roles, err := c.GetRolesForUser(user)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}

	// Get all policies from the enforcer
	policies, _ := c.enforcer.GetPolicy()

	// Track allowed and denied permissions separately
	allowedPermissions := make(map[string]bool)
	deniedPermissions := make(map[string]bool)

	for _, policy := range policies {
		if len(policy) >= 4 { // Now includes effect (allow/deny)
			role := policy[0]
			object := policy[1]
			action := policy[2]
			effect := policy[3]

			// Check if the user has this role
			for _, userRole := range roles {
				if role == userRole {
					// Format permission as "object:action"
					permission := fmt.Sprintf("%s:%s", object, action)

					if effect == "allow" {
						allowedPermissions[permission] = true
					} else if effect == "deny" {
						deniedPermissions[permission] = true
					}
				}
			}
		}
	}

	// Convert to slice, excluding denied permissions
	permissions := make([]string, 0, len(allowedPermissions))
	for permission := range allowedPermissions {
		// Only include if not explicitly denied
		if !deniedPermissions[permission] {
			permissions = append(permissions, permission)
		}
	}

	return permissions, nil
}

// GetEnforcer returns the Casbin enforcer (for testing purposes)
func (c *CasbinService) GetEnforcer() *casbin.Enforcer {
	return c.enforcer
}
