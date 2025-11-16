package apptest

import (
	"cim-backend/pkg"
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	tenv *TestEnv
)

// SetupTestContainer initializes the test container before all tests
var _ = BeforeSuite(func(ctx SpecContext) {
	tenv = &TestEnv{}

	By("Provision database", tenv.provisionDB)
	By("Load configuration", tenv.loadConfig)
	By("Initialize database connection", tenv.initDBConn)
	By("Run migrations", tenv.runMigrations)
	By("Initialize dependencies", tenv.initDependencies)
	By("Initialize mock dependencies", tenv.initMockDependencies)
	By("By initializing and starting server", tenv.initAndStartServer)

	By("Initialize default test context", func() {
		tenv.DefaultContext = pkg.WithUserEmail(context.Background(), "test-admin@cim.local")
		tenv.DefaultContext = pkg.WithUserID(tenv.DefaultContext, "test-admin-uid")
	})

	GinkgoWriter.Printf("Base URL: %s\n", tenv.BaseURL)
})

// CleanupTestContainer cleans up the test container after all tests
var _ = AfterSuite(func(ctx SpecContext) {
	By("Shutdown test server", tenv.shutdownServer)
	By("Close database connection", tenv.closeDBConn)
	By("Deprovision database", tenv.deprovisionDB)
})

func TestApplication(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CIM Backend Test Suite")
}
