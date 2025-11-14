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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
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
			assert.Equal(t, float64(12), importProductsResp["count"]) // CSV contains 12 unique products (including COCA COLA 500ML without supplier)
			assert.Equal(t, "Products imported successfully", importProductsResp["message"])

			// Verify first product: PEPSI 390 ml (has multiple suppliers due to duplicate name)
			var products []*models.Product
			err = db.WithContext(testCtx).Preload("Unit").Preload("Suppliers").Where("name = ?", "PEPSI 390 ml").Find(&products).Error
			require.NoError(t, err)
			assert.Equal(t, 1, len(products))
			assert.Equal(t, "PEPSI 390 ml", products[0].Name)
			assert.Equal(t, "PEPSI 390 ml", products[0].Description)
			assert.Equal(t, "NƯỚC", products[0].ProductType)
			assert.Equal(t, "active", products[0].Status)
			require.NotNil(t, products[0].Unit)
			assert.Equal(t, "lốc", products[0].Unit.Symbol)
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
			err = db.WithContext(testCtx).Preload("Unit").Preload("Suppliers").Where("name = ?", "PEPSI LON 320 ML").Find(&products2).Error
			require.NoError(t, err)
			assert.Equal(t, 1, len(products2))
			assert.Equal(t, "PEPSI LON 320 ML", products2[0].Name)
			assert.Equal(t, "PEPSI LON 320 ML", products2[0].Description)
			assert.Equal(t, "NƯỚC", products2[0].ProductType)
			require.NotNil(t, products2[0].Unit)
			assert.Equal(t, "thùng", products2[0].Unit.Symbol)
			// Verify product has PEPSI supplier
			assert.Equal(t, 1, len(products2[0].Suppliers))
			assert.Equal(t, "PEPSI", products2[0].Suppliers[0].Name)

			// Verify third product: FANTA LON XÁ XỊ 320 ML
			var products3 []*models.Product
			err = db.WithContext(testCtx).Preload("Unit").Preload("Suppliers").Where("name = ?", "FANTA LON XÁ XỊ  320 ML").Find(&products3).Error
			require.NoError(t, err)
			assert.Equal(t, 1, len(products3))
			assert.Equal(t, "FANTA LON XÁ XỊ  320 ML", products3[0].Name)
			assert.Equal(t, "FANTA LON XÁ XỊ  320 ML", products3[0].Description)
			assert.Equal(t, "NƯỚC", products3[0].ProductType)
			require.NotNil(t, products3[0].Unit)
			assert.Equal(t, "lốc", products3[0].Unit.Symbol)
			// Verify product has COCACOLA supplier
			assert.Equal(t, 1, len(products3[0].Suppliers))
			assert.Equal(t, "COCACOLA", products3[0].Suppliers[0].Name)

			// Verify fourth product: MILO NẮP VẬN 210 ML
			var products4 []*models.Product
			err = db.WithContext(testCtx).Preload("Unit").Preload("Suppliers").Where("name = ?", "MILO NẮP VẬN 210 ML").Find(&products4).Error
			require.NoError(t, err)
			assert.Equal(t, 1, len(products4))
			assert.Equal(t, "MILO NẮP VẬN 210 ML", products4[0].Name)
			assert.Equal(t, "MILO NẮP VẬN 210 ML", products4[0].Description)
			assert.Equal(t, "NƯỚC", products4[0].ProductType)
			require.NotNil(t, products4[0].Unit)
			assert.Equal(t, "thùng", products4[0].Unit.Symbol)
			// Verify product has SỮA MILO supplier
			assert.Equal(t, 1, len(products4[0].Suppliers))
			assert.Equal(t, "SỮA MILO", products4[0].Suppliers[0].Name)

			// Verify fifth product: SỮA BẮP THÁI SƠN
			var products5 []*models.Product
			err = db.WithContext(testCtx).Preload("Unit").Preload("Suppliers").Where("name = ?", "SỮA BẮP THÁI SƠN").Find(&products5).Error
			require.NoError(t, err)
			assert.Equal(t, 1, len(products5))
			assert.Equal(t, "SỮA BẮP THÁI SƠN", products5[0].Name)
			assert.Equal(t, "SỮA BẮP THÁI SƠN", products5[0].Description)
			assert.Equal(t, "ĂN NHẸ", products5[0].ProductType)
			require.NotNil(t, products5[0].Unit)
			assert.Equal(t, "chai", products5[0].Unit.Symbol)
			// Verify product has SỮA THÁI SƠN supplier
			assert.Equal(t, 1, len(products5[0].Suppliers))
			assert.Equal(t, "SỮA THÁI SƠN", products5[0].Suppliers[0].Name)

			// Verify product without supplier: COCA COLA 500ML (line 13 has product name but no supplier)
			var products6 []*models.Product
			err = db.WithContext(testCtx).Preload("Unit").Preload("Suppliers").Where("name = ?", "COCA COLA 500ML").Find(&products6).Error
			require.NoError(t, err)
			assert.Equal(t, 1, len(products6))
			assert.Equal(t, "COCA COLA 500ML", products6[0].Name)
			assert.Equal(t, "COCA COLA 500ML", products6[0].Description)
			assert.Equal(t, "NƯỚC", products6[0].ProductType)
			require.NotNil(t, products6[0].Unit)
			assert.Equal(t, "thùng", products6[0].Unit.Symbol)
			// Verify product has no suppliers (supplier name was missing in CSV)
			assert.Equal(t, 0, len(products6[0].Suppliers))

			// Verify product type: ĂN NHẸ
			var products7 []*models.Product
			err = db.WithContext(testCtx).Preload("Unit").Preload("Suppliers").Where("name = ?", "CƠM THỊT KHO TRỨNG").Find(&products7).Error
			require.NoError(t, err)
			assert.Equal(t, 1, len(products7))
			assert.Equal(t, "CƠM THỊT KHO TRỨNG", products7[0].Name)
			assert.Equal(t, "CƠM THỊT KHO TRỨNG", products7[0].Description)
			assert.Equal(t, "CƠM", products7[0].ProductType)
			assert.Equal(t, "active", products7[0].Status)
			assert.Equal(t, "phần", products7[0].Unit.Symbol)
			assert.Equal(t, "phần", products7[0].Unit.Name)
			assert.Equal(t, 1, len(products7[0].Suppliers))
			assert.Equal(t, "NHÀ HÀNG 5 SAO", products7[0].Suppliers[0].Name)
			assert.Equal(t, "5stars@example.com", products7[0].Suppliers[0].ContactEmail)
			assert.Equal(t, "028-3896100", products7[0].Suppliers[0].ContactPhone)
			assert.Equal(t, "1, Tân Kỳ Tân Quý, TPHCM", products7[0].Suppliers[0].Address)

			// Verify total unique products count
			var allProducts []*models.Product
			err = db.WithContext(testCtx).Find(&allProducts).Error
			require.NoError(t, err)
			assert.Equal(t, 12, len(allProducts))
			// Collect all product IDs for cleanup
			for _, p := range allProducts {
				productIDs = append(productIDs, p.ID)
			}

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
			if len(suppliers4) > 0 {
				supplierIDs = append(supplierIDs, suppliers4[0].ID)
			}

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

			// Verify total unique suppliers count
			var allSuppliers []*models.Supplier
			db.WithContext(testCtx).Find(&allSuppliers)
			assert.Equal(t, 6, len(allSuppliers))
			for _, s := range allSuppliers {
				supplierIDs = append(supplierIDs, s.ID)
			}

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

func (suite *ComponentTestSuite) TestExportProductsToCSV() {
	t := suite.T()
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
	db := suite.sharedTestContainer.DB

	// Setup test data
	testUnit1 := models.Unit{
		UnitType:         "general",
		Name:             "Thùng",
		Symbol:           "Thùng",
		ConversionFactor: 1,
	}
	testUnit2 := models.Unit{
		UnitType:         "general",
		Name:             "Lốc",
		Symbol:           "Lốc",
		ConversionFactor: 1,
	}
	err := db.WithContext(ctx).Create(&testUnit1).Error
	require.NoError(t, err, "Failed to create unit 1")
	err = db.WithContext(ctx).Create(&testUnit2).Error
	require.NoError(t, err, "Failed to create unit 2")

	testSuppliers := []models.Supplier{
		{
			Name:         "Supplier A",
			ContactEmail: "suppliera@example.com",
			ContactPhone: "123-456-7890",
			Address:      "123 Main St",
			Status:       "active",
		},
		{
			Name:         "Supplier B",
			ContactEmail: "supplierb@example.com",
			ContactPhone: "098-765-4321",
			Address:      "456 Oak Ave",
			Status:       "active",
		},
		{
			Name:         "Supplier C",
			ContactEmail: "",
			ContactPhone: "",
			Address:      "",
			Status:       "active",
		},
	}
	err = db.WithContext(ctx).Create(&testSuppliers).Error
	require.NoError(t, err, "Failed to create suppliers")

	testProducts := []models.Product{
		{
			Name:        "Product 1",
			Description: "Description 1",
			ProductType: "Type A",
			UnitID:      testUnit1.ID,
			Status:      "active",
			Suppliers: []*models.Supplier{
				{Base: models.Base{ID: testSuppliers[0].ID}},
				{Base: models.Base{ID: testSuppliers[1].ID}},
			},
		},
		{
			Name:        "Product 2",
			Description: "Description 2",
			ProductType: "Type A",
			UnitID:      testUnit2.ID,
			Status:      "active",
			Suppliers: []*models.Supplier{
				{Base: models.Base{ID: testSuppliers[1].ID}},
			},
		},
		{
			Name:        "Product 3",
			Description: "Description 3",
			ProductType: "Type B",
			UnitID:      testUnit1.ID,
			Status:      "inactive",
			Suppliers:   []*models.Supplier{},
		},
		{
			Name:        "Product 4",
			Description: "Description 4",
			ProductType: "Type B",
			UnitID:      testUnit2.ID,
			Status:      "active",
			Suppliers: []*models.Supplier{
				{Base: models.Base{ID: testSuppliers[2].ID}},
			},
		},
	}
	err = db.WithContext(ctx).Create(&testProducts).Error
	require.NoError(t, err, "Failed to create products")

	var productIDs []uint
	var supplierIDs []uint
	var unitIDs []uint
	for _, p := range testProducts {
		productIDs = append(productIDs, p.ID)
	}
	for _, s := range testSuppliers {
		supplierIDs = append(supplierIDs, s.ID)
	}
	unitIDs = append(unitIDs, testUnit1.ID, testUnit2.ID)

	defer pkg.CleanUp(t, func() error {
		if len(productIDs) > 0 {
			if err := db.WithContext(ctx).Where("id IN ?", productIDs).Delete(&models.Product{}).Error; err != nil {
				return err
			}
		}
		if len(supplierIDs) > 0 {
			if err := db.WithContext(ctx).Where("id IN ?", supplierIDs).Delete(&models.Supplier{}).Error; err != nil {
				return err
			}
		}
		if len(unitIDs) > 0 {
			if err := db.WithContext(ctx).Where("id IN ?", unitIDs).Delete(&models.Unit{}).Error; err != nil {
				return err
			}
		}
		return nil
	})

	_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
	require.NoError(t, err)

	t.Run("should export all products to CSV", func(t *testing.T) {
		resp, err := helpers.MakeRequest(t, "GET", suite.sharedTestContainer.BaseURL+"/api/v1/products/export-csv", token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "text/csv; charset=utf-8", resp.Header.Get("Content-Type"))
		assert.Contains(t, resp.Header.Get("Content-Disposition"), "attachment; filename=products_export.csv")

		// Parse CSV response
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		reader := csv.NewReader(strings.NewReader(string(body)))
		reader.Comma = ';'
		records, err := reader.ReadAll()
		require.NoError(t, err)

		// Verify header
		assert.Equal(t, []string{"Name", "Description", "ProductType", "Suppliers", "ContactEmail", "ContactPhone", "Address", "Unit"}, records[0])

		// Verify data rows
		// Product 1 has 2 suppliers, so should have 2 rows
		// Product 2 has 1 supplier, so should have 1 row
		// Product 3 has no suppliers, so should have 1 row with empty supplier fields
		// Product 4 has 1 supplier, so should have 1 row
		// Total: 1 header + 5 data rows = 6 rows
		assert.GreaterOrEqual(t, len(records), 5, "Should have at least 5 data rows")

		// Build a map of product names to their rows
		productRows := make(map[string][][]string)
		for i := 1; i < len(records); i++ {
			if len(records[i]) >= 1 {
				productName := records[i][0]
				productRows[productName] = append(productRows[productName], records[i])
			}
		}

		// Verify Product 1 (has 2 suppliers)
		product1Rows, exists := productRows["Product 1"]
		require.True(t, exists, "Product 1 should be in export")
		assert.Equal(t, 2, len(product1Rows), "Product 1 should have 2 rows (one per supplier)")
		for _, row := range product1Rows {
			assert.Equal(t, "Product 1", row[0])
			assert.Equal(t, "Description 1", row[1])
			assert.Equal(t, "Type A", row[2])
			assert.Contains(t, []string{"Supplier A", "Supplier B"}, row[3])
			if row[3] == "Supplier A" {
				assert.Equal(t, "suppliera@example.com", row[4])
				assert.Equal(t, "123-456-7890", row[5])
				assert.Equal(t, "123 Main St", row[6])
			} else if row[3] == "Supplier B" {
				assert.Equal(t, "supplierb@example.com", row[4])
				assert.Equal(t, "098-765-4321", row[5])
				assert.Equal(t, "456 Oak Ave", row[6])
			}
			assert.Equal(t, "Thùng", row[7])
		}

		// Verify Product 2 (has 1 supplier)
		product2Rows, exists := productRows["Product 2"]
		require.True(t, exists, "Product 2 should be in export")
		assert.Equal(t, 1, len(product2Rows), "Product 2 should have 1 row")
		assert.Equal(t, "Product 2", product2Rows[0][0])
		assert.Equal(t, "Description 2", product2Rows[0][1])
		assert.Equal(t, "Type A", product2Rows[0][2])
		assert.Equal(t, "Supplier B", product2Rows[0][3])
		assert.Equal(t, "supplierb@example.com", product2Rows[0][4])
		assert.Equal(t, "098-765-4321", product2Rows[0][5])
		assert.Equal(t, "456 Oak Ave", product2Rows[0][6])
		assert.Equal(t, "Lốc", product2Rows[0][7])

		// Verify Product 3 (no suppliers)
		product3Rows, exists := productRows["Product 3"]
		require.True(t, exists, "Product 3 should be in export")
		assert.Equal(t, 1, len(product3Rows), "Product 3 should have 1 row")
		assert.Equal(t, "Product 3", product3Rows[0][0])
		assert.Equal(t, "Description 3", product3Rows[0][1])
		assert.Equal(t, "Type B", product3Rows[0][2])
		assert.Equal(t, "", product3Rows[0][3], "Supplier should be empty")
		assert.Equal(t, "", product3Rows[0][4], "ContactEmail should be empty")
		assert.Equal(t, "", product3Rows[0][5], "ContactPhone should be empty")
		assert.Equal(t, "", product3Rows[0][6], "Address should be empty")
		assert.Equal(t, "Thùng", product3Rows[0][7])

		// Verify Product 4 (has 1 supplier with empty contact info)
		product4Rows, exists := productRows["Product 4"]
		require.True(t, exists, "Product 4 should be in export")
		assert.Equal(t, 1, len(product4Rows), "Product 4 should have 1 row")
		assert.Equal(t, "Product 4", product4Rows[0][0])
		assert.Equal(t, "Description 4", product4Rows[0][1])
		assert.Equal(t, "Type B", product4Rows[0][2])
		assert.Equal(t, "Supplier C", product4Rows[0][3])
		assert.Equal(t, "", product4Rows[0][4], "ContactEmail should be empty")
		assert.Equal(t, "", product4Rows[0][5], "ContactPhone should be empty")
		assert.Equal(t, "", product4Rows[0][6], "Address should be empty")
		assert.Equal(t, "Lốc", product4Rows[0][7])
	})

	t.Run("should export products filtered by status", func(t *testing.T) {
		resp, err := helpers.MakeRequest(t, "GET", suite.sharedTestContainer.BaseURL+"/api/v1/products/export-csv?status=active", token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		reader := csv.NewReader(strings.NewReader(string(body)))
		reader.Comma = ';'
		records, err := reader.ReadAll()
		require.NoError(t, err)

		// Verify only active products are exported (Product 1, 2, 4 - not Product 3)
		productNames := make(map[string]bool)
		for i := 1; i < len(records); i++ {
			if len(records[i]) >= 1 {
				productNames[records[i][0]] = true
			}
		}

		assert.True(t, productNames["Product 1"], "Product 1 should be in export")
		assert.True(t, productNames["Product 2"], "Product 2 should be in export")
		assert.False(t, productNames["Product 3"], "Product 3 (inactive) should not be in export")
		assert.True(t, productNames["Product 4"], "Product 4 should be in export")
	})

	t.Run("should export products filtered by product_type", func(t *testing.T) {
		urlPath := fmt.Sprintf("%s/api/v1/products/export-csv?product_type=%s", suite.sharedTestContainer.BaseURL, url.QueryEscape("Type A"))
		resp, err := helpers.MakeRequest(t, "GET", urlPath, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		reader := csv.NewReader(strings.NewReader(string(body)))
		reader.Comma = ';'
		records, err := reader.ReadAll()
		require.NoError(t, err)

		// Verify only Type A products are exported (Product 1, 2)
		productNames := make(map[string]bool)
		for i := 1; i < len(records); i++ {
			if len(records[i]) >= 1 {
				productNames[records[i][0]] = true
				assert.Equal(t, "Type A", records[i][2], "All exported products should be Type A")
			}
		}

		assert.True(t, productNames["Product 1"], "Product 1 should be in export")
		assert.True(t, productNames["Product 2"], "Product 2 should be in export")
		assert.False(t, productNames["Product 3"], "Product 3 (Type B) should not be in export")
		assert.False(t, productNames["Product 4"], "Product 4 (Type B) should not be in export")
	})

	t.Run("should export products filtered by supplier_id", func(t *testing.T) {
		supplierID := testSuppliers[1].ID // Supplier B
		resp, err := helpers.MakeRequest(t, "GET", fmt.Sprintf("%s/api/v1/products/export-csv?supplier_id=%d", suite.sharedTestContainer.BaseURL, supplierID), token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		reader := csv.NewReader(strings.NewReader(string(body)))
		reader.Comma = ';'
		records, err := reader.ReadAll()
		require.NoError(t, err)

		// Verify only products with Supplier B are exported (Product 1, 2)
		productNames := make(map[string]bool)
		for i := 1; i < len(records); i++ {
			if len(records[i]) >= 1 {
				productNames[records[i][0]] = true
			}
		}

		assert.True(t, productNames["Product 1"], "Product 1 should be in export (has Supplier B)")
		assert.True(t, productNames["Product 2"], "Product 2 should be in export (has Supplier B)")
		assert.False(t, productNames["Product 3"], "Product 3 should not be in export (no suppliers)")
		assert.False(t, productNames["Product 4"], "Product 4 should not be in export (has Supplier C)")
	})

	t.Run("should export products with combined filters", func(t *testing.T) {
		urlPath := fmt.Sprintf("%s/api/v1/products/export-csv?status=active&product_type=%s", suite.sharedTestContainer.BaseURL, url.QueryEscape("Type A"))
		resp, err := helpers.MakeRequest(t, "GET", urlPath, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		reader := csv.NewReader(strings.NewReader(string(body)))
		reader.Comma = ';'
		records, err := reader.ReadAll()
		require.NoError(t, err)

		// Verify only active Type A products are exported (Product 1, 2)
		productNames := make(map[string]bool)
		for i := 1; i < len(records); i++ {
			if len(records[i]) >= 1 {
				productNames[records[i][0]] = true
				assert.Equal(t, "Type A", records[i][2], "All exported products should be Type A")
			}
		}

		assert.True(t, productNames["Product 1"], "Product 1 should be in export")
		assert.True(t, productNames["Product 2"], "Product 2 should be in export")
		assert.False(t, productNames["Product 3"], "Product 3 should not be in export (inactive)")
		assert.False(t, productNames["Product 4"], "Product 4 should not be in export (Type B)")
	})
}

func (suite *ComponentTestSuite) TestExportProductsToExcel() {
	t := suite.T()
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
	db := suite.sharedTestContainer.DB

	// Setup test data (same as CSV export test)
	testUnit1 := models.Unit{
		UnitType:         "general",
		Name:             "Thùng",
		Symbol:           "Thùng",
		ConversionFactor: 1,
	}
	testUnit2 := models.Unit{
		UnitType:         "general",
		Name:             "Lốc",
		Symbol:           "Lốc",
		ConversionFactor: 1,
	}
	err := db.WithContext(ctx).Create(&testUnit1).Error
	require.NoError(t, err, "Failed to create unit 1")
	err = db.WithContext(ctx).Create(&testUnit2).Error
	require.NoError(t, err, "Failed to create unit 2")

	testSuppliers := []models.Supplier{
		{
			Name:         "Supplier A",
			ContactEmail: "suppliera@example.com",
			ContactPhone: "123-456-7890",
			Address:      "123 Main St",
			Status:       "active",
		},
		{
			Name:         "Supplier B",
			ContactEmail: "supplierb@example.com",
			ContactPhone: "098-765-4321",
			Address:      "456 Oak Ave",
			Status:       "active",
		},
		{
			Name:         "Supplier C",
			ContactEmail: "",
			ContactPhone: "",
			Address:      "",
			Status:       "active",
		},
	}
	err = db.WithContext(ctx).Create(&testSuppliers).Error
	require.NoError(t, err, "Failed to create suppliers")

	testProducts := []models.Product{
		{
			Name:        "Product 1",
			Description: "Description 1",
			ProductType: "Type A",
			UnitID:      testUnit1.ID,
			Status:      "active",
			Suppliers: []*models.Supplier{
				{Base: models.Base{ID: testSuppliers[0].ID}},
				{Base: models.Base{ID: testSuppliers[1].ID}},
			},
		},
		{
			Name:        "Product 2",
			Description: "Description 2",
			ProductType: "Type A",
			UnitID:      testUnit2.ID,
			Status:      "active",
			Suppliers: []*models.Supplier{
				{Base: models.Base{ID: testSuppliers[1].ID}},
			},
		},
		{
			Name:        "Product 3",
			Description: "Description 3",
			ProductType: "Type B",
			UnitID:      testUnit1.ID,
			Status:      "inactive",
			Suppliers:   []*models.Supplier{},
		},
		{
			Name:        "Product 4",
			Description: "Description 4",
			ProductType: "Type B",
			UnitID:      testUnit2.ID,
			Status:      "active",
			Suppliers: []*models.Supplier{
				{Base: models.Base{ID: testSuppliers[2].ID}},
			},
		},
	}
	err = db.WithContext(ctx).Create(&testProducts).Error
	require.NoError(t, err, "Failed to create products")

	var productIDs []uint
	var supplierIDs []uint
	var unitIDs []uint
	for _, p := range testProducts {
		productIDs = append(productIDs, p.ID)
	}
	for _, s := range testSuppliers {
		supplierIDs = append(supplierIDs, s.ID)
	}
	unitIDs = append(unitIDs, testUnit1.ID, testUnit2.ID)

	defer pkg.CleanUp(t, func() error {
		if len(productIDs) > 0 {
			if err := db.WithContext(ctx).Where("id IN ?", productIDs).Delete(&models.Product{}).Error; err != nil {
				return err
			}
		}
		if len(supplierIDs) > 0 {
			if err := db.WithContext(ctx).Where("id IN ?", supplierIDs).Delete(&models.Supplier{}).Error; err != nil {
				return err
			}
		}
		if len(unitIDs) > 0 {
			if err := db.WithContext(ctx).Where("id IN ?", unitIDs).Delete(&models.Unit{}).Error; err != nil {
				return err
			}
		}
		return nil
	})

	_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
	require.NoError(t, err)

	t.Run("should export all products to Excel", func(t *testing.T) {
		resp, err := helpers.MakeRequest(t, "GET", suite.sharedTestContainer.BaseURL+"/api/v1/products/export-excel", token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", resp.Header.Get("Content-Type"))
		assert.Contains(t, resp.Header.Get("Content-Disposition"), "attachment; filename=products_export.xlsx")

		// Read and parse Excel response
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		f, err := excelize.OpenReader(bytes.NewReader(body))
		require.NoError(t, err)
		defer f.Close()

		// Get sheet name
		sheetName := f.GetSheetList()[0]
		rows, err := f.GetRows(sheetName)
		require.NoError(t, err)

		// Verify header
		assert.Equal(t, []string{"Name", "Description", "ProductType", "Suppliers", "ContactEmail", "ContactPhone", "Address", "Unit"}, rows[0])

		// Verify data rows (at least 5 data rows expected)
		assert.GreaterOrEqual(t, len(rows), 6, "Should have at least header + 5 data rows")

		// Build a map of product names to their rows
		productRows := make(map[string][][]string)
		for i := 1; i < len(rows); i++ {
			if len(rows[i]) >= 1 {
				productName := rows[i][0]
				productRows[productName] = append(productRows[productName], rows[i])
			}
		}

		// Verify Product 1 (has 2 suppliers)
		product1Rows, exists := productRows["Product 1"]
		require.True(t, exists, "Product 1 should be in export")
		assert.Equal(t, 2, len(product1Rows), "Product 1 should have 2 rows (one per supplier)")

		// Verify Product 2 (has 1 supplier)
		product2Rows, exists := productRows["Product 2"]
		require.True(t, exists, "Product 2 should be in export")
		assert.Equal(t, 1, len(product2Rows), "Product 2 should have 1 row")
		assert.Equal(t, "Product 2", product2Rows[0][0])
		assert.Equal(t, "Description 2", product2Rows[0][1])
		assert.Equal(t, "Type A", product2Rows[0][2])
		assert.Equal(t, "Supplier B", product2Rows[0][3])
		assert.Equal(t, "supplierb@example.com", product2Rows[0][4])
		assert.Equal(t, "098-765-4321", product2Rows[0][5])
		assert.Equal(t, "456 Oak Ave", product2Rows[0][6])
		assert.Equal(t, "Lốc", product2Rows[0][7])

		// Verify Product 3 (no suppliers)
		product3Rows, exists := productRows["Product 3"]
		require.True(t, exists, "Product 3 should be in export")
		assert.Equal(t, 1, len(product3Rows), "Product 3 should have 1 row")
		assert.Equal(t, "Product 3", product3Rows[0][0])
		assert.Equal(t, "Description 3", product3Rows[0][1])
		assert.Equal(t, "Type B", product3Rows[0][2])
		assert.Equal(t, "", product3Rows[0][3], "Supplier should be empty")
		assert.Equal(t, "", product3Rows[0][4], "ContactEmail should be empty")
		assert.Equal(t, "", product3Rows[0][5], "ContactPhone should be empty")
		assert.Equal(t, "", product3Rows[0][6], "Address should be empty")
		assert.Equal(t, "Thùng", product3Rows[0][7])

		// Verify Product 4 (has 1 supplier with empty contact info)
		product4Rows, exists := productRows["Product 4"]
		require.True(t, exists, "Product 4 should be in export")
		assert.Equal(t, 1, len(product4Rows), "Product 4 should have 1 row")
		assert.Equal(t, "Product 4", product4Rows[0][0])
		assert.Equal(t, "Description 4", product4Rows[0][1])
		assert.Equal(t, "Type B", product4Rows[0][2])
		assert.Equal(t, "Supplier C", product4Rows[0][3])
		assert.Equal(t, "", product4Rows[0][4], "ContactEmail should be empty")
		assert.Equal(t, "", product4Rows[0][5], "ContactPhone should be empty")
		assert.Equal(t, "", product4Rows[0][6], "Address should be empty")
		assert.Equal(t, "Lốc", product4Rows[0][7])
	})

	t.Run("should export products filtered by status", func(t *testing.T) {
		resp, err := helpers.MakeRequest(t, "GET", suite.sharedTestContainer.BaseURL+"/api/v1/products/export-excel?status=active", token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		f, err := excelize.OpenReader(bytes.NewReader(body))
		require.NoError(t, err)
		defer f.Close()

		sheetName := f.GetSheetList()[0]
		rows, err := f.GetRows(sheetName)
		require.NoError(t, err)

		// Verify only active products are exported (Product 1, 2, 4 - not Product 3)
		productNames := make(map[string]bool)
		for i := 1; i < len(rows); i++ {
			if len(rows[i]) >= 1 {
				productNames[rows[i][0]] = true
			}
		}

		assert.True(t, productNames["Product 1"], "Product 1 should be in export")
		assert.True(t, productNames["Product 2"], "Product 2 should be in export")
		assert.False(t, productNames["Product 3"], "Product 3 (inactive) should not be in export")
		assert.True(t, productNames["Product 4"], "Product 4 should be in export")
	})

	t.Run("should export products filtered by product_type", func(t *testing.T) {
		urlPath := fmt.Sprintf("%s/api/v1/products/export-excel?product_type=%s", suite.sharedTestContainer.BaseURL, url.QueryEscape("Type A"))
		resp, err := helpers.MakeRequest(t, "GET", urlPath, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		f, err := excelize.OpenReader(bytes.NewReader(body))
		require.NoError(t, err)
		defer f.Close()

		sheetName := f.GetSheetList()[0]
		rows, err := f.GetRows(sheetName)
		require.NoError(t, err)

		// Verify only Type A products are exported (Product 1, 2)
		productNames := make(map[string]bool)
		for i := 1; i < len(rows); i++ {
			if len(rows[i]) >= 1 {
				productNames[rows[i][0]] = true
				assert.Equal(t, "Type A", rows[i][2], "All exported products should be Type A")
			}
		}

		assert.True(t, productNames["Product 1"], "Product 1 should be in export")
		assert.True(t, productNames["Product 2"], "Product 2 should be in export")
		assert.False(t, productNames["Product 3"], "Product 3 (Type B) should not be in export")
		assert.False(t, productNames["Product 4"], "Product 4 (Type B) should not be in export")
	})

	t.Run("should export products filtered by supplier_id", func(t *testing.T) {
		supplierID := testSuppliers[1].ID // Supplier B
		resp, err := helpers.MakeRequest(t, "GET", fmt.Sprintf("%s/api/v1/products/export-excel?supplier_id=%d", suite.sharedTestContainer.BaseURL, supplierID), token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		f, err := excelize.OpenReader(bytes.NewReader(body))
		require.NoError(t, err)
		defer f.Close()

		sheetName := f.GetSheetList()[0]
		rows, err := f.GetRows(sheetName)
		require.NoError(t, err)

		// Verify only products with Supplier B are exported (Product 1, 2)
		productNames := make(map[string]bool)
		for i := 1; i < len(rows); i++ {
			if len(rows[i]) >= 1 {
				productNames[rows[i][0]] = true
			}
		}

		assert.True(t, productNames["Product 1"], "Product 1 should be in export (has Supplier B)")
		assert.True(t, productNames["Product 2"], "Product 2 should be in export (has Supplier B)")
		assert.False(t, productNames["Product 3"], "Product 3 should not be in export (no suppliers)")
		assert.False(t, productNames["Product 4"], "Product 4 should not be in export (has Supplier C)")
	})

	t.Run("should export products with combined filters", func(t *testing.T) {
		urlPath := fmt.Sprintf("%s/api/v1/products/export-excel?status=active&product_type=%s", suite.sharedTestContainer.BaseURL, url.QueryEscape("Type A"))
		resp, err := helpers.MakeRequest(t, "GET", urlPath, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		f, err := excelize.OpenReader(bytes.NewReader(body))
		require.NoError(t, err)
		defer f.Close()

		sheetName := f.GetSheetList()[0]
		rows, err := f.GetRows(sheetName)
		require.NoError(t, err)

		// Verify only active Type A products are exported (Product 1, 2)
		productNames := make(map[string]bool)
		for i := 1; i < len(rows); i++ {
			if len(rows[i]) >= 1 {
				productNames[rows[i][0]] = true
				assert.Equal(t, "Type A", rows[i][2], "All exported products should be Type A")
			}
		}

		assert.True(t, productNames["Product 1"], "Product 1 should be in export")
		assert.True(t, productNames["Product 2"], "Product 2 should be in export")
		assert.False(t, productNames["Product 3"], "Product 3 should not be in export (inactive)")
		assert.False(t, productNames["Product 4"], "Product 4 should not be in export (Type B)")
	})
}
