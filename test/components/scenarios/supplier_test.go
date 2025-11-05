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

func (suite *ComponentTestSuite) TestCreateAndGetSupplier() {
	t := suite.T()
	t.Run("Create and Get Supplier", func(t *testing.T) {

		// Create test user with unique email
		user, err := helpers.CreateTestUser(context.Background(), suite.sharedTestContainer.DB, "test-supplier@example.com", "Test User", models.RoleAdmin)
		require.NoError(t, err)

		// Get auth token
		token := helpers.GetAuthToken(suite.sharedTestContainer.MockAuth, user.UID, user.Email, user.Name)

		// Create a supplier
		supplierData := map[string]interface{}{
			"name":          "Test Supplier",
			"contact_email": "supplier@example.com",
			"contact_phone": "+1234567890",
			"address":       "123 Test St",
		}

		resp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/suppliers", token, supplierData)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 201, resp.StatusCode)

		var supplierResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&supplierResp)
		require.NoError(t, err)

		supplierID := supplierResp["id"]
		assert.NotNil(t, supplierID)
		assert.Equal(t, "Test Supplier", supplierResp["name"])

		// Get the supplier
		resp, err = helpers.MakeRequest(t, "GET", suite.sharedTestContainer.BaseURL+"/api/v1/suppliers/"+helpers.ToString(supplierID), token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		var getSupplierResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&getSupplierResp)
		require.NoError(t, err)
		assert.Equal(t, "Test Supplier", getSupplierResp["name"])
	})
}
