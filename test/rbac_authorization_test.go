package apptest

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	firebaseAuth "firebase.google.com/go/v4/auth"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"cim-backend/internal/auth"
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil"
)

// usersPath is admin-only (p, admin, users, view); permissionsPath is held by every
// role, so it is the set comparison and usersPath is the allow/deny discriminator.
const (
	usersPath       = "/api/v1/users"
	permissionsPath = "/api/v1/users/permissions"
)

// rbacUser inserts a users row and registers a token the mock Firebase auth resolves
// to tokenUID, which is deliberately separate from the stored UID so the repair path
// can be driven.
func rbacUser(uid, tokenUID string, role models.UserRole, status string) (*models.User, *testutil.Client) {
	email := fmt.Sprintf("rbac-%s@cim.local", uuid.NewString())
	user := &models.User{UID: uid, Email: email, Name: email, Role: role, Status: status, Type: models.UserTypeUser}
	Expect(tenv.ContextfulDB().Create(user).Error).NotTo(HaveOccurred())
	DeferCleanup(func() { tenv.DB.Unscoped().Delete(user) })

	return user, rbacClient(tokenUID, email, email)
}

func rbacClient(tokenUID, email, name string) *testutil.Client {
	token := "token-rbac-" + uuid.NewString()
	tenv.AuthMock.On("VerifyToken", mock.Anything, token).Return(&firebaseAuth.Token{
		UID:      tokenUID,
		Claims:   map[string]interface{}{"email": email, "name": name},
		Expires:  time.Now().Add(time.Hour).Unix(),
		IssuedAt: time.Now().Unix(),
		Subject:  tokenUID,
	}, nil)
	return &testutil.Client{BaseURL: tenv.BaseURL, AuthToken: pkg.Ptr(token)}
}

func rbacGet(client *testutil.Client, path string) *http.Response {
	resp, err := client.MakeRequest(http.MethodGet, path, nil, testutil.WithAuth())
	Expect(err).NotTo(HaveOccurred())
	return resp
}

func rbacPermissions(client *testutil.Client) []string {
	resp := rbacGet(client, permissionsPath)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	raw, ok := testutil.ParseResponse(resp)["permissions"].([]interface{})
	Expect(ok).To(BeTrue())
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		out = append(out, p.(string))
	}
	return out
}

// Authorization derives a caller's subject on every request instead of storing a
// binding, so a role change is a single row write and enforcement reads it fresh.
// These specs drive the running server over HTTP; writes go through a second
// CasbinService and UserService, which is a different object graph from the server's
// and therefore stands in for a second backend process.
var _ = Describe("Authorization subject derivation", func() {
	const maintenanceUID = "demoMaintAdminUid00000000000"

	var (
		otherProcess  *services.UserService
		otherCasbin   *auth.CasbinService
		adminPerms    []string
		staffPerms    []string
		developerPerm []string
	)

	BeforeEach(func() {
		var err error
		otherCasbin, err = auth.NewCasbinService(tenv.DB, tenv.Config.Casbin)
		Expect(err).NotTo(HaveOccurred())
		base := repository.NewBaseRepository(tenv.DB)
		otherProcess = services.NewUserService(
			repository.NewUserRepository(base, tenv.Config.Environment), otherCasbin)

		adminPerms, err = otherCasbin.GetUserPermissions("admin")
		Expect(err).NotTo(HaveOccurred())
		staffPerms, err = otherCasbin.GetUserPermissions("staff")
		Expect(err).NotTo(HaveOccurred())
		developerPerm, err = otherCasbin.GetUserPermissions("developer")
		Expect(err).NotTo(HaveOccurred())
	})

	setRole := func(ctx SpecContext, user *models.User, role models.UserRole) {
		Expect(otherProcess.UpdateUser(ctx, user.ID.String(), "", user.Name, string(role), user.Status)).To(Succeed())
	}

	// Every role, end to end: what the policy file grants is exactly what the endpoints
	// allow and what the sidebar is built from. This is the no-regression sweep for
	// every account that has no g row, which is everyone but the two maintenance UIDs.
	Context("each role with no policy-file binding", func() {
		// A routed GET per policy resource that has one, with the resource:action the
		// middleware derives for it. Covers every endpoint class an ordinary role can
		// differ on.
		routes := map[string]string{
			usersPath:                   "users:view",
			"/api/v1/products":          "products:view",
			"/api/v1/suppliers":         "suppliers:view",
			"/api/v1/purchase-orders":   "purchase-orders:view",
			"/api/v1/menus":             "menus:view",
			"/api/v1/menu-items":        "menu-items:view",
			"/api/v1/sale-orders":       "sale-orders:view",
			"/api/v1/inventories":       "inventories:view",
			"/api/v1/inventory-items":   "inventory-items:view",
			"/api/v1/revenue-expenses":  "revenue-expenses:view",
			"/api/v1/units":             "units:view",
			"/api/v1/selling-prices":    "selling-prices:view",
			"/api/v1/settings":          "settings:view",
			"/api/v1/tools/inventories": "initial-stock-import:import",
		}

		for _, role := range []models.UserRole{
			models.RoleAdmin, models.RoleAccountant, models.RoleStaff, models.RoleBotForm,
			models.RoleChef, models.RoleWaiter, models.RoleCashier, models.RoleDeveloper,
		} {
			It(fmt.Sprintf("gives %s exactly its policy-file permissions", role), func() {
				want, err := otherCasbin.GetUserPermissions(string(role))
				Expect(err).NotTo(HaveOccurred())

				uid := "rbac-" + uuid.NewString()
				_, client := rbacUser(uid, uid, role, "active")

				// chef, waiter and cashier hold no permissions:view row, so the
				// endpoint the sidebar is built from 403s for them. Unchanged here.
				if holds(want, "permissions:view") {
					Expect(rbacPermissions(client)).To(ConsistOf(want),
						"GET /users/permissions is the whole sidebar")
				} else {
					Expect(rbacGet(client, permissionsPath).StatusCode).To(Equal(http.StatusForbidden))
				}

				for path, permission := range routes {
					status := rbacGet(client, path).StatusCode
					if holds(want, permission) {
						Expect(status).NotTo(Equal(http.StatusForbidden),
							"%s holds %s so %s must not 403", role, permission, path)
					} else {
						Expect(status).To(Equal(http.StatusForbidden),
							"%s does not hold %s so %s must 403", role, permission, path)
					}
				}
			})
		}
	})

	Context("a role change on an ordinary account", func() {
		It("takes effect on the next request, with no restart", func(ctx SpecContext) {
			uid := "rbac-" + uuid.NewString()
			user, client := rbacUser(uid, uid, models.RoleAdmin, "active")

			Expect(rbacGet(client, usersPath).StatusCode).To(Equal(http.StatusOK))
			Expect(rbacPermissions(client)).To(ConsistOf(adminPerms))

			setRole(ctx, user, models.RoleStaff)

			Expect(rbacGet(client, usersPath).StatusCode).To(Equal(http.StatusForbidden),
				"the downgrade must apply to the very next request")
			Expect(rbacPermissions(client)).To(ConsistOf(staffPerms))
		})

		It("applies an upgrade written by another process without a reload", func(ctx SpecContext) {
			uid := "rbac-" + uuid.NewString()
			user, client := rbacUser(uid, uid, models.RoleStaff, "active")

			Expect(rbacGet(client, usersPath).StatusCode).To(Equal(http.StatusForbidden))

			setRole(ctx, user, models.RoleAdmin)

			Expect(rbacGet(client, usersPath).StatusCode).To(Equal(http.StatusOK))
			Expect(rbacPermissions(client)).To(ConsistOf(adminPerms))
		})

		It("never leaves the old and the new role both in effect under concurrent traffic", func(ctx SpecContext) {
			uid := "rbac-" + uuid.NewString()
			user, client := rbacUser(uid, uid, models.RoleAdmin, "active")

			// Each goroutine keeps requesting across the write, so the change lands
			// inside the window rather than before or after it.
			const (
				parallel = 16
				rounds   = 8
			)
			observed := make([][][]string, parallel)

			var start, done sync.WaitGroup
			start.Add(1)
			for i := 0; i < parallel; i++ {
				done.Add(1)
				go func(i int) {
					defer GinkgoRecover()
					defer done.Done()
					start.Wait()
					for r := 0; r < rounds; r++ {
						observed[i] = append(observed[i], rbacPermissions(client))
					}
				}(i)
			}

			start.Done()
			setRole(ctx, user, models.RoleStaff)
			done.Wait()

			// A union would mean a binding survived the change somewhere. Every
			// observation must be exactly one of the two roles.
			for i, rounds := range observed {
				for r, got := range rounds {
					Expect(got).To(Or(ConsistOf(adminPerms), ConsistOf(staffPerms)),
						"goroutine %d request %d saw neither the old nor the new role cleanly", i, r)
				}
			}
			Expect(rbacPermissions(client)).To(ConsistOf(staffPerms))
			Expect(rbacGet(client, usersPath).StatusCode).To(Equal(http.StatusForbidden))
		})
	})

	Context("an account whose UID the policy file binds", func() {
		It("keeps its file-granted roles when users.role is changed to the narrowest role", func(ctx SpecContext) {
			user, client := rbacUser(maintenanceUID, maintenanceUID, models.RoleAdmin, "active")

			setRole(ctx, user, models.RoleStaff)

			Expect(rbacGet(client, usersPath).StatusCode).To(Equal(http.StatusOK),
				"a g row, not users.role, decides for this UID")
			Expect(rbacPermissions(client)).To(ConsistOf(union(adminPerms, developerPerm)))
		})

		It("keeps them when the account is deleted and recreated on the same UID", func(ctx SpecContext) {
			user, client := rbacUser(maintenanceUID, maintenanceUID, models.RoleStaff, "active")

			Expect(otherProcess.DeleteUser(ctx, user.ID.String())).To(Succeed())
			Expect(rbacGet(client, usersPath).StatusCode).To(Equal(http.StatusForbidden),
				"deleting the row revokes the account, g rows or not")

			_, revived := rbacUser(maintenanceUID, maintenanceUID, models.RoleStaff, "active")
			Expect(rbacPermissions(revived)).To(ConsistOf(union(adminPerms, developerPerm)))
		})
	})

	// The UID repair at authorization.go:61 logs and continues on failure, so the row
	// can keep a UID that is not the one enforced on. GET /users/permissions must still
	// report the subject that was enforced: the sidebar it builds has to agree with what
	// the endpoints answer, in both directions.
	Context("a request whose UID repair cannot persist", func() {
		// blockUIDRepair makes any UPDATE that changes this row's uid raise, which is
		// the failure the middleware swallows.
		blockUIDRepair := func(userID uuid.UUID) {
			name := "rbac_block_uid_repair_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			Expect(tenv.DB.Exec(fmt.Sprintf(`
				CREATE FUNCTION %[1]s() RETURNS trigger AS $fn$
				BEGIN
					IF NEW.uid IS DISTINCT FROM OLD.uid AND OLD.id = '%[2]s' THEN
						RAISE EXCEPTION 'uid repair blocked';
					END IF;
					RETURN NEW;
				END $fn$ LANGUAGE plpgsql;
				CREATE TRIGGER %[1]s_trg BEFORE UPDATE ON users
				FOR EACH ROW EXECUTE FUNCTION %[1]s();`, name, userID)).Error).NotTo(HaveOccurred())

			DeferCleanup(func() {
				Expect(tenv.DB.Exec(fmt.Sprintf(
					`DROP TRIGGER IF EXISTS %[1]s_trg ON users; DROP FUNCTION IF EXISTS %[1]s();`, name)).Error).
					NotTo(HaveOccurred())
			})
		}

		It("does not advertise a policy-file grant the stale row holds and the caller does not", func(ctx SpecContext) {
			// The row still carries a maintenance UID; the token presents an ordinary
			// one. Deriving from the row would report admin + developer.
			tokenUID := "rbac-ordinary-" + uuid.NewString()
			user, client := rbacUser(maintenanceUID, tokenUID, models.RoleStaff, "active")
			blockUIDRepair(user.ID)

			Expect(rbacPermissions(client)).To(ConsistOf(staffPerms),
				"the list must describe the token UID, which is what was enforced")
			Expect(rbacGet(client, usersPath).StatusCode).To(Equal(http.StatusForbidden),
				"and the API must agree with the list")

			var stored models.User
			Expect(tenv.DB.WithContext(ctx).First(&stored, "id = ?", user.ID).Error).NotTo(HaveOccurred())
			Expect(stored.UID).To(Equal(maintenanceUID), "precondition: the repair really did fail")
		})

		It("still reports the grant the caller does hold when the stale row hides it", func(ctx SpecContext) {
			// The mirror case: the token presents the maintenance UID, the row carries
			// an old one. Deriving from the row would report staff only.
			user, client := rbacUser("rbac-stale-"+uuid.NewString(), maintenanceUID, models.RoleStaff, "active")
			blockUIDRepair(user.ID)

			Expect(rbacPermissions(client)).To(ConsistOf(union(adminPerms, developerPerm)))
			Expect(rbacGet(client, usersPath).StatusCode).To(Equal(http.StatusOK))
		})
	})

	Context("callers that must behave exactly as before", func() {
		It("refuses an inactive account before any policy lookup", func() {
			uid := "rbac-" + uuid.NewString()
			_, client := rbacUser(uid, uid, models.RoleAdmin, "inactive")

			resp := rbacGet(client, usersPath)
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
			Expect(testutil.ParseResponse(resp)["error"]).To(Equal("User is inactive"))
		})

		It("refuses a pending account before any policy lookup", func() {
			uid := "rbac-" + uuid.NewString()
			_, client := rbacUser(uid, uid, models.RoleAdmin, "pending")

			resp := rbacGet(client, usersPath)
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
			Expect(testutil.ParseResponse(resp)["error"]).To(Equal("User is pending"))
		})

		It("refuses a valid token whose email has no users row", func() {
			client := rbacClient("rbac-"+uuid.NewString(), fmt.Sprintf("ghost-%s@cim.local", uuid.NewString()), "ghost")

			resp := rbacGet(client, usersPath)
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
			Expect(testutil.ParseResponse(resp)["error"]).To(Equal("Failed to fetch user information"))
		})

		It("keeps an ordinary account's permissions when its stored UID is stale", func(ctx SpecContext) {
			tokenUID := "rbac-token-" + uuid.NewString()
			user, client := rbacUser("rbac-stale-"+uuid.NewString(), tokenUID, models.RoleAdmin, "active")

			Expect(rbacGet(client, usersPath).StatusCode).To(Equal(http.StatusOK))
			Expect(rbacPermissions(client)).To(ConsistOf(adminPerms))

			var stored models.User
			Expect(tenv.DB.WithContext(ctx).First(&stored, "id = ?", user.ID).Error).NotTo(HaveOccurred())
			Expect(stored.UID).To(Equal(tokenUID), "the row must be repointed at the token UID")
			Expect(stored.Role).To(Equal(models.RoleAdmin), "the repair must not touch the role")
		})
	})
})

func holds(permissions []string, want string) bool {
	for _, p := range permissions {
		if p == want {
			return true
		}
	}
	return false
}

func union(sets ...[]string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, set := range sets {
		for _, s := range set {
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
