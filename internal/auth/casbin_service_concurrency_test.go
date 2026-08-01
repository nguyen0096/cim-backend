package auth

import (
	"sync"
	"sync/atomic"
	"testing"

	"cim-backend/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFileService(t *testing.T) *CasbinService {
	t.Helper()
	t.Setenv("RBAC_ADAPTER", "file")
	svc, err := NewCasbinService(nil, config.CasbinConfig{ModelFile: modelFile, PolicyFile: policyFile})
	require.NoError(t, err)
	return svc
}

// TestConcurrentDerivationIsRaceFreeAndNeverUnions mirrors what
// AuthorizationMiddleware does per request while a role change is in flight: the
// caller's users.role flips under it and every request re-derives its subject. Run
// under -race. Two things must hold: no data race on the shared enforcer, and no
// observation mixing the old and the new role, which is only possible because the
// derivation reads one value and writes nothing.
func TestConcurrentDerivationIsRaceFreeAndNeverUnions(t *testing.T) {
	svc := newFileService(t)

	// Stands in for users.role, flipped by a concurrent PUT /users/:id.
	var dbRole atomic.Value
	dbRole.Store("admin")

	const uid = "uid-being-downgraded"
	const parallel = 32

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < parallel; i++ {
			if i%2 == 0 {
				dbRole.Store("staff")
			} else {
				dbRole.Store("admin")
			}
		}
		dbRole.Store("staff")
	}()

	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			role := dbRole.Load().(string)
			subject := svc.EnforcementSubject(uid, role)
			assert.Equal(t, role, subject, "an unbound UID must enforce as its DB role")

			// users:delete is admin-only, so it is the old-vs-new discriminator.
			allowed, err := svc.Enforce(subject, "users", "delete")
			assert.NoError(t, err)
			assert.Equal(t, role == "admin", allowed, "%s must not decide as the other role", role)

			perms, err := svc.GetUserPermissions(subject)
			assert.NoError(t, err)
			assert.ElementsMatch(t, rolePermissionStrings(t, role), perms,
				"%s must resolve exactly its own role's permissions, never a union", role)
		}()
	}

	// Policy-bound UIDs read the same enforcer concurrently.
	for _, m := range maintenanceUIDs {
		for i := 0; i < parallel/4; i++ {
			wg.Add(1)
			go func(m string) {
				defer wg.Done()
				assert.Equal(t, m, svc.EnforcementSubject(m, "staff"),
					"a policy-bound UID ignores users.role")
				_, err := svc.Enforce(m, "initial-stock-import", "import")
				assert.NoError(t, err)
				_, err = svc.ImplicitPermissions(m)
				assert.NoError(t, err)
			}(m)
		}
	}
	wg.Wait()

	// Nothing was written: the UID still holds no g row, so no process-local binding
	// exists to go stale or to diverge from another instance.
	roles, err := svc.GetRolesForUser(uid)
	require.NoError(t, err)
	assert.Empty(t, roles)
}
