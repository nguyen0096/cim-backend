package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/test/components/helpers"

	"github.com/google/uuid"
	"github.com/r3labs/sse/v2"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func strPtr(value string) *string {
	return &value
}

func (suite *ComponentTestSuite) TestPaymentReceiptFormPendingStream() {
	t := suite.T()
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
	db := suite.sharedTestContainer.DB

	require.NoError(t, db.WithContext(ctx).Exec("DELETE FROM payment_receipt_forms").Error)

	unit := models.Unit{
		Name:             fmt.Sprintf("SSE Unit %s", uuid.New().String()),
		UnitType:         "general",
		ConversionFactor: 1,
	}
	require.NoError(t, db.WithContext(ctx).Create(&unit).Error)

	supplier := models.Supplier{
		Name: fmt.Sprintf("SSE Supplier %s", uuid.New().String()),
	}
	require.NoError(t, db.WithContext(ctx).Create(&supplier).Error)

	product := models.Product{
		Name:   fmt.Sprintf("SSE Product %s", uuid.New().String()),
		UnitID: unit.ID,
	}
	require.NoError(t, db.WithContext(ctx).Create(&product).Error)

	inventory := models.Inventory{
		Name:     fmt.Sprintf("SSE Inventory %s", uuid.New().String()),
		Location: "Automation Floor",
	}
	require.NoError(t, db.WithContext(ctx).Create(&inventory).Error)

	purchaseOrder := models.PurchaseOrder{
		Base: models.Base{
			CreatedAt: time.Now(),
		},
		OrderNumber: fmt.Sprintf("PO-%s", uuid.New().String()),
		InventoryID: &inventory.ID,
		Status:      models.PurchaseOrderStatusOrderPlaced,
		Items: []*models.PurchaseOrderItem{
			{
				ProductID:  &product.ID,
				SupplierID: &supplier.ID,
				UnitID:     &unit.ID,
				Quantity:   decimal.NewFromInt(1),
				UnitPrice:  125000,
			},
		},
	}
	require.NoError(t, db.WithContext(ctx).Create(&purchaseOrder).Error)

	t.Cleanup(func() {
		db.WithContext(ctx).Exec("DELETE FROM payment_receipt_forms WHERE purchase_order_id = ?", purchaseOrder.ID)
		db.WithContext(ctx).Delete(&models.PurchaseOrder{}, purchaseOrder.ID)
		db.WithContext(ctx).Delete(&models.Inventory{}, inventory.ID)
		db.WithContext(ctx).Delete(&models.Product{}, product.ID)
		db.WithContext(ctx).Delete(&models.Supplier{}, supplier.ID)
		db.WithContext(ctx).Delete(&models.Unit{}, unit.ID)
	})

	t.Run("should stream pending payment receipt form updates", func(t *testing.T) {
		_, adminToken, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)
		_, botToken, err := suite.CreateUniqueEmailAndToken(models.RoleBotForm)
		require.NoError(t, err)

		sseURL := suite.sharedTestContainer.BaseURL + "/api/v1/payment-receipt-forms/pending"
		sseCtx, sseCancel := context.WithCancel(context.Background())
		defer sseCancel()

		client := sse.NewClient(sseURL)
		client.Headers["Authorization"] = fmt.Sprintf("Bearer %s", botToken)

		eventsCh := make(chan *sse.Event, 5)
		require.NoError(t, client.SubscribeChanRawWithContext(sseCtx, eventsCh))
		t.Cleanup(func() {
			client.Unsubscribe(eventsCh)
		})

		// Should receive initial event
		initialEvent := waitForEvent(eventsCh, 5*time.Second)
		require.NotNil(t, initialEvent)
		require.Equal(t, "pending_form_update", string(initialEvent.Event))
		initialPayload := decodeEventPayload(initialEvent)
		require.NotNil(t, initialPayload)
		assert.Equal(t, "No pending payment receipt form found", initialPayload["message"])

		expectedDate := time.Now().Format("2006-01-02")
		payload := map[string]interface{}{
			"purchase_order_id": purchaseOrder.ID,
			"full_name":         "Automation Bot",
			"date":              time.Now(),
			"department":        "Operations",
			"details":           "Generated from SSE component test",
			"total_amount":      125000,
		}
		createResp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/payment-receipt-forms", adminToken, payload)
		require.NoError(t, err)
		defer createResp.Body.Close()
		assert.Equal(t, http.StatusCreated, createResp.StatusCode)

		var formResp map[string]interface{}
		err = json.NewDecoder(createResp.Body).Decode(&formResp)
		require.NoError(t, err)
		createdFormID := uint(formResp["id"].(float64))
		t.Cleanup(func() {
			db.WithContext(ctx).Delete(&models.PaymentReceiptForm{}, createdFormID)
		})

		pendingEvent := waitForEvent(eventsCh, 5*time.Second)
		require.NotNil(t, pendingEvent)
		require.Equal(t, "pending_form_update", string(pendingEvent.Event))
		pendingPayload := decodeEventPayload(pendingEvent)
		require.NotNil(t, pendingPayload)

		sseDate, ok := pendingPayload["date"].(string)
		require.True(t, ok)
		parsedSseDate, err := time.Parse(time.RFC3339, sseDate)
		require.NoError(t, err)
		assert.Equal(t, expectedDate, parsedSseDate.Format("2006-01-02"))

		statusValue, ok := pendingPayload["status"].(string)
		require.True(t, ok, "status field missing from SSE event")
		assert.Equal(t, string(models.PaymentReceiptFormStatusPending), statusValue)

		sseFormID := uint(pendingPayload["id"].(float64))
		assert.Equal(t, createdFormID, sseFormID)

		ssePurchaseOrderID := uint(pendingPayload["purchase_order_id"].(float64))
		assert.Equal(t, purchaseOrder.ID, ssePurchaseOrderID)
	})

	t.Run("should get initial pending forms if there are any pending forms", func(t *testing.T) {
		testPendingForm := models.PaymentReceiptForm{
			Base: models.Base{
				CreatedAt: time.Now(),
			},
			FormNumber:      strPtr(fmt.Sprintf("FORM-%s", uuid.New().String())),
			PurchaseOrderID: purchaseOrder.ID,
			FullName:        "Automation Bot",
			Date:            time.Now(),
			Department:      "Operations",
			Details:         "Generated from SSE component test",
			TotalAmount:     125000,
			Status:          models.PaymentReceiptFormStatusPending,
		}
		t.Cleanup(func() {
			db.WithContext(ctx).Delete(&models.PaymentReceiptForm{}, testPendingForm.ID)
		})
		require.NoError(t, db.WithContext(ctx).Create(&testPendingForm).Error)

		_, botToken, err := suite.CreateUniqueEmailAndToken(models.RoleBotForm)
		require.NoError(t, err)

		sseURL := suite.sharedTestContainer.BaseURL + "/api/v1/payment-receipt-forms/pending"
		sseCtx, sseCancel := context.WithCancel(context.Background())
		defer sseCancel()

		client := sse.NewClient(sseURL)
		client.Headers["Authorization"] = fmt.Sprintf("Bearer %s", botToken)

		eventsCh := make(chan *sse.Event, 5)
		require.NoError(t, client.SubscribeChanRawWithContext(sseCtx, eventsCh))
		t.Cleanup(func() {
			client.Unsubscribe(eventsCh)
		})

		initialEvent := waitForEvent(eventsCh, 5*time.Second)
		require.NotNil(t, initialEvent)
		require.Equal(t, "pending_form_update", string(initialEvent.Event))
		initialPayload := decodeEventPayload(initialEvent)
		require.NotNil(t, initialPayload)
		assert.Equal(t, string(models.PaymentReceiptFormStatusPending), initialPayload["status"])
		if testPendingForm.FormNumber != nil {
			formNumberValue, ok := initialPayload["form_number"].(string)
			require.True(t, ok)
			assert.Equal(t, *testPendingForm.FormNumber, formNumberValue)
		} else {
			assert.Nil(t, initialPayload["form_number"])
		}
		assert.Equal(t, testPendingForm.PurchaseOrderID, uint(initialPayload["purchase_order_id"].(float64)))
		assert.Equal(t, testPendingForm.FullName, initialPayload["full_name"])
		initialDateValue, ok := initialPayload["date"].(string)
		require.True(t, ok)
		parsedInitialDate, err := time.Parse(time.RFC3339, initialDateValue)
		require.NoError(t, err)
		assert.Equal(t, testPendingForm.Date.Format("2006-01-02"), parsedInitialDate.Format("2006-01-02"))
		assert.Equal(t, testPendingForm.Department, initialPayload["department"].(string))
		assert.Equal(t, testPendingForm.Details, initialPayload["details"].(string))
		assert.Equal(t, testPendingForm.TotalAmount, initialPayload["total_amount"].(float64))
	})

	t.Run("should respect limit parameter and stream multiple pending forms", func(t *testing.T) {
		require.NoError(t, db.WithContext(ctx).Exec("DELETE FROM payment_receipt_forms").Error)

		var createdForms []models.PaymentReceiptForm
		for i := 0; i < 3; i++ {
			form := models.PaymentReceiptForm{
				Base: models.Base{
					CreatedAt: time.Now().Add(time.Duration(i) * time.Minute),
				},
				PurchaseOrderID: purchaseOrder.ID,
				FullName:        fmt.Sprintf("Pending Form %d", i),
				Date:            time.Now().Add(time.Duration(i) * time.Hour),
				Department:      fmt.Sprintf("Department %d", i),
				Details:         fmt.Sprintf("Details %d", i),
				TotalAmount:     float64(1000 + i),
				Status:          models.PaymentReceiptFormStatusPending,
			}
			require.NoError(t, db.WithContext(ctx).Create(&form).Error)
			createdForms = append(createdForms, form)
		}

		_, botToken, err := suite.CreateUniqueEmailAndToken(models.RoleBotForm)
		require.NoError(t, err)

		sseURL := suite.sharedTestContainer.BaseURL + "/api/v1/payment-receipt-forms/pending?limit=2"
		sseCtx, sseCancel := context.WithCancel(context.Background())
		defer sseCancel()

		client := sse.NewClient(sseURL)
		client.Headers["Authorization"] = fmt.Sprintf("Bearer %s", botToken)

		eventsCh := make(chan *sse.Event, 5)
		require.NoError(t, client.SubscribeChanRawWithContext(sseCtx, eventsCh))
		t.Cleanup(func() {
			client.Unsubscribe(eventsCh)
		})

		expectedForms := []models.PaymentReceiptForm{createdForms[2], createdForms[1]}
		for _, expectedForm := range expectedForms {
			event := waitForEvent(eventsCh, 5*time.Second)
			require.NotNil(t, event)
			require.Equal(t, "pending_form_update", string(event.Event))
			payload := decodeEventPayload(event)
			require.NotNil(t, payload)
			assert.Equal(t, expectedForm.FullName, payload["full_name"])
			assert.Equal(t, expectedForm.Department, payload["department"])
			assert.Equal(t, expectedForm.Details, payload["details"])
			assert.Equal(t, expectedForm.TotalAmount, payload["total_amount"].(float64))
		}
	})

	t.Run("should generate incremental form numbers when forms are approved", func(t *testing.T) {
		require.NoError(t, db.WithContext(ctx).Exec("DELETE FROM payment_receipt_forms").Error)

		// Create multiple forms with the same date and purchase order (same inventory ID)
		formDate := time.Now()
		var createdFormIDs []uint
		for i := 0; i < 3; i++ {
			form := models.PaymentReceiptForm{
				Base: models.Base{
					CreatedAt: time.Now(),
				},
				PurchaseOrderID: purchaseOrder.ID,
				FullName:        fmt.Sprintf("Form %d", i+1),
				Date:            formDate,
				Department:      "Test Department",
				Details:         fmt.Sprintf("Test form %d", i+1),
				TotalAmount:     float64(1000 + i*100),
				Status:          models.PaymentReceiptFormStatusPending,
			}
			require.NoError(t, db.WithContext(ctx).Create(&form).Error)
			createdFormIDs = append(createdFormIDs, form.ID)
		}

		// Get admin token for approval
		_, adminToken, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		// Approve forms one by one and verify incremental form numbers
		expectedDatePrefix := formDate.Format("20060102")
		expectedInventoryID := *purchaseOrder.InventoryID

		for i, formID := range createdFormIDs {
			approveURL := fmt.Sprintf("%s/api/v1/payment-receipt-forms/%d/approve", suite.sharedTestContainer.BaseURL, formID)
			approveResp, err := helpers.MakeRequest(t, "PUT", approveURL, adminToken, nil)
			require.NoError(t, err)
			defer approveResp.Body.Close()
			assert.Equal(t, http.StatusOK, approveResp.StatusCode)

			// Fetch the approved form to verify form number
			getURL := fmt.Sprintf("%s/api/v1/payment-receipt-forms/%d", suite.sharedTestContainer.BaseURL, formID)
			getResp, err := helpers.MakeRequest(t, "GET", getURL, adminToken, nil)
			require.NoError(t, err)
			defer getResp.Body.Close()
			assert.Equal(t, http.StatusOK, getResp.StatusCode)

			var formResp map[string]interface{}
			err = json.NewDecoder(getResp.Body).Decode(&formResp)
			require.NoError(t, err)

			formNumber, ok := formResp["form_number"].(string)
			require.True(t, ok, "form_number should be a string")
			require.NotEmpty(t, formNumber, "form_number should not be empty")

			// Verify form number format: YYYYMMDD-inventoryID-increment
			expectedFormNumber := fmt.Sprintf("%s-%d-%d", expectedDatePrefix, expectedInventoryID, i+1)
			assert.Equal(t, expectedFormNumber, formNumber, "Form number should be incremental")
		}

		// Cleanup
		for _, formID := range createdFormIDs {
			t.Cleanup(func() {
				db.WithContext(ctx).Delete(&models.PaymentReceiptForm{}, formID)
			})
		}
	})
}
