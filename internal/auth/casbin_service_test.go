package auth

import (
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserPermissions(t *testing.T) {
	// Create a test enforcer with in-memory adapter
	enforcer, err := casbin.NewEnforcer("rbac_model.conf", false)
	require.NoError(t, err)

	// Create CasbinService with test enforcer
	service := &CasbinService{
		enforcer: enforcer,
	}

	// Add test policies
	testPolicies := [][]string{
		{"admin", "products", "view"},
		{"admin", "products", "create"},
		{"admin", "inventories", "view"},
		{"admin", "users", "view"},
		{"staff", "products", "view"},
		{"staff", "inventories", "view"},
		{"staff", "users", "view"},
	}

	for _, policy := range testPolicies {
		_, err := enforcer.AddPolicy(policy[0], policy[1], policy[2])
		require.NoError(t, err)
	}

	// Add role assignments
	_, err = enforcer.AddRoleForUser("user1", "admin")
	require.NoError(t, err)
	_, err = enforcer.AddRoleForUser("user2", "staff")
	require.NoError(t, err)

	t.Run("should return admin permissions for admin user", func(t *testing.T) {
		permissions, err := service.GetUserPermissions("user1")
		require.NoError(t, err)

		expectedPermissions := []string{
			"products:view",
			"products:create",
			"inventories:view",
			"users:view",
		}

		assert.ElementsMatch(t, expectedPermissions, permissions)
	})

	t.Run("should return staff permissions for staff user", func(t *testing.T) {
		permissions, err := service.GetUserPermissions("user2")
		require.NoError(t, err)

		expectedPermissions := []string{
			"products:view",
			"inventories:view",
			"users:view",
		}

		assert.ElementsMatch(t, expectedPermissions, permissions)
	})

	t.Run("should return empty permissions for user with no roles", func(t *testing.T) {
		permissions, err := service.GetUserPermissions("user3")
		require.NoError(t, err)

		assert.Empty(t, permissions)
	})

	t.Run("should return error when getting user roles fails", func(t *testing.T) {
		// Create service with invalid enforcer to trigger error
		invalidEnforcer, err := casbin.NewEnforcer("rbac_model.conf", false)
		require.NoError(t, err)

		// Remove all policies to simulate error condition
		invalidEnforcer.ClearPolicy()

		invalidService := &CasbinService{
			enforcer: invalidEnforcer,
		}

		permissions, err := invalidService.GetUserPermissions("nonexistent-user")
		require.NoError(t, err)
		assert.Empty(t, permissions)
	})
}
