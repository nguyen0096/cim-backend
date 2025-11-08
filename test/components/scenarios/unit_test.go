package scenarios

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/test/components/helpers"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *ComponentTestSuite) TestCreateAndGetUnit() {
	t := suite.T()

	db := suite.sharedTestContainer.DB
	baseUnit := models.Unit{
		Name:             uuid.New().String(),
		Symbol:           "test",
		UnitType:         "general",
		ConversionFactor: 1,
	}
	err := db.WithContext(pkg.WithUserEmail(context.Background(), "test@example.com")).Create(&baseUnit).Error
	require.NoError(t, err)

	t.Run("should create and get unit", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)

				name := "Test Unit" + uuid.New().String()

				payload := map[string]interface{}{
					"name":              name,
					"symbol":            "test",
					"unit_type":         "general",
					"base_unit_id":      1,
					"conversion_factor": 1,
				}
				resp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/units", token, payload)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 201, resp.StatusCode)

				var unitResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&unitResp)
				require.NoError(t, err)
				assert.Equal(t, name, unitResp["name"])
				assert.Equal(t, "test", unitResp["symbol"])
				assert.Equal(t, "general", unitResp["unit_type"])
				assert.Equal(t, float64(1), unitResp["base_unit_id"])
				assert.Equal(t, float64(1), unitResp["conversion_factor"])
			})
		}
	})

	t.Run("should not create unit", func(t *testing.T) {
		roles := []models.UserRole{models.RoleStaff, models.RoleBotForm}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)

				payload := map[string]interface{}{
					"name":              uuid.New().String(),
					"symbol":            "test",
					"unit_type":         "general",
					"base_unit_id":      1,
					"conversion_factor": 1,
				}
				resp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/units", token, payload)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 403, resp.StatusCode)

				var errorResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&errorResp)
				require.NoError(t, err)
				assert.Equal(t, fmt.Sprintf("Access denied: %s role cannot create units", role), errorResp["error"])
			})
		}
	})

	t.Run("when unit type is not same as base unit", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		payload := map[string]interface{}{
			"name":              "Test Unit" + uuid.New().String(),
			"symbol":            "test",
			"unit_type":         "volume",
			"base_unit_id":      baseUnit.ID,
			"conversion_factor": 1,
		}

		resp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/units", token, payload)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 400, resp.StatusCode)

		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)
		assert.Equal(t, "Validation failed: base unit must have the same unit_type", errorResp["error"])
	})
}
