package apptest

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil"
	"cim-backend/pkg/testutil/fixture"
	"fmt"
	"strings"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

var _ = Describe("Unit API", func() {
	Describe("Create Unit", func() {
		var testUnits []*models.Unit

		BeforeEach(func() {
			// Setup unit hierarchy for testing
			baseUnit := models.Unit{
				Name:             fmt.Sprintf("Base Unit %s", uuid.New().String()),
				Symbol:           "bu",
				UnitType:         "general",
				Level:            1,
				ConversionFactor: 1,
			}
			unitLevel2 := models.Unit{
				Name:             fmt.Sprintf("Derived Unit 1 %s", uuid.New().String()),
				Symbol:           "du1",
				UnitType:         "general",
				Level:            2,
				ConversionFactor: 2,
			}
			unitLevel3 := models.Unit{
				Name:             fmt.Sprintf("Derived Unit 2 %s", uuid.New().String()),
				Symbol:           "du2",
				UnitType:         "general",
				Level:            3,
				ConversionFactor: 4,
			}
			unitLevel4 := models.Unit{
				Name:             fmt.Sprintf("Derived Unit 3 %s", uuid.New().String()),
				Symbol:           "du3",
				UnitType:         "general",
				Level:            4,
				ConversionFactor: 8,
			}

			testUnits = fixture.WithUnits(tenv.ContextfulDB(), []*models.Unit{&baseUnit, &unitLevel2, &unitLevel3, &unitLevel4})
			// Set up base unit references after creation
			testUnits[1].BaseUnitID = pkg.Ptr(testUnits[0].ID)
			testUnits[2].BaseUnitID = pkg.Ptr(testUnits[1].ID)
			testUnits[3].BaseUnitID = pkg.Ptr(testUnits[2].ID)
			tenv.DB.WithContext(tenv.DefaultContext).Save(testUnits[1])
			tenv.DB.WithContext(tenv.DefaultContext).Save(testUnits[2])
			tenv.DB.WithContext(tenv.DefaultContext).Save(testUnits[3])
		})

		Context("when user has authorized role", func() {
			It("should create and get unit with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)
				name := fmt.Sprintf("Test Unit %s", uuid.New().String())

				payload := map[string]interface{}{
					"name":              name,
					"symbol":            "test",
					"unit_type":         "general",
					"base_unit_id":      testUnits[0].ID,
					"conversion_factor": 10,
					"decimal_places":    2,
				}

				resp, err := client.MakeRequest("POST", "/api/v1/units", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				unitResp := testutil.ParseResponse(resp)
				Expect(unitResp["name"]).To(Equal(strings.ToUpper(name)))
				Expect(unitResp["symbol"]).To(Equal("test"))
				Expect(unitResp["unit_type"]).To(Equal("general"))
				Expect(unitResp["base_unit_id"]).To(Equal(float64(testUnits[0].ID)))
				Expect(unitResp["conversion_factor"]).To(Equal(float64(10)))
				Expect(unitResp["decimal_places"]).To(Equal(float64(2)))
			})

			It("should create and get unit with accountant role", func() {
				client := testutil.NewClient(tenv, models.RoleAccountant)
				name := fmt.Sprintf("Test Unit %s", uuid.New().String())

				payload := map[string]interface{}{
					"name":              name,
					"symbol":            "test",
					"unit_type":         "general",
					"base_unit_id":      testUnits[0].ID,
					"conversion_factor": 10,
					"decimal_places":    2,
				}

				resp, err := client.MakeRequest("POST", "/api/v1/units", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				unitResp := testutil.ParseResponse(resp)
				Expect(unitResp["name"]).To(Equal(strings.ToUpper(name)))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not create unit with staff role", func() {
				client := testutil.NewClient(tenv, models.RoleStaff)

				payload := map[string]interface{}{
					"name":              uuid.New().String(),
					"symbol":            "test",
					"unit_type":         "general",
					"base_unit_id":      1,
					"conversion_factor": 1,
				}

				resp, err := client.MakeRequest("POST", "/api/v1/units", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot create units", models.RoleStaff)))
			})

			It("should not create unit with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				payload := map[string]interface{}{
					"name":              uuid.New().String(),
					"symbol":            "test",
					"unit_type":         "general",
					"base_unit_id":      1,
					"conversion_factor": 1,
				}

				resp, err := client.MakeRequest("POST", "/api/v1/units", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot create units", models.RoleBotForm)))
			})
		})

		Context("when reaching maximum hierarchy depth", func() {
			It("should return error", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				payload := map[string]interface{}{
					"name":              "Test Unit" + uuid.New().String(),
					"symbol":            "test",
					"unit_type":         "general",
					"base_unit_id":      testUnits[3].ID,
					"conversion_factor": 1,
				}

				resp, err := client.MakeRequest("POST", "/api/v1/units", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(400))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(ContainSubstring("Maximum allowed hierarchy depth is 4 levels"))
			})
		})
	})

	Describe("Get Unit", func() {
		var testUnits []*models.Unit

		BeforeEach(func() {
			baseUnit := models.Unit{
				Name:             fmt.Sprintf("Base Unit %s", uuid.New().String()),
				Symbol:           "bu",
				UnitType:         "general",
				Level:            1,
				ConversionFactor: 1,
			}
			unitLevel2 := models.Unit{
				Name:             fmt.Sprintf("Derived Unit 1 %s", uuid.New().String()),
				Symbol:           "du1",
				UnitType:         "general",
				Level:            2,
				ConversionFactor: 2,
			}
			unitLevel3 := models.Unit{
				Name:             fmt.Sprintf("Derived Unit 2 %s", uuid.New().String()),
				Symbol:           "du2",
				UnitType:         "general",
				Level:            3,
				ConversionFactor: 4,
			}
			unitLevel4 := models.Unit{
				Name:             fmt.Sprintf("Derived Unit 3 %s", uuid.New().String()),
				Symbol:           "du3",
				UnitType:         "general",
				Level:            4,
				ConversionFactor: 8,
			}

			testUnits = fixture.WithUnits(tenv.ContextfulDB(), []*models.Unit{&baseUnit, &unitLevel2, &unitLevel3, &unitLevel4})
			testUnits[1].BaseUnitID = pkg.Ptr(testUnits[0].ID)
			testUnits[2].BaseUnitID = pkg.Ptr(testUnits[1].ID)
			testUnits[3].BaseUnitID = pkg.Ptr(testUnits[2].ID)
			tenv.DB.WithContext(tenv.DefaultContext).Save(testUnits[1])
			tenv.DB.WithContext(tenv.DefaultContext).Save(testUnits[2])
			tenv.DB.WithContext(tenv.DefaultContext).Save(testUnits[3])
		})

		It("should get base unit with all derived units", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			urlPath := fmt.Sprintf("/api/v1/units/%d", testUnits[0].ID)
			resp, err := client.MakeRequest("GET", urlPath, nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			unitResp := testutil.ParseResponse(resp)
			Expect(unitResp["name"]).To(Equal(testUnits[0].Name))
			Expect(unitResp["symbol"]).To(Equal(testUnits[0].Symbol))
			Expect(unitResp["unit_type"]).To(Equal(testUnits[0].UnitType))
			Expect(unitResp["level"]).To(Equal(float64(1)))
			Expect(unitResp["conversion_factor"]).To(Equal(testUnits[0].ConversionFactor))
			Expect(unitResp["conversion_factor_to_current"]).To(Equal(float64(1)))

			derivedUnits := unitResp["derived_units"].([]interface{})
			Expect(derivedUnits).To(HaveLen(3))

			currentConversionFactor := 1.0
			for i, derivedUnit := range derivedUnits {
				derivedUnitMap := derivedUnit.(map[string]interface{})
				Expect(derivedUnitMap["id"]).To(Equal(float64(testUnits[i+1].ID)))
				Expect(derivedUnitMap["name"]).To(Equal(testUnits[i+1].Name))
				Expect(derivedUnitMap["symbol"]).To(Equal(testUnits[i+1].Symbol))
				Expect(derivedUnitMap["unit_type"]).To(Equal(testUnits[i+1].UnitType))
				Expect(derivedUnitMap["level"]).To(Equal(float64(i + 2)))
				Expect(derivedUnitMap["conversion_factor"]).To(Equal(testUnits[i+1].ConversionFactor))
				currentConversionFactor /= float64(testUnits[i+1].ConversionFactor)
				Expect(derivedUnitMap["conversion_factor_to_current"]).To(Equal(float64(currentConversionFactor)))
			}
		})

		It("should get derived unit with base unit and all derived units with conversion factor to current unit", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			urlPath := fmt.Sprintf("/api/v1/units/%d", testUnits[1].ID)
			resp, err := client.MakeRequest("GET", urlPath, nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			unitResp := testutil.ParseResponse(resp)
			Expect(unitResp["name"]).To(Equal(testUnits[1].Name))
			Expect(unitResp["symbol"]).To(Equal(testUnits[1].Symbol))
			Expect(unitResp["unit_type"]).To(Equal(testUnits[1].UnitType))
			Expect(unitResp["level"]).To(Equal(float64(2)))
			Expect(unitResp["conversion_factor"]).To(Equal(testUnits[1].ConversionFactor))
			Expect(unitResp["conversion_factor_to_current"]).To(Equal(float64(1)))

			derivedUnits := unitResp["derived_units"].([]interface{})
			Expect(derivedUnits).To(HaveLen(2))

			currentConversionFactor := 1.0
			for i, derivedUnit := range derivedUnits {
				derivedUnitMap := derivedUnit.(map[string]interface{})
				Expect(derivedUnitMap["id"]).To(Equal(float64(testUnits[i+2].ID)))
				Expect(derivedUnitMap["name"]).To(Equal(testUnits[i+2].Name))
				Expect(derivedUnitMap["symbol"]).To(Equal(testUnits[i+2].Symbol))
				Expect(derivedUnitMap["unit_type"]).To(Equal(testUnits[i+2].UnitType))
				Expect(derivedUnitMap["level"]).To(Equal(float64(testUnits[i+2].Level)))
				Expect(derivedUnitMap["conversion_factor"]).To(Equal(testUnits[i+2].ConversionFactor))
				currentConversionFactor /= float64(testUnits[i+2].ConversionFactor)
				Expect(derivedUnitMap["conversion_factor_to_current"]).To(Equal(float64(currentConversionFactor)))
			}

			baseUnitResp := unitResp["base_unit"].(map[string]interface{})
			Expect(baseUnitResp["id"]).To(Equal(float64(testUnits[0].ID)))
			Expect(baseUnitResp["name"]).To(Equal(testUnits[0].Name))
			Expect(baseUnitResp["symbol"]).To(Equal(testUnits[0].Symbol))
			Expect(baseUnitResp["unit_type"]).To(Equal(testUnits[0].UnitType))
			Expect(baseUnitResp["level"]).To(Equal(float64(testUnits[0].Level)))
			Expect(baseUnitResp["conversion_factor"]).To(Equal(float64(1)))
			Expect(baseUnitResp["conversion_factor_to_current"]).To(Equal(float64(testUnits[1].ConversionFactor)))
		})

		It("should get derived unit with root base unit and all derived units with conversion factor to current unit", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			urlPath := fmt.Sprintf("/api/v1/units/%d", testUnits[2].ID)
			resp, err := client.MakeRequest("GET", urlPath, nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			unitResp := testutil.ParseResponse(resp)
			Expect(unitResp["name"]).To(Equal(testUnits[2].Name))
			Expect(unitResp["symbol"]).To(Equal(testUnits[2].Symbol))
			Expect(unitResp["unit_type"]).To(Equal(testUnits[2].UnitType))
			Expect(unitResp["level"]).To(Equal(float64(3)))
			Expect(unitResp["conversion_factor"]).To(Equal(testUnits[2].ConversionFactor))
			Expect(unitResp["conversion_factor_to_current"]).To(Equal(float64(1)))

			derivedUnits := unitResp["derived_units"].([]interface{})
			Expect(derivedUnits).To(HaveLen(1))

			for i, derivedUnit := range derivedUnits {
				derivedUnitMap := derivedUnit.(map[string]interface{})
				Expect(derivedUnitMap["id"]).To(Equal(float64(testUnits[i+3].ID)))
				Expect(derivedUnitMap["name"]).To(Equal(testUnits[i+3].Name))
				Expect(derivedUnitMap["symbol"]).To(Equal(testUnits[i+3].Symbol))
				Expect(derivedUnitMap["unit_type"]).To(Equal(testUnits[i+3].UnitType))
				Expect(derivedUnitMap["level"]).To(Equal(float64(testUnits[i+3].Level)))
				Expect(derivedUnitMap["conversion_factor"]).To(Equal(testUnits[i+3].ConversionFactor))
				expectedConversionFactor := 1.0 / float64(testUnits[i+3].ConversionFactor)
				Expect(derivedUnitMap["conversion_factor_to_current"]).To(Equal(float64(expectedConversionFactor)))
			}

			baseUnitResp := unitResp["base_unit"].(map[string]interface{})
			Expect(baseUnitResp["id"]).To(Equal(float64(testUnits[1].ID)))
			Expect(baseUnitResp["name"]).To(Equal(testUnits[1].Name))
			Expect(baseUnitResp["symbol"]).To(Equal(testUnits[1].Symbol))
			Expect(baseUnitResp["unit_type"]).To(Equal(testUnits[1].UnitType))
			Expect(baseUnitResp["level"]).To(Equal(float64(testUnits[1].Level)))
			Expect(baseUnitResp["conversion_factor"]).To(Equal(testUnits[1].ConversionFactor))
			Expect(baseUnitResp["conversion_factor_to_current"]).To(Equal(float64(testUnits[2].ConversionFactor)))

			rootBaseUnitResp := baseUnitResp["base_unit"].(map[string]interface{})
			Expect(rootBaseUnitResp["id"]).To(Equal(float64(testUnits[0].ID)))
			Expect(rootBaseUnitResp["name"]).To(Equal(testUnits[0].Name))
			Expect(rootBaseUnitResp["symbol"]).To(Equal(testUnits[0].Symbol))
			Expect(rootBaseUnitResp["unit_type"]).To(Equal(testUnits[0].UnitType))
			Expect(rootBaseUnitResp["level"]).To(Equal(float64(testUnits[0].Level)))
			Expect(rootBaseUnitResp["conversion_factor"]).To(Equal(testUnits[0].ConversionFactor))
			expectedRootFactor := float64(testUnits[1].ConversionFactor) * float64(testUnits[2].ConversionFactor)
			Expect(rootBaseUnitResp["conversion_factor_to_current"]).To(Equal(float64(expectedRootFactor)))
		})
	})

	Describe("Search Unit", func() {
		var testUnits []*models.Unit

		BeforeEach(func() {
			baseUnits := []*models.Unit{
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

			testUnits = fixture.WithUnits(tenv.ContextfulDB(), baseUnits)

			// Create derived units that reference base units
			derivedUnits := []*models.Unit{
				{
					Name:             "Gram Search",
					Symbol:           "g",
					UnitType:         "mass",
					ConversionFactor: 1000,
					BaseUnitID:       pkg.Ptr(testUnits[0].ID),
				},
				{
					Name:             "Milliliter Search",
					Symbol:           "ml",
					UnitType:         "volume",
					ConversionFactor: 1000,
					BaseUnitID:       pkg.Ptr(testUnits[1].ID),
				},
			}

			derivedTestUnits := fixture.WithUnits(tenv.ContextfulDB(), derivedUnits)
			testUnits = append(testUnits, derivedTestUnits...)
		})

		It("should search units by name", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			resp, err := client.MakeRequest("GET", "/api/v1/units", nil, testutil.WithAuth(), testutil.WithParams(map[string]string{"q": "Kilogram"}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			searchResp := testutil.ParseResponse(resp)
			Expect(searchResp["total"]).To(BeNumerically(">=", 1))
			data := searchResp["data"].([]interface{})
			found := false
			for _, item := range data {
				unitMap := item.(map[string]interface{})
				if unitMap["name"] == strings.ToUpper("Kilogram Search") {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "Should find unit by name")
		})

		It("should search units by symbol", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			resp, err := client.MakeRequest("GET", "/api/v1/units", nil, testutil.WithAuth(), testutil.WithParams(map[string]string{"q": "ml"}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			searchResp := testutil.ParseResponse(resp)
			Expect(searchResp["total"]).To(BeNumerically(">=", 1))
			data := searchResp["data"].([]interface{})
			found := false
			for _, item := range data {
				unitMap := item.(map[string]interface{})
				if unitMap["symbol"] == "ml" {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "Should find unit by symbol")
		})

		It("should search units case-insensitively", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			resp, err := client.MakeRequest("GET", "/api/v1/units", nil, testutil.WithAuth(), testutil.WithParams(map[string]string{"q": "LITER"}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			searchResp := testutil.ParseResponse(resp)
			Expect(searchResp["total"]).To(BeNumerically(">=", 1))
			data := searchResp["data"].([]interface{})
			found := false
			for _, item := range data {
				unitMap := item.(map[string]interface{})
				if unitMap["name"] == strings.ToUpper("Liter Search") {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "Should find unit case-insensitively")
		})

		It("should search units with unit_type filter", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			resp, err := client.MakeRequest("GET", "/api/v1/units", nil, testutil.WithAuth(), testutil.WithParams(map[string]string{"q": "Search", "unit_type": "mass"}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			searchResp := testutil.ParseResponse(resp)
			data := searchResp["data"].([]interface{})
			for _, item := range data {
				unitMap := item.(map[string]interface{})
				Expect(unitMap["unit_type"]).To(Equal("mass"), "All results should be of type mass")
			}
		})

		It("should search units with base_only filter", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			resp, err := client.MakeRequest("GET", "/api/v1/units", nil, testutil.WithAuth(), testutil.WithParams(map[string]string{"q": "Search", "base_only": "true"}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			searchResp := testutil.ParseResponse(resp)
			data := searchResp["data"].([]interface{})
			for _, item := range data {
				unitMap := item.(map[string]interface{})
				Expect(unitMap["base_unit_id"]).To(BeNil(), "All results should be base units")
			}
		})

		It("should search units with pagination", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			resp, err := client.MakeRequest("GET", "/api/v1/units", nil, testutil.WithAuth(), testutil.WithParams(map[string]string{"q": "Search", "limit": "2", "page": "1"}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			searchResp := testutil.ParseResponse(resp)
			Expect(searchResp["limit"]).To(Equal(float64(2)))
			Expect(searchResp["page"]).To(Equal(float64(1)))
			data := searchResp["data"].([]interface{})
			Expect(len(data)).To(BeNumerically("<=", 2), "Should return at most 2 items per page")
		})

		It("should return empty results for non-matching query", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			resp, err := client.MakeRequest("GET", "/api/v1/units", nil, testutil.WithAuth(), testutil.WithParams(map[string]string{"q": "NonExistentUnit12345"}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			searchResp := testutil.ParseResponse(resp)
			Expect(searchResp["total"]).To(Equal(float64(0)))
			data := searchResp["data"].([]interface{})
			Expect(len(data)).To(Equal(0), "Should return empty array for non-matching query")
		})
	})

	Describe("Update Unit", func() {
		var baseUnit *models.Unit
		var level2Unit *models.Unit
		var level3Unit *models.Unit
		var level4Unit *models.Unit

		BeforeEach(func() {
			baseUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             "Update Test Base Unit",
				Symbol:           "utbu",
				UnitType:         "general",
				ConversionFactor: 1,
			})

			level2Unit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             "Level 2 Unit",
				Symbol:           "l2",
				UnitType:         "general",
				ConversionFactor: 2,
				BaseUnitID:       pkg.Ptr(baseUnit.ID),
				Level:            2,
			})

			level3Unit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             "Level 3 Unit",
				Symbol:           "l3",
				UnitType:         "general",
				ConversionFactor: 3,
				BaseUnitID:       pkg.Ptr(level2Unit.ID),
				Level:            3,
			})

			level4Unit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             "Level 4 Unit",
				Symbol:           "l4",
				UnitType:         "general",
				ConversionFactor: 4,
				BaseUnitID:       pkg.Ptr(level3Unit.ID),
				Level:            4,
			})
		})

		Context("when user has authorized role", func() {
			It("should update unit with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)
				updatedName := fmt.Sprintf("Updated Unit Name %s", uuid.New().String())

				payload := map[string]interface{}{
					"name":              updatedName,
					"symbol":            "updated",
					"unit_type":         "general",
					"base_unit_id":      baseUnit.ID,
					"conversion_factor": 5,
					"decimal_places":    3,
				}

				urlPath := fmt.Sprintf("/api/v1/units/%d", level2Unit.ID)
				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				unitResp := testutil.ParseResponse(resp)
				Expect(unitResp["name"]).To(Equal(updatedName))
				Expect(unitResp["symbol"]).To(Equal("updated"))
				Expect(unitResp["unit_type"]).To(Equal("general"))
				Expect(unitResp["base_unit_id"]).To(Equal(float64(baseUnit.ID)))
				Expect(unitResp["conversion_factor"]).To(Equal(float64(5)))
				Expect(unitResp["decimal_places"]).To(Equal(float64(3)))
			})

			It("should update unit with accountant role", func() {
				client := testutil.NewClient(tenv, models.RoleAccountant)
				updatedName := fmt.Sprintf("Updated Unit Name %s", uuid.New().String())

				payload := map[string]interface{}{
					"name":              updatedName,
					"symbol":            "updated",
					"unit_type":         "general",
					"base_unit_id":      baseUnit.ID,
					"conversion_factor": 5,
					"decimal_places":    3,
				}

				urlPath := fmt.Sprintf("/api/v1/units/%d", level2Unit.ID)
				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				unitResp := testutil.ParseResponse(resp)
				Expect(unitResp["name"]).To(Equal(updatedName))
			})
		})

		It("should update unit level and conversion factor to current when change base unit", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			// Create a fresh level2Unit for this test
			freshLevel2Unit := fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Fresh Level 2 Unit %s", uuid.New().String()),
				Symbol:           "fl2",
				UnitType:         "general",
				ConversionFactor: 2,
				BaseUnitID:       pkg.Ptr(baseUnit.ID),
				Level:            2,
			})

			// Create a unit for level change test
			testUnitForLevelChange := fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Level Change Test Unit %s", uuid.New().String()),
				Symbol:           "lctu",
				UnitType:         "general",
				ConversionFactor: 3,
				BaseUnitID:       pkg.Ptr(freshLevel2Unit.ID),
				Level:            3,
			})

			// Get base unit to see its derived units
			urlPathGet := fmt.Sprintf("/api/v1/units/%d", baseUnit.ID)
			respGet, err := client.MakeRequest("GET", urlPathGet, nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(respGet.StatusCode).To(Equal(200))

			conversionFactorResp := testutil.ParseResponse(respGet)
			derivedUnits := conversionFactorResp["derived_units"].([]interface{})

			// Find the conversion factor for testUnitForLevelChange
			var testDerivedUnitResp map[string]interface{}
			for _, du := range derivedUnits {
				derivedUnitRespItem := du.(map[string]interface{})
				if uint(derivedUnitRespItem["id"].(float64)) == testUnitForLevelChange.ID {
					testDerivedUnitResp = derivedUnitRespItem
					break
				}
			}

			Expect(testDerivedUnitResp["conversion_factor_to_current"]).To(Equal(float64(1.0) / float64(6)))

			// Update unit to change base unit
			payload := map[string]interface{}{
				"name":              testUnitForLevelChange.Name,
				"symbol":            testUnitForLevelChange.Symbol,
				"unit_type":         "general",
				"base_unit_id":      baseUnit.ID,
				"conversion_factor": 10,
			}

			urlPath := fmt.Sprintf("/api/v1/units/%d", testUnitForLevelChange.ID)
			resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			unitResp := testutil.ParseResponse(resp)
			Expect(unitResp["level"]).To(Equal(float64(2)))
			Expect(unitResp["conversion_factor"]).To(Equal(float64(10)))
			Expect(unitResp["base_unit_id"]).To(Equal(float64(baseUnit.ID)))

			// Get base unit again to verify conversion factor
			respGet2, err := client.MakeRequest("GET", urlPathGet, nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(respGet2.StatusCode).To(Equal(200))

			conversionFactorResp2 := testutil.ParseResponse(respGet2)
			derivedUnits2 := conversionFactorResp2["derived_units"].([]interface{})

			var testDerivedUnitResp2 map[string]interface{}
			for _, du := range derivedUnits2 {
				derivedUnitRespItem := du.(map[string]interface{})
				if uint(derivedUnitRespItem["id"].(float64)) == testUnitForLevelChange.ID {
					testDerivedUnitResp2 = derivedUnitRespItem
					break
				}
			}

			Expect(testDerivedUnitResp2["conversion_factor_to_current"]).To(Equal(float64(1.0) / float64(10)))
		})

		Context("when user has unauthorized role", func() {
			It("should not update unit with staff role", func() {
				client := testutil.NewClient(tenv, models.RoleStaff)

				payload := map[string]interface{}{
					"name":              "Updated Name",
					"symbol":            "updated",
					"unit_type":         "general",
					"base_unit_id":      baseUnit.ID,
					"conversion_factor": 5,
				}

				urlPath := fmt.Sprintf("/api/v1/units/%d", level2Unit.ID)
				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update units", models.RoleStaff)))
			})

			It("should not update unit with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				payload := map[string]interface{}{
					"name":              "Updated Name",
					"symbol":            "updated",
					"unit_type":         "general",
					"base_unit_id":      baseUnit.ID,
					"conversion_factor": 5,
				}

				urlPath := fmt.Sprintf("/api/v1/units/%d", level2Unit.ID)
				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update units", models.RoleBotForm)))
			})
		})

		Context("when unit not found", func() {
			It("should return error", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				payload := map[string]interface{}{
					"name":              "Updated Name",
					"symbol":            "updated",
					"unit_type":         "general",
					"conversion_factor": 1,
				}

				resp, err := client.MakeRequest("PUT", "/api/v1/units/999999", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(404))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(ContainSubstring("not found"))
			})
		})

		Context("when duplicate name exists", func() {
			It("should return error", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				duplicateTestUnit := fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
					Name:             "Duplicate Test Unit",
					Symbol:           "dtu",
					UnitType:         "general",
					ConversionFactor: 1,
				})

				payload := map[string]interface{}{
					"name":              duplicateTestUnit.Name,
					"symbol":            "updated",
					"unit_type":         duplicateTestUnit.UnitType,
					"conversion_factor": 1,
				}

				urlPath := fmt.Sprintf("/api/v1/units/%d", level2Unit.ID)
				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(409))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(ContainSubstring("already exists"))
			})
		})

		Context("when reaching maximum hierarchy depth", func() {
			It("should return error", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				testUnitForDepth := fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
					Name:             "Depth Test Unit" + uuid.New().String(),
					Symbol:           "dtu",
					UnitType:         "general",
					ConversionFactor: 2,
					BaseUnitID:       pkg.Ptr(baseUnit.ID),
					Level:            2,
				})

				payload := map[string]interface{}{
					"name":              "Test Unit" + uuid.New().String(),
					"symbol":            "test",
					"unit_type":         "general",
					"base_unit_id":      level4Unit.ID,
					"conversion_factor": 1,
				}

				urlPath := fmt.Sprintf("/api/v1/units/%d", testUnitForDepth.ID)
				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(400))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(ContainSubstring("Maximum allowed hierarchy depth is 4 levels"))
			})
		})

		Context("when base unit has different unit_type", func() {
			It("should return error", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				differentTypeUnit := fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
					Name:             "Different Type Unit",
					Symbol:           "dtu",
					UnitType:         "mass",
					ConversionFactor: 1,
				})

				payload := map[string]interface{}{
					"name":              "Test Unit" + uuid.New().String(),
					"symbol":            "test",
					"unit_type":         "general",
					"base_unit_id":      differentTypeUnit.ID,
					"conversion_factor": 1,
				}

				urlPath := fmt.Sprintf("/api/v1/units/%d", level2Unit.ID)
				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(400))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(ContainSubstring("base unit must have the same unit_type"))
			})
		})

		Context("when base unit conversion factor is not 1", func() {
			It("should return error", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				payload := map[string]interface{}{
					"name":              "Test Unit" + uuid.New().String(),
					"symbol":            "test",
					"unit_type":         "general",
					"base_unit_id":      nil,
					"conversion_factor": 5,
				}

				urlPath := fmt.Sprintf("/api/v1/units/%d", baseUnit.ID)
				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(400))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(ContainSubstring("conversion_factor must be 1 for base units"))
			})
		})
	})

	Describe("Delete Unit", func() {
		var testUnit *models.Unit

		BeforeEach(func() {
			testUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             "Delete Test Unit",
				Symbol:           "dtu",
				UnitType:         "general",
				ConversionFactor: 1,
			})
		})

		Context("when user has admin role", func() {
			It("should delete unit", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				urlPath := fmt.Sprintf("/api/v1/units/%d", testUnit.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(204))

				// Verify unit is deleted
				err = tenv.DB.WithContext(ctx).First(&models.Unit{}, "id = ?", testUnit.ID).Error
				Expect(err).To(Equal(gorm.ErrRecordNotFound))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not delete unit with staff role", func() {
				client := testutil.NewClient(tenv, models.RoleStaff)

				urlPath := fmt.Sprintf("/api/v1/units/%d", testUnit.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot delete units", models.RoleStaff)))
			})

			It("should not delete unit with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				urlPath := fmt.Sprintf("/api/v1/units/%d", testUnit.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot delete units", models.RoleBotForm)))
			})
		})

		Context("when unit ID is 0", func() {
			It("should return error", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				resp, err := client.MakeRequest("DELETE", "/api/v1/units/0", nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(400))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(ContainSubstring("Unit ID is required"))
			})
		})

		Context("when products reference the unit", func() {
			It("should return error", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				// Create a product that references the unit
				testProduct := fixture.WithProduct(tenv.ContextfulDB(), models.Product{
					Name:        fmt.Sprintf("Test Product for Unit %s", uuid.New().String()),
					Description: "Test Description",
					ProductType: "test",
					UnitID:      testUnit.ID,
					Status:      "active",
				})

				// Try to delete the unit
				urlPath := fmt.Sprintf("/api/v1/units/%d", testUnit.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(400))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(ContainSubstring("Cannot delete unit: products reference this unit"))

				// Verify unit is not deleted
				var unit models.Unit
				err = tenv.DB.WithContext(ctx).First(&unit, "id = ?", testUnit.ID).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(unit.ID).To(Equal(testUnit.ID))

				// Verify product still exists
				var product models.Product
				err = tenv.DB.WithContext(ctx).First(&product, "id = ?", testProduct.ID).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(product.ID).To(Equal(testProduct.ID))
			})
		})
	})
})
