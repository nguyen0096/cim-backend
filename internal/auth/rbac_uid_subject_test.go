package auth

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"cim-backend/internal/models"

	"github.com/casbin/casbin/v2"
	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	policyFile = "../../rbac_policy.csv"
	modelFile  = "../../rbac_model.conf"
)

// maintenanceUIDs are the only UIDs allowed a g row; see rbac_policy.csv.
var maintenanceUIDs = []string{
	"demoMaintAdminUid00000000000",
	"demoRootAdminUid000000000000",
}

type policyRow struct{ sub, obj, act, eft string }

// readPolicyFile parses the shipped csv into its p and g rows. Parsing the file rather
// than the loaded enforcer is what lets the g-row assertions be exhaustive, and what
// makes the permission oracle below independent of casbin.
func readPolicyFile(t *testing.T) (pRows []policyRow, gRows map[string][]string) {
	t.Helper()
	raw, err := os.ReadFile(policyFile)
	require.NoError(t, err)

	gRows = map[string][]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, ",")
		for i := range f {
			f[i] = strings.TrimSpace(f[i])
		}
		switch f[0] {
		case "p":
			require.Len(t, f, 5, "p row must be sub,obj,act,eft: %q", line)
			pRows = append(pRows, policyRow{f[1], f[2], f[3], f[4]})
		case "g":
			require.Len(t, f, 3, "g row must be sub,role: %q", line)
			gRows[f[1]] = append(gRows[f[1]], f[2])
		default:
			t.Fatalf("unexpected policy line: %q", line)
		}
	}
	return pRows, gRows
}

func roleSet(pRows []policyRow) []string {
	seen := map[string]struct{}{}
	var roles []string
	for _, r := range pRows {
		if _, ok := seen[r.sub]; !ok {
			seen[r.sub] = struct{}{}
			roles = append(roles, r.sub)
		}
	}
	sort.Strings(roles)
	return roles
}

// rolePermissionStrings is the origin/main oracle for GET /users/permissions: the allow
// rows a role holds directly, minus anything the same role is denied. On origin/main
// the middleware seeded the UID with exactly this one role, so this is what both the
// endpoint and pkg.HasPermission resolved.
func rolePermissionStrings(t *testing.T, role string) []string {
	t.Helper()
	pRows, _ := readPolicyFile(t)

	allowed := map[string]bool{}
	denied := map[string]bool{}
	for _, r := range pRows {
		if r.sub != role {
			continue
		}
		perm := fmt.Sprintf("%s:%s", r.obj, r.act)
		switch r.eft {
		case "allow":
			allowed[perm] = true
		case "deny":
			denied[perm] = true
		}
	}

	out := make([]string, 0, len(allowed))
	for p := range allowed {
		if !denied[p] {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func union(a, b []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(a)+len(b))
	for _, s := range append(append([]string{}, a...), b...) {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func newFileEnforcer(t *testing.T) *casbin.Enforcer {
	t.Helper()
	e, err := casbin.NewEnforcer(modelFile, fileadapter.NewAdapter(policyFile))
	require.NoError(t, err)
	return e
}

// TestOnlyMaintenanceUIDsHaveBindings pins the whole g section of the shipped policy,
// so a future per-account grant cannot be added silently.
func TestOnlyMaintenanceUIDsHaveBindings(t *testing.T) {
	_, gRows := readPolicyFile(t)

	subjects := make([]string, 0, len(gRows))
	for sub := range gRows {
		subjects = append(subjects, sub)
	}
	assert.ElementsMatch(t, maintenanceUIDs, subjects)

	for _, uid := range maintenanceUIDs {
		assert.ElementsMatch(t, []string{"admin", "developer"}, gRows[uid], "uid %s", uid)
	}
}

// TestCasbinServiceExposesNoMutator is the structural guard on the class of defects
// this design closes: process-local writes to the enforcer are what let a role change
// race the request path, wipe a file grant, or diverge between instances. Reintroduce
// a mutator and this fails before anything can call it.
func TestCasbinServiceExposesNoMutator(t *testing.T) {
	var got []string
	typ := reflect.TypeOf(&CasbinService{})
	for i := 0; i < typ.NumMethod(); i++ {
		got = append(got, typ.Method(i).Name)
	}

	assert.ElementsMatch(t, []string{
		"Enforce",
		"EnforcementSubject",
		"GetEnforcer",
		"GetRolesForUser",
		"GetUserPermissions",
		"GetUsersForRole",
		"ImplicitPermissions",
	}, got, "CasbinService must stay read-only after boot")
}

// TestPolicyBoundSubjectsMatchTheFile pins the boot snapshot the derivation keys on.
func TestPolicyBoundSubjectsMatchTheFile(t *testing.T) {
	_, gRows := readPolicyFile(t)
	svc := newFileService(t)

	subjects := make([]string, 0, len(svc.policyBoundSubjects))
	for sub := range svc.policyBoundSubjects {
		subjects = append(subjects, sub)
		roles, err := svc.GetRolesForUser(sub)
		require.NoError(t, err)
		assert.ElementsMatch(t, gRows[sub], roles, "uid %s", sub)
	}

	want := make([]string, 0, len(gRows))
	for sub := range gRows {
		want = append(want, sub)
	}
	assert.ElementsMatch(t, want, subjects)
}

// TestUnboundSubjectIsByteIdenticalToOriginMain is the no-regression check for every
// account with no g row, which is everyone but the two maintenance UIDs. Its subject
// must be the bare users.role origin/main enforced on, its Enforce answer must match
// that role's for every (obj, act) in the policy, and both permission sources must
// equal the role's own allow-minus-deny rows read straight from the csv.
func TestUnboundSubjectIsByteIdenticalToOriginMain(t *testing.T) {
	pRows, _ := readPolicyFile(t)
	svc := newFileService(t)

	for _, role := range roleSet(pRows) {
		subject := svc.EnforcementSubject("uid-of-"+role, role)
		require.Equal(t, role, subject, "an unbound UID must enforce as its DB role")

		for _, r := range pRows {
			viaRole, err := svc.Enforce(role, r.obj, r.act)
			require.NoError(t, err)
			viaSubject, err := svc.Enforce(subject, r.obj, r.act)
			require.NoError(t, err)
			assert.Equal(t, viaRole, viaSubject, "%s on %s:%s", role, r.obj, r.act)
		}

		perms, err := svc.GetUserPermissions(subject)
		require.NoError(t, err)
		assert.ElementsMatch(t, rolePermissionStrings(t, role), perms,
			"GET /users/permissions for %s", role)

		implicit, err := svc.ImplicitPermissions(subject)
		require.NoError(t, err)
		direct, err := svc.GetEnforcer().GetFilteredPolicy(0, role)
		require.NoError(t, err)
		assert.ElementsMatch(t, direct, implicit,
			"the in-handler permission source for %s must still be the role's own p rows", role)
	}
}

// TestUnknownRoleFailsClosed covers a users.role no policy row answers to, which is
// what a leftover restaurant-admin holder would be: nothing is allowed, and nothing is
// accidentally allowed either.
func TestUnknownRoleFailsClosed(t *testing.T) {
	pRows, _ := readPolicyFile(t)
	svc := newFileService(t)

	for _, role := range []string{"restaurant-admin", "", "not-a-role"} {
		subject := svc.EnforcementSubject("uid-with-unknown-role", role)
		require.Equal(t, role, subject)

		for _, r := range pRows {
			allowed, err := svc.Enforce(subject, r.obj, r.act)
			require.NoError(t, err)
			assert.False(t, allowed, "role %q must hold nothing, got %s:%s", role, r.obj, r.act)
		}

		perms, err := svc.GetUserPermissions(subject)
		require.NoError(t, err)
		assert.Empty(t, perms, "role %q", role)
	}
}

// TestMaintenanceAccountsHoldAdminAndDeveloper covers both halves at once: the two
// accounts reach admin-only and developer-only permissions whatever users.role says,
// and no other account does.
func TestMaintenanceAccountsHoldAdminAndDeveloper(t *testing.T) {
	pRows, _ := readPolicyFile(t)
	svc := newFileService(t)

	for _, uid := range maintenanceUIDs {
		// users.role is deliberately the narrowest role: it must not narrow the account.
		subject := svc.EnforcementSubject(uid, "staff")
		require.Equal(t, uid, subject)

		for _, c := range []struct{ obj, act string }{
			{"users", "delete"},                // admin-only
			{"admin-tools", "view"},            // admin-only
			{"developer-tools", "view"},        // developer-only
			{"initial-stock-import", "import"}, // developer-only
		} {
			ok, err := svc.Enforce(subject, c.obj, c.act)
			require.NoError(t, err)
			assert.True(t, ok, "uid %s must be allowed %s:%s", uid, c.obj, c.act)
		}

		perms, err := svc.GetUserPermissions(subject)
		require.NoError(t, err)
		assert.ElementsMatch(t,
			union(rolePermissionStrings(t, "admin"), rolePermissionStrings(t, "developer")),
			perms, "uid %s resolves the union of its g rows", uid)
	}

	// Every other account holds only its own role, so none of them gains the tool.
	for _, role := range roleSet(pRows) {
		subject := svc.EnforcementSubject("uid-of-"+role, role)
		ok, err := svc.Enforce(subject, "initial-stock-import", "import")
		require.NoError(t, err)
		assert.Equal(t, role == "developer", ok, "tool access for role %s", role)
	}
}

// TestPolicyRolesMatchUserRoleEnum keeps the policy's role set and models.IsValidRole
// from drifting apart: a role accepted by the enum but absent from the policy holds no
// permission at all and locks its holder out of every endpoint.
func TestPolicyRolesMatchUserRoleEnum(t *testing.T) {
	pRows, _ := readPolicyFile(t)

	var valid []string
	for _, r := range []models.UserRole{
		models.RoleAdmin, models.RoleAccountant, models.RoleStaff, models.RoleBotForm,
		models.RoleChef, models.RoleWaiter, models.RoleCashier, models.RoleDeveloper,
	} {
		require.True(t, r.IsValidRole(), "%s must be a valid role", r)
		valid = append(valid, string(r))
	}
	assert.ElementsMatch(t, valid, roleSet(pRows))
	assert.False(t, models.UserRole("restaurant-admin").IsValidRole())
}

// TestUnreleasedWritePermissionsHeldByNoRole pins the accepted consequence of #150: the
// menu and sale-order write permissions the removed role carried alone are now
// unreachable, while the read and status paths survive through chef/waiter/cashier.
func TestUnreleasedWritePermissionsHeldByNoRole(t *testing.T) {
	pRows, _ := readPolicyFile(t)
	e := newFileEnforcer(t)

	orphaned := []struct{ obj, act string }{
		{"menus", "create"}, {"menus", "update"}, {"menus", "delete"},
		{"menu-items", "create"}, {"menu-items", "update"}, {"menu-items", "delete"},
		{"sale-orders", "create"}, {"sale-order-items", "create"},
	}
	for _, role := range roleSet(pRows) {
		for _, c := range orphaned {
			ok, err := e.Enforce(role, c.obj, c.act)
			require.NoError(t, err)
			assert.False(t, ok, "%s must not hold %s:%s", role, c.obj, c.act)
		}
	}
	for _, uid := range maintenanceUIDs {
		for _, c := range orphaned {
			ok, err := e.Enforce(uid, c.obj, c.act)
			require.NoError(t, err)
			assert.False(t, ok, "uid %s must not hold %s:%s", uid, c.obj, c.act)
		}
	}

	survives := []struct{ role, obj, act string }{
		{"chef", "menus", "view"},
		{"chef", "sale-orders", "update_status"},
		{"waiter", "sale-orders", "update"},
		{"cashier", "sale-order-items", "update"},
	}
	for _, c := range survives {
		ok, err := e.Enforce(c.role, c.obj, c.act)
		require.NoError(t, err)
		assert.True(t, ok, "%s must keep %s:%s", c.role, c.obj, c.act)
	}
}

// TestDenyOverridesUnderUIDSubject exercises the deny half of the policy effect, which
// is why a multi-role account resolves through one Enforce on the UID rather than an OR
// over its roles. The shipped file carries no deny row, so one is added here.
func TestDenyOverridesUnderUIDSubject(t *testing.T) {
	e := newFileEnforcer(t)

	const uid = "demoMaintAdminUid00000000000"
	ok, err := e.Enforce(uid, "users", "delete")
	require.NoError(t, err)
	require.True(t, ok, "precondition: allowed through g -> admin")

	// developer is the account's second role; a deny on either role must win.
	_, err = e.AddPolicy("developer", "users", "delete", "deny")
	require.NoError(t, err)

	ok, err = e.Enforce(uid, "users", "delete")
	require.NoError(t, err)
	assert.False(t, ok, "deny must override the allow reached through the other role")
}
