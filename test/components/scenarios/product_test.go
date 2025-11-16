package scenarios

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/test/components/helpers"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func (suite *ComponentTestSuite) TestCreateAndGetProduct() {
	t := suite.T()
	db := suite.sharedTestContainer.DB
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")

	testUnit := models.Unit{
		UnitType: uuid.New().String(),
		Name:     fmt.Sprintf("Test Unit %s", uuid.New().String()),
	}
	err := db.WithContext(ctx).Create(&testUnit).Error
	require.NoError(t, err, "Failed to create units")
	productName := fmt.Sprintf("Test Product %s", uuid.New().String())
	productDescription := fmt.Sprintf("Test Description %s", uuid.New().String())

	productData := map[string]interface{}{
		"name":         productName,
		"description":  productDescription,
		"product_type": "test",
		"unit_id":      testUnit.ID,
		"supplier_ids": []uint{1, 2, 3},
		"status":       "active",
	}

	t.Run("should create and get product", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)
				resp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/products", token, productData)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, 201, resp.StatusCode)

				var productResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&productResp)
				require.NoError(t, err)
				productID := productResp["id"]
				assert.NotNil(t, productID)
				assert.Equal(t, productName, productResp["name"])
			})
		}
	})

	t.Run("should not create product", func(t *testing.T) {
		roles := []models.UserRole{models.RoleStaff, models.RoleBotForm}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)
				resp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/products", token, productData)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 403, resp.StatusCode)

				var errorResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&errorResp)
				require.NoError(t, err)
				assert.Equal(t, "Access denied: "+string(role)+" role cannot create products", errorResp["error"])
			})
		}
	})
}

func (suite *ComponentTestSuite) TestUpdateProduct() {
	t := suite.T()
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
	db := suite.sharedTestContainer.DB

	testUnit := models.Unit{
		Name: fmt.Sprintf("Test Unit %s", uuid.New().String()),
	}
	err := db.WithContext(ctx).Create(&testUnit).Error
	require.NoError(t, err, "Failed to create units")
	testSuppliers := []models.Supplier{
		{
			Name: fmt.Sprintf("Test Supplier 1 %s", uuid.New().String()),
		},
		{
			Name: fmt.Sprintf("Test Supplier 2 %s", uuid.New().String()),
		},
		{
			Name: fmt.Sprintf("Test Supplier 3 %s", uuid.New().String()),
		},
		{
			Name: fmt.Sprintf("Test Supplier 4 %s", uuid.New().String()),
		},
	}
	err = db.WithContext(ctx).Create(&testSuppliers).Error
	require.NoError(t, err, "Failed to create suppliers")
	testProduct := models.Product{
		Name:        fmt.Sprintf("Test Product %s", uuid.New().String()),
		Description: fmt.Sprintf("Test Description %s", uuid.New().String()),
		ProductType: "test",
		UnitID:      1,
		Unit:        &testUnit,
		Status:      "active",
		Suppliers:   []*models.Supplier{&testSuppliers[0], &testSuppliers[1]},
	}
	err = db.WithContext(ctx).Create(&testProduct).Error
	require.NoError(t, err, "Failed to create product")
	productID := testProduct.ID
	newProductName := fmt.Sprintf("Test Product Edited %s", uuid.New().String())
	newProductDescription := fmt.Sprintf("Test Description Edited %s", uuid.New().String())

	updatedProductData := map[string]interface{}{
		"name":         newProductName,
		"description":  newProductDescription,
		"product_type": "test_edited",
		"supplier_ids": []uint{testSuppliers[1].ID, testSuppliers[2].ID, testSuppliers[3].ID},
		"unit_id":      testUnit.ID,
		"status":       "inactive",
	}

	t.Run("should update product", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)
				urlPath := fmt.Sprintf("/api/v1/products/%d", productID)
				resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, updatedProductData)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 200, resp.StatusCode)

				var updatedProductResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&updatedProductResp)
				require.NoError(t, err)
				assert.Equal(t, newProductName, updatedProductResp["name"])
				assert.Equal(t, newProductDescription, updatedProductResp["description"])
				assert.Equal(t, "test_edited", updatedProductResp["product_type"])
				assert.Equal(t, "inactive", updatedProductResp["status"])
				suppliers := updatedProductResp["suppliers"].([]interface{})
				assert.Equal(t, 3, len(suppliers))
			})
		}
	})

	t.Run("should not update product", func(t *testing.T) {
		roles := []models.UserRole{models.RoleStaff, models.RoleBotForm}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)
				urlPath := fmt.Sprintf("/api/v1/products/%d", productID)
				resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, updatedProductData)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 403, resp.StatusCode)

				var errorResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&errorResp)
				require.NoError(t, err)
				assert.Equal(t, "Access denied: "+string(role)+" role cannot update products", errorResp["error"])
			})
		}
	})
}

func (suite *ComponentTestSuite) TestUpdateProductStatus() {
	t := suite.T()
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
	db := suite.sharedTestContainer.DB
	testProduct := models.Product{
		Name:        "Test Product",
		Description: "Test Description",
		UnitID:      1,
	}
	err := db.WithContext(ctx).Create(&testProduct).Error
	require.NoError(t, err, "Failed to create product")
	productID := testProduct.ID

	t.Run("should update product status", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)
				urlPath := fmt.Sprintf("/api/v1/products/%d/status", productID)
				resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, map[string]interface{}{"status": "inactive"})
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 200, resp.StatusCode)

				var updatedProductResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&updatedProductResp)
				require.NoError(t, err)
				assert.Equal(t, "inactive", updatedProductResp["status"])

				var updatedProduct models.Product
				err = db.WithContext(ctx).First(&updatedProduct, "id = ?", productID).Error
				require.NoError(t, err)
				assert.Equal(t, "inactive", updatedProduct.Status)
			})
		}
	})

	t.Run("should not update product status", func(t *testing.T) {
		roles := []models.UserRole{models.RoleStaff, models.RoleBotForm}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)
				urlPath := fmt.Sprintf("/api/v1/products/%d/status", productID)
				resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, map[string]interface{}{"status": "inactive"})
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 403, resp.StatusCode)

				var errorResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&errorResp)
				require.NoError(t, err)
				assert.Equal(t, "Access denied: "+string(role)+" role cannot update products", errorResp["error"])
			})
		}
	})
}

func (suite *ComponentTestSuite) TestDeleteProduct() {
	t := suite.T()
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
	db := suite.sharedTestContainer.DB
	testUnit := models.Unit{
		UnitType:         uuid.New().String(),
		Name:             fmt.Sprintf("Test Unit %s", uuid.New().String()),
		ConversionFactor: 1,
	}
	err := db.WithContext(ctx).Create(&testUnit).Error
	require.NoError(t, err, "Failed to create units")
	testSuppliers := []*models.Supplier{
		{
			Name: fmt.Sprintf("Test Supplier 1 %s", uuid.New().String()),
		},
	}
	err = db.WithContext(ctx).Create(&testSuppliers).Error
	require.NoError(t, err, "Failed to create suppliers")
	testProduct := models.Product{
		Name:        fmt.Sprintf("Test Product %s", uuid.New().String()),
		Description: fmt.Sprintf("Test Description %s", uuid.New().String()),
		ProductType: "test",
		UnitID:      testUnit.ID,
		Unit:        &testUnit,
		Status:      "active",
		Suppliers:   testSuppliers,
	}
	err = db.WithContext(ctx).Create(&testProduct).Error
	require.NoError(t, err, "Failed to create product")
	productID := testProduct.ID

	t.Run("should delete product when user has admin role", func(t *testing.T) {
		role := models.RoleAdmin
		_, token, err := suite.CreateUniqueEmailAndToken(role)
		require.NoError(t, err)
		resp, err := helpers.MakeRequest(t, "DELETE", suite.sharedTestContainer.BaseURL+"/api/v1/products/"+strconv.Itoa(int(productID)), token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		err = db.WithContext(ctx).First(&models.Product{}, "id = ?", productID).Error
		assert.Equal(t, gorm.ErrRecordNotFound, err)
	})

	t.Run("should not delete product when user has not admin role", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAccountant, models.RoleStaff, models.RoleBotForm}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)
				urlPath := fmt.Sprintf("/api/v1/products/%d", productID)
				resp, err := helpers.MakeRequest(t, "DELETE", suite.sharedTestContainer.BaseURL+urlPath, token, nil)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 403, resp.StatusCode)

				var errorResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&errorResp)
				require.NoError(t, err)
				assert.Equal(t, "Access denied: "+string(role)+" role cannot delete products", errorResp["error"])
			})
		}
	})
}

func (suite *ComponentTestSuite) TestImportProductsFromCsvAndExcel() {
	t := suite.T()

	db := suite.sharedTestContainer.DB
	ctx := context.Background()

	files := []string{
		"test/data/excel/Products_template.csv",
		"test/data/excel/Products_template.xlsx",
	}
	user, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
	require.NoError(t, err)
	testCtx := pkg.WithUserEmail(ctx, user.Email)

	for _, file := range files {
		t.Run("Import Products From "+file, func(t *testing.T) {
			// Import products from file
			resp, err := helpers.MakeMultipartRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/products/import-csv", token, file, "file")
			require.NoError(t, err)
			defer resp.Body.Close()
			var importProductsResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&importProductsResp)
			require.NoError(t, err)

			assert.Equal(t, 200, resp.StatusCode, importProductsResp["error"])
			assert.Equal(t, float64(12), importProductsResp["count"]) // CSV contains 12 unique products (including COCA COLA 500ML without supplier)
			assert.Equal(t, "Products imported successfully", importProductsResp["message"])

			// verifyProduct verifies a product by name
			verifyProduct := func(name string, expectedType, expectedUnit, expectedStatus string, expectedSupplierNames []string, checkCreatedBy bool) *models.Product {
				var products []*models.Product
				err = db.WithContext(testCtx).Preload("Unit").Preload("Suppliers").Where("name = ?", name).Find(&products).Error
				require.NoError(t, err)
				require.Len(t, products, 1, "Product %s should exist", name)
				p := products[0]
				assert.Equal(t, name, p.Name)
				assert.Equal(t, name, p.Description)
				assert.Equal(t, expectedType, p.ProductType)
				if expectedStatus != "" {
					assert.Equal(t, expectedStatus, p.Status)
				}
				require.NotNil(t, p.Unit, "Product %s should have unit", name)
				assert.Equal(t, expectedUnit, p.Unit.Name)
				if checkCreatedBy {
					assert.Equal(t, user.Email, p.CreatedBy)
					assert.Equal(t, user.Email, p.UpdatedBy)
				}
				if len(expectedSupplierNames) > 0 {
					assert.Equal(t, len(expectedSupplierNames), len(p.Suppliers), "Product %s should have %d suppliers", name, len(expectedSupplierNames))
					supplierNames := make(map[string]bool)
					for _, s := range p.Suppliers {
						supplierNames[s.Name] = true
					}
					for _, expectedName := range expectedSupplierNames {
						assert.True(t, supplierNames[expectedName], "Product %s should have supplier %s", name, expectedName)
					}
				} else {
					assert.Equal(t, 0, len(p.Suppliers), "Product %s should have no suppliers", name)
				}
				return p
			}

			// Verify first product: PEPSI 390 ml (has multiple suppliers)
			verifyProduct("PEPSI 390 ml", "NƯỚC", "lốc", "active", []string{"PEPSI", "COCACOLA"}, true)

			// Verify other products
			verifyProduct("PEPSI LON 320 ML", "NƯỚC", "thùng", "", []string{"PEPSI"}, false)
			verifyProduct("FANTA LON XÁ XỊ  320 ML", "NƯỚC", "lốc", "", []string{"COCACOLA"}, false)
			verifyProduct("MILO NẮP VẬN 210 ML", "NƯỚC", "thùng", "", []string{"SỮA MILO"}, false)
			verifyProduct("SỮA BẮP THÁI SƠN", "ĂN NHẸ", "chai", "", []string{"SỮA THÁI SƠN"}, false)
			verifyProduct("COCA COLA 500ML", "NƯỚC", "thùng", "", nil, false)

			// Verify product with full supplier details
			p7 := verifyProduct("CƠM THỊT KHO TRỨNG", "CƠM", "phần", "active", []string{"NHÀ HÀNG 5 SAO"}, false)
			assert.Equal(t, "phần", p7.Unit.Name)
			assert.Equal(t, "5stars@example.com", p7.Suppliers[0].ContactEmail)
			assert.Equal(t, "028-3896100", p7.Suppliers[0].ContactPhone)
			assert.Equal(t, "1, Tân Kỳ Tân Quý, TPHCM", p7.Suppliers[0].Address)

			// Verify Supplier: PEPSI
			var suppliers []*models.Supplier
			err = db.WithContext(testCtx).Preload("Products").Where("name = ?", "PEPSI").Find(&suppliers).Error
			require.NoError(t, err)
			assert.Equal(t, 1, len(suppliers))
			assert.Equal(t, "PEPSI", suppliers[0].Name)
			assert.Equal(t, "", suppliers[0].ContactEmail)
			assert.Equal(t, "098-7513328", suppliers[0].ContactPhone)
			assert.Equal(t, "202,QUỐC LỘ 13,PHƯỜNG HIỆP BÌNH THÀNH PHỐ HỒ CHÍ MINH VIỆT NAM", suppliers[0].Address)
			// Verify PEPSI supplier has 2 products (PEPSI LON 320 ML and PEPSI 390 ml)
			assert.Equal(t, 2, len(suppliers[0].Products))

			// Verify Supplier: COCACOLA
			var suppliers2 []*models.Supplier
			err = db.WithContext(testCtx).Preload("Products").Where("name = ?", "COCACOLA").Find(&suppliers2).Error
			require.NoError(t, err)
			assert.Equal(t, 1, len(suppliers2))
			assert.Equal(t, "COCACOLA", suppliers2[0].Name)
			assert.Equal(t, "", suppliers2[0].ContactEmail)
			assert.Equal(t, "028-3896100", suppliers2[0].ContactPhone)
			assert.Equal(t, "LÔ C 12,ĐƯỜNG DỌC 2,KHU CÔNG NGHIỆP PHÚ AN,XÃ BẾN LỨC,TỈNH TÂY NINH,VIỆT NAM", suppliers2[0].Address)
			// Verify COCACOLA supplier has 3 products (FANTA LON XÁ XỊ 320 ML, FANTA XÁ XỊ 390 ML, and PEPSI 390 ml)
			// Note: Line 14 has supplier name but no product name, so it only updates the supplier but doesn't add a product
			assert.Equal(t, 3, len(suppliers2[0].Products))

			// Verify Supplier: SỮA MILO
			var suppliers3 []*models.Supplier
			err = db.WithContext(testCtx).Preload("Products").Where("name = ?", "SỮA MILO").Find(&suppliers3).Error
			require.NoError(t, err)
			assert.Equal(t, 1, len(suppliers3))
			assert.Equal(t, "SỮA MILO", suppliers3[0].Name)
			assert.Equal(t, "", suppliers3[0].ContactEmail)
			assert.Equal(t, "", suppliers3[0].ContactPhone)
			assert.Equal(t, "415/4 A HOÀNG VĂN THỤ, PHƯỜNG TÂN SƠN HÒA, THÀNH PH", suppliers3[0].Address)
			// Verify SỮA MILO supplier has 2 products
			assert.Equal(t, 2, len(suppliers3[0].Products))

			// Verify Supplier: SỮA THÁI SƠN
			var suppliers4 []*models.Supplier
			err = db.WithContext(testCtx).Preload("Products").Where("name = ?", "SỮA THÁI SƠN").Find(&suppliers4).Error
			require.NoError(t, err)
			assert.Equal(t, 1, len(suppliers4))
			assert.Equal(t, "SỮA THÁI SƠN", suppliers4[0].Name)
			assert.Equal(t, "", suppliers4[0].ContactEmail)
			assert.Equal(t, "", suppliers4[0].ContactPhone)
			assert.Equal(t, "", suppliers4[0].Address)
			// Verify SỮA THÁI SƠN supplier has 4 products
			assert.Equal(t, 4, len(suppliers4[0].Products))

			// Verify Supplier: TH TRUE MILK (created from line 14 which has supplier name but no product name)
			var suppliers5 []*models.Supplier
			err = db.WithContext(testCtx).Preload("Products").Where("name = ?", "TH TRUE MILK").Find(&suppliers5).Error
			require.NoError(t, err)
			assert.Equal(t, 1, len(suppliers5))
			assert.Equal(t, "TH TRUE MILK", suppliers5[0].Name)
			assert.Equal(t, "", suppliers5[0].ContactEmail)
			assert.Equal(t, "028-3896100", suppliers5[0].ContactPhone)                                                             // From line 14
			assert.Equal(t, "LÔ C 12,ĐƯỜNG DỌC 2,KHU CÔNG NGHIỆP PHÚ AN,XÃ BẾN LỨC,TỈNH TÂY NINH,VIỆT NAM", suppliers5[0].Address) // From line 14
			// Verify TH TRUE MILK supplier has 0 products (line 14 has no product name)
			assert.Equal(t, 0, len(suppliers5[0].Products))

			var suppliers6 []*models.Supplier
			err = db.WithContext(testCtx).Preload("Products").Where("name = ?", "NHÀ HÀNG 5 SAO").Find(&suppliers6).Error
			require.NoError(t, err)
			assert.Equal(t, 1, len(suppliers6))
			assert.Equal(t, "NHÀ HÀNG 5 SAO", suppliers6[0].Name)
			assert.Equal(t, "5stars@example.com", suppliers6[0].ContactEmail)
			assert.Equal(t, "028-3896100", suppliers6[0].ContactPhone)
			assert.Equal(t, "1, Tân Kỳ Tân Quý, TPHCM", suppliers6[0].Address)
			assert.Equal(t, 1, len(suppliers6[0].Products))
			assert.Equal(t, "CƠM THỊT KHO TRỨNG", suppliers6[0].Products[0].Name)

			// Verify number of products by supplier
			assert.Equal(t, 2, len(suppliers[0].Products))
			assert.Equal(t, 3, len(suppliers2[0].Products))
			assert.Equal(t, 2, len(suppliers3[0].Products))
			assert.Equal(t, 4, len(suppliers4[0].Products))
			assert.Equal(t, 0, len(suppliers5[0].Products))
			assert.Equal(t, 1, len(suppliers6[0].Products))

			// Verify number of product types
			var productTypeSettings models.Settings
			err = db.WithContext(testCtx).Where("key = ?", "product_types").First(&productTypeSettings).Error
			require.NoError(t, err)
			var productTypes []string
			err = json.Unmarshal(productTypeSettings.Value, &productTypes)
			require.NoError(t, err)
			assert.Equal(t, []string{"CƠM", "NƯỚC", "ĂN NHẸ"}, productTypes)
		})
	}
}

// setupExportTestData creates test data for export tests
func (suite *ComponentTestSuite) setupExportTestData(t *testing.T) (ctx context.Context, units []models.Unit, suppliers []models.Supplier, products []models.Product, cleanup func()) {
	ctx = pkg.WithUserEmail(context.Background(), "test@example.com")
	db := suite.sharedTestContainer.DB

	units = []models.Unit{
		{UnitType: uuid.New().String(), Name: fmt.Sprintf("Thùng %s", uuid.New().String()), ConversionFactor: 1},
		{UnitType: uuid.New().String(), Name: fmt.Sprintf("Lốc %s", uuid.New().String()), ConversionFactor: 1},
	}
	require.NoError(t, db.WithContext(ctx).Create(&units).Error, "Failed to create units")

	suppliers = []models.Supplier{
		{Name: fmt.Sprintf("Supplier A %s", uuid.New().String()), ContactEmail: fmt.Sprintf("suppliera@example.com-%s", uuid.New().String()), ContactPhone: fmt.Sprintf("123-456-7890-%s", uuid.New().String()), Address: fmt.Sprintf("123 Main St %s", uuid.New().String()), Status: "active"},
		{Name: fmt.Sprintf("Supplier B %s", uuid.New().String()), ContactEmail: fmt.Sprintf("supplierb@example.com-%s", uuid.New().String()), ContactPhone: fmt.Sprintf("098-765-4321-%s", uuid.New().String()), Address: fmt.Sprintf("456 Oak Ave %s", uuid.New().String()), Status: "active"},
		{Name: fmt.Sprintf("Supplier C %s", uuid.New().String()), ContactEmail: fmt.Sprintf("supplierc@example.com-%s", uuid.New().String()), ContactPhone: fmt.Sprintf("098-765-4322-%s", uuid.New().String()), Address: fmt.Sprintf("456 Oak Ave %s", uuid.New().String()), Status: "active"},
	}
	require.NoError(t, db.WithContext(ctx).Create(&suppliers).Error, "Failed to create suppliers")

	products = []models.Product{
		{
			Name: fmt.Sprintf("Product 1 %s", uuid.New().String()), Description: fmt.Sprintf("Description 1 %s", uuid.New().String()), ProductType: "Type A", UnitID: units[0].ID, Status: "active",
			Suppliers: []*models.Supplier{{Base: models.Base{ID: suppliers[0].ID}}, {Base: models.Base{ID: suppliers[1].ID}}},
		},
		{
			Name: fmt.Sprintf("Product 2 %s", uuid.New().String()), Description: fmt.Sprintf("Description 2 %s", uuid.New().String()), ProductType: "Type A", UnitID: units[1].ID, Status: "active",
			Suppliers: []*models.Supplier{{Base: models.Base{ID: suppliers[1].ID}}},
		},
		{
			Name: fmt.Sprintf("Product 3 %s", uuid.New().String()), Description: fmt.Sprintf("Description 3 %s", uuid.New().String()), ProductType: "Type B", UnitID: units[0].ID, Status: "inactive",
			Suppliers: []*models.Supplier{},
		},
		{
			Name: fmt.Sprintf("Product 4 %s", uuid.New().String()), Description: fmt.Sprintf("Description 4 %s", uuid.New().String()), ProductType: "Type B", UnitID: units[1].ID, Status: "active",
			Suppliers: []*models.Supplier{{Base: models.Base{ID: suppliers[2].ID}}},
		},
	}
	require.NoError(t, db.WithContext(ctx).Create(&products).Error, "Failed to create products")

	productIDs := make([]uint, len(products))
	supplierIDs := make([]uint, len(suppliers))
	unitIDs := make([]uint, len(units))
	for i, p := range products {
		productIDs[i] = p.ID
	}
	for i, s := range suppliers {
		supplierIDs[i] = s.ID
	}
	for i, u := range units {
		unitIDs[i] = u.ID
	}

	cleanup = func() {
		if len(productIDs) > 0 {
			db.WithContext(ctx).Where("id IN ?", productIDs).Delete(&models.Product{})
		}
		if len(supplierIDs) > 0 {
			db.WithContext(ctx).Where("id IN ?", supplierIDs).Delete(&models.Supplier{})
		}
		if len(unitIDs) > 0 {
			db.WithContext(ctx).Where("id IN ?", unitIDs).Delete(&models.Unit{})
		}
	}

	return ctx, units, suppliers, products, cleanup
}

// parseCSVExport parses CSV export response
func parseCSVExport(t *testing.T, body []byte) [][]string {
	reader := csv.NewReader(strings.NewReader(string(body)))
	reader.Comma = ';'
	records, err := reader.ReadAll()
	require.NoError(t, err)
	return records
}

// parseExcelExport parses Excel export response
func parseExcelExport(t *testing.T, body []byte) [][]string {
	f, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer f.Close()
	sheetName := f.GetSheetList()[0]
	rows, err := f.GetRows(sheetName)
	require.NoError(t, err)
	if len(rows) == 0 {
		return rows
	}

	expectedLen := len(rows[0])
	for i := range rows {
		if len(rows[i]) < expectedLen {
			padded := make([]string, expectedLen)
			copy(padded, rows[i])
			rows[i] = padded
		}
	}
	return rows
}

// buildProductRowsMap groups export rows by product name
func buildProductRowsMap(rows [][]string) map[string][][]string {
	productRows := make(map[string][][]string)
	for i := 1; i < len(rows); i++ {
		if len(rows[i]) >= 1 && rows[i][0] != "" {
			productRows[rows[i][0]] = append(productRows[rows[i][0]], rows[i])
		}
	}
	return productRows
}

// verifyExportFilter verifies exported products match expected filters
func verifyExportFilter(t *testing.T, rows [][]string, expectedProducts map[string]bool) {
	productNames := make(map[string]bool)
	for i := 1; i < len(rows); i++ {
		if len(rows[i]) >= 1 && rows[i][0] != "" {
			productNames[rows[i][0]] = true
		}
	}
	for name, shouldExist := range expectedProducts {
		assert.Equal(t, shouldExist, productNames[name], fmt.Sprintf("Product %s should%s be in export", name, map[bool]string{true: "", false: " not"}[shouldExist]))
	}
}

func (suite *ComponentTestSuite) TestExportProductsToCSV() {
	t := suite.T()
	_, units, suppliers, products, cleanup := suite.setupExportTestData(t)
	defer pkg.CleanUp(t, func() error { cleanup(); return nil })

	_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
	require.NoError(t, err)

	t.Run("should export all products to CSV", func(t *testing.T) {
		resp, err := helpers.MakeRequest(t, "GET", suite.sharedTestContainer.BaseURL+"/api/v1/products/export-csv", token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "text/csv; charset=utf-8", resp.Header.Get("Content-Type"))
		assert.Contains(t, resp.Header.Get("Content-Disposition"), "attachment; filename=products_export.csv")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		records := parseCSVExport(t, body)

		expectedHeader := []string{"Name", "Description", "ProductType", "Unit", "Suppliers", "ContactEmail", "ContactPhone", "Address"}
		assert.Equal(t, expectedHeader, records[0])
		assert.GreaterOrEqual(t, len(records), 6, "Should have at least 1 header + 5 data rows")

		productRows := buildProductRowsMap(records)
		supplierInfo := make(map[string]*models.Supplier)
		for i := range suppliers {
			supplierInfo[suppliers[i].Name] = &suppliers[i]
		}

		// Verify Product 1 (2 suppliers = 2 rows)
		p1Rows := productRows[products[0].Name]
		require.Len(t, p1Rows, 2, fmt.Sprintf("%s should have 2 rows", products[0].Name))
		for _, row := range p1Rows {
			assert.Equal(t, []string{products[0].Name, products[0].Description, products[0].ProductType}, row[:3])
			assert.Equal(t, units[0].Name, row[3])
			assert.Contains(t, []string{suppliers[0].Name, suppliers[1].Name}, row[4])

			info, ok := supplierInfo[row[4]]
			require.True(t, ok, "unexpected supplier for Product 1 row")
			assert.Equal(t, info.ContactEmail, row[5])
			assert.Equal(t, info.ContactPhone, row[6])
			assert.Equal(t, info.Address, row[7])
		}

		// Verify Product 2
		p2Rows := productRows[products[1].Name]
		require.Len(t, p2Rows, 1)
		assert.Equal(t, []string{
			products[1].Name,
			products[1].Description,
			products[1].ProductType,
			units[1].Name,
			suppliers[1].Name,
			suppliers[1].ContactEmail,
			suppliers[1].ContactPhone,
			suppliers[1].Address,
		}, p2Rows[0])

		// Verify Product 3 (no suppliers)
		p3Rows := productRows[products[2].Name]
		require.Len(t, p3Rows, 1)
		assert.Equal(t, []string{
			products[2].Name,
			products[2].Description,
			products[2].ProductType,
			units[0].Name,
			"",
			"",
			"",
			"",
		}, p3Rows[0])

		// Verify Product 4
		p4Rows := productRows[products[3].Name]
		require.Len(t, p4Rows, 1)
		assert.Equal(t, []string{
			products[3].Name,
			products[3].Description,
			products[3].ProductType,
			units[1].Name,
			suppliers[2].Name,
			suppliers[2].ContactEmail,
			suppliers[2].ContactPhone,
			suppliers[2].Address,
		}, p4Rows[0])
	})

	testCases := []struct {
		name             string
		url              string
		expectedProducts map[string]bool
	}{
		{
			name: "should export products filtered by status",
			url:  suite.sharedTestContainer.BaseURL + "/api/v1/products/export-csv?status=active",
			expectedProducts: map[string]bool{
				products[0].Name: true,
				products[1].Name: true,
				products[2].Name: false,
				products[3].Name: true,
			},
		},
		{
			name: "should export products filtered by product_type",
			url:  fmt.Sprintf("%s/api/v1/products/export-csv?product_type=%s", suite.sharedTestContainer.BaseURL, url.QueryEscape("Type A")),
			expectedProducts: map[string]bool{
				products[0].Name: true,
				products[1].Name: true,
				products[2].Name: false,
				products[3].Name: false,
			},
		},
		{
			name: "should export products filtered by supplier_id",
			url:  fmt.Sprintf("%s/api/v1/products/export-csv?supplier_id=%d", suite.sharedTestContainer.BaseURL, suppliers[1].ID),
			expectedProducts: map[string]bool{
				products[0].Name: true,
				products[1].Name: true,
				products[2].Name: false,
				products[3].Name: false,
			},
		},
		{
			name: "should export products with combined filters",
			url:  fmt.Sprintf("%s/api/v1/products/export-csv?status=active&product_type=%s", suite.sharedTestContainer.BaseURL, url.QueryEscape("Type A")),
			expectedProducts: map[string]bool{
				products[0].Name: true,
				products[1].Name: true,
				products[2].Name: false,
				products[3].Name: false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := helpers.MakeRequest(t, "GET", tc.url, token, nil)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, 200, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			records := parseCSVExport(t, body)
			verifyExportFilter(t, records, tc.expectedProducts)
		})
	}
}

func (suite *ComponentTestSuite) TestExportProductsToExcel() {
	t := suite.T()
	_, units, suppliers, products, cleanup := suite.setupExportTestData(t)
	defer pkg.CleanUp(t, func() error { cleanup(); return nil })

	_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
	require.NoError(t, err)

	t.Run("should export all products to Excel", func(t *testing.T) {
		resp, err := helpers.MakeRequest(t, "GET", suite.sharedTestContainer.BaseURL+"/api/v1/products/export-excel", token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", resp.Header.Get("Content-Type"))
		assert.Contains(t, resp.Header.Get("Content-Disposition"), "attachment; filename=products_export.xlsx")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		rows := parseExcelExport(t, body)

		expectedHeader := []string{"Name", "Description", "ProductType", "Unit", "Suppliers", "ContactEmail", "ContactPhone", "Address"}
		assert.Equal(t, expectedHeader, rows[0])
		assert.GreaterOrEqual(t, len(rows), 6, "Should have at least 1 header + 5 data rows")

		productRows := buildProductRowsMap(rows)
		supplierInfo := make(map[string]*models.Supplier)
		for i := range suppliers {
			supplierInfo[suppliers[i].Name] = &suppliers[i]
		}

		// Verify Product 1 (2 suppliers = 2 rows)
		p1Rows := productRows[products[0].Name]
		require.Len(t, p1Rows, 2, fmt.Sprintf("%s should have 2 rows", products[0].Name))
		for _, row := range p1Rows {
			assert.Equal(t, []string{products[0].Name, products[0].Description, products[0].ProductType}, row[:3])
			assert.Equal(t, units[0].Name, row[3])
			assert.Contains(t, []string{suppliers[0].Name, suppliers[1].Name}, row[4])

			info, ok := supplierInfo[row[4]]
			require.True(t, ok, "unexpected supplier for Product 1 row")
			assert.Equal(t, info.ContactEmail, row[5])
			assert.Equal(t, info.ContactPhone, row[6])
			assert.Equal(t, info.Address, row[7])
		}

		// Verify Product 2
		p2Rows := productRows[products[1].Name]
		require.Len(t, p2Rows, 1)
		assert.Equal(t, []string{
			products[1].Name,
			products[1].Description,
			products[1].ProductType,
			units[1].Name,
			suppliers[1].Name,
			suppliers[1].ContactEmail,
			suppliers[1].ContactPhone,
			suppliers[1].Address,
		}, p2Rows[0])

		// Verify Product 3 (no suppliers)
		p3Rows := productRows[products[2].Name]
		require.Len(t, p3Rows, 1)
		assert.Equal(t, []string{
			products[2].Name,
			products[2].Description,
			products[2].ProductType,
			units[0].Name,
			"",
			"",
			"",
			"",
		}, p3Rows[0])

		// Verify Product 4
		p4Rows := productRows[products[3].Name]
		require.Len(t, p4Rows, 1)
		assert.Equal(t, []string{
			products[3].Name,
			products[3].Description,
			products[3].ProductType,
			units[1].Name,
			suppliers[2].Name,
			suppliers[2].ContactEmail,
			suppliers[2].ContactPhone,
			suppliers[2].Address,
		}, p4Rows[0])
	})

	testCases := []struct {
		name             string
		url              string
		expectedProducts map[string]bool
	}{
		{
			name: "should export products filtered by status",
			url:  suite.sharedTestContainer.BaseURL + "/api/v1/products/export-excel?status=active",
			expectedProducts: map[string]bool{
				products[0].Name: true,
				products[1].Name: true,
				products[2].Name: false,
				products[3].Name: true,
			},
		},
		{
			name: "should export products filtered by product_type",
			url:  fmt.Sprintf("%s/api/v1/products/export-excel?product_type=%s", suite.sharedTestContainer.BaseURL, url.QueryEscape("Type A")),
			expectedProducts: map[string]bool{
				products[0].Name: true,
				products[1].Name: true,
				products[2].Name: false,
				products[3].Name: false,
			},
		},
		{
			name: "should export products filtered by supplier_id",
			url:  fmt.Sprintf("%s/api/v1/products/export-excel?supplier_id=%d", suite.sharedTestContainer.BaseURL, suppliers[1].ID),
			expectedProducts: map[string]bool{
				products[0].Name: true,
				products[1].Name: true,
				products[2].Name: false,
				products[3].Name: false,
			},
		},
		{
			name: "should export products with combined filters",
			url:  fmt.Sprintf("%s/api/v1/products/export-excel?status=active&product_type=%s", suite.sharedTestContainer.BaseURL, url.QueryEscape("Type A")),
			expectedProducts: map[string]bool{
				products[0].Name: true,
				products[1].Name: true,
				products[2].Name: false,
				products[3].Name: false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := helpers.MakeRequest(t, "GET", tc.url, token, nil)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, 200, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			rows := parseExcelExport(t, body)
			verifyExportFilter(t, rows, tc.expectedProducts)
		})
	}
}
