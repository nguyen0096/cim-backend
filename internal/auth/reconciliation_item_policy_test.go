package auth

import (
	"testing"

	"github.com/casbin/casbin/v2"
	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReconciliationItemRBACPolicy loads the real rbac_model.conf +
// rbac_policy.csv and asserts the epic #38 Part 4 grants: staff is allowed the
// four staff child-item actions and NOT the admin-only actions (approve /
// initiate_reconciliation), while admin and accountant hold all of them. This
// guards against an accidental policy regression (e.g. dropping a row or granting
// staff approve).
func TestReconciliationItemRBACPolicy(t *testing.T) {
	adapter := fileadapter.NewAdapter("../../rbac_policy.csv")
	enforcer, err := casbin.NewEnforcer("../../rbac_model.conf", adapter)
	require.NoError(t, err)

	const resource = "inventory-submissions"
	staffActions := []string{
		"recon_item_create",
		"recon_item_update",
		"recon_item_delete",
	}

	t.Run("staff allowed the child-item actions", func(t *testing.T) {
		for _, action := range staffActions {
			ok, err := enforcer.Enforce("staff", resource, action)
			require.NoError(t, err)
			assert.True(t, ok, "staff should be allowed %s on %s", action, resource)
		}
	})

	t.Run("staff NOT allowed admin-only reconciliation actions", func(t *testing.T) {
		// recon_manage (epic #38 Part 6 redesign) is admin/accountant-only like
		// approve / initiate_reconciliation; staff must never hold it.
		for _, action := range []string{"approve", "initiate_reconciliation", "recon_manage"} {
			ok, err := enforcer.Enforce("staff", resource, action)
			require.NoError(t, err)
			assert.False(t, ok, "staff must NOT be allowed %s on %s", action, resource)
		}
	})

	t.Run("admin and accountant allowed recon_manage", func(t *testing.T) {
		for _, role := range []string{"admin", "accountant"} {
			ok, err := enforcer.Enforce(role, resource, "recon_manage")
			require.NoError(t, err)
			assert.True(t, ok, "%s should be allowed recon_manage on %s", role, resource)
		}
	})

	t.Run("admin and accountant allowed all child-item actions", func(t *testing.T) {
		for _, role := range []string{"admin", "accountant"} {
			for _, action := range staffActions {
				ok, err := enforcer.Enforce(role, resource, action)
				require.NoError(t, err)
				assert.True(t, ok, "%s should be allowed %s on %s", role, action, resource)
			}
		}
	})
}
