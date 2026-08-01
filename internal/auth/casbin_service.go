package auth

import (
	"fmt"
	"os"

	"cim-backend/internal/config"

	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"

	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"

	"gorm.io/gorm"
)

// CasbinService answers authorization questions from state fixed at boot. It exposes
// no mutator: a subject's effective roles are derived per request by
// EnforcementSubject, never written into the enforcer.
type CasbinService struct {
	// SyncedEnforcer, not Enforcer: one enforcer serves every request goroutine and
	// casbin's own read path mutates the role manager's map
	// (rbac/default-role-manager/role_manager.go:355-366 creates and removes roles
	// inside HasLink).
	enforcer *casbin.SyncedEnforcer

	// Subjects the policy file binds with a g row, snapshotted at boot. Nothing
	// writes g rows afterwards, so this is also the live set.
	policyBoundSubjects map[string]struct{}
}

// NewCasbinService creates a new Casbin service with PostgreSQL adapter
func NewCasbinService(db *gorm.DB, casbinCfg config.CasbinConfig) (*CasbinService, error) {
	// Get the path to the model file
	fileAdapter := fileadapter.NewAdapter(casbinCfg.PolicyFile)

	// Initialize Casbin enforcer
	enforcer, err := casbin.NewSyncedEnforcer(casbinCfg.ModelFile, fileAdapter)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize casbin enforcer: %w", err)
	}

	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("failed to load casbin policy: %w", err)
	}

	grouping, err := enforcer.GetGroupingPolicy()
	if err != nil {
		return nil, fmt.Errorf("failed to read casbin grouping policy: %w", err)
	}
	policyBoundSubjects := make(map[string]struct{}, len(grouping))
	for _, g := range grouping {
		if len(g) < 2 {
			continue
		}
		policyBoundSubjects[g[0]] = struct{}{}
	}

	if os.Getenv("RBAC_ADAPTER") == "sql" {
		adapter, err := gormadapter.NewAdapterByDB(db)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize casbin adapter: %w", err)
		}

		enforcer.SetAdapter(adapter)
		if err := enforcer.SavePolicy(); err != nil {
			return nil, fmt.Errorf("failed to save casbin policy: %w", err)
		}
	}

	return &CasbinService{
		enforcer:            enforcer,
		policyBoundSubjects: policyBoundSubjects,
	}, nil
}

// EnforcementSubject returns the Casbin subject for a caller. A UID the policy file
// binds enforces as itself, so its g rows decide and the union of them applies under
// one deny-override evaluation; every other caller enforces as the users.role just
// read from the database. Nothing is stored, so there is no binding to go stale, to
// race a concurrent write, or to differ between processes.
func (c *CasbinService) EnforcementSubject(uid, dbRole string) string {
	if _, bound := c.policyBoundSubjects[uid]; bound {
		return uid
	}
	return dbRole
}

// Enforce checks if the user has permission to perform the action
func (c *CasbinService) Enforce(subject, object, action string) (bool, error) {
	allowed, err := c.enforcer.Enforce(subject, object, action)
	if err != nil {
		return false, fmt.Errorf("failed to enforce authorization: %w", err)
	}
	return allowed, nil
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

// ImplicitPermissions returns the p rows a subject reaches: its own rows plus those of
// every role it resolves to through g. A role string reaches its own rows, a
// policy-bound UID reaches the union of the roles its g rows name.
func (c *CasbinService) ImplicitPermissions(subject string) ([][]string, error) {
	rows, err := c.enforcer.GetImplicitPermissionsForUser(subject)
	if err != nil {
		return nil, fmt.Errorf("failed to get implicit permissions: %w", err)
	}
	return rows, nil
}

// GetUserPermissions returns a subject's allowed "object:action" permissions, excluding
// any the same subject is denied.
func (c *CasbinService) GetUserPermissions(subject string) ([]string, error) {
	rows, err := c.ImplicitPermissions(subject)
	if err != nil {
		return nil, err
	}

	allowed := make(map[string]bool)
	denied := make(map[string]bool)
	for _, row := range rows {
		if len(row) < 4 {
			continue
		}
		permission := fmt.Sprintf("%s:%s", row[1], row[2])
		switch row[3] {
		case "allow":
			allowed[permission] = true
		case "deny":
			denied[permission] = true
		}
	}

	permissions := make([]string, 0, len(allowed))
	for permission := range allowed {
		if !denied[permission] {
			permissions = append(permissions, permission)
		}
	}

	return permissions, nil
}

// GetEnforcer returns the Casbin enforcer. Tests only: production code reaches the
// enforcer through this type's read-only methods, which is what keeps the policy
// immutable after boot.
func (c *CasbinService) GetEnforcer() *casbin.SyncedEnforcer {
	return c.enforcer
}
