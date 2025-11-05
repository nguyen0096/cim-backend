package scenarios

import (
	"cim-backend/test/components/helpers"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ComponentTestSuite struct {
	suite.Suite
	sharedTestContainer *helpers.TestContainer
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
