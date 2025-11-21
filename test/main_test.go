package apptest

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"cim-backend/pkg"
	"cim-backend/pkg/testutil"
)

var (
	tenv *testutil.TestBox
)

// SetupTestContainer initializes the test container before all tests
var _ = BeforeSuite(func(ctx SpecContext) {
	tenv = &testutil.TestBox{}

	By("Provision database", tenv.ProvisionDB)
	By("Load configuration", tenv.GetConfig)
	By("Initialize database connection", tenv.InitDBConn)
	By("Run migrations", tenv.RunMigrations)
	By("Initialize dependencies", tenv.InitDependencies)
	By("Initialize mock dependencies", tenv.InitMockDependencies)
	By("By initializing and starting server", tenv.InitAndStartServer)

	By("Initialize default test context", func() {
		tenv.DefaultContext = pkg.WithUserEmail(context.Background(), "test-admin@cim.local")
		tenv.DefaultContext = pkg.WithUserID(tenv.DefaultContext, "test-admin-uid")
	})

	GinkgoWriter.Printf("Base URL: %s\n", tenv.BaseURL)
})

// CleanupTestContainer cleans up the test container after all tests
var _ = AfterSuite(func(ctx SpecContext) {
	By("Shutdown test server", tenv.ShutdownServer)
	By("Close database connection", tenv.CloseDBConn)
	By("Deprovision database", tenv.DeprovisionDB)
})

func TestApplication(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CIM Backend Test Suite")
}
