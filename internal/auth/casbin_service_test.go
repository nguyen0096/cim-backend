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

	// Add test policies with effects
	testPolicies := [][]string{
		{"admin", "products", "view", "allow"},
		{"admin", "products", "create", "allow"},
		{"admin", "inventories", "view", "allow"},
		{"admin", "users", "view", "allow"},
		{"staff", "products", "view", "allow"},
		{"staff", "inventories", "view", "allow"},
		{"staff", "users", "view", "allow"},
	}

	for _, policy := range testPolicies {
		_, err := enforcer.AddPolicy(policy[0], policy[1], policy[2], policy[3])
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

		// Staff should have view permissions
		expectedPermissions := []string{
			"products:view",
			"inventories:view",
			"users:view",
		}

		assert.ElementsMatch(t, expectedPermissions, permissions)
	})

	t.Run("should filter out denied permissions", func(t *testing.T) {
		// Add a role with both allow and deny policies
		_, err := enforcer.AddPolicy("test_role", "test_resource", "test_action", "allow")
		require.NoError(t, err)
		_, err = enforcer.AddPolicy("test_role", "test_resource", "denied_action", "deny")
		require.NoError(t, err)
		_, err = enforcer.AddRoleForUser("user3", "test_role")
		require.NoError(t, err)

		permissions, err := service.GetUserPermissions("user3")
		require.NoError(t, err)

		// Should only include allowed permission, not denied one
		expectedPermissions := []string{
			"test_resource:test_action",
		}

		assert.ElementsMatch(t, expectedPermissions, permissions)

		// Verify denied permission is NOT in the list
		for _, perm := range permissions {
			assert.NotEqual(t, "test_resource:denied_action", perm)
		}
	})

	t.Run("should return empty permissions for user with no roles", func(t *testing.T) {
		permissions, err := service.GetUserPermissions("user_no_roles")
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

func TestDenyRules(t *testing.T) {
	// Create a test enforcer
	enforcer, err := casbin.NewEnforcer("rbac_model.conf", false)
	require.NoError(t, err)

	// Create CasbinService with test enforcer
	service := &CasbinService{
		enforcer: enforcer,
	}

	// Add test policies for accountant (allow complete)
	_, err = enforcer.AddPolicy("accountant", "purchase-orders", "view", "allow")
	require.NoError(t, err)
	_, err = enforcer.AddPolicy("accountant", "purchase-orders", "update", "allow")
	require.NoError(t, err)
	_, err = enforcer.AddPolicy("accountant", "purchase-orders", "complete", "allow")
	require.NoError(t, err)

	// Add test policies for staff (deny complete)
	_, err = enforcer.AddPolicy("staff", "purchase-orders", "view", "allow")
	require.NoError(t, err)
	_, err = enforcer.AddPolicy("staff", "purchase-orders", "complete", "deny")
	require.NoError(t, err)

	// Add role assignments
	_, err = enforcer.AddRoleForUser("accountant1", "accountant")
	require.NoError(t, err)
	_, err = enforcer.AddRoleForUser("staff1", "staff")
	require.NoError(t, err)

	t.Run("should allow accountant to complete purchase orders", func(t *testing.T) {
		allowed, err := service.Enforce("accountant1", "purchase-orders", "complete")
		require.NoError(t, err)
		assert.True(t, allowed, "Accountant should be able to complete purchase orders")
	})

	t.Run("should allow accountant to view purchase orders", func(t *testing.T) {
		allowed, err := service.Enforce("accountant1", "purchase-orders", "view")
		require.NoError(t, err)
		assert.True(t, allowed, "Accountant should be able to view purchase orders")
	})

	t.Run("should deny staff from completing purchase orders", func(t *testing.T) {
		allowed, err := service.Enforce("staff1", "purchase-orders", "complete")
		require.NoError(t, err)
		assert.False(t, allowed, "Staff should NOT be able to complete purchase orders (explicit deny)")
	})

	t.Run("should allow staff to view purchase orders", func(t *testing.T) {
		allowed, err := service.Enforce("staff1", "purchase-orders", "view")
		require.NoError(t, err)
		assert.True(t, allowed, "Staff should be able to view purchase orders")
	})

	t.Run("deny should override allow", func(t *testing.T) {
		// Add both allow and deny for the same action
		_, err := enforcer.AddPolicy("test_role", "test_resource", "test_action", "allow")
		require.NoError(t, err)
		_, err = enforcer.AddPolicy("test_role", "test_resource", "test_action", "deny")
		require.NoError(t, err)
		_, err = enforcer.AddRoleForUser("testuser", "test_role")
		require.NoError(t, err)

		// Deny should override allow
		allowed, err := service.Enforce("testuser", "test_resource", "test_action")
		require.NoError(t, err)
		assert.False(t, allowed, "Deny should override allow")
	})
}
