package apptest

import (
	"cim-backend/internal/models"
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

var _ = Describe("Product API", func() {
	Describe("Create and Get Product", func() {
		var testUnit *models.Unit

		BeforeEach(func() {
			testUnit = tenv.WithUnit(models.Unit{
				UnitType:         uuid.New().String(),
				Name:             fmt.Sprintf("Test Unit %s", uuid.New().String()),
				ConversionFactor: 1,
			})
		})

		Context("when user has authorized role", func() {
			It("should create and get product with admin role", func() {
				client := NewClient(tenv, models.RoleAdmin)
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

				resp, err := client.MakeRequest("POST", "/api/v1/products", productData, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				productResp := ParseResponse(resp)
				Expect(productResp["id"]).NotTo(BeNil())
				Expect(productResp["name"]).To(Equal(productName))
			})

			It("should create and get product with accountant role", func() {
				client := NewClient(tenv, models.RoleAccountant)
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

				resp, err := client.MakeRequest("POST", "/api/v1/products", productData, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				productResp := ParseResponse(resp)
				Expect(productResp["id"]).NotTo(BeNil())
				Expect(productResp["name"]).To(Equal(productName))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not create product with staff role", func() {
				client := NewClient(tenv, models.RoleStaff)

				productData := map[string]interface{}{
					"name":         "Test Product",
					"description":  "Test Description",
					"product_type": "test",
					"unit_id":      testUnit.ID,
					"supplier_ids": []uint{},
					"status":       "active",
				}

				resp, err := client.MakeRequest("POST", "/api/v1/products", productData, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot create products", models.RoleStaff)))
			})

			It("should not create product with bot form role", func() {
				client := NewClient(tenv, models.RoleBotForm)

				productData := map[string]interface{}{
					"name":         "Test Product",
					"description":  "Test Description",
					"product_type": "test",
					"unit_id":      testUnit.ID,
					"supplier_ids": []uint{},
					"status":       "active",
				}

				resp, err := client.MakeRequest("POST", "/api/v1/products", productData, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot create products", models.RoleBotForm)))
			})
		})
	})

	Describe("Update Product", func() {
		var testUnit *models.Unit
		var testSuppliers []models.Supplier
		var testProduct *models.Product

		BeforeEach(func() {
			testUnit = tenv.WithUnit(models.Unit{
				Name:             fmt.Sprintf("Test Unit %s", uuid.New().String()),
				ConversionFactor: 1,
			})

			supplier1 := tenv.WithSupplier(models.Supplier{
				Name: fmt.Sprintf("Test Supplier 1 %s", uuid.New().String()),
			})
			supplier2 := tenv.WithSupplier(models.Supplier{
				Name: fmt.Sprintf("Test Supplier 2 %s", uuid.New().String()),
			})
			supplier3 := tenv.WithSupplier(models.Supplier{
				Name: fmt.Sprintf("Test Supplier 3 %s", uuid.New().String()),
			})
			supplier4 := tenv.WithSupplier(models.Supplier{
				Name: fmt.Sprintf("Test Supplier 4 %s", uuid.New().String()),
			})
			testSuppliers = []models.Supplier{*supplier1, *supplier2, *supplier3, *supplier4}

			testProduct = tenv.WithProduct(models.Product{
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
				client := NewClient(tenv, models.RoleAdmin)
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
				resp, err := client.MakeRequest("PUT", urlPath, updatedProductData, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedProductResp := ParseResponse(resp)
				Expect(updatedProductResp["name"]).To(Equal(newProductName))
				Expect(updatedProductResp["description"]).To(Equal(newProductDescription))
				Expect(updatedProductResp["product_type"]).To(Equal("test_edited"))
				Expect(updatedProductResp["status"]).To(Equal("inactive"))

				suppliers := updatedProductResp["suppliers"].([]interface{})
				Expect(suppliers).To(HaveLen(3))
			})

			It("should update product with accountant role", func() {
				client := NewClient(tenv, models.RoleAccountant)
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
				resp, err := client.MakeRequest("PUT", urlPath, updatedProductData, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedProductResp := ParseResponse(resp)
				Expect(updatedProductResp["name"]).To(Equal(newProductName))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not update product with staff role", func() {
				client := NewClient(tenv, models.RoleStaff)

				updatedProductData := map[string]interface{}{
					"name":         "Updated Name",
					"description":  "Updated Description",
					"product_type": "test_edited",
					"supplier_ids": []uint{},
					"unit_id":      testUnit.ID,
					"status":       "inactive",
				}

				urlPath := fmt.Sprintf("/api/v1/products/%d", testProduct.ID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedProductData, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update products", models.RoleStaff)))
			})

			It("should not update product with bot form role", func() {
				client := NewClient(tenv, models.RoleBotForm)

				updatedProductData := map[string]interface{}{
					"name":         "Updated Name",
					"description":  "Updated Description",
					"product_type": "test_edited",
					"supplier_ids": []uint{},
					"unit_id":      testUnit.ID,
					"status":       "inactive",
				}

				urlPath := fmt.Sprintf("/api/v1/products/%d", testProduct.ID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedProductData, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update products", models.RoleBotForm)))
			})
		})
	})

	Describe("Update Product Status", func() {
		var testProduct *models.Product

		BeforeEach(func() {
			testProduct = tenv.WithProduct(models.Product{
				Name:        "Test Product",
				Description: "Test Description",
				UnitID:      1,
				Status:      "active",
			})
		})

		Context("when user has authorized role", func() {
			It("should update product status with admin role", func(ctx SpecContext) {
				client := NewClient(tenv, models.RoleAdmin)

				urlPath := fmt.Sprintf("/api/v1/products/%d/status", testProduct.ID)
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "inactive"}, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedProductResp := ParseResponse(resp)
				Expect(updatedProductResp["status"]).To(Equal("inactive"))

				// Verify in database
				var updatedProduct models.Product
				err = tenv.DB.WithContext(ctx).First(&updatedProduct, "id = ?", testProduct.ID).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedProduct.Status).To(Equal("inactive"))
			})

			It("should update product status with accountant role", func(ctx SpecContext) {
				client := NewClient(tenv, models.RoleAccountant)

				urlPath := fmt.Sprintf("/api/v1/products/%d/status", testProduct.ID)
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "inactive"}, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedProductResp := ParseResponse(resp)
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
				client := NewClient(tenv, models.RoleStaff)

				urlPath := fmt.Sprintf("/api/v1/products/%d/status", testProduct.ID)
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "inactive"}, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update products", models.RoleStaff)))
			})

			It("should not update product status with bot form role", func() {
				client := NewClient(tenv, models.RoleBotForm)

				urlPath := fmt.Sprintf("/api/v1/products/%d/status", testProduct.ID)
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "inactive"}, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update products", models.RoleBotForm)))
			})
		})
	})

	Describe("Delete Product", func() {
		var testUnit *models.Unit
		var testSupplier *models.Supplier
		var testProduct *models.Product

		BeforeEach(func() {
			testUnit = tenv.WithUnit(models.Unit{
				UnitType:         uuid.New().String(),
				Name:             fmt.Sprintf("Test Unit %s", uuid.New().String()),
				ConversionFactor: 1,
			})

			testSupplier = tenv.WithSupplier(models.Supplier{
				Name: fmt.Sprintf("Test Supplier 1 %s", uuid.New().String()),
			})

			testProduct = tenv.WithProduct(models.Product{
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
				client := NewClient(tenv, models.RoleAdmin)

				urlPath := fmt.Sprintf("/api/v1/products/%d", testProduct.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Verify product is deleted
				err = tenv.DB.WithContext(ctx).First(&models.Product{}, "id = ?", testProduct.ID).Error
				Expect(err).To(Equal(gorm.ErrRecordNotFound))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not delete product with accountant role", func() {
				client := NewClient(tenv, models.RoleAccountant)

				urlPath := fmt.Sprintf("/api/v1/products/%d", testProduct.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot delete products", models.RoleAccountant)))
			})

			It("should not delete product with staff role", func() {
				client := NewClient(tenv, models.RoleStaff)

				urlPath := fmt.Sprintf("/api/v1/products/%d", testProduct.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot delete products", models.RoleStaff)))
			})

			It("should not delete product with bot form role", func() {
				client := NewClient(tenv, models.RoleBotForm)

				urlPath := fmt.Sprintf("/api/v1/products/%d", testProduct.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot delete products", models.RoleBotForm)))
			})
		})
	})
})

