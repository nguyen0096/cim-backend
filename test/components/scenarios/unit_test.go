package scenarios

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/test/components/helpers"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *ComponentTestSuite) setupUnitTestData(t *testing.T) ([]models.Unit) {
	db := suite.sharedTestContainer.DB
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")

	baseUnit := models.Unit{
		Name: fmt.Sprintf("Base Unit %s", uuid.New().String()),
		Symbol: "bu",
		UnitType: "general",
		Level: 1,
		ConversionFactor: 1,
	}
	err := db.WithContext(ctx).Create(&baseUnit).Error
	require.NoError(t, err)

	unitLevel2 := models.Unit{
		BaseUnitID:       pkg.Ptr(baseUnit.ID),
		Name:             fmt.Sprintf("Derived Unit 1 %s", uuid.New().String()),
		Symbol:           "du1",
		UnitType:         "general",
		Level:            2,
		ConversionFactor: 2,
	}
	err = db.WithContext(ctx).Create(&unitLevel2).Error
	require.NoError(t, err)
	unitLevel3 := models.Unit{
		BaseUnitID:       pkg.Ptr(unitLevel2.ID),
		Name:             fmt.Sprintf("Derived Unit 2 %s", uuid.New().String()),
		Symbol:           "du2",
		UnitType:         "general",
		Level:            3,
		ConversionFactor: 4,
	}
	err = db.WithContext(ctx).Create(&unitLevel3).Error
	require.NoError(t, err)
	unitLevel4 := models.Unit{
		BaseUnitID:       pkg.Ptr(unitLevel3.ID),
		Name:             fmt.Sprintf("Derived Unit 3 %s", uuid.New().String()),
		Symbol:           "du3",
		UnitType:         "general",
		Level:            4,
		ConversionFactor: 8,
	}
	err = db.WithContext(ctx).Create(&unitLevel4).Error
	require.NoError(t, err)
    return []models.Unit{baseUnit, unitLevel2, unitLevel3, unitLevel4}
}


func (suite *ComponentTestSuite) TestCreateUnit() {
	t := suite.T()
	db := suite.sharedTestContainer.DB

	units := suite.setupUnitTestData(t)

	t.Cleanup(func() {
		db.WithContext(pkg.WithUserEmail(context.Background(), "test@example.com")).Delete(&units)
	})

	t.Run("should create and get unit", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)

				name := fmt.Sprintf("Test Unit %s", uuid.New().String())

				payload := map[string]interface{}{
					"name":              name,
					"symbol":            "test",
					"unit_type":         "general",
					"base_unit_id":      units[0].ID,
					"conversion_factor": 10,
					"decimal_places":    2,
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
				assert.Equal(t, float64(units[0].ID), unitResp["base_unit_id"])
				assert.Equal(t, float64(10), unitResp["conversion_factor"])
				assert.Equal(t, float64(2), unitResp["decimal_places"])
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

		t.Run("when reaching maximum hierarchy depth", func(t *testing.T) {
			_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
			require.NoError(t, err)

			payload := map[string]interface{}{
				"name":              "Test Unit" + uuid.New().String(),
				"symbol":            "test",
				"unit_type":         "general",
				"base_unit_id":      units[3].ID,
				"conversion_factor": 1,
			}

			resp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/units", token, payload)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, 400, resp.StatusCode)

			var errorResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&errorResp)
			require.NoError(t, err)
			assert.Equal(t, "Validation failed: cannot create/update unit: base unit is at level 4, which would result in level 5. Maximum allowed hierarchy depth is 4 levels", errorResp["error"])
		})
	})
}

func (suite *ComponentTestSuite) TestGetUnit() {
	t := suite.T()
	db := suite.sharedTestContainer.DB

	units := suite.setupUnitTestData(t)
	t.Cleanup(func() {
		db.WithContext(pkg.WithUserEmail(context.Background(), "test@example.com")).Delete(&units)
	})

	t.Run("should get base unit with all derived units", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		urlPath := fmt.Sprintf("%s/api/v1/units/%d", suite.sharedTestContainer.BaseURL, units[0].ID)
		resp, err := helpers.MakeRequest(t, "GET", urlPath, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		var unitResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&unitResp)
		require.NoError(t, err)
		assert.Equal(t, units[0].Name, unitResp["name"])
		assert.Equal(t, units[0].Symbol, unitResp["symbol"])
		assert.Equal(t, units[0].UnitType, unitResp["unit_type"])
		assert.Equal(t, float64(1), unitResp["level"])
		assert.Equal(t, units[0].ConversionFactor, unitResp["conversion_factor"])
		assert.Equal(t, float64(1), unitResp["conversion_factor_to_current"])
		assert.Equal(t, 3, len(unitResp["derived_units"].([]interface{})))
		currentConversionFactor := 1.0
		for i, derivedUnit := range unitResp["derived_units"].([]interface{}) {
			derivedUnitMap := derivedUnit.(map[string]interface{})
			assert.Equal(t, float64(units[i+1].ID), derivedUnitMap["id"])
			assert.Equal(t, units[i+1].Name, derivedUnitMap["name"])
			assert.Equal(t, units[i+1].Symbol, derivedUnitMap["symbol"])
			assert.Equal(t, units[i+1].UnitType, derivedUnitMap["unit_type"])
			assert.Equal(t, float64(i+2), derivedUnitMap["level"])
			assert.Equal(t, units[i+1].ConversionFactor, derivedUnitMap["conversion_factor"])
			currentConversionFactor /= float64(units[i+1].ConversionFactor)
			assert.Equal(t, float64(currentConversionFactor), derivedUnitMap["conversion_factor_to_current"])
		}
	})

	t.Run("should get derived unit with base unit and all derived units with conversion factor to current unit", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		urlPath := fmt.Sprintf("%s/api/v1/units/%d", suite.sharedTestContainer.BaseURL, units[1].ID)
		resp, err := helpers.MakeRequest(t, "GET", urlPath, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		var unitResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&unitResp)
		require.NoError(t, err)
		assert.Equal(t, units[1].Name, unitResp["name"])
		assert.Equal(t, units[1].Symbol, unitResp["symbol"])
		assert.Equal(t, units[1].UnitType, unitResp["unit_type"])
		assert.Equal(t, float64(2), unitResp["level"])
		assert.Equal(t, units[1].ConversionFactor, unitResp["conversion_factor"])
		assert.Equal(t, float64(1), unitResp["conversion_factor_to_current"])
		assert.Equal(t, 2, len(unitResp["derived_units"].([]interface{})))
		currentConversionFactor := 1.0
		for i, derivedUnit := range unitResp["derived_units"].([]interface{}) {
			derivedUnitMap := derivedUnit.(map[string]interface{})
			assert.Equal(t, float64(units[i+2].ID), derivedUnitMap["id"])
			assert.Equal(t, units[i+2].Name, derivedUnitMap["name"])
			assert.Equal(t, units[i+2].Symbol, derivedUnitMap["symbol"])
			assert.Equal(t, units[i+2].UnitType, derivedUnitMap["unit_type"])
			assert.Equal(t, float64(units[i+2].Level), derivedUnitMap["level"])
			assert.Equal(t, units[i+2].ConversionFactor, derivedUnitMap["conversion_factor"])
			currentConversionFactor /= float64(units[i+2].ConversionFactor)
			assert.Equal(t, float64(currentConversionFactor), derivedUnitMap["conversion_factor_to_current"])
		}

		baseUnitResp := unitResp["base_unit"].(map[string]interface{})
		assert.Equal(t, float64(units[0].ID), baseUnitResp["id"])
		assert.Equal(t, units[0].Name, baseUnitResp["name"])
		assert.Equal(t, units[0].Symbol, baseUnitResp["symbol"])
		assert.Equal(t, units[0].UnitType, baseUnitResp["unit_type"])
		assert.Equal(t, float64(units[0].Level), baseUnitResp["level"])
		assert.Equal(t, float64(1), baseUnitResp["conversion_factor"])
		assert.Equal(t, float64(units[1].ConversionFactor), baseUnitResp["conversion_factor_to_current"])
	})

	t.Run("should get derived unit with root base unit and all derived units with conversion factor to current unit", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		urlPath := fmt.Sprintf("%s/api/v1/units/%d", suite.sharedTestContainer.BaseURL, units[2].ID)
		resp, err := helpers.MakeRequest(t, "GET", urlPath, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
		var unitResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&unitResp)
		require.NoError(t, err)
		assert.Equal(t, units[2].Name, unitResp["name"])
		assert.Equal(t, units[2].Symbol, unitResp["symbol"])
		assert.Equal(t, units[2].UnitType, unitResp["unit_type"])
		assert.Equal(t, float64(3), unitResp["level"])
		assert.Equal(t, units[2].ConversionFactor, unitResp["conversion_factor"])
		assert.Equal(t, float64(1), unitResp["conversion_factor_to_current"])
		assert.Equal(t, 1, len(unitResp["derived_units"].([]interface{})))
		for i, derivedUnit := range unitResp["derived_units"].([]interface{}) {
			derivedUnitMap := derivedUnit.(map[string]interface{})
			assert.Equal(t, float64(units[i+3].ID), derivedUnitMap["id"])
			assert.Equal(t, units[i+3].Name, derivedUnitMap["name"])
			assert.Equal(t, units[i+3].Symbol, derivedUnitMap["symbol"])
			assert.Equal(t, units[i+3].UnitType, derivedUnitMap["unit_type"])
			assert.Equal(t, float64(units[i+3].Level), derivedUnitMap["level"])
			assert.Equal(t, units[i+3].ConversionFactor, derivedUnitMap["conversion_factor"])
			// For single direct child, conversion_factor_to_current = 1.0 / conversion_factor
			expectedConversionFactor := 1.0 / float64(units[i+3].ConversionFactor)
			assert.Equal(t, float64(expectedConversionFactor), derivedUnitMap["conversion_factor_to_current"])
		}

		baseUnitResp := unitResp["base_unit"].(map[string]interface{})
		assert.Equal(t, float64(units[1].ID), baseUnitResp["id"])
		assert.Equal(t, units[1].Name, baseUnitResp["name"])
		assert.Equal(t, units[1].Symbol, baseUnitResp["symbol"])
		assert.Equal(t, units[1].UnitType, baseUnitResp["unit_type"])
		assert.Equal(t, float64(units[1].Level), baseUnitResp["level"])
		assert.Equal(t, units[1].ConversionFactor, baseUnitResp["conversion_factor"])
		assert.Equal(t, float64(units[2].ConversionFactor), baseUnitResp["conversion_factor_to_current"])

		rootBaseUnitResp := baseUnitResp["base_unit"].(map[string]interface{})
		assert.Equal(t, float64(units[0].ID), rootBaseUnitResp["id"])
		assert.Equal(t, units[0].Name, rootBaseUnitResp["name"])
		assert.Equal(t, units[0].Symbol, rootBaseUnitResp["symbol"])
		assert.Equal(t, units[0].UnitType, rootBaseUnitResp["unit_type"])
		assert.Equal(t, float64(units[0].Level), rootBaseUnitResp["level"])
		assert.Equal(t, units[0].ConversionFactor, rootBaseUnitResp["conversion_factor"])
		expectedRootFactor := float64(units[1].ConversionFactor) * float64(units[2].ConversionFactor)
		assert.Equal(t, float64(expectedRootFactor), rootBaseUnitResp["conversion_factor_to_current"])
	})
}

func (suite *ComponentTestSuite) TestSearchUnit() {
	t := suite.T()
	db := suite.sharedTestContainer.DB
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")

	// Create base units first
	baseUnits := []models.Unit{
		{
			Name:             "Kilogram Search",
			Symbol:           "kg",
			UnitType:         "mass",
			ConversionFactor: 1,
		},
		{
			Name:             "Liter Search",
			Symbol:           "L",
			UnitType:         "volume",
			ConversionFactor: 1,
		},
		{
			Name:             "Meter Search",
			Symbol:           "m",
			UnitType:         "length",
			ConversionFactor: 1,
		},
	}

	err := db.WithContext(ctx).Create(&baseUnits).Error
	require.NoError(t, err)

	// Create derived units that reference base units
	derivedUnits := []models.Unit{
		{
			Name:             "Gram Search",
			Symbol:           "g",
			UnitType:         "mass",
			ConversionFactor: 1000,
			BaseUnitID:       pkg.Ptr(baseUnits[0].ID),
		},
		{
			Name:             "Milliliter Search",
			Symbol:           "ml",
			UnitType:         "volume",
			ConversionFactor: 1000,
			BaseUnitID:       pkg.Ptr(baseUnits[1].ID),
		},
	}

	err = db.WithContext(ctx).Create(&derivedUnits).Error
	require.NoError(t, err)

	// Collect all created unit IDs for cleanup
	t.Cleanup(func() {
		db.WithContext(ctx).Delete([]models.Unit{baseUnits[0], baseUnits[1], baseUnits[2], derivedUnits[0], derivedUnits[1]})
	})

	t.Run("should search units by name", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		url := fmt.Sprintf("%s/api/v1/units?q=%s", suite.sharedTestContainer.BaseURL, url.QueryEscape("Kilogram"))
		resp, err := helpers.MakeRequest(t, "GET", url, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		var searchResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&searchResp)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, searchResp["total"], float64(1))
		data := searchResp["data"].([]interface{})
		found := false
		for _, item := range data {
			unitMap := item.(map[string]interface{})
			if unitMap["name"] == "Kilogram Search" {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find unit by name")
	})

	t.Run("should search units by symbol", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		url := fmt.Sprintf("%s/api/v1/units?q=%s", suite.sharedTestContainer.BaseURL, url.QueryEscape("ml"))
		resp, err := helpers.MakeRequest(t, "GET", url, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		var searchResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&searchResp)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, searchResp["total"], float64(1))
		data := searchResp["data"].([]interface{})
		found := false
		for _, item := range data {
			unitMap := item.(map[string]interface{})
			if unitMap["symbol"] == "ml" {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find unit by symbol")
	})

	t.Run("should search units case-insensitively", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		url := fmt.Sprintf("%s/api/v1/units?q=%s", suite.sharedTestContainer.BaseURL, url.QueryEscape("LITER"))
		resp, err := helpers.MakeRequest(t, "GET", url, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		var searchResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&searchResp)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, searchResp["total"], float64(1))
		data := searchResp["data"].([]interface{})
		found := false
		for _, item := range data {
			unitMap := item.(map[string]interface{})
			if unitMap["name"] == "Liter Search" {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find unit case-insensitively")
	})

	t.Run("should search units with unit_type filter", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		url := fmt.Sprintf("%s/api/v1/units?q=%s&unit_type=%s", suite.sharedTestContainer.BaseURL, url.QueryEscape("Search"), url.QueryEscape("mass"))
		resp, err := helpers.MakeRequest(t, "GET", url, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		var searchResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&searchResp)
		require.NoError(t, err)
		data := searchResp["data"].([]interface{})
		for _, item := range data {
			unitMap := item.(map[string]interface{})
			assert.Equal(t, "mass", unitMap["unit_type"], "All results should be of type mass")
		}
	})

	t.Run("should search units with base_only filter", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		url := fmt.Sprintf("%s/api/v1/units?q=%s&base_only=true", suite.sharedTestContainer.BaseURL, url.QueryEscape("Search"))
		resp, err := helpers.MakeRequest(t, "GET", url, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		var searchResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&searchResp)
		require.NoError(t, err)
		data := searchResp["data"].([]interface{})
		for _, item := range data {
			unitMap := item.(map[string]interface{})
			assert.Nil(t, unitMap["base_unit_id"], "All results should be base units")
		}
	})

	t.Run("should search units with pagination", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		url := fmt.Sprintf("%s/api/v1/units?q=%s&limit=2&page=1", suite.sharedTestContainer.BaseURL, url.QueryEscape("Search"))
		resp, err := helpers.MakeRequest(t, "GET", url, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		var searchResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&searchResp)
		require.NoError(t, err)
		assert.Equal(t, float64(2), searchResp["limit"])
		assert.Equal(t, float64(1), searchResp["page"])
		data := searchResp["data"].([]interface{})
		assert.LessOrEqual(t, len(data), 2, "Should return at most 2 items per page")
	})

	t.Run("should return empty results for non-matching query", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		url := fmt.Sprintf("%s/api/v1/units?q=%s", suite.sharedTestContainer.BaseURL, url.QueryEscape("NonExistentUnit12345"))
		resp, err := helpers.MakeRequest(t, "GET", url, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		var searchResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&searchResp)
		require.NoError(t, err)
		assert.Equal(t, float64(0), searchResp["total"])
		data := searchResp["data"].([]interface{})
		assert.Equal(t, 0, len(data), "Should return empty array for non-matching query")
	})
}

func (suite *ComponentTestSuite) TestUpdateUnit() {
	t := suite.T()
	db := suite.sharedTestContainer.DB
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")

	// Create base units for testing
	baseUnit := models.Unit{
		Name:             "Update Test Base Unit",
		Symbol:           "utbu",
		UnitType:         "general",
		ConversionFactor: 1,
	}
	err := db.WithContext(ctx).Create(&baseUnit).Error
	require.NoError(t, err)

	// Create units for hierarchy depth test (level 2, 3, 4)
	// Note: Level must be set explicitly when creating directly, as the service normally calculates it
	level2Unit := models.Unit{
		Name:             "Level 2 Unit",
		Symbol:           "l2",
		UnitType:         "general",
		ConversionFactor: 2,
		BaseUnitID:       pkg.Ptr(baseUnit.ID),
		Level:            2,
	}
	err = db.WithContext(ctx).Create(&level2Unit).Error
	require.NoError(t, err)

	level3Unit := models.Unit{
		Name:             "Level 3 Unit",
		Symbol:           "l3",
		UnitType:         "general",
		ConversionFactor: 3,
		BaseUnitID:       pkg.Ptr(level2Unit.ID),
		Level:            3,
	}
	err = db.WithContext(ctx).Create(&level3Unit).Error
	require.NoError(t, err)

	level4Unit := models.Unit{
		Name:             "Level 4 Unit",
		Symbol:           "l4",
		UnitType:         "general",
		ConversionFactor: 4,
		BaseUnitID:       pkg.Ptr(level3Unit.ID),
		Level:            4,
	}
	err = db.WithContext(ctx).Create(&level4Unit).Error
	require.NoError(t, err)
	t.Cleanup(func() {
		db.WithContext(ctx).Delete([]models.Unit{baseUnit, level2Unit, level3Unit, level4Unit})
	})

	t.Run("should update unit", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)

				updatedName := fmt.Sprintf("Updated Unit Name %s", uuid.New().String())
				payload := map[string]interface{}{
					"name":              updatedName,
					"symbol":            "updated",
					"unit_type":         "general",
					"base_unit_id":      baseUnit.ID,
					"conversion_factor": 5,
					"decimal_places":    3,
				}

				urlPath := fmt.Sprintf("%s/api/v1/units/%d", suite.sharedTestContainer.BaseURL, level2Unit.ID)
				resp, err := helpers.MakeRequest(t, "PUT", urlPath, token, payload)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 200, resp.StatusCode)

				var unitResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&unitResp)
				require.NoError(t, err)
				assert.Equal(t, updatedName, unitResp["name"])
				assert.Equal(t, "updated", unitResp["symbol"])
				assert.Equal(t, "general", unitResp["unit_type"])
				assert.Equal(t, float64(baseUnit.ID), unitResp["base_unit_id"])
				assert.Equal(t, float64(5), unitResp["conversion_factor"])
				assert.Equal(t, float64(3), unitResp["decimal_places"])
			})
		}
	})

	t.Run("should update unit level and conversion factor to current when change base unit", func(t *testing.T) {
		// Create a fresh level2Unit for this test to avoid conflicts with previous updates
		// The previous test modifies the shared level2Unit, so we need a fresh one
		freshLevel2Unit := models.Unit{
			Name:             fmt.Sprintf("Fresh Level 2 Unit %s", uuid.New().String()),
			Symbol:           "fl2",
			UnitType:         "general",
			ConversionFactor: 2,
			BaseUnitID:       pkg.Ptr(baseUnit.ID),
			Level:            2,
		}
		err := db.WithContext(ctx).Create(&freshLevel2Unit).Error
		require.NoError(t, err)
		defer db.WithContext(ctx).Delete(&freshLevel2Unit)

		// Create a fresh unit for this test to avoid conflicts with previous updates
		// This unit starts at level 3 (with freshLevel2Unit at level 2 as base)
		testUnitForLevelChange := models.Unit{
			Name:             fmt.Sprintf("Level Change Test Unit %s", uuid.New().String()),
			Symbol:           "lctu",
			UnitType:         "general",
			ConversionFactor: 3,
			BaseUnitID:       pkg.Ptr(freshLevel2Unit.ID),
			Level:            3,
		}
		err = db.WithContext(ctx).Create(&testUnitForLevelChange).Error
		require.NoError(t, err)
		defer db.WithContext(ctx).Delete(&testUnitForLevelChange)

		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		// Get base unit to see its dervied units
		urlPathGet := fmt.Sprintf("%s/api/v1/units/%d", suite.sharedTestContainer.BaseURL, baseUnit.ID)
		respGet, err := helpers.MakeRequest(t, "GET", urlPathGet, token, nil)
		require.NoError(t, err)
		defer respGet.Body.Close()
		assert.Equal(t, 200, respGet.StatusCode)
		var conversionFactorToCurrentResp map[string]interface{}
		err = json.NewDecoder(respGet.Body).Decode(&conversionFactorToCurrentResp)
		require.NoError(t, err)
		derivedUnits := conversionFactorToCurrentResp["derived_units"].([]interface{})
		// Find the index in derived_units for the testUnitForLevelChange
		var testDerivedUnitResp map[string]interface{}
		for _, du := range derivedUnits {
			if derivedUnitRespItem := du.(map[string]interface{}); uint(derivedUnitRespItem["id"].(float64)) == testUnitForLevelChange.ID {
				testDerivedUnitResp = derivedUnitRespItem
				break
			}
		}

		assert.Equal(t, float64(1.0)/float64(6), testDerivedUnitResp["conversion_factor_to_current"])

		payload := map[string]interface{}{
			"name":              testUnitForLevelChange.Name,
			"symbol":            testUnitForLevelChange.Symbol,
			"unit_type":         "general",
			"base_unit_id":      baseUnit.ID,
			"conversion_factor": 10,
		}

		urlPath := fmt.Sprintf("%s/api/v1/units/%d", suite.sharedTestContainer.BaseURL, testUnitForLevelChange.ID)
		resp, err := helpers.MakeRequest(t, "PUT", urlPath, token, payload)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		var unitResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&unitResp)
		require.NoError(t, err)
		assert.Equal(t, float64(2), unitResp["level"])
		assert.Equal(t, float64(10), unitResp["conversion_factor"])
		assert.Equal(t, float64(baseUnit.ID), unitResp["base_unit_id"])
		assert.Equal(t, float64(2), unitResp["level"])

		// Get base unit again to see its dervied units
		respGet2, err := helpers.MakeRequest(t, "GET", urlPathGet, token, nil)
		require.NoError(t, err)
		defer respGet2.Body.Close()
		assert.Equal(t, 200, respGet2.StatusCode)
		var conversionFactorToCurrentResp2 map[string]interface{}
		err = json.NewDecoder(respGet2.Body).Decode(&conversionFactorToCurrentResp2)
		require.NoError(t, err)
		derivedUnits2 := conversionFactorToCurrentResp2["derived_units"].([]interface{})
		// Find the index in derived_units for the testUnitForLevelChange
		var testDerivedUnitResp2 map[string]interface{}
		for _, du := range derivedUnits2 {
			if derivedUnitRespItem := du.(map[string]interface{}); uint(derivedUnitRespItem["id"].(float64)) == testUnitForLevelChange.ID {
				testDerivedUnitResp2 = derivedUnitRespItem
				break
			}
		}

		assert.Equal(t, float64(1.0)/float64(10), testDerivedUnitResp2["conversion_factor_to_current"])
	})

	t.Run("should not update unit", func(t *testing.T) {
		roles := []models.UserRole{models.RoleStaff, models.RoleBotForm}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)

				payload := map[string]interface{}{
					"name":              "Updated Name",
					"symbol":            "updated",
					"unit_type":         "general",
					"base_unit_id":      baseUnit.ID,
					"conversion_factor": 5,
				}

				urlPath := fmt.Sprintf("%s/api/v1/units/%d", suite.sharedTestContainer.BaseURL, level2Unit.ID)
				resp, err := helpers.MakeRequest(t, "PUT", urlPath, token, payload)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 403, resp.StatusCode)

				var errorResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&errorResp)
				require.NoError(t, err)
				assert.Equal(t, fmt.Sprintf("Access denied: %s role cannot update units", role), errorResp["error"])
			})
		}
	})

	t.Run("when unit not found", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		payload := map[string]interface{}{
			"name":              "Updated Name",
			"symbol":            "updated",
			"unit_type":         "general",
			"conversion_factor": 1,
		}

		urlPath := fmt.Sprintf("%s/api/v1/units/999999", suite.sharedTestContainer.BaseURL)
		resp, err := helpers.MakeRequest(t, "PUT", urlPath, token, payload)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 404, resp.StatusCode)

		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)
		assert.Contains(t, errorResp["error"].(string), "not found")
	})

	t.Run("when duplicate name exists", func(t *testing.T) {
		// Create another unit for duplicate name test
		duplicateTestUnit := models.Unit{
			Name:             "Duplicate Test Unit",
			Symbol:           "dtu",
			UnitType:         "general",
			ConversionFactor: 1,
		}
		err = db.WithContext(ctx).Create(&duplicateTestUnit).Error
		require.NoError(t, err)
		defer db.WithContext(ctx).Delete(&duplicateTestUnit)

		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		payload := map[string]interface{}{
			"name":              duplicateTestUnit.Name,
			"symbol":            "updated",
			"unit_type":         duplicateTestUnit.UnitType,
			"conversion_factor": 1,
		}

		urlPath := fmt.Sprintf("%s/api/v1/units/%d", suite.sharedTestContainer.BaseURL, level2Unit.ID)
		resp, err := helpers.MakeRequest(t, "PUT", urlPath, token, payload)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 409, resp.StatusCode)

		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)
		assert.Contains(t, errorResp["error"].(string), "already exists")
	})

	t.Run("when reaching maximum hierarchy depth", func(t *testing.T) {
		// Create a fresh unit for this test to avoid conflicts with previous updates
		// This unit is at level 2 (baseUnit is at level 1)
		testUnitForDepth := models.Unit{
			Name:             "Depth Test Unit" + uuid.New().String(),
			Symbol:           "dtu",
			UnitType:         "general",
			ConversionFactor: 2,
			BaseUnitID:       pkg.Ptr(baseUnit.ID),
			Level:            2,
		}
		err := db.WithContext(ctx).Create(&testUnitForDepth).Error
		require.NoError(t, err)
		defer db.WithContext(ctx).Delete(&testUnitForDepth)

		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		payload := map[string]interface{}{
			"name":              "Test Unit" + uuid.New().String(),
			"symbol":            "test",
			"unit_type":         "general",
			"base_unit_id":      level4Unit.ID,
			"conversion_factor": 1,
		}

		urlPath := fmt.Sprintf("%s/api/v1/units/%d", suite.sharedTestContainer.BaseURL, testUnitForDepth.ID)
		resp, err := helpers.MakeRequest(t, "PUT", urlPath, token, payload)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 400, resp.StatusCode)

		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)
		assert.Contains(t, errorResp["error"].(string), "Maximum allowed hierarchy depth is 4 levels")
	})

	t.Run("when base unit has different unit_type", func(t *testing.T) {
		// Create a unit with different type
		differentTypeUnit := models.Unit{
			Name:             "Different Type Unit",
			Symbol:           "dtu",
			UnitType:         "mass",
			ConversionFactor: 1,
		}
		err := db.WithContext(ctx).Create(&differentTypeUnit).Error
		require.NoError(t, err)
		defer db.WithContext(ctx).Delete(&differentTypeUnit)

		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		payload := map[string]interface{}{
			"name":              "Test Unit" + uuid.New().String(),
			"symbol":            "test",
			"unit_type":         "general",
			"base_unit_id":      differentTypeUnit.ID,
			"conversion_factor": 1,
		}

		urlPath := fmt.Sprintf("%s/api/v1/units/%d", suite.sharedTestContainer.BaseURL, level2Unit.ID)
		resp, err := helpers.MakeRequest(t, "PUT", urlPath, token, payload)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 400, resp.StatusCode)

		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)
		assert.Contains(t, errorResp["error"].(string), "base unit must have the same unit_type")
	})

	t.Run("when base unit conversion factor is not 1", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		payload := map[string]interface{}{
			"name":              "Test Unit" + uuid.New().String(),
			"symbol":            "test",
			"unit_type":         "general",
			"base_unit_id":      nil,
			"conversion_factor": 5,
		}

		urlPath := fmt.Sprintf("%s/api/v1/units/%d", suite.sharedTestContainer.BaseURL, baseUnit.ID)
		resp, err := helpers.MakeRequest(t, "PUT", urlPath, token, payload)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 400, resp.StatusCode)

		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)
		assert.Contains(t, errorResp["error"].(string), "conversion_factor must be 1 for base units")
	})
}
