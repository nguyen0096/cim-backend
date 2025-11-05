package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/test/components/helpers"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *ComponentTestSuite) TestCreateAndGetSupplier() {
	t := suite.T()
	// Create a supplier
	supplierData := map[string]interface{}{
		"name":          "Test Supplier",
		"contact_email": "supplier@example.com",
		"contact_phone": "+1234567890",
		"address":       "123 Test St",
	}

	t.Run("Should create and get supplier when user has admin or accountant role", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				uniqueEmail := fmt.Sprintf("test-supplier-%s@example.com", uuid.New().String())
				// Create test user with unique email
				user, err := helpers.CreateTestUser(context.Background(), suite.sharedTestContainer.DB, uniqueEmail, "Test User", role)
				require.NoError(t, err)

				// Get auth token
				token := helpers.GetAuthToken(suite.sharedTestContainer.MockAuth, user.UID, user.Email, user.Name)

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
	})

	t.Run("Should not create supplier when user has staff or bot_form role", func(t *testing.T) {
		roles := []models.UserRole{models.RoleStaff, models.RoleBotForm}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				uniqueEmail := fmt.Sprintf("test-supplier-%s@example.com", uuid.New().String())
				// Create test user with unique email
				user, err := helpers.CreateTestUser(context.Background(), suite.sharedTestContainer.DB, uniqueEmail, "Test User", role)
				require.NoError(t, err)

				// Get auth token
				token := helpers.GetAuthToken(suite.sharedTestContainer.MockAuth, user.UID, user.Email, user.Name)
				resp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/suppliers", token, supplierData)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, 403, resp.StatusCode)

				var errorResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&errorResp)
				require.NoError(t, err)
				assert.Equal(t, "Access denied: "+string(role)+" role cannot create suppliers", errorResp["error"])
			})
		}
	})
}

func (suite *ComponentTestSuite) TestUpdateSupplier() {
	t := suite.T()
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
	db := suite.sharedTestContainer.DB
	testSupplier := models.Supplier{
		Name:         "Test Supplier",
		ContactEmail: "supplier@example.com",
		ContactPhone: "+1234567890",
		Address:      "123 Test St",
	}
	err := db.WithContext(ctx).Create(&testSupplier).Error
	require.NoError(t, err, "Failed to create supplier")
	supplierID := testSupplier.ID

	updatedSupplierData := map[string]interface{}{
		"name":          "Test Supplier Edited",
		"contact_email": "supplier_edited@example.com",
		"contact_phone": "+1234567891",
		"address":       "123 Test St Edited",
	}

	t.Run("should update supplier when user has admin or accountant role", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				uniqueEmail := fmt.Sprintf("test-supplier-%s@example.com", uuid.New().String())
				// Create test user with unique email
				user, err := helpers.CreateTestUser(context.Background(), suite.sharedTestContainer.DB, uniqueEmail, "Test User", role)
				require.NoError(t, err)

				// Get auth token
				token := helpers.GetAuthToken(suite.sharedTestContainer.MockAuth, user.UID, user.Email, user.Name)

				urlPath := fmt.Sprintf("/api/v1/suppliers/%d", supplierID)
				resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, updatedSupplierData)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, 200, resp.StatusCode, urlPath)

				var updatedSupplierResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&updatedSupplierResp)
				require.NoError(t, err)
				assert.Equal(t, "Test Supplier Edited", updatedSupplierResp["name"])
				assert.Equal(t, "supplier_edited@example.com", updatedSupplierResp["contact_email"])
				assert.Equal(t, "+1234567891", updatedSupplierResp["contact_phone"])
				assert.Equal(t, "123 Test St Edited", updatedSupplierResp["address"])
			})
		}
	})

	t.Run("should not update supplier when user has staff or bot_form role", func(t *testing.T) {
		roles := []models.UserRole{models.RoleStaff, models.RoleBotForm}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				uniqueEmail := fmt.Sprintf("test-supplier-%s@example.com", uuid.New().String())
				user, err := helpers.CreateTestUser(context.Background(), suite.sharedTestContainer.DB, uniqueEmail, "Test User", role)
				require.NoError(t, err)

				// Get auth token
				token := helpers.GetAuthToken(suite.sharedTestContainer.MockAuth, user.UID, user.Email, user.Name)
				urlPath := fmt.Sprintf("/api/v1/suppliers/%d", supplierID)
				resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, updatedSupplierData)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, 403, resp.StatusCode)

				var errorResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&errorResp)
				require.NoError(t, err)
				assert.Equal(t, "Access denied: "+string(role)+" role cannot update suppliers", errorResp["error"])
			})
		}
	})
}
