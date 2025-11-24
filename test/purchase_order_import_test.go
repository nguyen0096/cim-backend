package apptest

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"cim-backend/internal/models"
	"cim-backend/pkg/testutil"
	"cim-backend/pkg/testutil/fixture"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Purchase Order Import API", func() {
	Describe("Upload Purchase Order File", func() {
		var testSupplier *models.Supplier
		var testUnit *models.Unit
		var testProduct *models.Product
		var testInventory *models.Inventory
		var excelFilePath string

		BeforeEach(func() {
			testSupplier = fixture.WithSupplier(tenv.ContextfulDB(), fixture.ValidSupplier())
			testUnit = fixture.WithUnit(tenv.ContextfulDB(), fixture.ValidBaseUnit())
			testProduct = fixture.WithProduct(tenv.ContextfulDB(), fixture.ValidProduct(testUnit.ID))
			testInventory = fixture.WithInventory(tenv.ContextfulDB(), fixture.ValidInventory())
			excelFilePath = fixture.CreatePurchaseOrderExcelFile(testSupplier, testProduct, testUnit, testInventory)
		})

		Context("when user has authorized role", func() {
			It("should upload file with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				// Create multipart form with file
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)

				// Open the Excel file
				file, err := os.Open(excelFilePath)
				Expect(err).NotTo(HaveOccurred())
				defer file.Close()

				// Create form file
				part, err := writer.CreateFormFile("file", filepath.Base(excelFilePath))
				Expect(err).NotTo(HaveOccurred())

				// Copy file content
				_, err = io.Copy(part, file)
				Expect(err).NotTo(HaveOccurred())

				err = writer.Close()
				Expect(err).NotTo(HaveOccurred())

				// Make POST request
				resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders/upload", body, testutil.WithAuth(), testutil.WithContentType(writer.FormDataContentType()))
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Validate response structure
				uploadResp := testutil.ParseResponse(resp)
				Expect(uploadResp["file_uid"]).NotTo(BeEmpty())
				Expect(uploadResp["file_name"]).NotTo(BeEmpty())
				Expect(uploadResp["sheets"]).To(BeAssignableToTypeOf([]interface{}{}))

				sheets := uploadResp["sheets"].([]interface{})
				Expect(sheets).To(HaveLen(1))

				sheet := sheets[0].(map[string]interface{})
				Expect(sheet["sheet_name"]).To(Equal("Sheet1"))
				Expect(sheet["is_valid"]).To(BeTrue())
			})

			It("should upload file with accountant role", func() {
				client := testutil.NewClient(tenv, models.RoleAccountant)

				// Create multipart form with file
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)

				// Open the Excel file
				file, err := os.Open(excelFilePath)
				Expect(err).NotTo(HaveOccurred())
				defer file.Close()

				// Create form file
				part, err := writer.CreateFormFile("file", filepath.Base(excelFilePath))
				Expect(err).NotTo(HaveOccurred())

				// Copy file content
				_, err = io.Copy(part, file)
				Expect(err).NotTo(HaveOccurred())

				err = writer.Close()
				Expect(err).NotTo(HaveOccurred())

				// Make POST request
				resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders/upload", body, testutil.WithAuth(), testutil.WithContentType(writer.FormDataContentType()))
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Validate response structure
				uploadResp := testutil.ParseResponse(resp)
				Expect(uploadResp["file_uid"]).NotTo(BeEmpty())
				Expect(uploadResp["sheets"]).To(BeAssignableToTypeOf([]interface{}{}))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should deny upload with staff role", func() {
				client := testutil.NewClient(tenv, models.RoleStaff)

				// Create multipart form with file
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)

				// Open the Excel file
				file, err := os.Open(excelFilePath)
				Expect(err).NotTo(HaveOccurred())
				defer file.Close()

				// Create form file
				part, err := writer.CreateFormFile("file", filepath.Base(excelFilePath))
				Expect(err).NotTo(HaveOccurred())

				// Copy file content
				_, err = io.Copy(part, file)
				Expect(err).NotTo(HaveOccurred())

				err = writer.Close()
				Expect(err).NotTo(HaveOccurred())

				// Make POST request
				resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders/upload", body, testutil.WithAuth(), testutil.WithContentType(writer.FormDataContentType()))
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(ContainSubstring("Access denied"))
			})

			It("should deny upload with bot_form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				// Create multipart form with file
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)

				// Open the Excel file
				file, err := os.Open(excelFilePath)
				Expect(err).NotTo(HaveOccurred())
				defer file.Close()

				// Create form file
				part, err := writer.CreateFormFile("file", filepath.Base(excelFilePath))
				Expect(err).NotTo(HaveOccurred())

				// Copy file content
				_, err = io.Copy(part, file)
				Expect(err).NotTo(HaveOccurred())

				err = writer.Close()
				Expect(err).NotTo(HaveOccurred())

				// Make POST request
				resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders/upload", body, testutil.WithAuth(), testutil.WithContentType(writer.FormDataContentType()))
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(ContainSubstring("Access denied"))
			})
		})
	})

	Describe("Process Import Purchase Order", func() {
		var testSupplier *models.Supplier
		var testUnit *models.Unit
		var testProduct *models.Product
		var testInventory *models.Inventory
		var excelFilePath string

		BeforeEach(func() {
			testSupplier = fixture.WithSupplier(tenv.ContextfulDB(), fixture.ValidSupplier())
			testUnit = fixture.WithUnit(tenv.ContextfulDB(), fixture.ValidBaseUnit())
			testProduct = fixture.WithProduct(tenv.ContextfulDB(), fixture.ValidProduct(testUnit.ID))
			testInventory = fixture.WithInventory(tenv.ContextfulDB(), fixture.ValidInventory())
			excelFilePath = fixture.CreatePurchaseOrderExcelFile(testSupplier, testProduct, testUnit, testInventory)
		})

		// Helper function to upload file and get file_uid
		uploadFile := func(client *testutil.Client) string {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			file, err := os.Open(excelFilePath)
			Expect(err).NotTo(HaveOccurred())
			defer file.Close()

			part, err := writer.CreateFormFile("file", filepath.Base(excelFilePath))
			Expect(err).NotTo(HaveOccurred())

			_, err = io.Copy(part, file)
			Expect(err).NotTo(HaveOccurred())

			err = writer.Close()
			Expect(err).NotTo(HaveOccurred())

			resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders/upload", body, testutil.WithAuth(), testutil.WithContentType(writer.FormDataContentType()))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			uploadResp := testutil.ParseResponse(resp)
			return uploadResp["file_uid"].(string)
		}

		Context("when user has authorized role", func() {
			It("should process import with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				// Upload file first
				fileUID := uploadFile(client)

				// Process import
				processReq := map[string]interface{}{
					"sheet_name": "Sheet1",
				}

				urlPath := fmt.Sprintf("/api/v1/purchase-orders/upload-files/%s/process", fileUID)
				resp, err := client.MakeRequest("POST", urlPath, processReq, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())

				// Debug: Print response body
				if resp.StatusCode != 201 {
					bodyBytes, _ := io.ReadAll(resp.Body)
					resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
					fmt.Printf("\n[DEBUG] Admin Process Response (Status %d):\n%s\n\n", resp.StatusCode, string(bodyBytes))
				}

				Expect(resp.StatusCode).To(Equal(201))

				// Validate purchase order created
				poResp := testutil.ParseResponse(resp)
				Expect(poResp["id"]).NotTo(BeNil())
				Expect(poResp["order_number"]).NotTo(BeEmpty())
				Expect(poResp["status"]).To(Equal("order_placed"))
				Expect(poResp["inventory_id"]).To(Equal(float64(testInventory.ID)))
			})

			It("should process import with accountant role", func() {
				client := testutil.NewClient(tenv, models.RoleAccountant)

				// Upload file first
				fileUID := uploadFile(client)

				// Process import
				processReq := map[string]interface{}{
					"sheet_name": "Sheet1",
				}

				urlPath := fmt.Sprintf("/api/v1/purchase-orders/upload-files/%s/process", fileUID)
				resp, err := client.MakeRequest("POST", urlPath, processReq, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())

				// Debug: Print response body
				if resp.StatusCode != 201 {
					bodyBytes, _ := io.ReadAll(resp.Body)
					resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
					fmt.Printf("\n[DEBUG] Accountant Process Response (Status %d):\n%s\n\n", resp.StatusCode, string(bodyBytes))
				}

				Expect(resp.StatusCode).To(Equal(201))

				// Validate purchase order created
				poResp := testutil.ParseResponse(resp)
				Expect(poResp["id"]).NotTo(BeNil())
				Expect(poResp["order_number"]).NotTo(BeEmpty())
				Expect(poResp["status"]).To(Equal("order_placed"))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should deny process with staff role", func() {
				// Upload file with authorized user first
				adminClient := testutil.NewClient(tenv, models.RoleAdmin)
				fileUID := uploadFile(adminClient)

				// Try to process with staff user
				staffClient := testutil.NewClient(tenv, models.RoleStaff)
				processReq := map[string]interface{}{
					"sheet_name": "Sheet1",
				}

				urlPath := fmt.Sprintf("/api/v1/purchase-orders/upload-files/%s/process", fileUID)
				resp, err := staffClient.MakeRequest("POST", urlPath, processReq, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(ContainSubstring("Access denied"))
			})

			It("should deny process with bot_form role", func() {
				// Upload file with authorized user first
				adminClient := testutil.NewClient(tenv, models.RoleAdmin)
				fileUID := uploadFile(adminClient)

				// Try to process with bot_form user
				botClient := testutil.NewClient(tenv, models.RoleBotForm)
				processReq := map[string]interface{}{
					"sheet_name": "Sheet1",
				}

				urlPath := fmt.Sprintf("/api/v1/purchase-orders/upload-files/%s/process", fileUID)
				resp, err := botClient.MakeRequest("POST", urlPath, processReq, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(ContainSubstring("Access denied"))
			})
		})
	})
})
