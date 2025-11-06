package scenarios

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/test/components/helpers"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *ComponentTestSuite) TestCreatePurchaseOrder() {
	t := suite.T()
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
	db := suite.sharedTestContainer.DB
	testSuppliers := []models.Supplier{
		{
			Base: models.Base{ID: 1},
			Name: "Test Supplier 1",
		},
		{
			Base: models.Base{ID: 2},
			Name: "Test Supplier 2",
		},
	}
	err := db.WithContext(ctx).Create(&testSuppliers).Error
	require.NoError(t, err, "Failed to create suppliers")
	testProducts := []models.Product{
		{
			Base: models.Base{ID: 1},
			Name: "Test Product 1",
		},
		{
			Base: models.Base{ID: 2},
			Name: "Test Product 2",
		},
	}
	err = db.WithContext(ctx).Create(&testProducts).Error
	require.NoError(t, err, "Failed to create products")
	testInventories := []models.Inventory{
		{
			Base: models.Base{ID: 1},
			Name: "Test Inventory 1",
		},
	}
	err = db.WithContext(ctx).Create(&testInventories).Error
	require.NoError(t, err, "Failed to create inventories")

	purchaseOrderData := map[string]interface{}{
		"inventory_id": 1,
		"items": []map[string]interface{}{
			{
				"product_id":  1,
				"supplier_id": 1,
				"quantity":    1,
				"unit_price":  100,
			},
			{
				"product_id":  2,
				"supplier_id": 2,
				"quantity":    2,
				"unit_price":  200,
			},
		},
		"notes": "Test purchase order",
	}
	t.Run("should create purchase order", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				uniqueEmail := fmt.Sprintf("test-purchase-order-%s@example.com", uuid.New().String())
				user, err := helpers.CreateTestUser(context.Background(), suite.sharedTestContainer.DB, uniqueEmail, "Test User", role)
				require.NoError(t, err)

				// Get auth token
				token := helpers.GetAuthToken(suite.sharedTestContainer.MockAuth, user.UID, user.Email, user.Name)
				// Create purchase order

				resp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/purchase-orders", token, purchaseOrderData)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 201, resp.StatusCode)

				var purchaseOrderResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&purchaseOrderResp)
				require.NoError(t, err)
				assert.True(t, strings.HasPrefix(purchaseOrderResp["order_number"].(string), "PO-"+time.Now().Format("060102-150405")), purchaseOrderResp["order_number"])
				assert.Equal(t, 1, int(purchaseOrderResp["inventory_id"].(float64)))
				assert.Equal(t, "order_placed", purchaseOrderResp["status"])
				assert.Equal(t, 500, int(purchaseOrderResp["total_amount"].(float64)))
				assert.Equal(t, "Test purchase order", purchaseOrderResp["notes"])
				assert.Equal(t, 2, len(purchaseOrderResp["items"].([]interface{})))
				items := purchaseOrderResp["items"].([]interface{})
				assert.Equal(t, 1, int(items[0].(map[string]interface{})["product_id"].(float64)))
				assert.Equal(t, 1, int(items[0].(map[string]interface{})["supplier_id"].(float64)))
				assert.Equal(t, 1, int(items[0].(map[string]interface{})["quantity"].(float64)))
				assert.Equal(t, 100, int(items[0].(map[string]interface{})["unit_price"].(float64)))
				assert.Equal(t, 2, int(items[1].(map[string]interface{})["product_id"].(float64)))
				assert.Equal(t, 2, int(items[1].(map[string]interface{})["supplier_id"].(float64)))
				assert.Equal(t, 2, int(items[1].(map[string]interface{})["quantity"].(float64)))
				assert.Equal(t, 200, int(items[1].(map[string]interface{})["unit_price"].(float64)))

				var purchaseOrder models.PurchaseOrder
				err = suite.sharedTestContainer.DB.WithContext(ctx).First(&purchaseOrder, "id = ?", purchaseOrderResp["id"]).Error
				require.NoError(t, err)
				assert.True(t, strings.HasPrefix(purchaseOrder.OrderNumber, "PO-"+time.Now().Format("060102-150405")), purchaseOrder.OrderNumber)
				assert.Equal(t, uint(1), *purchaseOrder.InventoryID)
				assert.Equal(t, models.PurchaseOrderStatusOrderPlaced, purchaseOrder.Status)
				assert.Equal(t, "Test purchase order", purchaseOrder.Notes)
			})
		}
	})

	t.Run("should not create purchase order", func(t *testing.T) {
		roles := []models.UserRole{models.RoleStaff, models.RoleBotForm}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				uniqueEmail := fmt.Sprintf("test-purchase-order-%s@example.com", uuid.New().String())
				user, err := helpers.CreateTestUser(context.Background(), suite.sharedTestContainer.DB, uniqueEmail, "Test User", role)
				require.NoError(t, err)

				// Get auth token
				token := helpers.GetAuthToken(suite.sharedTestContainer.MockAuth, user.UID, user.Email, user.Name)

				resp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/purchase-orders", token, purchaseOrderData)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 403, resp.StatusCode)

				var errorResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&errorResp)
				require.NoError(t, err)
				assert.Equal(t, "Access denied: "+string(role)+" role cannot create purchase-orders", errorResp["error"])
			})
		}
	})
}

func (suite *ComponentTestSuite) TestCancelPurchaseOrder() {
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

	testProducts := []models.Product{
		{
			Base: models.Base{ID: 1},
			Name: "Test Product 1",
		},
	}
	err = db.WithContext(ctx).Create(&testProducts).Error
	require.NoError(t, err, "Failed to create products")

	testInventories := []models.Inventory{
		{
			Base: models.Base{ID: 1},
			Name: "Test Inventory 1",
		},
	}
	err = db.WithContext(ctx).Create(&testInventories).Error
	require.NoError(t, err, "Failed to create inventories")

	currentId := uint(1000)

	t.Run("should cancel purchase order", func(t *testing.T) {
		testCases := []struct {
			role           models.UserRole
			currentStatus  models.PurchaseOrderStatus
			expectedStatus models.PurchaseOrderStatus
		}{
			{role: models.RoleAdmin, currentStatus: models.PurchaseOrderStatusOrderPlaced, expectedStatus: models.PurchaseOrderStatusCancelled},
			{role: models.RoleAccountant, currentStatus: models.PurchaseOrderStatusOrderPlaced, expectedStatus: models.PurchaseOrderStatusCancelled},
			{role: models.RoleAdmin, currentStatus: models.PurchaseOrderStatusPartiallyDelivered, expectedStatus: models.PurchaseOrderStatusCancelled},
			{role: models.RoleAccountant, currentStatus: models.PurchaseOrderStatusPartiallyDelivered, expectedStatus: models.PurchaseOrderStatusCancelled},
			{role: models.RoleAdmin, currentStatus: models.PurchaseOrderStatusFullyDelivered, expectedStatus: models.PurchaseOrderStatusCancelled},
			{role: models.RoleAccountant, currentStatus: models.PurchaseOrderStatusFullyDelivered, expectedStatus: models.PurchaseOrderStatusCancelled},
			// can update status by permission but not change immutable status in db
			{role: models.RoleAdmin, currentStatus: models.PurchaseOrderStatusCompleted, expectedStatus: models.PurchaseOrderStatusCompleted},
			{role: models.RoleAccountant, currentStatus: models.PurchaseOrderStatusCompleted, expectedStatus: models.PurchaseOrderStatusCompleted},
		}
		for _, testCase := range testCases {
			currentId++
			t.Run(fmt.Sprintf("When current status is %s and user has %s role", testCase.currentStatus, testCase.role), func(t *testing.T) {
				uniqueEmail := fmt.Sprintf("test-purchase-order-%s@example.com", uuid.New().String())
				user, err := helpers.CreateTestUser(context.Background(), suite.sharedTestContainer.DB, uniqueEmail, "Test User", testCase.role)
				require.NoError(t, err)

				testPurchaseOrder := models.PurchaseOrder{
					Base:        models.Base{ID: currentId},
					OrderNumber: uuid.New().String(),
					Status:      testCase.currentStatus,
					InventoryID: &testInventories[0].ID,
					Items: []*models.PurchaseOrderItem{
						{
							ProductID:  &testProducts[0].ID,
							SupplierID: &testSuppliers[0].ID,
							Quantity:   1,
						},
					},
				}

				err = db.WithContext(ctx).Create(&testPurchaseOrder).Error
				require.NoError(t, err, "Failed to create purchase order")
				purchaseOrderID := testPurchaseOrder.ID

				// Get auth token
				token := helpers.GetAuthToken(suite.sharedTestContainer.MockAuth, user.UID, user.Email, user.Name)
				// Cancel purchase order
				urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d/status", purchaseOrderID)
				resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, map[string]interface{}{"status": "cancelled"})
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 200, resp.StatusCode)
				var purchaseOrderResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&purchaseOrderResp)
				require.NoError(t, err)
				assert.Equal(t, "Purchase order status updated successfully", purchaseOrderResp["message"])

				var purchaseOrder models.PurchaseOrder
				err = suite.sharedTestContainer.DB.WithContext(ctx).First(&purchaseOrder, "id = ?", purchaseOrderID).Error
				require.NoError(t, err)
				assert.Equal(t, testCase.expectedStatus, purchaseOrder.Status)
			})
		}
	})

	t.Run("should not cancel purchase order", func(t *testing.T) {
		testCases := []struct {
			role           models.UserRole
			currentStatus  models.PurchaseOrderStatus
			expectedStatus models.PurchaseOrderStatus
		}{
			{role: models.RoleStaff, currentStatus: models.PurchaseOrderStatusOrderPlaced, expectedStatus: models.PurchaseOrderStatusOrderPlaced},
			{role: models.RoleBotForm, currentStatus: models.PurchaseOrderStatusOrderPlaced, expectedStatus: models.PurchaseOrderStatusOrderPlaced},
		}
		for _, testCase := range testCases {
			currentId++
			t.Run(fmt.Sprintf("When current status is %s and user has %s role", testCase.currentStatus, testCase.role), func(t *testing.T) {
				uniqueEmail := fmt.Sprintf("test-purchase-order-%s@example.com", uuid.New().String())
				user, err := helpers.CreateTestUser(context.Background(), suite.sharedTestContainer.DB, uniqueEmail, "Test User", testCase.role)
				require.NoError(t, err)
				testPurchaseOrder := models.PurchaseOrder{
					Base:        models.Base{ID: currentId},
					OrderNumber: uuid.New().String(),
					Status:      testCase.currentStatus,
					InventoryID: &testInventories[0].ID,
					Items: []*models.PurchaseOrderItem{
						{
							ProductID:  &testProducts[0].ID,
							SupplierID: &testSuppliers[0].ID,
							Quantity:   1,
						},
					},
				}
				err = db.WithContext(ctx).Create(&testPurchaseOrder).Error
				require.NoError(t, err, "Failed to create purchase order")
				purchaseOrderID := testPurchaseOrder.ID

				// Get auth token
				token := helpers.GetAuthToken(suite.sharedTestContainer.MockAuth, user.UID, user.Email, user.Name)
				// Cancel purchase order
				urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d/status", purchaseOrderID)
				resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, map[string]interface{}{"status": "cancelled"})
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 403, resp.StatusCode)
				var errorResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&errorResp)
				require.NoError(t, err)
				assert.Equal(t, "Access denied: "+string(testCase.role)+" role cannot update purchase-orders", errorResp["error"])
			})
		}
	})
}
