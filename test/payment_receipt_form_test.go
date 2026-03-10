package apptest

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil"
	"cim-backend/pkg/testutil/fixture"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/r3labs/sse/v2"
	"github.com/shopspring/decimal"
)

// Helper functions for SSE testing
func decodeEventPayload(event *sse.Event) map[string]interface{} {
	var payload map[string]interface{}
	err := json.Unmarshal(event.Data, &payload)
	if err != nil {
		return nil
	}
	return payload
}

func waitForEvent(eventsCh chan *sse.Event, timeout time.Duration) *sse.Event {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case event := <-eventsCh:
		return event
	case <-timer.C:
		return nil
	}
}

var _ = Describe("Payment Receipt Form API", func() {
	var testUnit *models.Unit
	var testSupplier *models.Supplier
	var testProduct *models.Product
	var testInventory *models.Inventory
	var testPurchaseOrder *models.PurchaseOrder

	BeforeEach(func() {
		ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")

		// Clean up existing payment receipt forms
		tenv.DB.WithContext(ctx).Exec("DELETE FROM payment_receipt_forms")

		// Create test data
		testUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
			Name:             fmt.Sprintf("SSE Unit %s", uuid.New().String()),
			UnitType:         "general",
			ConversionFactor: 1,
		})

		testSupplier = fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
			Name: fmt.Sprintf("SSE Supplier %s", uuid.New().String()),
		})

		testProduct = fixture.WithProduct(tenv.ContextfulDB(), models.Product{
			Name:   fmt.Sprintf("SSE Product %s", uuid.New().String()),
			UnitID: testUnit.ID,
		})

		testInventory = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
			Name:     fmt.Sprintf("SSE Inventory %s", uuid.New().String()),
			Location: "Automation Floor",
		})

		testPurchaseOrder = fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
			OrderNumber: fmt.Sprintf("PO-%s", uuid.New().String()),
			InventoryID: &testInventory.ID,
			Status:      models.PurchaseOrderStatusOrderPlaced,
			Items: []*models.PurchaseOrderItem{
				{
					ProductID:  &testProduct.ID,
					SupplierID: &testSupplier.ID,
					UnitID:     &testUnit.ID,
					Quantity:   decimal.NewFromInt(1),
					UnitPrice:  125000,
				},
			},
		})
	})

	Describe("Payment Receipt Form Pending Stream", func() {
		It("should stream pending payment receipt form updates", func() {
			adminClient := testutil.NewClient(tenv, models.RoleAdmin)
			botClient := testutil.NewClient(tenv, models.RoleBotForm)

			sseURL := tenv.BaseURL + "/api/v1/payment-receipt-forms/pending"
			sseCtx, sseCancel := context.WithCancel(context.Background())
			DeferCleanup(sseCancel)

			client := sse.NewClient(sseURL)
			client.Headers["Authorization"] = fmt.Sprintf("Bearer %s", *botClient.AuthToken)

			eventsCh := make(chan *sse.Event, 5)
			err := client.SubscribeChanRawWithContext(sseCtx, eventsCh)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				client.Unsubscribe(eventsCh)
			})

			// Should receive initial event
			initialEvent := waitForEvent(eventsCh, 5*time.Second)
			Expect(initialEvent).NotTo(BeNil())
			Expect(string(initialEvent.Event)).To(Equal("pending_form_update"))
			initialPayload := decodeEventPayload(initialEvent)
			Expect(initialPayload).NotTo(BeNil())
			Expect(initialPayload["message"]).To(Equal("No pending payment receipt form found"))

			// Create a payment receipt form
			expectedDate := time.Now().Format("2006-01-02")
			payload := map[string]interface{}{
				"purchase_order_id": testPurchaseOrder.ID,
				"full_name":         "Automation Bot",
				"date":              time.Now(),
				"department":        "Operations",
				"details":           "Generated from SSE component test",
				"total_amount":      125000,
			}
			resp, err := adminClient.MakeRequest("POST", "/api/v1/payment-receipt-forms", payload, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(201))

			formResp := testutil.ParseResponse(resp)
			createdFormID := uint(formResp["id"].(float64))

			// Should receive pending event
			pendingEvent := waitForEvent(eventsCh, 5*time.Second)
			Expect(pendingEvent).NotTo(BeNil())
			Expect(string(pendingEvent.Event)).To(Equal("pending_form_update"))
			pendingPayload := decodeEventPayload(pendingEvent)
			Expect(pendingPayload).NotTo(BeNil())

			sseDate, ok := pendingPayload["date"].(string)
			Expect(ok).To(BeTrue())
			parsedSseDate, err := time.Parse(time.RFC3339, sseDate)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsedSseDate.Format("2006-01-02")).To(Equal(expectedDate))

			statusValue, ok := pendingPayload["status"].(string)
			Expect(ok).To(BeTrue(), "status field missing from SSE event")
			Expect(statusValue).To(Equal(string(models.PaymentReceiptFormStatusPending)))

			sseFormID := uint(pendingPayload["id"].(float64))
			Expect(sseFormID).To(Equal(createdFormID))

			ssePurchaseOrderID := uint(pendingPayload["purchase_order_id"].(float64))
			Expect(ssePurchaseOrderID).To(Equal(testPurchaseOrder.ID))
		})

		It("should get initial pending forms if there are any pending forms", func() {
			ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")

			testPendingForm := models.PaymentReceiptForm{
				FormNumber:      pkg.Ptr(fmt.Sprintf("FORM-%s", uuid.New().String())),
				PurchaseOrderID: testPurchaseOrder.ID,
				FullName:        "Automation Bot",
				Date:            time.Now(),
				Department:      "Operations",
				Details:         "Generated from SSE component test",
				TotalAmount:     125000,
				Status:          models.PaymentReceiptFormStatusPending,
			}
			err := tenv.DB.WithContext(ctx).Create(&testPendingForm).Error
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				cleanupCtx := pkg.WithUserEmail(context.Background(), "test@cim.local")
				tenv.DB.WithContext(cleanupCtx).Delete(&models.PaymentReceiptForm{}, testPendingForm.ID)
			})

			botClient := testutil.NewClient(tenv, models.RoleBotForm)

			sseURL := tenv.BaseURL + "/api/v1/payment-receipt-forms/pending"
			sseCtx, sseCancel := context.WithCancel(context.Background())
			DeferCleanup(sseCancel)

			client := sse.NewClient(sseURL)
			client.Headers["Authorization"] = fmt.Sprintf("Bearer %s", *botClient.AuthToken)

			eventsCh := make(chan *sse.Event, 5)
			err = client.SubscribeChanRawWithContext(sseCtx, eventsCh)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				client.Unsubscribe(eventsCh)
			})

			initialEvent := waitForEvent(eventsCh, 5*time.Second)
			Expect(initialEvent).NotTo(BeNil())
			Expect(string(initialEvent.Event)).To(Equal("pending_form_update"))
			initialPayload := decodeEventPayload(initialEvent)
			Expect(initialPayload).NotTo(BeNil())
			Expect(initialPayload["status"]).To(Equal(string(models.PaymentReceiptFormStatusPending)))

			if testPendingForm.FormNumber != nil {
				formNumberValue, ok := initialPayload["form_number"].(string)
				Expect(ok).To(BeTrue())
				Expect(formNumberValue).To(Equal(*testPendingForm.FormNumber))
			} else {
				Expect(initialPayload["form_number"]).To(BeNil())
			}

			Expect(uint(initialPayload["purchase_order_id"].(float64))).To(Equal(testPendingForm.PurchaseOrderID))
			Expect(initialPayload["full_name"]).To(Equal(testPendingForm.FullName))

			initialDateValue, ok := initialPayload["date"].(string)
			Expect(ok).To(BeTrue())
			parsedInitialDate, err := time.Parse(time.RFC3339, initialDateValue)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsedInitialDate.Format("2006-01-02")).To(Equal(testPendingForm.Date.Format("2006-01-02")))

			Expect(initialPayload["department"].(string)).To(Equal(testPendingForm.Department))
			Expect(initialPayload["details"]).To(Equal(testPendingForm.Details))
			Expect(initialPayload["total_amount"].(float64)).To(Equal(testPendingForm.TotalAmount))
		})

		It("should respect limit parameter and stream multiple pending forms", func() {
			ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")
			tenv.DB.WithContext(ctx).Exec("DELETE FROM payment_receipt_forms")

			var createdForms []models.PaymentReceiptForm
			for i := 0; i < 3; i++ {
				form := models.PaymentReceiptForm{
					PurchaseOrderID: testPurchaseOrder.ID,
					FullName:        fmt.Sprintf("Pending Form %d", i),
					Date:            time.Now().Add(time.Duration(i) * time.Hour),
					Department:      fmt.Sprintf("Department %d", i),
					Details:         fmt.Sprintf("Details %d", i),
					TotalAmount:     float64(1000 + i),
					Status:          models.PaymentReceiptFormStatusPending,
				}
				err := tenv.DB.WithContext(ctx).Create(&form).Error
				Expect(err).NotTo(HaveOccurred())
				createdForms = append(createdForms, form)

				formID := form.ID
				DeferCleanup(func() {
					cleanupCtx := pkg.WithUserEmail(context.Background(), "test@cim.local")
					tenv.DB.WithContext(cleanupCtx).Delete(&models.PaymentReceiptForm{}, formID)
				})
			}

			botClient := testutil.NewClient(tenv, models.RoleBotForm)

			sseURL := tenv.BaseURL + "/api/v1/payment-receipt-forms/pending?limit=2"
			sseCtx, sseCancel := context.WithCancel(context.Background())
			DeferCleanup(sseCancel)

			client := sse.NewClient(sseURL)
			client.Headers["Authorization"] = fmt.Sprintf("Bearer %s", *botClient.AuthToken)

			eventsCh := make(chan *sse.Event, 5)
			err := client.SubscribeChanRawWithContext(sseCtx, eventsCh)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				client.Unsubscribe(eventsCh)
			})

			expectedForms := []models.PaymentReceiptForm{createdForms[2], createdForms[1]}
			for _, expectedForm := range expectedForms {
				event := waitForEvent(eventsCh, 5*time.Second)
				Expect(event).NotTo(BeNil())
				Expect(string(event.Event)).To(Equal("pending_form_update"))
				payload := decodeEventPayload(event)
				Expect(payload).NotTo(BeNil())
				Expect(payload["full_name"]).To(Equal(expectedForm.FullName))
				Expect(payload["department"]).To(Equal(expectedForm.Department))
				Expect(payload["details"]).To(Equal(expectedForm.Details))
				Expect(payload["total_amount"].(float64)).To(Equal(expectedForm.TotalAmount))
			}
		})

		It("should generate incremental form numbers when forms are approved", func() {
			ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")
			tenv.DB.WithContext(ctx).Exec("DELETE FROM payment_receipt_forms")

			adminClient := testutil.NewClient(tenv, models.RoleAdmin)

			// Create multiple forms with the same date and purchase order (same inventory ID)
			formDate := time.Now()
			var createdFormIDs []uint
			for i := 0; i < 3; i++ {
				form := models.PaymentReceiptForm{
					PurchaseOrderID: testPurchaseOrder.ID,
					FullName:        fmt.Sprintf("Form %d", i+1),
					Date:            formDate,
					Department:      "Test Department",
					Details:         fmt.Sprintf("Test form %d", i+1),
					TotalAmount:     float64(1000 + i*100),
					Status:          models.PaymentReceiptFormStatusPending,
				}
				err := tenv.DB.WithContext(ctx).Create(&form).Error
				Expect(err).NotTo(HaveOccurred())
				createdFormIDs = append(createdFormIDs, form.ID)

				formID := form.ID
				DeferCleanup(func() {
					cleanupCtx := pkg.WithUserEmail(context.Background(), "test@cim.local")
					tenv.DB.WithContext(cleanupCtx).Delete(&models.PaymentReceiptForm{}, formID)
				})
			}

			// Approve forms one by one and verify incremental form numbers
			expectedDatePrefix := formDate.Format("20060102")
			expectedInventoryID := *testPurchaseOrder.InventoryID

			for i, formID := range createdFormIDs {
				approveURL := fmt.Sprintf("/api/v1/payment-receipt-forms/%d/approve", formID)
				approveResp, err := adminClient.MakeRequest("PUT", approveURL, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(approveResp.StatusCode).To(Equal(200))

				// Fetch the approved form to verify form number
				getURL := fmt.Sprintf("/api/v1/payment-receipt-forms/%d", formID)
				getResp, err := adminClient.MakeRequest("GET", getURL, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(getResp.StatusCode).To(Equal(200))

				formResp := testutil.ParseResponse(getResp)
				formNumber, ok := formResp["form_number"].(string)
				Expect(ok).To(BeTrue(), "form_number should be a string")
				Expect(formNumber).NotTo(BeEmpty(), "form_number should not be empty")

				// Verify form number format: YYYYMMDD-inventoryID-increment
				expectedFormNumber := fmt.Sprintf("%s-%d-%d", expectedDatePrefix, expectedInventoryID, i+1)
				Expect(formNumber).To(Equal(expectedFormNumber), "Form number should be incremental")
			}
		})

		It("should use finalized date from revenue expense finalization table when generating form number", func() {
			ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")
			tenv.DB.WithContext(ctx).Exec("DELETE FROM payment_receipt_forms")

			adminClient := testutil.NewClient(tenv, models.RoleAdmin)

			// Create a successful finalization record with finalized date (different from form date)
			lastFinalizedDate := time.Now().AddDate(0, 0, -5) // 5 days ago
			finalizationRepo := repository.NewRevenueExpenseFinalizationRepository(tenv.ContextfulDB())
			finalization := &models.RevenueExpenseFinalization{
				FinalizedDate: lastFinalizedDate.AddDate(0, 0, -1),
				Status:        pkg.Ptr(models.RevenueExpenseFinalizationStatusSuccess),
				Base: models.Base{
					CreatedAt: lastFinalizedDate,
				},
			}
			err := finalizationRepo.Create(ctx, finalization)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				cleanupCtx := pkg.WithUserEmail(context.Background(), "test@cim.local")
				tenv.DB.WithContext(cleanupCtx).Unscoped().Delete(&models.RevenueExpenseFinalization{}, finalization.ID)
			})

			// Create a form with a different date (today)
			formDate := time.Now()
			form := models.PaymentReceiptForm{
				PurchaseOrderID: testPurchaseOrder.ID,
				FullName:        "Test Form",
				Date:            formDate,
				Department:      "Test Department",
				Details:         "Test form with finalized date",
				TotalAmount:     1000,
				Status:          models.PaymentReceiptFormStatusPending,
			}
			err = tenv.DB.WithContext(ctx).Create(&form).Error
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				cleanupCtx := pkg.WithUserEmail(context.Background(), "test@cim.local")
				tenv.DB.WithContext(cleanupCtx).Delete(&models.PaymentReceiptForm{}, form.ID)
			})

			// Approve the form
			approveURL := fmt.Sprintf("/api/v1/payment-receipt-forms/%d/approve", form.ID)
			approveResp, err := adminClient.MakeRequest("PUT", approveURL, nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(approveResp.StatusCode).To(Equal(200))

			// Fetch the approved form to verify form number
			getURL := fmt.Sprintf("/api/v1/payment-receipt-forms/%d", form.ID)
			getResp, err := adminClient.MakeRequest("GET", getURL, nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(getResp.StatusCode).To(Equal(200))

			formResp := testutil.ParseResponse(getResp)
			formNumber, ok := formResp["form_number"].(string)
			Expect(ok).To(BeTrue(), "form_number should be a string")
			Expect(formNumber).NotTo(BeEmpty(), "form_number should not be empty")

			// Verify form number uses finalized date from finalization table, not form.Date
			expectedDatePrefix := lastFinalizedDate.Format("20060102")
			expectedInventoryID := *testPurchaseOrder.InventoryID
			expectedFormNumber := fmt.Sprintf("%s-%d-1", expectedDatePrefix, expectedInventoryID)
			Expect(formNumber).To(Equal(expectedFormNumber), "Form number should use finalized date from revenue expense finalization table, not form date")

			// Verify it's NOT using form.Date
			formDatePrefix := formDate.Format("20060102")
			unexpectedFormNumber := fmt.Sprintf("%s-%d-1", formDatePrefix, expectedInventoryID)
			Expect(formNumber).NotTo(Equal(unexpectedFormNumber), "Form number should NOT use form.Date")
		})

		It("should create form number with incremental order when there are many concurrent approval", func() {
			ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")
			tenv.DB.WithContext(ctx).Exec("DELETE FROM payment_receipt_forms")

			adminClient := testutil.NewClient(tenv, models.RoleAdmin)

			// Create multiple forms with the same date and purchase order (same inventory ID)
			formDate := time.Now()
			numForms := 20
			var createdForms []models.PaymentReceiptForm
			for i := 0; i < numForms; i++ {
				form := models.PaymentReceiptForm{
					PurchaseOrderID: testPurchaseOrder.ID,
					FullName:        fmt.Sprintf("Concurrent Form %d", i+1),
					Date:            formDate,
					Department:      "Test Department",
					Details:         fmt.Sprintf("Concurrent test form %d", i+1),
					TotalAmount:     float64(1000 + i*100),
					Status:          models.PaymentReceiptFormStatusPending,
				}
				err := tenv.DB.WithContext(ctx).Create(&form).Error
				Expect(err).NotTo(HaveOccurred())
				createdForms = append(createdForms, form)

				formID := form.ID
				DeferCleanup(func() {
					cleanupCtx := pkg.WithUserEmail(context.Background(), "test@cim.local")
					tenv.DB.WithContext(cleanupCtx).Delete(&models.PaymentReceiptForm{}, formID)
				})
			}

			// Approve all forms concurrently
			expectedDatePrefix := formDate.Format("20060102")
			expectedInventoryID := *testPurchaseOrder.InventoryID

			var wg sync.WaitGroup
			var mu sync.Mutex
			formNumbers := &sync.Map{} // form_number -> form ID
			errors := make([]error, 0)

			for _, form := range createdForms {
				wg.Add(1)
				go func(formID uint) {
					defer wg.Done()

					approveURL := fmt.Sprintf("/api/v1/payment-receipt-forms/%d/approve", formID)
					approveResp, err := adminClient.MakeRequest("PUT", approveURL, nil, testutil.WithAuth())
					if err != nil {
						mu.Lock()
						errors = append(errors, fmt.Errorf("failed to approve form %d: %w", formID, err))
						mu.Unlock()
						return
					}
					if approveResp.StatusCode != 200 {
						mu.Lock()
						errors = append(errors, fmt.Errorf("approve form %d returned status %d", formID, approveResp.StatusCode))
						mu.Unlock()
						return
					}

					// Fetch the approved form to get form number
					getURL := fmt.Sprintf("/api/v1/payment-receipt-forms/%d", formID)
					getResp, err := adminClient.MakeRequest("GET", getURL, nil, testutil.WithAuth())
					if err != nil {
						mu.Lock()
						errors = append(errors, fmt.Errorf("failed to get form %d: %w", formID, err))
						mu.Unlock()
						return
					}
					if getResp.StatusCode != 200 {
						mu.Lock()
						errors = append(errors, fmt.Errorf("get form %d returned status %d", formID, getResp.StatusCode))
						mu.Unlock()
						return
					}

					formResp := testutil.ParseResponse(getResp)
					formNumber, ok := formResp["form_number"].(string)
					if !ok || formNumber == "" {
						mu.Lock()
						errors = append(errors, fmt.Errorf("form %d has invalid form_number", formID))
						mu.Unlock()
						return
					}

					formNumbers.Store(formNumber, formID)
				}(form.ID)
			}

			// Wait for all approvals to complete
			wg.Wait()

			// Verify no errors occurred
			Expect(errors).To(BeEmpty(), "No errors should occur during concurrent approvals")

			// Verify all forms got form numbers
			formNumberCount := 0
			formNumbers.Range(func(key, value interface{}) bool {
				formNumberCount++
				return true
			})
			Expect(formNumberCount).To(Equal(numForms), "All forms should have form numbers")

			// Verify all form numbers are unique (no duplicates)
			formNumberSet := make(map[string]bool)
			formNumbers.Range(func(key, value interface{}) bool {
				formNumber := key.(string)
				Expect(formNumberSet[formNumber]).To(BeFalse(), "Form number %s should be unique", formNumber)
				formNumberSet[formNumber] = true
				return true
			})

			// Extract increment numbers and verify they are sequential (1, 2, 3, ..., numForms)
			increments := make([]int, 0, numForms)
			formNumbers.Range(func(key, value interface{}) bool {
				formNumber := key.(string)
				// Parse form number format: YYYYMMDD-inventoryID-increment
				// Use a more robust parsing approach
				parts := strings.Split(formNumber, "-")
				Expect(len(parts)).To(Equal(3), "Form number %s should have 3 parts separated by '-'", formNumber)

				datePrefix := parts[0]
				Expect(datePrefix).To(Equal(expectedDatePrefix), "Form number should use correct date prefix")

				var inventoryID uint
				_, err := fmt.Sscanf(parts[1], "%d", &inventoryID)
				Expect(err).NotTo(HaveOccurred(), "Form number %s should have valid inventory ID", formNumber)
				Expect(inventoryID).To(Equal(expectedInventoryID), "Form number should use correct inventory ID")

				var increment int
				_, err = fmt.Sscanf(parts[2], "%d", &increment)
				Expect(err).NotTo(HaveOccurred(), "Form number %s should have valid increment", formNumber)
				increments = append(increments, increment)
				return true
			})

			// Verify increments are sequential (1 to numForms)
			Expect(len(increments)).To(Equal(numForms), "Should have %d increments", numForms)

			// Check that all increments from 1 to numForms are present
			incrementSet := make(map[int]bool)
			for _, inc := range increments {
				Expect(inc).To(BeNumerically(">=", 1), "Increment should be at least 1")
				Expect(inc).To(BeNumerically("<=", numForms), "Increment should not exceed %d", numForms)
				Expect(incrementSet[inc]).To(BeFalse(), "Increment %d should be unique", inc)
				incrementSet[inc] = true
			}

			// Verify all increments from 1 to numForms are present
			for i := 1; i <= numForms; i++ {
				Expect(incrementSet[i]).To(BeTrue(), "Increment %d should be present", i)
			}
		})
	})

	Describe("ListPaymentReceiptForms", func() {
		It("should list payment receipt forms successfully with filters and approval status as default", func() {
			ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")
			tenv.DB.WithContext(ctx).Exec("DELETE FROM payment_receipt_forms")

			adminClient := testutil.NewClient(tenv, models.RoleAdmin)

			createForm := func(formNumbers []int) {
				for _, formNumber := range formNumbers {
					form := models.PaymentReceiptForm{
						PurchaseOrderID: testPurchaseOrder.ID,
						FormNumber:      pkg.Ptr(fmt.Sprintf("%s-%d-%d", time.Now().Format("20060102"), *testPurchaseOrder.InventoryID, formNumber)),
						FullName:        "Test Form",
						Date:            time.Now(),
						Department:      "Test Department",
						Details:         "Test form",
						TotalAmount:     1000,
						Status:          models.PaymentReceiptFormStatusApproved,
					}
					err := tenv.DB.WithContext(ctx).Create(&form).Error
					Expect(err).NotTo(HaveOccurred())
				}
			}

			createForm([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20})

			listURL := fmt.Sprintf("/api/v1/payment-receipt-forms?date=%s&inventory_id=%d&limit=10", time.Now().Format("2006-01-02"), *testPurchaseOrder.InventoryID)
			listResp, err := adminClient.MakeRequest("GET", listURL, nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(listResp.StatusCode).To(Equal(200), "Expected status 200, got %d", listResp.StatusCode)

			listRespBody := testutil.ParseResponse(listResp)
			Expect(listRespBody["total"]).To(Equal(float64(20)), "Expected total 20, got %f", listRespBody["total"])
			Expect(listRespBody["page"]).To(Equal(float64(1)), "Expected page 1, got %f", listRespBody["page"])
			Expect(listRespBody["limit"]).To(Equal(float64(10)), "Expected limit 10, got %f", listRespBody["limit"])
			Expect(listRespBody["totalPages"]).To(Equal(float64(2)), "Expected total pages 2, got %f", listRespBody["totalPages"])
			Expect(len(listRespBody["data"].([]interface{}))).To(Equal(10), "Expected data length 10, got %d", len(listRespBody["data"].([]interface{})))
		})
	})
})
