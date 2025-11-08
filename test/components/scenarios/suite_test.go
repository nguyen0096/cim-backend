package scenarios

import (
	"cim-backend/internal/models"
	"cim-backend/test/components/helpers"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type ComponentTestSuite struct {
	suite.Suite
	sharedTestContainer *helpers.TestContainer
}

func (c *ComponentTestSuite) CreateUniqueEmailAndToken(role models.UserRole) (*models.User, string, error) {
	email := fmt.Sprintf("test-user-%s@example.com", uuid.New().String())
	user, err := helpers.CreateTestUser(context.Background(), c.sharedTestContainer.DB, email, email, role)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create test user: %w", err)
	}
	token := helpers.GetAuthToken(c.sharedTestContainer.MockAuth, user.UID, user.Email, user.Name)
	return user, token, nil
}

func (suite *ComponentTestSuite) SetupSuite() {
	ctx := context.Background()

	// Setup test containers once
	tc, err := helpers.SetupTestContainers(ctx)
	if err != nil {
		fmt.Printf("Failed to setup test containers: %v\n", err)
		os.Exit(1)
	}

	suite.sharedTestContainer = tc
}

func (suite *ComponentTestSuite) TearDownSuite() {
	if suite.sharedTestContainer != nil {
		suite.sharedTestContainer.Cleanup()
	}
}

func TestComponentTestSuite(t *testing.T) {
	suite.Run(t, new(ComponentTestSuite))
}
