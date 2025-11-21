package apptest

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil"
	"cim-backend/pkg/testutil/fixture"
	"context"
	"encoding/json"
	"fmt"
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
	})
})
