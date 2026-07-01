package apptest

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"cim-backend/internal/auth"
	"cim-backend/internal/repository"
	"cim-backend/internal/services"
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
		tenv.DefaultContext = pkg.WithUserID(tenv.DefaultContext, "test-admi\n-uid")
	})

	GinkgoWriter.Printf("Base URL: %s\n", tenv.BaseURL)
})

// CleanupTestContainer cleans up the test container after all tests
var _ = AfterSuite(func(ctx SpecContext) {
	By("Shutdown test server", tenv.ShutdownServer)
	By("Close database connection", tenv.CloseDBConn)
	By("Deprovision database", tenv.DeprovisionDB)
	By("Clean upload files", tenv.CleanUploadFiles)
})

func TestApplication(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CIM Backend Test Suite")
}

// buildReconInventoryService wires the real inventoryService from the suite DB,
// mirroring internal/server/server.go so reconciliation specs drive production
// code (incl. the user-role + casbin lookup that excludes manager-owned sessions
// from the readiness signal).
func buildReconInventoryService(base repository.BaseRepository) services.InventoryService {
	casbinService, err := auth.NewCasbinService(tenv.DB, tenv.Config.Casbin)
	Expect(err).NotTo(HaveOccurred(), "Failed to build casbin service")
	return services.NewInventoryService(
		repository.NewInventoryRepository(base),
		repository.NewInventoryItemRepository(base),
		repository.NewInventorySubmissionRepository(base),
		repository.NewReconciliationSnapshotRepository(base),
		repository.NewReconciliationRequestItemRepository(base),
		repository.NewProductRepository(base),
		repository.NewUserRepository(base, tenv.Config.Environment),
		casbinService,
		nil, // fileStorageService: unused by the reconciliation paths
		base,
		tenv.DB,
	)
}
