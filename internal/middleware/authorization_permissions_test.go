package middleware

import (
	"context"
	"testing"

	"cim-backend/internal/auth"
	"cim-backend/internal/config"
	"cim-backend/pkg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPolicyService(t *testing.T) *auth.CasbinService {
	t.Helper()
	t.Setenv("RBAC_ADAPTER", "file")
	svc, err := auth.NewCasbinService(nil, config.CasbinConfig{
		ModelFile:  "../../rbac_model.conf",
		PolicyFile: "../../rbac_policy.csv",
	})
	require.NoError(t, err)
	return svc
}

// permissionsFromDirectRows is the origin/main source: the direct p rows of a role,
// read with the role string as the subject.
func permissionsFromDirectRows(t *testing.T, svc *auth.CasbinService, role string) map[pkg.UserPermission]struct{} {
	t.Helper()
	rows, err := svc.GetEnforcer().GetFilteredPolicy(0, role)
	require.NoError(t, err)

	out := make(map[pkg.UserPermission]struct{})
	for _, r := range rows {
		require.GreaterOrEqual(t, len(r), 3)
		out[pkg.UserPermission{Resource: r[1], Action: r[2]}] = struct{}{}
	}
	return out
}

func policyRoles(t *testing.T, svc *auth.CasbinService) []string {
	t.Helper()
	rows, err := svc.GetEnforcer().GetPolicy()
	require.NoError(t, err)

	seen := map[string]struct{}{}
	var roles []string
	for _, r := range rows {
		if _, ok := seen[r[0]]; !ok {
			seen[r[0]] = struct{}{}
			roles = append(roles, r[0])
		}
	}
	return roles
}

// TestHasPermissionUnchangedForUnboundAccounts pins the in-handler permission map for
// everyone without a g row: pkg.HasPermission must answer exactly as it did when
// origin/main enforced on the role string and read that role's direct p rows.
func TestHasPermissionUnchangedForUnboundAccounts(t *testing.T) {
	svc := newPolicyService(t)

	for _, role := range policyRoles(t, svc) {
		subject := svc.EnforcementSubject("uid-of-"+role, role)
		require.Equal(t, role, subject)

		got, err := getUserPermissions(svc, subject)
		require.NoError(t, err)

		want := permissionsFromDirectRows(t, svc, role)
		assert.Equal(t, want, got, "permission map for %s", role)

		ctx := context.WithValue(context.Background(), pkg.AuthContextKeyUserPermissions, got)
		for perm := range want {
			assert.True(t, pkg.HasPermission(ctx, perm.Resource, perm.Action),
				"%s must hold %s:%s", role, perm.Resource, perm.Action)
		}
		assert.False(t, pkg.HasPermission(ctx, "no-such-resource", "view"))
	}
}

// TestHasPermissionForMaintenanceAccounts: the two accounts resolve the union of admin
// and developer whatever users.role holds, so both the admin screens and the tool are
// reachable.
func TestHasPermissionForMaintenanceAccounts(t *testing.T) {
	svc := newPolicyService(t)

	admin := permissionsFromDirectRows(t, svc, "admin")
	developer := permissionsFromDirectRows(t, svc, "developer")
	want := make(map[pkg.UserPermission]struct{}, len(admin)+len(developer))
	for p := range admin {
		want[p] = struct{}{}
	}
	for p := range developer {
		want[p] = struct{}{}
	}

	for _, uid := range []string{"demoMaintAdminUid00000000000", "demoRootAdminUid000000000000"} {
		subject := svc.EnforcementSubject(uid, "staff")
		require.Equal(t, uid, subject)

		got, err := getUserPermissions(svc, subject)
		require.NoError(t, err)
		assert.Equal(t, want, got, "uid %s", uid)

		ctx := context.WithValue(context.Background(), pkg.AuthContextKeyUserPermissions, got)
		assert.True(t, pkg.HasPermission(ctx, "developer-tools", "view"))
		assert.True(t, pkg.HasPermission(ctx, "users", "delete"))
	}
}
