package scenarios

import (
	"context"
	"encoding/json"
	"testing"

	"cim-backend/internal/models"
	"cim-backend/test/components/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *ComponentTestSuite) TestInventoryOperations() {
	t := suite.T()
	t.Run("Inventory Operations", func(t *testing.T) {

		// Create test user with unique email
		user, err := helpers.CreateTestUser(context.Background(), suite.sharedTestContainer.DB, "test-inventory@example.com", "Test User", models.RoleAdmin)
		require.NoError(t, err)

		// Get auth token
		token := helpers.GetAuthToken(suite.sharedTestContainer.MockAuth, user.UID, user.Email, user.Name)

		// Create an inventory
		inventoryData := map[string]interface{}{
			"name":        "Test Inventory",
			"description": "Test Inventory Description",
			"location":    "Test Location",
		}

		resp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/inventories", token, inventoryData)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 201, resp.StatusCode)

		var inventoryResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&inventoryResp)
		require.NoError(t, err)

		inventoryID := inventoryResp["id"]
		assert.NotNil(t, inventoryID)
		assert.Equal(t, "Test Inventory", inventoryResp["name"])

		// List inventories
		resp, err = helpers.MakeRequest(t, "GET", suite.sharedTestContainer.BaseURL+"/api/v1/inventories", token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		// ListInventory returns an array directly, not wrapped in a map
		var listResp []interface{}
		err = json.NewDecoder(resp.Body).Decode(&listResp)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(listResp), 1)
	})
}
