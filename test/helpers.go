package apptest

import (
	"cim-backend/internal/models"
	"cim-backend/test/components/helpers"
	"context"
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	testContainer *helpers.TestContainer
)

// SetupTestContainer initializes the test container before all tests
var _ = BeforeSuite(func() {
	ctx := context.Background()

	By("Setting up test containers")
	tc, err := helpers.SetupTestContainers(ctx)
	Expect(err).NotTo(HaveOccurred(), "Failed to setup test containers")
	Expect(tc).NotTo(BeNil())

	testContainer = tc

	GinkgoWriter.Printf("Test container initialized successfully\n")
	GinkgoWriter.Printf("Base URL: %s\n", testContainer.BaseURL)
})

// CleanupTestContainer cleans up the test container after all tests
var _ = AfterSuite(func() {
	By("Cleaning up test containers")
	if testContainer != nil {
		testContainer.Cleanup()
		GinkgoWriter.Printf("Test container cleaned up successfully\n")
	}
})

// CreateUniqueUserWithToken is a helper function to create a test user with a token
func CreateUniqueUserWithToken(role models.UserRole) (*models.User, string, error) {
	email := fmt.Sprintf("test-user-%s@example.com", uuid.New().String())
	user, err := helpers.CreateTestUser(context.Background(), testContainer.DB, email, email, role)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create test user: %w", err)
	}
	token := helpers.GetAuthToken(testContainer.MockAuth, user.UID, user.Email, user.Name)
	return user, token, nil
}

// GetTestContainer returns the shared test container
func GetTestContainer() *helpers.TestContainer {
	return testContainer
}
