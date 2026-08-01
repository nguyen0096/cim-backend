package apptest

import (
	"fmt"
	"time"

	firebaseAuth "firebase.google.com/go/v4/auth"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil"
)

// A maintenance account authenticating while its users row still carries an old UID
// takes the repair branch at internal/middleware/authorization.go:58-65 before any
// enforcement. Enforcement uses the token UID, so the repair only keeps the row
// honest; the account must reach both of its g rows either way.
var _ = Describe("Maintenance account UID repair", func() {
	const maintenanceUID = "demoMaintAdminUid00000000000"

	It("keeps both bindings when the stored UID is stale", func(ctx SpecContext) {
		email := fmt.Sprintf("maint-%s@cim.local", uuid.NewString())
		user := &models.User{
			UID:    "stale-" + uuid.NewString(),
			Email:  email,
			Name:   email,
			Role:   models.RoleAdmin,
			Status: "active",
			Type:   models.UserTypeUser,
		}
		Expect(tenv.ContextfulDB().Create(user).Error).NotTo(HaveOccurred())
		DeferCleanup(func() { tenv.DB.Unscoped().Delete(user) })

		token := "token-maint-" + uuid.NewString()
		tenv.AuthMock.On("VerifyToken", mock.Anything, token).Return(&firebaseAuth.Token{
			UID:      maintenanceUID,
			Claims:   map[string]interface{}{"email": email, "name": user.Name},
			Expires:  time.Now().Add(time.Hour).Unix(),
			IssuedAt: time.Now().Unix(),
			Subject:  maintenanceUID,
		}, nil)

		client := &testutil.Client{BaseURL: tenv.BaseURL, AuthToken: pkg.Ptr(token)}
		resp, err := client.MakeRequest("GET", "/api/v1/users/permissions", nil, testutil.WithAuth())
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(200))

		permissions := testutil.ParseResponse(resp)["permissions"]
		Expect(permissions).To(ContainElement("users:delete"), "admin binding must survive the repair")
		Expect(permissions).To(ContainElement("developer-tools:view"), "developer binding must survive the repair")

		var stored models.User
		Expect(tenv.DB.WithContext(ctx).First(&stored, "id = ?", user.ID).Error).NotTo(HaveOccurred())
		Expect(stored.UID).To(Equal(maintenanceUID), "the row must be repointed at the token UID")
	})
})
