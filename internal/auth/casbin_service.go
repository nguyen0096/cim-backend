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
	// Define default policies
	policies := [][]string{
		// Admin policies
		{"admin", "products", "view"},
		{"admin", "products", "create"},
		{"admin", "products", "update"},
		{"admin", "products", "delete"},
		{"admin", "suppliers", "view"},
		{"admin", "suppliers", "create"},
		{"admin", "suppliers", "update"},
		{"admin", "suppliers", "delete"},
		{"admin", "inventories", "view"},
		{"admin", "inventories", "create"},
		{"admin", "inventories", "update"},
		{"admin", "inventories", "delete"},
		{"admin", "purchase_orders", "view"},
		{"admin", "purchase_orders", "create"},
		{"admin", "purchase_orders", "update"},
		{"admin", "purchase_orders", "delete"},
		{"admin", "excel", "view"},
		{"admin", "excel", "create"},
		{"admin", "excel", "update"},
		{"admin", "excel", "delete"},
		{"admin", "settings", "view"},
		{"admin", "settings", "create"},
		{"admin", "settings", "update"},
		{"admin", "settings", "delete"},
		{"admin", "prices", "view"},

		// Accountant policies
		{"accountant", "products", "view"},
		{"accountant", "suppliers", "view"},
		{"accountant", "inventories", "view"},
		{"accountant", "purchase_orders", "view"},
		{"accountant", "purchase_orders", "create"},
		{"accountant", "purchase_orders", "update"},
		{"accountant", "purchase_orders", "delete"},
		{"accountant", "excel", "view"},
		{"accountant", "settings", "view"},
		{"accountant", "prices", "view"},

		// Staff policies
		{"staff", "products", "view"},
		{"staff", "suppliers", "view"},
		{"staff", "inventories", "view"},
		{"staff", "purchase_orders", "view"},
		{"staff", "excel", "view"},
		{"staff", "settings", "view"},
	}

	// Add policies to Casbin
	for _, policy := range policies {
		_, err := c.enforcer.AddPolicy(policy[0], policy[1], policy[2])
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

// GetEnforcer returns the Casbin enforcer (for testing purposes)
func (c *CasbinService) GetEnforcer() *casbin.Enforcer {
	return c.enforcer
}
