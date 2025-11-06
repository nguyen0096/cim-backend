package scenarios

import (
	"context"
	"encoding/json"
	"testing"

	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/test/components/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *ComponentTestSuite) TestCreateAndGetProduct() {
	t := suite.T()
	t.Run("Create and Get Product", func(t *testing.T) {

		// Create test user with unique email
		user, err := helpers.CreateTestUser(context.Background(), suite.sharedTestContainer.DB, "test-product@example.com", "Test User", models.RoleAdmin)
		require.NoError(t, err)

		// Get auth token
		token := helpers.GetAuthToken(suite.sharedTestContainer.MockAuth, user.UID, user.Email, user.Name)

		// Create a supplier first (products require a supplier)
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

		// Create a product
		productData := map[string]interface{}{
			"name":         "Test Product",
			"description":  "Test Description",
			"product_type": "test",
			"supplier_id":  supplierID,
			"status":       "active",
		}

		resp, err = helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/products", token, productData)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 201, resp.StatusCode)

		var productResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&productResp)
		require.NoError(t, err)

		productID := productResp["id"]
		assert.NotNil(t, productID)
		assert.Equal(t, "Test Product", productResp["name"])

		// Get the product
		resp, err = helpers.MakeRequest(t, "GET", suite.sharedTestContainer.BaseURL+"/api/v1/products/"+helpers.ToString(productID), token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		var getProductResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&getProductResp)
		require.NoError(t, err)
		assert.Equal(t, "Test Product", getProductResp["name"])
	})
}

func (suite *ComponentTestSuite) TestImportProductsFromCsv() {
	t := suite.T()

	db := suite.sharedTestContainer.DB
	ctx := context.Background()

	files := []string{
		"test/data/excel/Products_template.csv",
		"test/data/excel/Products_template.xlsx",
	}
	user, err := helpers.CreateTestUser(ctx, suite.sharedTestContainer.DB, "test-product-template@example.com", "Test User", models.RoleAdmin)
	require.NoError(t, err)
	testCtx := pkg.WithUserEmail(ctx, user.Email)

	for _, file := range files {
		t.Run("Import Products From "+file, func(t *testing.T) {
			// Collect product IDs for cleanup
			var productIDs []uint
			var supplierIDs []uint

			// Setup cleanup
			defer pkg.CleanUp(t, func() error {
				if len(productIDs) > 0 {
					return db.WithContext(testCtx).Where("id IN ?", productIDs).Delete(&models.Product{}).Error
				}
				if len(supplierIDs) > 0 {
					return db.WithContext(testCtx).Where("id IN ?", supplierIDs).Delete(&models.Supplier{}).Error
				}
				return nil
			})

			// Get auth token
			token := helpers.GetAuthToken(suite.sharedTestContainer.MockAuth, user.UID, user.Email, user.Name)

			// Import products from Csv
			resp, err := helpers.MakeMultipartRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/products/import-csv", token, "test/data/excel/Products_template.csv", "file")
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, 200, resp.StatusCode)

			var importProductsResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&importProductsResp)
			require.NoError(t, err)
			assert.Equal(t, float64(3), importProductsResp["count"]) // CSV contains 3 products
			assert.Equal(t, "Products imported successfully", importProductsResp["message"])

			// Verify first product: Cơm tấm
			var products []*models.Product
			db.WithContext(testCtx).Where("name = ?", "Cơm tấm").Find(&products)
			assert.Equal(t, 1, len(products))
			assert.Equal(t, "Cơm tấm", products[0].Name)
			assert.Equal(t, "High-performance laptop", products[0].Description)
			assert.Equal(t, "Cơm", products[0].ProductType)
			assert.Equal(t, "active", products[0].Status)
			assert.Equal(t, user.Email, products[0].CreatedBy)
			assert.Equal(t, user.Email, products[0].UpdatedBy)
			if len(products) > 0 {
				productIDs = append(productIDs, products[0].ID)
			}

			// Verify second product: Khoai tây chiên
			var products2 []*models.Product
			db.WithContext(testCtx).Where("name = ?", "Khoai tây chiên").Find(&products2)
			assert.Equal(t, 1, len(products2))
			assert.Equal(t, "Khoai tây chiên", products2[0].Name)
			assert.Equal(t, "High-performance tablet", products2[0].Description)
			assert.Equal(t, "Ăn nhẹ", products2[0].ProductType)
			if len(products2) > 0 {
				productIDs = append(productIDs, products2[0].ID)
			}

			// Verify third product: Pepsi
			var products3 []*models.Product
			db.WithContext(testCtx).Where("name = ?", "Pepsi").Find(&products3)
			assert.Equal(t, 1, len(products3))
			assert.Equal(t, "Pepsi", products3[0].Name)
			assert.Equal(t, "", products3[0].Description) // Pepsi has no description in CSV
			assert.Equal(t, "Nước", products3[0].ProductType)
			if len(products3) > 0 {
				productIDs = append(productIDs, products3[0].ID)
			}

			// Verify Supplier
			var suppliers []*models.Supplier
			db.WithContext(testCtx).Where("name = ?", "Tech Electronics Inc").Find(&suppliers)
			assert.Equal(t, 1, len(suppliers))
			assert.Equal(t, "Tech Electronics Inc", suppliers[0].Name)
			assert.Equal(t, "contact@techelectronics.com", suppliers[0].ContactEmail)
			assert.Equal(t, "+1-555-0123", suppliers[0].ContactPhone)
			assert.Equal(t, "123 Silicon Valley Blvd, San Jose, CA 95110", suppliers[0].Address)
			if len(suppliers) > 0 {
				supplierIDs = append(supplierIDs, suppliers[0].ID)
			}

			// Verify Supplier
			var suppliers2 []*models.Supplier
			db.WithContext(testCtx).Where("name = ?", "Xiaomi").Find(&suppliers2)
			assert.Equal(t, 1, len(suppliers2))
			assert.Equal(t, "Xiaomi", suppliers2[0].Name)
			assert.Equal(t, "contact@xiaomi.com", suppliers2[0].ContactEmail)
			assert.Equal(t, "+1-555-0456", suppliers2[0].ContactPhone)
			assert.Equal(t, "456 Business Park Dr, Dallas, TX 75201", suppliers2[0].Address)
			if len(suppliers2) > 0 {
				supplierIDs = append(supplierIDs, suppliers2[0].ID)
			}
			// Verify Supplier
			var suppliers3 []*models.Supplier
			db.WithContext(testCtx).Where("name = ?", "Global Parts Ltd").Find(&suppliers3)
			assert.Equal(t, 1, len(suppliers3))
			assert.Equal(t, "Global Parts Ltd", suppliers3[0].Name)
			assert.Equal(t, "orders@globalparts.com", suppliers3[0].ContactEmail)
			assert.Equal(t, "+1-555-0789", suppliers3[0].ContactPhone)
			assert.Equal(t, "789 Industrial Way, Seattle, WA 98101", suppliers3[0].Address)
			if len(suppliers3) > 0 {
				supplierIDs = append(supplierIDs, suppliers3[0].ID)
			}
			// Verify Supplier
			var suppliers4 []*models.Supplier
			db.WithContext(testCtx).Where("name = ?", "Apple").Find(&suppliers4)
			assert.Equal(t, 1, len(suppliers4))
			assert.Equal(t, "Apple", suppliers4[0].Name)
			assert.Equal(t, "contact@apple.com", suppliers4[0].ContactEmail)
			assert.Equal(t, "+1-555-0123", suppliers4[0].ContactPhone)
			assert.Equal(t, "123 Silicon Valley Blvd, San Jose, CA 95110", suppliers4[0].Address)
			if len(suppliers4) > 0 {
				supplierIDs = append(supplierIDs, suppliers4[0].ID)
			}
			// Verify Supplier
			var suppliers5 []*models.Supplier
			db.WithContext(testCtx).Where("name = ?", "Samsung").Find(&suppliers5)
			assert.Equal(t, 1, len(suppliers5))
			assert.Equal(t, "Samsung", suppliers5[0].Name)
			assert.Equal(t, "contact@samsung.com", suppliers5[0].ContactEmail)
			assert.Equal(t, "+1-555-0123", suppliers5[0].ContactPhone)
			assert.Equal(t, "123 Silicon Valley Blvd, San Jose, CA 95110", suppliers5[0].Address)
			if len(suppliers5) > 0 {
				supplierIDs = append(supplierIDs, suppliers5[0].ID)
			}
			// Verify Supplier
			var suppliers6 []*models.Supplier
			db.WithContext(testCtx).Where("name = ?", "Google").Find(&suppliers6)
			assert.Equal(t, 1, len(suppliers6))
			assert.Equal(t, "Google", suppliers6[0].Name)
			assert.Equal(t, "contact@google.com", suppliers6[0].ContactEmail)
			assert.Equal(t, "+1-555-0123", suppliers6[0].ContactPhone)
			assert.Equal(t, "123 Silicon Valley Blvd, San Jose, CA 95110", suppliers6[0].Address)
			if len(suppliers6) > 0 {
				supplierIDs = append(supplierIDs, suppliers6[0].ID)
			}
			// Verify Supplier
			var suppliers7 []*models.Supplier
			db.WithContext(testCtx).Where("name = ?", "Công ty TNHH Giải khát Sài Gòn").Find(&suppliers7)
			assert.Equal(t, 1, len(suppliers7))
			assert.Equal(t, "Công ty TNHH Giải khát Sài Gòn", suppliers7[0].Name)
			assert.Equal(t, "contact@email.com", suppliers7[0].ContactEmail)
			assert.Equal(t, "0123456789", suppliers7[0].ContactPhone)
			assert.Equal(t, "D5 Khu dân cư Thảo Nguyên", suppliers7[0].Address)
			if len(suppliers7) > 0 {
				supplierIDs = append(supplierIDs, suppliers7[0].ID)
			}
			// Verify Supplier
			var suppliers8 []*models.Supplier
			db.WithContext(testCtx).Where("name = ?", "Vạn Thịnh Phát").Find(&suppliers8)
			assert.Equal(t, 1, len(suppliers8))
			assert.Equal(t, "Vạn Thịnh Phát", suppliers8[0].Name)
			assert.Equal(t, "contact@email.com", suppliers8[0].ContactEmail)
			assert.Equal(t, "0123456789", suppliers8[0].ContactPhone)
			assert.Equal(t, "D9 Khu dân cư Thảo Nguyên", suppliers8[0].Address)
			if len(suppliers8) > 0 {
				supplierIDs = append(supplierIDs, suppliers8[0].ID)
			}
		})
	}
}
