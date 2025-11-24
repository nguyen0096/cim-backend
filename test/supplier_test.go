package apptest

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil"
	"cim-backend/pkg/testutil/fixture"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

var _ = Describe("Supplier API", func() {
	Describe("Create and Get Supplier", func() {
		supplierData := map[string]interface{}{
			"name":          "Test Supplier",
			"contact_email": "supplier@example.com",
			"contact_phone": "+1234567890",
			"address":       "123 Test St",
		}

		It("should create and get supplier with admin role", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			// Create supplier
			resp, err := client.MakeRequest("POST", "/api/v1/suppliers", supplierData, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(201))

			supplierResp := testutil.ParseResponse(resp)
			Expect(supplierResp["id"]).NotTo(BeNil())
			Expect(supplierResp["name"]).To(Equal("Test Supplier"))

			supplierID := pkg.ToString(supplierResp["id"])

			// Get the supplier
			resp, err = client.MakeRequest("GET", "/api/v1/suppliers/"+supplierID, nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			getSupplierResp := testutil.ParseResponse(resp)
			Expect(getSupplierResp["name"]).To(Equal("Test Supplier"))
		})

		It("should create and get supplier with accountant role", func() {
			client := testutil.NewClient(tenv, models.RoleAccountant)

			// Create supplier
			resp, err := client.MakeRequest("POST", "/api/v1/suppliers", supplierData, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(201))

			supplierResp := testutil.ParseResponse(resp)
			Expect(supplierResp["id"]).NotTo(BeNil())
			Expect(supplierResp["name"]).To(Equal("Test Supplier"))

			supplierID := pkg.ToString(supplierResp["id"])

			// Get the supplier
			resp, err = client.MakeRequest("GET", "/api/v1/suppliers/"+supplierID, nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			getSupplierResp := testutil.ParseResponse(resp)
			Expect(getSupplierResp["name"]).To(Equal("Test Supplier"))
		})

		It("should not create supplier with staff role", func() {
			client := testutil.NewClient(tenv, models.RoleStaff)

			resp, err := client.MakeRequest("POST", "/api/v1/suppliers", supplierData, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(403))

			errorResp := testutil.ParseResponse(resp)
			Expect(errorResp["error"]).To(Equal("Access denied: " + string(models.RoleStaff) + " role cannot create suppliers"))
		})

		It("should not create supplier with bot form role", func() {
			client := testutil.NewClient(tenv, models.RoleBotForm)

			resp, err := client.MakeRequest("POST", "/api/v1/suppliers", supplierData, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(403))

			errorResp := testutil.ParseResponse(resp)
			Expect(errorResp["error"]).To(Equal("Access denied: " + string(models.RoleBotForm) + " role cannot create suppliers"))
		})

		It("should create supplier with only name field", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			minimalSupplierData := map[string]interface{}{
				"name": "Minimal Supplier",
			}

			resp, err := client.MakeRequest("POST", "/api/v1/suppliers", minimalSupplierData, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(201))

			supplierResp := testutil.ParseResponse(resp)
			Expect(supplierResp["id"]).NotTo(BeNil())
			Expect(supplierResp["name"]).To(Equal("Minimal Supplier"))
		})
	})

	Describe("Update Supplier", func() {
		var testSupplier *models.Supplier
		var supplierID uint

		updatedSupplierData := map[string]interface{}{
			"name":          "Test Supplier Edited",
			"contact_email": "supplier_edited@example.com",
			"contact_phone": "+1234567891",
			"address":       "123 Test St Edited",
		}

		BeforeEach(func() {
			testSupplier = fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name:         "Test Supplier",
				ContactEmail: "supplier@example.com",
				ContactPhone: "+1234567890",
				Address:      "123 Test St",
			})
			supplierID = testSupplier.ID
		})

		Context("when user has authorized role", func() {
			It("should update supplier with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				urlPath := fmt.Sprintf("/api/v1/suppliers/%d", supplierID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedSupplierData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedSupplierResp := testutil.ParseResponse(resp)
				Expect(updatedSupplierResp["name"]).To(Equal("Test Supplier Edited"))
				Expect(updatedSupplierResp["contact_email"]).To(Equal("supplier_edited@example.com"))
				Expect(updatedSupplierResp["contact_phone"]).To(Equal("+1234567891"))
				Expect(updatedSupplierResp["address"]).To(Equal("123 Test St Edited"))
			})

			It("should update supplier with accountant role", func() {
				client := testutil.NewClient(tenv, models.RoleAccountant)

				urlPath := fmt.Sprintf("/api/v1/suppliers/%d", supplierID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedSupplierData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedSupplierResp := testutil.ParseResponse(resp)
				Expect(updatedSupplierResp["name"]).To(Equal("Test Supplier Edited"))
				Expect(updatedSupplierResp["contact_email"]).To(Equal("supplier_edited@example.com"))
				Expect(updatedSupplierResp["contact_phone"]).To(Equal("+1234567891"))
				Expect(updatedSupplierResp["address"]).To(Equal("123 Test St Edited"))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not update supplier with staff role", func() {
				client := testutil.NewClient(tenv, models.RoleStaff)

				urlPath := fmt.Sprintf("/api/v1/suppliers/%d", supplierID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedSupplierData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal("Access denied: " + string(models.RoleStaff) + " role cannot update suppliers"))
			})

			It("should not update supplier with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				urlPath := fmt.Sprintf("/api/v1/suppliers/%d", supplierID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedSupplierData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal("Access denied: " + string(models.RoleBotForm) + " role cannot update suppliers"))
			})
		})
	})

	Describe("Delete Supplier", func() {
		var testSupplier *models.Supplier
		var supplierID uint

		BeforeEach(func() {
			testSupplier = fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name:         "Test Supplier",
				ContactEmail: "supplier@example.com",
				ContactPhone: "+1234567890",
				Address:      "123 Test St",
			})
			supplierID = testSupplier.ID
		})

		Context("when user has admin role", func() {
			It("should delete supplier", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				urlPath := fmt.Sprintf("/api/v1/suppliers/%d", supplierID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Verify supplier is deleted
				err = tenv.DB.WithContext(ctx).First(&models.Supplier{}, "id = ?", supplierID).Error
				Expect(err).To(Equal(gorm.ErrRecordNotFound))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not delete supplier with accountant role", func() {
				client := testutil.NewClient(tenv, models.RoleAccountant)

				urlPath := fmt.Sprintf("/api/v1/suppliers/%d", supplierID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal("Access denied: " + string(models.RoleAccountant) + " role cannot delete suppliers"))
			})

			It("should not delete supplier with staff role", func() {
				client := testutil.NewClient(tenv, models.RoleStaff)

				urlPath := fmt.Sprintf("/api/v1/suppliers/%d", supplierID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal("Access denied: " + string(models.RoleStaff) + " role cannot delete suppliers"))
			})

			It("should not delete supplier with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				urlPath := fmt.Sprintf("/api/v1/suppliers/%d", supplierID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal("Access denied: " + string(models.RoleBotForm) + " role cannot delete suppliers"))
			})
		})
	})

	Describe("Update Supplier Status", func() {
		var testSupplier *models.Supplier
		var supplierID uint

		BeforeEach(func() {
			testSupplier = fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name:         "Test Supplier",
				ContactEmail: "supplier@example.com",
				ContactPhone: "+1234567890",
				Address:      "123 Test St",
				Status:       "active",
			})
			supplierID = testSupplier.ID
		})

		Context("when user has authorized role", func() {
			It("should update supplier status with admin role", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				urlPath := fmt.Sprintf("/api/v1/suppliers/%d/status", supplierID)
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "inactive"}, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedSupplierResp := testutil.ParseResponse(resp)
				Expect(updatedSupplierResp["status"]).To(Equal("inactive"))

				// Verify in database
				var updatedSupplier models.Supplier
				err = tenv.DB.WithContext(ctx).First(&updatedSupplier, "id = ?", supplierID).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedSupplier.Status).To(Equal("inactive"))
			})

			It("should update supplier status with accountant role", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAccountant)

				urlPath := fmt.Sprintf("/api/v1/suppliers/%d/status", supplierID)
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "inactive"}, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedSupplierResp := testutil.ParseResponse(resp)
				Expect(updatedSupplierResp["status"]).To(Equal("inactive"))

				// Verify in database
				var updatedSupplier models.Supplier
				err = tenv.DB.WithContext(ctx).First(&updatedSupplier, "id = ?", supplierID).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedSupplier.Status).To(Equal("inactive"))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not update supplier status with staff role", func() {
				client := testutil.NewClient(tenv, models.RoleStaff)

				urlPath := fmt.Sprintf("/api/v1/suppliers/%d/status", supplierID)
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "inactive"}, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal("Access denied: " + string(models.RoleStaff) + " role cannot update suppliers"))
			})

			It("should not update supplier status with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				urlPath := fmt.Sprintf("/api/v1/suppliers/%d/status", supplierID)
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "inactive"}, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal("Access denied: " + string(models.RoleBotForm) + " role cannot update suppliers"))
			})
		})
	})
})
