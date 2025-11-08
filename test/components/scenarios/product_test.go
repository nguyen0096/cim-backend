package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/test/components/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func (suite *ComponentTestSuite) TestCreateAndGetProduct() {
	t := suite.T()

	productData := map[string]interface{}{
		"name":         "Test Product",
		"description":  "Test Description",
		"product_type": "test",
		"unit_id":      1,
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
				assert.Equal(t, "Test Product", productResp["name"])
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
		Base: models.Base{ID: 2},
		Name: "Test Unit 2",
	}
	err := db.WithContext(ctx).Create(&testUnit).Error
	require.NoError(t, err, "Failed to create units")
	testSuppliers := []models.Supplier{
		{
			Base: models.Base{ID: 1},
			Name: "Test Supplier 1",
		},
		{
			Base: models.Base{ID: 2},
			Name: "Test Supplier 2",
		},
		{
			Base: models.Base{ID: 3},
			Name: "Test Supplier 3",
		},
		{
			Base: models.Base{ID: 4},
			Name: "Test Supplier 4",
		},
	}
	err = db.WithContext(ctx).Create(&testSuppliers).Error
	require.NoError(t, err, "Failed to create suppliers")
	testProduct := models.Product{
		Name:        "Test Product",
		Description: "Test Description",
		ProductType: "test",
		UnitID:      1,
		Unit: &models.Unit{
			Base:             models.Base{ID: 1},
			UnitType:         "general",
			Name:             "test",
			Symbol:           "test",
			ConversionFactor: 1,
		},
		Status: "active",
		Suppliers: []*models.Supplier{
			{
				Base: models.Base{ID: 1},
				Name: "Test Supplier 1",
			},
			{
				Base: models.Base{ID: 2},
				Name: "Test Supplier 2",
			},
		},
	}
	err = db.WithContext(ctx).Create(&testProduct).Error
	require.NoError(t, err, "Failed to create product")
	productID := testProduct.ID

	updatedProductData := map[string]interface{}{
		"name":         "Test Product Edited",
		"description":  "Test Description Edited",
		"product_type": "test_edited",
		"supplier_ids": []uint{2, 3, 4},
		"unit_id":      2,
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
				assert.Equal(t, "Test Product Edited", updatedProductResp["name"])
				assert.Equal(t, "Test Description Edited", updatedProductResp["description"])
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
	testSuppliers := []models.Supplier{
		{
			Base: models.Base{ID: 1},
			Name: "Test Supplier 1",
		},
	}
	err := db.WithContext(ctx).Create(&testSuppliers).Error
	require.NoError(t, err, "Failed to create suppliers")
	testProduct := models.Product{
		Name:        "Test Product",
		Description: "Test Description",
		ProductType: "test",
		UnitID:      1,
		Unit: &models.Unit{
			Base:             models.Base{ID: 1},
			UnitType:         "general",
			Name:             "test",
			Symbol:           "test",
			ConversionFactor: 1,
		},
		Status: "active",
		Suppliers: []*models.Supplier{
			{
				Base: models.Base{ID: 1},
				Name: "Test Supplier 1",
			},
		},
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
			// Collect product IDs for cleanup
			var productIDs []uint
			var supplierIDs []uint

			// Setup cleanup
			defer pkg.CleanUp(t, func() error {
				// Delete products first (this will also clean up the many-to-many join table)
				if len(productIDs) > 0 {
					if err := db.WithContext(testCtx).Where("id IN ?", productIDs).Delete(&models.Product{}).Error; err != nil {
						return err
					}
				}
				// Then delete suppliers
				if len(supplierIDs) > 0 {
					if err := db.WithContext(testCtx).Where("id IN ?", supplierIDs).Delete(&models.Supplier{}).Error; err != nil {
						return err
					}
				}
				return nil
			})

			// Import products from file
			resp, err := helpers.MakeMultipartRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/products/import-csv", token, file, "file")
			require.NoError(t, err)
			defer resp.Body.Close()
			var importProductsResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&importProductsResp)
			require.NoError(t, err)

			assert.Equal(t, 200, resp.StatusCode, importProductsResp["error"])
			assert.Equal(t, float64(11), importProductsResp["count"]) // CSV contains 11 unique products (including COCA COLA 500ML without supplier)
			assert.Equal(t, "Products imported successfully", importProductsResp["message"])

			// Verify first product: PEPSI 390 ml (has multiple suppliers due to duplicate name)
			var products []*models.Product
			db.WithContext(testCtx).Preload("Unit").Preload("Suppliers").Where("name = ?", "PEPSI 390 ml").Find(&products)
			assert.Equal(t, 1, len(products))
			assert.Equal(t, "PEPSI 390 ml", products[0].Name)
			assert.Equal(t, "PEPSI 390 ml", products[0].Description)
			assert.Equal(t, "NƯỚC", products[0].ProductType)
			assert.Equal(t, "active", products[0].Status)
			require.NotNil(t, products[0].Unit)
			assert.Equal(t, "Lốc", products[0].Unit.Symbol)
			assert.Equal(t, user.Email, products[0].CreatedBy)
			assert.Equal(t, user.Email, products[0].UpdatedBy)
			// Verify product has both PEPSI and COCACOLA suppliers (n-n relationship)
			assert.Equal(t, 2, len(products[0].Suppliers))
			supplierNames := make(map[string]bool)
			for _, s := range products[0].Suppliers {
				supplierNames[s.Name] = true
			}
			assert.True(t, supplierNames["PEPSI"])
			assert.True(t, supplierNames["COCACOLA"])

			// Verify second product: PEPSI LON 320 ML
			var products2 []*models.Product
			db.WithContext(testCtx).Preload("Unit").Preload("Suppliers").Where("name = ?", "PEPSI LON 320 ML").Find(&products2)
			assert.Equal(t, 1, len(products2))
			assert.Equal(t, "PEPSI LON 320 ML", products2[0].Name)
			assert.Equal(t, "PEPSI LON 320 ML", products2[0].Description)
			assert.Equal(t, "NƯỚC", products2[0].ProductType)
			require.NotNil(t, products2[0].Unit)
			assert.Equal(t, "Thùng", products2[0].Unit.Symbol)
			// Verify product has PEPSI supplier
			assert.Equal(t, 1, len(products2[0].Suppliers))
			assert.Equal(t, "PEPSI", products2[0].Suppliers[0].Name)

			// Verify third product: FANTA LON XÁ XỊ 320 ML
			var products3 []*models.Product
			db.WithContext(testCtx).Preload("Unit").Preload("Suppliers").Where("name = ?", "FANTA LON XÁ XỊ  320 ML").Find(&products3)
			assert.Equal(t, 1, len(products3))
			assert.Equal(t, "FANTA LON XÁ XỊ  320 ML", products3[0].Name)
			assert.Equal(t, "FANTA LON XÁ XỊ  320 ML", products3[0].Description)
			assert.Equal(t, "NƯỚC", products3[0].ProductType)
			require.NotNil(t, products3[0].Unit)
			assert.Equal(t, "Lốc", products3[0].Unit.Symbol)
			// Verify product has COCACOLA supplier
			assert.Equal(t, 1, len(products3[0].Suppliers))
			assert.Equal(t, "COCACOLA", products3[0].Suppliers[0].Name)

			// Verify fourth product: MILO NẮP VẬN 210 ML
			var products4 []*models.Product
			db.WithContext(testCtx).Preload("Unit").Preload("Suppliers").Where("name = ?", "MILO NẮP VẬN 210 ML").Find(&products4)
			assert.Equal(t, 1, len(products4))
			assert.Equal(t, "MILO NẮP VẬN 210 ML", products4[0].Name)
			assert.Equal(t, "MILO NẮP VẬN 210 ML", products4[0].Description)
			assert.Equal(t, "NƯỚC", products4[0].ProductType)
			require.NotNil(t, products4[0].Unit)
			assert.Equal(t, "Thùng", products4[0].Unit.Symbol)
			// Verify product has SỮA MILO supplier
			assert.Equal(t, 1, len(products4[0].Suppliers))
			assert.Equal(t, "SỮA MILO", products4[0].Suppliers[0].Name)

			// Verify fifth product: SỮA BẮP THÁI SƠN
			var products5 []*models.Product
			db.WithContext(testCtx).Preload("Unit").Preload("Suppliers").Where("name = ?", "SỮA BẮP THÁI SƠN").Find(&products5)
			assert.Equal(t, 1, len(products5))
			assert.Equal(t, "SỮA BẮP THÁI SƠN", products5[0].Name)
			assert.Equal(t, "SỮA BẮP THÁI SƠN", products5[0].Description)
			assert.Equal(t, "NƯỚC", products5[0].ProductType)
			require.NotNil(t, products5[0].Unit)
			assert.Equal(t, "Chai", products5[0].Unit.Symbol)
			// Verify product has SỮA THÁI SƠN supplier
			assert.Equal(t, 1, len(products5[0].Suppliers))
			assert.Equal(t, "SỮA THÁI SƠN", products5[0].Suppliers[0].Name)

			// Verify product without supplier: COCA COLA 500ML (line 13 has product name but no supplier)
			var products6 []*models.Product
			db.WithContext(testCtx).Preload("Unit").Preload("Suppliers").Where("name = ?", "COCA COLA 500ML").Find(&products6)
			assert.Equal(t, 1, len(products6))
			assert.Equal(t, "COCA COLA 500ML", products6[0].Name)
			assert.Equal(t, "COCA COLA 500ML", products6[0].Description)
			assert.Equal(t, "NƯỚC", products6[0].ProductType)
			require.NotNil(t, products6[0].Unit)
			assert.Equal(t, "Thùng", products6[0].Unit.Symbol)
			// Verify product has no suppliers (supplier name was missing in CSV)
			assert.Equal(t, 0, len(products6[0].Suppliers))

			// Verify total unique products count
			var allProducts []*models.Product
			db.WithContext(testCtx).Find(&allProducts)
			assert.Equal(t, 11, len(allProducts))
			// Collect all product IDs for cleanup
			for _, p := range allProducts {
				productIDs = append(productIDs, p.ID)
			}

			// Verify Supplier: PEPSI
			var suppliers []*models.Supplier
			db.WithContext(testCtx).Preload("Products").Where("name = ?", "PEPSI").Find(&suppliers)
			assert.Equal(t, 1, len(suppliers))
			assert.Equal(t, "PEPSI", suppliers[0].Name)
			assert.Equal(t, "", suppliers[0].ContactEmail)
			assert.Equal(t, "098-7513328", suppliers[0].ContactPhone)
			assert.Equal(t, "202,QUỐC LỘ 13,PHƯỜNG HIỆP BÌNH THÀNH PHỐ HỒ CHÍ MINH VIỆT NAM", suppliers[0].Address)
			// Verify PEPSI supplier has 2 products (PEPSI LON 320 ML and PEPSI 390 ml)
			assert.Equal(t, 2, len(suppliers[0].Products))
			if len(suppliers) > 0 {
				supplierIDs = append(supplierIDs, suppliers[0].ID)
			}

			// Verify Supplier: COCACOLA
			var suppliers2 []*models.Supplier
			db.WithContext(testCtx).Preload("Products").Where("name = ?", "COCACOLA").Find(&suppliers2)
			assert.Equal(t, 1, len(suppliers2))
			assert.Equal(t, "COCACOLA", suppliers2[0].Name)
			assert.Equal(t, "", suppliers2[0].ContactEmail)
			assert.Equal(t, "028-3896100", suppliers2[0].ContactPhone)
			assert.Equal(t, "LÔ C 12,ĐƯỜNG DỌC 2,KHU CÔNG NGHIỆP PHÚ AN,XÃ BẾN LỨC,TỈNH TÂY NINH,VIỆT NAM", suppliers2[0].Address)
			// Verify COCACOLA supplier has 3 products (FANTA LON XÁ XỊ 320 ML, FANTA XÁ XỊ 390 ML, and PEPSI 390 ml)
			// Note: Line 14 has supplier name but no product name, so it only updates the supplier but doesn't add a product
			assert.Equal(t, 3, len(suppliers2[0].Products))
			if len(suppliers2) > 0 {
				supplierIDs = append(supplierIDs, suppliers2[0].ID)
			}

			// Verify Supplier: SỮA MILO
			var suppliers3 []*models.Supplier
			db.WithContext(testCtx).Preload("Products").Where("name = ?", "SỮA MILO").Find(&suppliers3)
			assert.Equal(t, 1, len(suppliers3))
			assert.Equal(t, "SỮA MILO", suppliers3[0].Name)
			assert.Equal(t, "", suppliers3[0].ContactEmail)
			assert.Equal(t, "", suppliers3[0].ContactPhone)
			assert.Equal(t, "415/4 A HOÀNG VĂN THỤ, PHƯỜNG TÂN SƠN HÒA, THÀNH PH", suppliers3[0].Address)
			// Verify SỮA MILO supplier has 2 products
			assert.Equal(t, 2, len(suppliers3[0].Products))
			if len(suppliers3) > 0 {
				supplierIDs = append(supplierIDs, suppliers3[0].ID)
			}

			// Verify Supplier: SỮA THÁI SƠN
			var suppliers4 []*models.Supplier
			db.WithContext(testCtx).Preload("Products").Where("name = ?", "SỮA THÁI SƠN").Find(&suppliers4)
			assert.Equal(t, 1, len(suppliers4))
			assert.Equal(t, "SỮA THÁI SƠN", suppliers4[0].Name)
			assert.Equal(t, "", suppliers4[0].ContactEmail)
			assert.Equal(t, "", suppliers4[0].ContactPhone)
			assert.Equal(t, "", suppliers4[0].Address)
			// Verify SỮA THÁI SƠN supplier has 4 products
			assert.Equal(t, 4, len(suppliers4[0].Products))
			if len(suppliers4) > 0 {
				supplierIDs = append(supplierIDs, suppliers4[0].ID)
			}

			// Verify Supplier: TH TRUE MILK (created from line 14 which has supplier name but no product name)
			var suppliers5 []*models.Supplier
			db.WithContext(testCtx).Preload("Products").Where("name = ?", "TH TRUE MILK").Find(&suppliers5)
			assert.Equal(t, 1, len(suppliers5))
			assert.Equal(t, "TH TRUE MILK", suppliers5[0].Name)
			assert.Equal(t, "", suppliers5[0].ContactEmail)
			assert.Equal(t, "028-3896100", suppliers5[0].ContactPhone)                                                             // From line 14
			assert.Equal(t, "LÔ C 12,ĐƯỜNG DỌC 2,KHU CÔNG NGHIỆP PHÚ AN,XÃ BẾN LỨC,TỈNH TÂY NINH,VIỆT NAM", suppliers5[0].Address) // From line 14
			// Verify TH TRUE MILK supplier has 0 products (line 14 has no product name)
			assert.Equal(t, 0, len(suppliers5[0].Products))
			if len(suppliers5) > 0 {
				supplierIDs = append(supplierIDs, suppliers5[0].ID)
			}

			// Verify total unique suppliers count
			var allSuppliers []*models.Supplier
			db.WithContext(testCtx).Find(&allSuppliers)
			assert.Equal(t, 5, len(allSuppliers))

			// Verify number of products by supplier
			assert.Equal(t, 2, len(suppliers[0].Products))
			assert.Equal(t, 3, len(suppliers2[0].Products))
			assert.Equal(t, 2, len(suppliers3[0].Products))
			assert.Equal(t, 4, len(suppliers4[0].Products))
			assert.Equal(t, 0, len(suppliers5[0].Products))
		})
	}
}
