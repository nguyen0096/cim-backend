package apptest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil"
	"cim-backend/pkg/testutil/fixture"
)

var _ = Describe("Product API", func() {
	Describe("Create and Get Product", func() {
		var testUnit *models.Unit

		BeforeEach(func() {
			testUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				UnitType:         uuid.New().String(),
				Name:             fmt.Sprintf("Test Unit %s", uuid.New().String()),
				ConversionFactor: 1,
			})
		})

		Context("when user has authorized role", func() {
			It("should create and get product with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)
				productName := fmt.Sprintf("Test Product %s", uuid.New().String())
				productDescription := fmt.Sprintf("Test Description %s", uuid.New().String())

				productData := map[string]interface{}{
					"name":         productName,
					"description":  productDescription,
					"product_type": "test",
					"unit_id":      testUnit.ID,
					"supplier_ids": []uint{},
					"status":       "active",
				}

				resp, err := client.MakeRequest("POST", "/api/v1/products", productData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				productResp := testutil.ParseResponse(resp)
				Expect(productResp["id"]).NotTo(BeNil())
				Expect(productResp["name"]).To(Equal(productName))
			})

			It("should create and get product with accountant role", func() {
				client := testutil.NewClient(tenv, models.RoleAccountant)
				productName := fmt.Sprintf("Test Product %s", uuid.New().String())
				productDescription := fmt.Sprintf("Test Description %s", uuid.New().String())

				productData := map[string]interface{}{
					"name":         productName,
					"description":  productDescription,
					"product_type": "test",
					"unit_id":      testUnit.ID,
					"supplier_ids": []uint{},
					"status":       "active",
				}

				resp, err := client.MakeRequest("POST", "/api/v1/products", productData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				productResp := testutil.ParseResponse(resp)
				Expect(productResp["id"]).NotTo(BeNil())
				Expect(productResp["name"]).To(Equal(productName))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not create product with staff role", func() {
				client := testutil.NewClient(tenv, models.RoleStaff)

				productData := map[string]interface{}{
					"name":         "Test Product",
					"description":  "Test Description",
					"product_type": "test",
					"unit_id":      testUnit.ID,
					"supplier_ids": []uint{},
					"status":       "active",
				}

				resp, err := client.MakeRequest("POST", "/api/v1/products", productData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot create products", models.RoleStaff)))
			})

			It("should not create product with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				productData := map[string]interface{}{
					"name":         "Test Product",
					"description":  "Test Description",
					"product_type": "test",
					"unit_id":      testUnit.ID,
					"supplier_ids": []uint{},
					"status":       "active",
				}

				resp, err := client.MakeRequest("POST", "/api/v1/products", productData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot create products", models.RoleBotForm)))
			})
		})

		Context("Image in payload", func() {
			It("should create product with image in form data request", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)
				productName := fmt.Sprintf("Test Product %s", uuid.New().String())
				productDescription := fmt.Sprintf("Test Description %s", uuid.New().String())

				// Create test image
				img := testutil.CreateImage(100, 100)
				imageData := bytes.NewBuffer(nil)
				err := png.Encode(imageData, img)
				Expect(err).NotTo(HaveOccurred())
				imageDataBase64 := base64.StdEncoding.EncodeToString(imageData.Bytes())
				expectedImageDataURL := fmt.Sprintf("data:image/png;base64,%s", imageDataBase64)

				// Create multipart form data
				imageDataForUpload := bytes.NewBuffer(imageData.Bytes())
				multipartData := &testutil.MultipartFormData{
					Fields: map[string]string{
						"name":         productName,
						"description":  productDescription,
						"product_type": "test",
						"unit_id":      strconv.Itoa(int(testUnit.ID)),
						"status":       "active",
					},
					Files: map[string]struct {
						Filename string
						Content  io.Reader
					}{
						"image": {
							Filename: "test.png",
							Content:  imageDataForUpload,
						},
					},
				}

				// Make request
				resp, err := client.MakeRequest("POST", "/api/v1/products", nil, testutil.WithAuth(), testutil.WithMultipartFormData(multipartData))
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				// Verify response
				productResp := testutil.ParseResponse(resp)
				Expect(productResp["id"]).NotTo(BeNil())
				Expect(productResp["name"]).To(Equal(productName))
				Expect(productResp["description"]).To(Equal(productDescription))
				Expect(productResp["product_image"]).To(Equal(expectedImageDataURL))
			})

		})
	})

	Describe("Update Product", func() {
		var testUnit *models.Unit
		var testSuppliers []models.Supplier
		var testProduct *models.Product

		BeforeEach(func() {
			testUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Test Unit %s", uuid.New().String()),
				ConversionFactor: 1,
			})

			supplier1 := fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name: fmt.Sprintf("Test Supplier 1 %s", uuid.New().String()),
			})
			supplier2 := fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name: fmt.Sprintf("Test Supplier 2 %s", uuid.New().String()),
			})
			supplier3 := fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name: fmt.Sprintf("Test Supplier 3 %s", uuid.New().String()),
			})
			supplier4 := fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name: fmt.Sprintf("Test Supplier 4 %s", uuid.New().String()),
			})
			testSuppliers = []models.Supplier{*supplier1, *supplier2, *supplier3, *supplier4}

			testProduct = fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Test Product %s", uuid.New().String()),
				Description: fmt.Sprintf("Test Description %s", uuid.New().String()),
				ProductType: "test",
				UnitID:      testUnit.ID,
				Status:      "active",
				Suppliers:   []*models.Supplier{supplier1, supplier2},
			})
		})

		Context("when user has authorized role", func() {
			It("should update product with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)
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

				urlPath := fmt.Sprintf("/api/v1/products/%d", testProduct.ID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedProductData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedProductResp := testutil.ParseResponse(resp)
				Expect(updatedProductResp["name"]).To(Equal(newProductName))
				Expect(updatedProductResp["description"]).To(Equal(newProductDescription))
				Expect(updatedProductResp["product_type"]).To(Equal("test_edited"))
				Expect(updatedProductResp["status"]).To(Equal("inactive"))

				suppliers := updatedProductResp["suppliers"].([]interface{})
				Expect(suppliers).To(HaveLen(3))
			})

			It("should update product with accountant role", func() {
				client := testutil.NewClient(tenv, models.RoleAccountant)
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

				urlPath := fmt.Sprintf("/api/v1/products/%d", testProduct.ID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedProductData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedProductResp := testutil.ParseResponse(resp)
				Expect(updatedProductResp["name"]).To(Equal(newProductName))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not update product with staff role", func() {
				client := testutil.NewClient(tenv, models.RoleStaff)

				updatedProductData := map[string]interface{}{
					"name":         "Updated Name",
					"description":  "Updated Description",
					"product_type": "test_edited",
					"supplier_ids": []uint{},
					"unit_id":      testUnit.ID,
					"status":       "inactive",
				}

				urlPath := fmt.Sprintf("/api/v1/products/%d", testProduct.ID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedProductData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update products", models.RoleStaff)))
			})

			It("should not update product with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				updatedProductData := map[string]interface{}{
					"name":         "Updated Name",
					"description":  "Updated Description",
					"product_type": "test_edited",
					"supplier_ids": []uint{},
					"unit_id":      testUnit.ID,
					"status":       "inactive",
				}

				urlPath := fmt.Sprintf("/api/v1/products/%d", testProduct.ID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedProductData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update products", models.RoleBotForm)))
			})
		})

		When("Image in payload", func() {
			It("should update product with image in form data request", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)
				newProductName := fmt.Sprintf("Test Product Edited %s", uuid.New().String())
				newProductDescription := fmt.Sprintf("Test Description Edited %s", uuid.New().String())

				img := testutil.CreateImage(100, 100)
				imageData := bytes.NewBuffer(nil)
				err := png.Encode(imageData, img)
				Expect(err).NotTo(HaveOccurred())
				imageDataBase64 := base64.StdEncoding.EncodeToString(imageData.Bytes())
				expectedImageDataURL := fmt.Sprintf("data:image/png;base64,%s", imageDataBase64)

				// Create fresh buffer for file upload (imageData was already read for base64)
				imageDataForUpload := bytes.NewBuffer(imageData.Bytes())

				multipartData := &testutil.MultipartFormData{
					Fields: map[string]string{
						"name":         newProductName,
						"description":  newProductDescription,
						"product_type": "test_edited",
						"unit_id":      strconv.Itoa(int(testUnit.ID)),
						"status":       "active",
					},
					Files: map[string]struct {
						Filename string
						Content  io.Reader
					}{
						"image": {
							Filename: "test.png",
							Content:  imageDataForUpload,
						},
					},
				}

				urlPath := fmt.Sprintf("/api/v1/products/%d", testProduct.ID)
				resp, err := client.MakeRequest("PUT", urlPath, nil, testutil.WithAuth(), testutil.WithMultipartFormData(multipartData))
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedProductResp := testutil.ParseResponse(resp)
				Expect(updatedProductResp["name"]).To(Equal(newProductName))
				Expect(updatedProductResp["description"]).To(Equal(newProductDescription))
				Expect(updatedProductResp["product_image"]).To(Equal(expectedImageDataURL))

				// Should not clear image
				multipartData = &testutil.MultipartFormData{
					Fields: map[string]string{
						"name":         newProductName,
						"description":  newProductDescription,
						"product_type": "test_edited",
						"unit_id":      strconv.Itoa(int(testUnit.ID)),
						"status":       "inactive",
					},
				}
				resp, err = client.MakeRequest("PUT", urlPath, nil, testutil.WithAuth(), testutil.WithMultipartFormData(multipartData))
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedProductResp = testutil.ParseResponse(resp)
				Expect(updatedProductResp["product_image"]).To(Equal(expectedImageDataURL))

				// Verify in database
				var updatedProduct models.Product
				err = tenv.ContextfulDB().First(&updatedProduct, "id = ?", testProduct.ID).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedProductResp["product_image"]).To(Equal(expectedImageDataURL))

				// Should clear image
				multipartData = &testutil.MultipartFormData{
					Fields: map[string]string{
						"name":         newProductName,
						"description":  newProductDescription,
						"product_type": "test_edited",
						"unit_id":      strconv.Itoa(int(testUnit.ID)),
						"status":       "inactive",
						"clear_image":  "true",
					},
				}
				resp, err = client.MakeRequest("PUT", urlPath, nil, testutil.WithAuth(), testutil.WithMultipartFormData(multipartData))
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedProductResp = testutil.ParseResponse(resp)
				Expect(updatedProductResp["product_image"]).To(BeNil())

				// Verify in database
				var updatedProduct2 models.Product
				err = tenv.ContextfulDB().First(&updatedProduct2, "id = ?", testProduct.ID).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedProduct2.ProductImage).To(BeEmpty())
			})
		})
	})

	Describe("Update Product Status", func() {
		var testProduct *models.Product

		BeforeEach(func() {
			testProduct = fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        "Test Product",
				Description: "Test Description",
				UnitID:      1,
				Status:      "active",
			})
		})

		Context("when user has authorized role", func() {
			It("should update product status with admin role", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				urlPath := fmt.Sprintf("/api/v1/products/%d/status", testProduct.ID)
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "inactive"}, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedProductResp := testutil.ParseResponse(resp)
				Expect(updatedProductResp["status"]).To(Equal("inactive"))

				// Verify in database
				var updatedProduct models.Product
				err = tenv.DB.WithContext(ctx).First(&updatedProduct, "id = ?", testProduct.ID).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedProduct.Status).To(Equal("inactive"))
			})

			It("should update product status with accountant role", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAccountant)

				urlPath := fmt.Sprintf("/api/v1/products/%d/status", testProduct.ID)
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "inactive"}, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedProductResp := testutil.ParseResponse(resp)
				Expect(updatedProductResp["status"]).To(Equal("inactive"))

				// Verify in database
				var updatedProduct models.Product
				err = tenv.DB.WithContext(ctx).First(&updatedProduct, "id = ?", testProduct.ID).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedProduct.Status).To(Equal("inactive"))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not update product status with staff role", func() {
				client := testutil.NewClient(tenv, models.RoleStaff)

				urlPath := fmt.Sprintf("/api/v1/products/%d/status", testProduct.ID)
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "inactive"}, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update products", models.RoleStaff)))
			})

			It("should not update product status with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				urlPath := fmt.Sprintf("/api/v1/products/%d/status", testProduct.ID)
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "inactive"}, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update products", models.RoleBotForm)))
			})
		})
	})

	Describe("Delete Product", func() {
		var testUnit *models.Unit
		var testSupplier *models.Supplier
		var testProduct *models.Product

		BeforeEach(func() {
			testUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				UnitType:         uuid.New().String(),
				Name:             fmt.Sprintf("Test Unit %s", uuid.New().String()),
				ConversionFactor: 1,
			})

			testSupplier = fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name: fmt.Sprintf("Test Supplier 1 %s", uuid.New().String()),
			})

			testProduct = fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Test Product %s", uuid.New().String()),
				Description: fmt.Sprintf("Test Description %s", uuid.New().String()),
				ProductType: "test",
				UnitID:      testUnit.ID,
				Status:      "active",
				Suppliers:   []*models.Supplier{testSupplier},
			})
		})

		Context("when user has admin role", func() {
			It("should delete product", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				urlPath := fmt.Sprintf("/api/v1/products/%d", testProduct.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Verify product is deleted
				err = tenv.DB.WithContext(ctx).First(&models.Product{}, "id = ?", testProduct.ID).Error
				Expect(err).To(Equal(gorm.ErrRecordNotFound))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not delete product with accountant role", func() {
				client := testutil.NewClient(tenv, models.RoleAccountant)

				urlPath := fmt.Sprintf("/api/v1/products/%d", testProduct.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot delete products", models.RoleAccountant)))
			})

			It("should not delete product with staff role", func() {
				client := testutil.NewClient(tenv, models.RoleStaff)

				urlPath := fmt.Sprintf("/api/v1/products/%d", testProduct.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot delete products", models.RoleStaff)))
			})

			It("should not delete product with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				urlPath := fmt.Sprintf("/api/v1/products/%d", testProduct.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot delete products", models.RoleBotForm)))
			})
		})
	})

	Describe("Import Products from CSV and Excel", Ordered, func() {
		files := []string{
			pkg.TranslateCallerRelativePath("data/excel/Products_template.csv"),
			pkg.TranslateCallerRelativePath("data/excel/Products_template.xlsx"),
		}

		DescribeTableSubtree("should import products from CSV and Excel", func(file string) {
			var importedProductIDs []uint

			AfterEach(func() {
				// Clean up imported products and their associations before units are cleaned up
				if len(importedProductIDs) > 0 {
					// Delete product-supplier associations first
					tenv.DB.Exec("DELETE FROM product_suppliers WHERE product_id IN ?", importedProductIDs)
					// Delete products
					tenv.DB.Exec("DELETE FROM products WHERE id IN ?", importedProductIDs)
					importedProductIDs = nil
				}
			})

			It(fmt.Sprintf("should import products from %s", file), func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				// Get user email from database
				var user models.User
				err := tenv.DB.Where("role = ?", models.RoleAdmin).Order("created_at DESC").First(&user).Error
				Expect(err).NotTo(HaveOccurred())
				testCtx := pkg.WithUserEmail(ctx, user.Email)

				// Import products from file
				urlPath := fmt.Sprintf("%s/api/v1/products/import-csv", tenv.BaseURL)
				resp, err := testutil.MakeMultipartRequest("POST", urlPath, *client.AuthToken, file, "file")
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()

				var importProductsResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&importProductsResp)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp.StatusCode).To(Equal(200))
				Expect(importProductsResp["count"]).To(Equal(float64(12))) // CSV contains 12 unique products
				Expect(importProductsResp["message"]).To(Equal("Products imported successfully"))

				// Collect all imported product IDs for cleanup
				var allProducts []models.Product
				err = tenv.DB.WithContext(testCtx).Select("id").Where("created_by = ?", user.Email).Find(&allProducts).Error
				Expect(err).NotTo(HaveOccurred())
				for _, p := range allProducts {
					importedProductIDs = append(importedProductIDs, p.ID)
				}

				// verifyProduct verifies a product by name
				verifyProduct := func(name string, expectedType, expectedUnit, expectedStatus string, expectedSupplierNames []string, checkCreatedBy bool) *models.Product {
					var products []*models.Product
					err = tenv.DB.WithContext(testCtx).Preload("Unit").Preload("Suppliers").Where("name = ?", name).Find(&products).Error
					Expect(err).NotTo(HaveOccurred())
					Expect(products).To(HaveLen(1), "Product %s should exist", name)
					p := products[0]
					Expect(p.Name).To(Equal(name))
					Expect(p.ProductType).To(Equal(expectedType))
					if expectedStatus != "" {
						Expect(p.Status).To(Equal(expectedStatus))
					}
					Expect(p.Unit).NotTo(BeNil(), "Product %s should have unit", name)
					Expect(p.Unit.Name).To(Equal(expectedUnit))
					if len(expectedSupplierNames) > 0 {
						Expect(p.Suppliers).To(HaveLen(len(expectedSupplierNames)), "Product %s should have %d suppliers", name, len(expectedSupplierNames))
						supplierNames := make(map[string]bool)
						for _, s := range p.Suppliers {
							supplierNames[s.Name] = true
						}
						for _, expectedName := range expectedSupplierNames {
							Expect(supplierNames[expectedName]).To(BeTrue(), "Product %s should have supplier %s", name, expectedName)
						}
					} else {
						Expect(p.Suppliers).To(HaveLen(0), "Product %s should have no suppliers", name)
					}
					return p
				}

				// Verify first product: PEPSI 390 ml (has multiple suppliers)
				verifyProduct("PEPSI 390 ml", "NƯỚC", "LỐC", "active", []string{"PEPSI", "COCACOLA"}, false)

				// Verify other products
				verifyProduct("PEPSI LON 320 ML", "NƯỚC", "THÙNG", "", []string{"PEPSI"}, false)
				verifyProduct("FANTA LON XÁ XỊ  320 ML", "NƯỚC", "LỐC", "", []string{"COCACOLA"}, false)
				verifyProduct("MILO NẮP VẬN 210 ML", "NƯỚC", "THÙNG", "", []string{"SỮA MILO"}, false)
				verifyProduct("SỮA BẮP THÁI SƠN", "ĂN NHẸ", "CHAI", "", []string{"SỮA THÁI SƠN"}, false)
				verifyProduct("COCA COLA 500ML", "NƯỚC", "THÙNG", "", nil, false)

				// Verify product with full supplier details
				p7 := verifyProduct("CƠM THỊT KHO TRỨNG", "CƠM", "PHẦN", "active", []string{"NHÀ HÀNG 5 SAO"}, false)
				Expect(p7.Unit.Name).To(Equal("PHẦN"))
				Expect(p7.Suppliers[0].ContactEmail).To(Equal("5stars@example.com"))
				Expect(p7.Suppliers[0].ContactPhone).To(Equal("028-3896100"))
				Expect(p7.Suppliers[0].Address).To(Equal("1, Tân Kỳ Tân Quý, TPHCM"))

				// Verify Supplier: PEPSI
				verifySupplier := func(name string, expectedContactEmail, expectedContactPhone, expectedAddress string, expectedLinkedProducts int) {
					var suppliers []*models.Supplier
					err = tenv.DB.WithContext(testCtx).Preload("Products").Where("name = ?", name).Find(&suppliers).Error
					Expect(err).NotTo(HaveOccurred())
					Expect(suppliers).To(HaveLen(1))
					Expect(suppliers[0].Name).To(Equal(name))
					Expect(suppliers[0].ContactEmail).To(Equal(expectedContactEmail))
					Expect(suppliers[0].ContactPhone).To(Equal(expectedContactPhone))
					Expect(suppliers[0].Address).To(Equal(expectedAddress))
					Expect(suppliers[0].Products).To(HaveLen(expectedLinkedProducts))
				}
				verifySupplier("PEPSI", "", "098-7513328", "202,QUỐC LỘ 13,PHƯỜNG HIỆP BÌNH THÀNH PHỐ HỒ CHÍ MINH VIỆT NAM", 2)
				verifySupplier("COCACOLA", "", "028-3896100", "LÔ C 12,ĐƯỜNG DỌC 2,KHU CÔNG NGHIỆP PHÚ AN,XÃ BẾN LỨC,TỈNH TÂY NINH,VIỆT NAM", 3)
				verifySupplier("SỮA MILO", "", "", "415/4 A HOÀNG VĂN THỤ, PHƯỜNG TÂN SƠN HÒA, THÀNH PH", 2)
				verifySupplier("SỮA THÁI SƠN", "", "", "", 4)
				verifySupplier("TH TRUE MILK", "", "028-3896100", "LÔ C 12,ĐƯỜNG DỌC 2,KHU CÔNG NGHIỆP PHÚ AN,XÃ BẾN LỨC,TỈNH TÂY NINH,VIỆT NAM", 0)
				verifySupplier("NHÀ HÀNG 5 SAO", "5stars@example.com", "028-3896100", "1, Tân Kỳ Tân Quý, TPHCM", 1)

				// Verify units are created
				verifyUnit := func(name string, symbol string, conversionFactor float64) {
					var units []*models.Unit
					err = tenv.DB.WithContext(testCtx).Where("name = ?", name).Find(&units).Error
					Expect(err).NotTo(HaveOccurred())
					Expect(units).To(HaveLen(1))
					Expect(units[0].Name).To(Equal(name))
					Expect(units[0].Symbol).To(Equal(symbol))
					Expect(units[0].ConversionFactor).To(Equal(conversionFactor))
				}
				verifyUnit("CHAI", "chai", 1)
				verifyUnit("THÙNG", "thùng", 1)
				verifyUnit("LỐC", "lốc", 1)
				verifyUnit("PHẦN", "phần", 1)

				// Verify number of product types
				var productTypeSettings models.Settings
				err = tenv.DB.WithContext(testCtx).Where("key = ?", "product_types").First(&productTypeSettings).Error
				Expect(err).NotTo(HaveOccurred())
				var productTypes []string
				err = json.Unmarshal(productTypeSettings.Value, &productTypes)
				Expect(err).NotTo(HaveOccurred())
				Expect(productTypes).To(Equal([]string{"CƠM", "NƯỚC", "ĂN NHẸ"}))
			})
		},
			Entry("CSV", files[0]),
			Entry("Excel", files[1]),
		)
	})

	Describe("Export Products to CSV", func() {
		var units []models.Unit
		var suppliers []models.Supplier
		var products []models.Product

		BeforeEach(func() {
			// Create units
			unit1 := fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				UnitType:         uuid.New().String(),
				Name:             fmt.Sprintf("Thùng %s", uuid.New().String()),
				ConversionFactor: 1,
			})
			unit2 := fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				UnitType:         uuid.New().String(),
				Name:             fmt.Sprintf("Lốc %s", uuid.New().String()),
				ConversionFactor: 1,
			})
			units = []models.Unit{*unit1, *unit2}

			// Create suppliers
			supplier1 := fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name:         fmt.Sprintf("Supplier A %s", uuid.New().String()),
				ContactEmail: fmt.Sprintf("suppliera@example.com-%s", uuid.New().String()),
				ContactPhone: fmt.Sprintf("123-456-7890-%s", uuid.New().String()),
				Address:      fmt.Sprintf("123 Main St %s", uuid.New().String()),
				Status:       "active",
			})
			supplier2 := fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name:         fmt.Sprintf("Supplier B %s", uuid.New().String()),
				ContactEmail: fmt.Sprintf("supplierb@example.com-%s", uuid.New().String()),
				ContactPhone: fmt.Sprintf("098-765-4321-%s", uuid.New().String()),
				Address:      fmt.Sprintf("456 Oak Ave %s", uuid.New().String()),
				Status:       "active",
			})
			supplier3 := fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name:         fmt.Sprintf("Supplier C %s", uuid.New().String()),
				ContactEmail: fmt.Sprintf("supplierc@example.com-%s", uuid.New().String()),
				ContactPhone: fmt.Sprintf("098-765-4322-%s", uuid.New().String()),
				Address:      fmt.Sprintf("456 Oak Ave %s", uuid.New().String()),
				Status:       "active",
			})
			suppliers = []models.Supplier{*supplier1, *supplier2, *supplier3}

			// Create products
			product1 := fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Product 1 %s", uuid.New().String()),
				Description: fmt.Sprintf("Description 1 %s", uuid.New().String()),
				ProductType: "Type A",
				UnitID:      units[0].ID,
				Status:      "active",
				Suppliers:   []*models.Supplier{supplier1, supplier2},
			})
			product2 := fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Product 2 %s", uuid.New().String()),
				Description: fmt.Sprintf("Description 2 %s", uuid.New().String()),
				ProductType: "Type A",
				UnitID:      units[1].ID,
				Status:      "active",
				Suppliers:   []*models.Supplier{supplier2},
			})
			product3 := fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Product 3 %s", uuid.New().String()),
				Description: fmt.Sprintf("Description 3 %s", uuid.New().String()),
				ProductType: "Type B",
				UnitID:      units[0].ID,
				Status:      "inactive",
				Suppliers:   []*models.Supplier{},
			})
			product4 := fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Product 4 %s", uuid.New().String()),
				Description: fmt.Sprintf("Description 4 %s", uuid.New().String()),
				ProductType: "Type B",
				UnitID:      units[1].ID,
				Status:      "active",
				Suppliers:   []*models.Supplier{supplier3},
			})
			products = []models.Product{*product1, *product2, *product3, *product4}
		})

		AfterEach(func() {
			ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
			if len(products) > 0 {
				productIDs := make([]uint, len(products))
				for i, p := range products {
					productIDs[i] = p.ID
				}
				tenv.DB.WithContext(ctx).Where("id IN ?", productIDs).Delete(&models.Product{})
			}
			if len(suppliers) > 0 {
				supplierIDs := make([]uint, len(suppliers))
				for i, s := range suppliers {
					supplierIDs[i] = s.ID
				}
				tenv.DB.WithContext(ctx).Where("id IN ?", supplierIDs).Delete(&models.Supplier{})
			}
			if len(units) > 0 {
				unitIDs := make([]uint, len(units))
				for i, u := range units {
					unitIDs[i] = u.ID
				}
				tenv.DB.WithContext(ctx).Where("id IN ?", unitIDs).Delete(&models.Unit{})
			}
		})

		It("should export all products to CSV", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			resp, err := client.MakeRequest("GET", "/api/v1/products/export-csv", nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(200))
			Expect(resp.Header.Get("Content-Type")).To(Equal("text/csv; charset=utf-8"))
			Expect(resp.Header.Get("Content-Disposition")).To(ContainSubstring("attachment; filename=products_export.csv"))

			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			records := parseCSVExport(body)

			expectedHeader := []string{"Name", "Description", "ProductType", "Unit", "Suppliers", "ContactEmail", "ContactPhone", "Address"}
			Expect(records[0]).To(Equal(expectedHeader))
			Expect(len(records)).To(BeNumerically(">=", 6), "Should have at least 1 header + 5 data rows")

			productRows := buildProductRowsMap(records)
			supplierInfo := make(map[string]*models.Supplier)
			for i := range suppliers {
				supplierInfo[suppliers[i].Name] = &suppliers[i]
			}

			// Verify Product 1 (2 suppliers = 2 rows)
			p1Rows := productRows[products[0].Name]
			Expect(p1Rows).To(HaveLen(2), "%s should have 2 rows", products[0].Name)
			for _, row := range p1Rows {
				Expect(row[:3]).To(Equal([]string{products[0].Name, products[0].Description, products[0].ProductType}))
				Expect(row[3]).To(Equal(units[0].Name))
				Expect(row[4]).To(BeElementOf(suppliers[0].Name, suppliers[1].Name))

				info, ok := supplierInfo[row[4]]
				Expect(ok).To(BeTrue(), "unexpected supplier for Product 1 row")
				Expect(row[5]).To(Equal(info.ContactEmail))
				Expect(row[6]).To(Equal(info.ContactPhone))
				Expect(row[7]).To(Equal(info.Address))
			}

			// Verify Product 2
			p2Rows := productRows[products[1].Name]
			Expect(p2Rows).To(HaveLen(1))
			Expect(p2Rows[0]).To(Equal([]string{
				products[1].Name,
				products[1].Description,
				products[1].ProductType,
				units[1].Name,
				suppliers[1].Name,
				suppliers[1].ContactEmail,
				suppliers[1].ContactPhone,
				suppliers[1].Address,
			}))

			// Verify Product 3 (no suppliers)
			p3Rows := productRows[products[2].Name]
			Expect(p3Rows).To(HaveLen(1))
			Expect(p3Rows[0]).To(Equal([]string{
				products[2].Name,
				products[2].Description,
				products[2].ProductType,
				units[0].Name,
				"",
				"",
				"",
				"",
			}))

			// Verify Product 4
			p4Rows := productRows[products[3].Name]
			Expect(p4Rows).To(HaveLen(1))
			Expect(p4Rows[0]).To(Equal([]string{
				products[3].Name,
				products[3].Description,
				products[3].ProductType,
				units[1].Name,
				suppliers[2].Name,
				suppliers[2].ContactEmail,
				suppliers[2].ContactPhone,
				suppliers[2].Address,
			}))
		})

		testCases := []struct {
			name             string
			params           func() map[string]string
			expectedProducts func() map[string]bool
		}{
			{
				name: "should export products filtered by status",
				params: func() map[string]string {
					return map[string]string{"status": "active"}
				},
				expectedProducts: func() map[string]bool {
					return map[string]bool{
						products[0].Name: true,
						products[1].Name: true,
						products[2].Name: false,
						products[3].Name: true,
					}
				},
			},
			{
				name: "should export products filtered by product_type",
				params: func() map[string]string {
					return map[string]string{"product_type": "Type A"}
				},
				expectedProducts: func() map[string]bool {
					return map[string]bool{
						products[0].Name: true,
						products[1].Name: true,
						products[2].Name: false,
						products[3].Name: false,
					}
				},
			},
			{
				name: "should export products filtered by supplier_id",
				params: func() map[string]string {
					return map[string]string{"supplier_id": fmt.Sprintf("%d", suppliers[1].ID)}
				},
				expectedProducts: func() map[string]bool {
					return map[string]bool{
						products[0].Name: true,
						products[1].Name: true,
						products[2].Name: false,
						products[3].Name: false,
					}
				},
			},
			{
				name: "should export products with combined filters",
				params: func() map[string]string {
					return map[string]string{"status": "active", "product_type": "Type A"}
				},
				expectedProducts: func() map[string]bool {
					return map[string]bool{
						products[0].Name: true,
						products[1].Name: true,
						products[2].Name: false,
						products[3].Name: false,
					}
				},
			},
		}

		for _, tc := range testCases {
			tc := tc // capture loop variable
			It(tc.name, func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				resp, err := client.MakeRequest("GET", "/api/v1/products/export-csv", nil, testutil.WithAuth(), testutil.WithParams(tc.params()))
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(200))

				body, err := io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				records := parseCSVExport(body)
				verifyExportFilter(records, tc.expectedProducts())
			})
		}
	})

	Describe("Export Products to Excel", func() {
		var units []models.Unit
		var suppliers []models.Supplier
		var products []models.Product

		BeforeEach(func() {
			// Create units
			unit1 := fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				UnitType:         uuid.New().String(),
				Name:             fmt.Sprintf("Thùng %s", uuid.New().String()),
				ConversionFactor: 1,
			})
			unit2 := fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				UnitType:         uuid.New().String(),
				Name:             fmt.Sprintf("Lốc %s", uuid.New().String()),
				ConversionFactor: 1,
			})
			units = []models.Unit{*unit1, *unit2}

			// Create suppliers
			supplier1 := fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name:         fmt.Sprintf("Supplier A %s", uuid.New().String()),
				ContactEmail: fmt.Sprintf("suppliera@example.com-%s", uuid.New().String()),
				ContactPhone: fmt.Sprintf("123-456-7890-%s", uuid.New().String()),
				Address:      fmt.Sprintf("123 Main St %s", uuid.New().String()),
				Status:       "active",
			})
			supplier2 := fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name:         fmt.Sprintf("Supplier B %s", uuid.New().String()),
				ContactEmail: fmt.Sprintf("supplierb@example.com-%s", uuid.New().String()),
				ContactPhone: fmt.Sprintf("098-765-4321-%s", uuid.New().String()),
				Address:      fmt.Sprintf("456 Oak Ave %s", uuid.New().String()),
				Status:       "active",
			})
			supplier3 := fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name:         fmt.Sprintf("Supplier C %s", uuid.New().String()),
				ContactEmail: fmt.Sprintf("supplierc@example.com-%s", uuid.New().String()),
				ContactPhone: fmt.Sprintf("098-765-4322-%s", uuid.New().String()),
				Address:      fmt.Sprintf("456 Oak Ave %s", uuid.New().String()),
				Status:       "active",
			})
			suppliers = []models.Supplier{*supplier1, *supplier2, *supplier3}

			// Create products
			product1 := fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Product 1 %s", uuid.New().String()),
				Description: fmt.Sprintf("Description 1 %s", uuid.New().String()),
				ProductType: "Type A",
				UnitID:      units[0].ID,
				Status:      "active",
				Suppliers:   []*models.Supplier{supplier1, supplier2},
			})
			product2 := fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Product 2 %s", uuid.New().String()),
				Description: fmt.Sprintf("Description 2 %s", uuid.New().String()),
				ProductType: "Type A",
				UnitID:      units[1].ID,
				Status:      "active",
				Suppliers:   []*models.Supplier{supplier2},
			})
			product3 := fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Product 3 %s", uuid.New().String()),
				Description: fmt.Sprintf("Description 3 %s", uuid.New().String()),
				ProductType: "Type B",
				UnitID:      units[0].ID,
				Status:      "inactive",
				Suppliers:   []*models.Supplier{},
			})
			product4 := fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Product 4 %s", uuid.New().String()),
				Description: fmt.Sprintf("Description 4 %s", uuid.New().String()),
				ProductType: "Type B",
				UnitID:      units[1].ID,
				Status:      "active",
				Suppliers:   []*models.Supplier{supplier3},
			})
			product5 := fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Product 5 %s", uuid.New().String()),
				Description: fmt.Sprintf("Description 5 %s", uuid.New().String()),
				ProductType: "Type B",
				UnitID:      units[1].ID,
				Status:      "active",
				Suppliers:   []*models.Supplier{supplier3},
			})
			products = []models.Product{*product1, *product2, *product3, *product4, *product5}
		})

		AfterEach(func() {
			ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
			if len(products) > 0 {
				productIDs := make([]uint, len(products))
				for i, p := range products {
					productIDs[i] = p.ID
				}
				tenv.DB.WithContext(ctx).Where("id IN ?", productIDs).Delete(&models.Product{})
			}
			if len(suppliers) > 0 {
				supplierIDs := make([]uint, len(suppliers))
				for i, s := range suppliers {
					supplierIDs[i] = s.ID
				}
				tenv.DB.WithContext(ctx).Where("id IN ?", supplierIDs).Delete(&models.Supplier{})
			}
			if len(units) > 0 {
				unitIDs := make([]uint, len(units))
				for i, u := range units {
					unitIDs[i] = u.ID
				}
				tenv.DB.WithContext(ctx).Where("id IN ?", unitIDs).Delete(&models.Unit{})
			}
		})

		It("should export all products to Excel", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			resp, err := client.MakeRequest("GET", "/api/v1/products/export-excel", nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(200))
			Expect(resp.Header.Get("Content-Type")).To(Equal("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"))
			Expect(resp.Header.Get("Content-Disposition")).To(ContainSubstring("attachment; filename=products_export.xlsx"))

			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			rows := parseExcelExport(body)

			expectedHeader := []string{"Name", "Description", "ProductType", "Unit", "Suppliers", "ContactEmail", "ContactPhone", "Address"}
			Expect(rows[0]).To(Equal(expectedHeader))
			Expect(len(rows)).To(BeNumerically(">=", 6), "Should have at least 1 header + 5 data rows")

			productRows := buildProductRowsMap(rows)
			supplierInfo := make(map[string]*models.Supplier)
			for i := range suppliers {
				supplierInfo[suppliers[i].Name] = &suppliers[i]
			}

			// Verify Product 1 (2 suppliers = 2 rows)
			p1Rows := productRows[products[0].Name]
			Expect(p1Rows).To(HaveLen(2), "%s should have 2 rows", products[0].Name)
			for _, row := range p1Rows {
				Expect(row[:3]).To(Equal([]string{products[0].Name, products[0].Description, products[0].ProductType}))
				Expect(row[3]).To(Equal(units[0].Name))
				Expect(row[4]).To(BeElementOf(suppliers[0].Name, suppliers[1].Name))

				info, ok := supplierInfo[row[4]]
				Expect(ok).To(BeTrue(), "unexpected supplier for Product 1 row")
				Expect(row[5]).To(Equal(info.ContactEmail))
				Expect(row[6]).To(Equal(info.ContactPhone))
				Expect(row[7]).To(Equal(info.Address))
			}

			// Verify Product 2
			p2Rows := productRows[products[1].Name]
			Expect(p2Rows).To(HaveLen(1))
			Expect(p2Rows[0]).To(Equal([]string{
				products[1].Name,
				products[1].Description,
				products[1].ProductType,
				units[1].Name,
				suppliers[1].Name,
				suppliers[1].ContactEmail,
				suppliers[1].ContactPhone,
				suppliers[1].Address,
			}))

			// Verify Product 3 (no suppliers)
			p3Rows := productRows[products[2].Name]
			Expect(p3Rows).To(HaveLen(1))
			Expect(p3Rows[0]).To(Equal([]string{
				products[2].Name,
				products[2].Description,
				products[2].ProductType,
				units[0].Name,
				"",
				"",
				"",
				"",
			}))

			// Verify Product 4
			p4Rows := productRows[products[3].Name]
			Expect(p4Rows).To(HaveLen(1))
			Expect(p4Rows[0]).To(Equal([]string{
				products[3].Name,
				products[3].Description,
				products[3].ProductType,
				units[1].Name,
				suppliers[2].Name,
				suppliers[2].ContactEmail,
				suppliers[2].ContactPhone,
				suppliers[2].Address,
			}))
		})

		testCases := []struct {
			name             string
			params           func() map[string]string
			expectedProducts func() map[string]bool
		}{
			{
				name: "should export products filtered by status",
				params: func() map[string]string {
					return map[string]string{"status": "active"}
				},
				expectedProducts: func() map[string]bool {
					return map[string]bool{
						products[0].Name: true,
						products[1].Name: true,
						products[2].Name: false,
						products[3].Name: true,
						products[4].Name: true,
					}
				},
			},
			{
				name: "should export products filtered by product_type",
				params: func() map[string]string {
					return map[string]string{"product_type": "Type A"}
				},
				expectedProducts: func() map[string]bool {
					return map[string]bool{
						products[0].Name: true,
						products[1].Name: true,
						products[2].Name: false,
						products[3].Name: false,
						products[4].Name: false,
					}
				},
			},
			{
				name: "should export products filtered by supplier_id",
				params: func() map[string]string {
					return map[string]string{"supplier_id": fmt.Sprintf("%d", suppliers[1].ID)}
				},
				expectedProducts: func() map[string]bool {
					return map[string]bool{
						products[0].Name: true,
						products[1].Name: true,
						products[2].Name: false,
						products[3].Name: false,
						products[4].Name: false,
					}
				},
			},
			{
				name: "should export products with combined filters",
				params: func() map[string]string {
					return map[string]string{"status": "active", "product_type": "Type A"}
				},
				expectedProducts: func() map[string]bool {
					return map[string]bool{
						products[0].Name: true,
						products[1].Name: true,
						products[2].Name: false,
						products[3].Name: false,
						products[4].Name: false,
					}
				},
			},
		}

		for _, tc := range testCases {
			tc := tc // capture loop variable
			It(tc.name, func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				resp, err := client.MakeRequest("GET", "/api/v1/products/export-excel", nil, testutil.WithAuth(), testutil.WithParams(tc.params()))
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(200))

				body, err := io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				rows := parseExcelExport(body)
				verifyExportFilter(rows, tc.expectedProducts())
			})
		}
	})
})

// Helper functions for export tests

// parseCSVExport parses CSV export response
func parseCSVExport(body []byte) [][]string {
	reader := csv.NewReader(strings.NewReader(string(body)))
	reader.Comma = ';'
	records, err := reader.ReadAll()
	Expect(err).NotTo(HaveOccurred())
	return records
}

// parseExcelExport parses Excel export response
func parseExcelExport(body []byte) [][]string {
	f, err := excelize.OpenReader(bytes.NewReader(body))
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()
	sheetName := f.GetSheetList()[0]
	rows, err := f.GetRows(sheetName)
	Expect(err).NotTo(HaveOccurred())
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
func verifyExportFilter(rows [][]string, expectedProducts map[string]bool) {
	productNames := make(map[string]bool)
	for i := 1; i < len(rows); i++ {
		if len(rows[i]) >= 1 && rows[i][0] != "" {
			productNames[rows[i][0]] = true
		}
	}
	for name, shouldExist := range expectedProducts {
		Expect(productNames[name]).To(Equal(shouldExist), "Product %s should%s be in export", name, map[bool]string{true: "", false: " not"}[shouldExist])
	}
}
