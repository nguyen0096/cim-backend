package auth

import (
	"sync"
	"testing"

	"cim-backend/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSeedRoleConcurrently mirrors what AuthorizationMiddleware does per request
// (`internal/middleware/authorization.go:67-71`, `:85`, `:103`): a check-then-add of
// the caller's role, then an enforce and a policy read. With no `g` rows in
// rbac_policy.csv every account takes that branch on the first requests after a
// boot, and the frontend issues several calls in parallel on load. Run with -race:
// under a plain casbin.Enforcer the concurrent mutation and reads of the role
// manager and policy are unsynchronized.
func TestSeedRoleConcurrently(t *testing.T) {
	t.Setenv("RBAC_ADAPTER", "file")
	svc, err := NewCasbinService(nil, config.CasbinConfig{
		ModelFile:  "../../rbac_model.conf",
		PolicyFile: "../../rbac_policy.csv",
	})
	require.NoError(t, err)

	// The UIDs whose static g rows this change removes, plus one fresh account.
	accounts := map[string]string{
		"demoAdminUid0000000000000000": "admin",
		"demoMaintAdminUid00000000000": "developer",
		"demoAccountantUid00000000000": "accountant",
		"demoStaffUid0000000000000000": "staff",
		"demoBotFormUid00000000000000": "bot_form",
		"fresh-uid-never-seen-before":  "staff",
	}

	const parallelPerAccount = 24
	var wg sync.WaitGroup
	for uid, role := range accounts {
		for i := 0; i < parallelPerAccount; i++ {
			wg.Add(1)
			go func(uid, role string) {
				defer wg.Done()
				// Seed step, verbatim from the middleware.
				if roles, err := svc.GetRolesForUser(uid); err == nil && len(roles) == 0 {
					assert.NoError(t, svc.AddRoleForUser(uid, role))
				}
				// Reads that run concurrently with other goroutines' seeding.
				_, err := svc.Enforce(role, "permissions", "view")
				assert.NoError(t, err)
				_, err = svc.GetEnforcer().GetFilteredPolicy(0, role)
				assert.NoError(t, err)
				_, err = svc.GetUserPermissions(uid)
				assert.NoError(t, err)
			}(uid, role)
		}
	}
	wg.Wait()

	// Every account ends bound to exactly its DB role, with no duplicate from the
	// racing adds.
	for uid, role := range accounts {
		roles, err := svc.GetRolesForUser(uid)
		require.NoError(t, err)
		assert.Equal(t, []string{role}, roles, "uid %s", uid)
	}
}
