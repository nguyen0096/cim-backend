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
	"gorm.io/gorm"
)

func (suite *ComponentTestSuite) TestCreateAndGetSupplier() {
	t := suite.T()
	// Create a supplier
	supplierName := fmt.Sprintf("TestCreateAndGetSupplier %s", uuid.New().String())
	supplierContactEmail := fmt.Sprintf("%s@create-get-supplier.com", uuid.New().String())
	supplierContactPhone := "+1234567890"
	supplierAddress := "123 Test St"
	supplierData := map[string]interface{}{
		"name":          supplierName,
		"contact_email": supplierContactEmail,
		"contact_phone": supplierContactPhone,
		"address":       supplierAddress,
	}

	t.Run("Should create and get supplier", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)
				resp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/suppliers", token, supplierData)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, 201, resp.StatusCode)

				var supplierResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&supplierResp)
				require.NoError(t, err)

				supplierID := supplierResp["id"]
				assert.NotNil(t, supplierID)
				assert.Equal(t, supplierName, supplierResp["name"])

				// Get the supplier
				resp, err = helpers.MakeRequest(t, "GET", suite.sharedTestContainer.BaseURL+"/api/v1/suppliers/"+helpers.ToString(supplierID), token, nil)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, 200, resp.StatusCode)

				var getSupplierResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&getSupplierResp)
				require.NoError(t, err)
				assert.Equal(t, supplierName, getSupplierResp["name"])
			})
		}
	})

	t.Run("Should not create supplier", func(t *testing.T) {
		roles := []models.UserRole{models.RoleStaff, models.RoleBotForm}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)
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
		Name:         fmt.Sprintf("Test Supplier %s", uuid.New().String()),
		ContactEmail: fmt.Sprintf("supplier_%s@example.com", uuid.New().String()),
		ContactPhone: "+1234567890",
		Address:      "123 Test St",
	}
	err := db.WithContext(ctx).Create(&testSupplier).Error
	require.NoError(t, err, "Failed to create supplier")
	supplierID := testSupplier.ID

	updatedSupplierName := fmt.Sprintf("Test Supplier Edited %s", uuid.New().String())
	updatedSupplierContactEmail := fmt.Sprintf("supplier_edited_%s@example.com", uuid.New().String())
	updatedSupplierContactPhone := "+1234567891"
	updatedSupplierAddress := "123 Test St Edited"
	updatedSupplierData := map[string]interface{}{
		"name":          updatedSupplierName,
		"contact_email": updatedSupplierContactEmail,
		"contact_phone": updatedSupplierContactPhone,
		"address":       updatedSupplierAddress,
	}

	t.Run("should update supplier", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)

				urlPath := fmt.Sprintf("/api/v1/suppliers/%d", supplierID)
				resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, updatedSupplierData)
				require.NoError(t, err)
				defer resp.Body.Close()

				var updatedSupplierResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&updatedSupplierResp)
				require.NoError(t, err)

				assert.Equal(t, 200, resp.StatusCode, urlPath, updatedSupplierResp["error"])

				assert.Equal(t, updatedSupplierName, updatedSupplierResp["name"])
				assert.Equal(t, updatedSupplierContactEmail, updatedSupplierResp["contact_email"])
				assert.Equal(t, updatedSupplierContactPhone, updatedSupplierResp["contact_phone"])
				assert.Equal(t, updatedSupplierAddress, updatedSupplierResp["address"])
			})
		}
	})

	t.Run("should not update supplier", func(t *testing.T) {
		roles := []models.UserRole{models.RoleStaff, models.RoleBotForm}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)
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

func (suite *ComponentTestSuite) TestDeleteSupplier() {
	t := suite.T()
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
	db := suite.sharedTestContainer.DB
	testSupplier := models.Supplier{
		Name:         fmt.Sprintf("Test Supplier %s", uuid.New().String()),
		ContactEmail: fmt.Sprintf("%s@delete-supplier.com", uuid.New().String()),
		ContactPhone: "+1234567890",
		Address:      "123 Test St",
	}
	err := db.WithContext(ctx).Create(&testSupplier).Error
	require.NoError(t, err, "Failed to create supplier")
	supplierID := testSupplier.ID

	t.Run("should delete supplier when user has admin role", func(t *testing.T) {
		role := models.RoleAdmin
		_, token, err := suite.CreateUniqueEmailAndToken(role)
		require.NoError(t, err)
		urlPath := fmt.Sprintf("/api/v1/suppliers/%d", supplierID)
		resp, err := helpers.MakeRequest(t, "DELETE", suite.sharedTestContainer.BaseURL+urlPath, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		err = db.WithContext(ctx).First(&models.Supplier{}, "id = ?", supplierID).Error
		assert.Equal(t, gorm.ErrRecordNotFound, err)
	})

	t.Run("should not delete supplier", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAccountant, models.RoleStaff, models.RoleBotForm}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)
				urlPath := fmt.Sprintf("/api/v1/suppliers/%d", supplierID)
				resp, err := helpers.MakeRequest(t, "DELETE", suite.sharedTestContainer.BaseURL+urlPath, token, nil)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, 403, resp.StatusCode)

				var errorResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&errorResp)
				require.NoError(t, err)
				assert.Equal(t, "Access denied: "+string(role)+" role cannot delete suppliers", errorResp["error"])
			})
		}
	})
}

func (suite *ComponentTestSuite) TestUpdateSupplierStatus() {
	t := suite.T()
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
	db := suite.sharedTestContainer.DB
	testSupplier := models.Supplier{
		Name:         fmt.Sprintf("Test Supplier %s", uuid.New().String()),
		ContactEmail: fmt.Sprintf("supplier_%s@example.com", uuid.New().String()),
		ContactPhone: "+1234567890",
		Address:      "123 Test St",
		Status:       "active",
	}
	err := db.WithContext(ctx).Create(&testSupplier).Error
	require.NoError(t, err, "Failed to create supplier")
	supplierID := testSupplier.ID

	t.Run("should update supplier status", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)
				urlPath := fmt.Sprintf("/api/v1/suppliers/%d/status", supplierID)
				resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, map[string]interface{}{"status": "inactive"})
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 200, resp.StatusCode)

				var updatedSupplierResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&updatedSupplierResp)
				require.NoError(t, err)
				assert.Equal(t, "inactive", updatedSupplierResp["status"])

				var updatedSupplier models.Supplier
				err = db.WithContext(ctx).First(&updatedSupplier, "id = ?", supplierID).Error
				require.NoError(t, err)
				assert.Equal(t, "inactive", updatedSupplier.Status)
			})
		}
	})

	t.Run("should not update supplier status", func(t *testing.T) {
		roles := []models.UserRole{models.RoleStaff, models.RoleBotForm}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				uniqueEmail := fmt.Sprintf("test-supplier-%s@example.com", uuid.New().String())
				user, err := helpers.CreateTestUser(context.Background(), suite.sharedTestContainer.DB, uniqueEmail, "Test User", role)
				require.NoError(t, err)

				// Get auth token
				token := helpers.GetAuthToken(suite.sharedTestContainer.MockAuth, user.UID, user.Email, user.Name)
				urlPath := fmt.Sprintf("/api/v1/suppliers/%d/status", supplierID)
				resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, map[string]interface{}{"status": "inactive"})
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
