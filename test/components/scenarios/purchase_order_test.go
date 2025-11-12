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
			Base:   models.Base{ID: 1},
			Name:   "Test Product 1",
			UnitID: 1,
		},
		{
			Base:   models.Base{ID: 2},
			Name:   "Test Product 2",
			UnitID: 1,
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
				"unit_id":     1,
				"unit_price":  100,
			},
			{
				"product_id":  2,
				"supplier_id": 2,
				"quantity":    2,
				"unit_id":     1,
				"unit_price":  200,
			},
		},
		"notes": "Test purchase order",
	}
	t.Run("should create purchase order", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)
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
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)
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

func (suite *ComponentTestSuite) TestCreatePurchaseOrderWithDifferentUnits() {
	t := suite.T()
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
	db := suite.sharedTestContainer.DB

	testUnits := []models.Unit{
		{
			BaseUnitID:       pkg.Ptr(uint(1)),
			Name:             "Test Unit 2",
			ConversionFactor: 20,
			UnitType:         "general",
		},
	}

	err := db.WithContext(ctx).Create(&testUnits).Error
	require.NoError(t, err, "Failed to create units")

	testSuppliers := []models.Supplier{
		{
			Base: models.Base{ID: 1},
			Name: "Test Supplier 1",
		},
	}
	err = db.WithContext(ctx).Create(&testSuppliers).Error
	require.NoError(t, err, "Failed to create suppliers")

	testProducts := []models.Product{
		{
			Base:   models.Base{ID: 1},
			Name:   "Test Product 1",
			UnitID: 1,
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
				"quantity":    2,
				"unit_id":     2,
				"unit_price":  100,
			},
		},
	}
	t.Run("should create purchase order with different units and same base unit", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)
		resp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/purchase-orders", token, purchaseOrderData)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 201, resp.StatusCode)

		var purchaseOrderResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&purchaseOrderResp)
		require.NoError(t, err)
		assert.Equal(t, 200, int(purchaseOrderResp["total_amount"].(float64)))
		assert.Equal(t, 1, len(purchaseOrderResp["items"].([]interface{})))
		items := purchaseOrderResp["items"].([]interface{})
		assert.Equal(t, 40, int(items[0].(map[string]interface{})["quantity"].(float64)))
		assert.Equal(t, uint(1), uint(items[0].(map[string]interface{})["unit_id"].(float64)))
		assert.Equal(t, 5.0, items[0].(map[string]interface{})["unit_price"].(float64))
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
			Base:   models.Base{ID: 1},
			Name:   "Test Product 1",
			UnitID: 1,
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
				_, token, err := suite.CreateUniqueEmailAndToken(testCase.role)
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
			{role: models.RoleStaff, currentStatus: models.PurchaseOrderStatusPartiallyDelivered, expectedStatus: models.PurchaseOrderStatusOrderPlaced},
			{role: models.RoleStaff, currentStatus: models.PurchaseOrderStatusFullyDelivered, expectedStatus: models.PurchaseOrderStatusOrderPlaced},
			{role: models.RoleBotForm, currentStatus: models.PurchaseOrderStatusOrderPlaced, expectedStatus: models.PurchaseOrderStatusOrderPlaced},
			{role: models.RoleBotForm, currentStatus: models.PurchaseOrderStatusPartiallyDelivered, expectedStatus: models.PurchaseOrderStatusOrderPlaced},
			{role: models.RoleBotForm, currentStatus: models.PurchaseOrderStatusFullyDelivered, expectedStatus: models.PurchaseOrderStatusOrderPlaced},
		}
		for _, testCase := range testCases {
			currentId++
			t.Run(fmt.Sprintf("When current status is %s and user has %s role", testCase.currentStatus, testCase.role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(testCase.role)
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

func (suite *ComponentTestSuite) TestReceivePurchaseOrder() {
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
			Base:   models.Base{ID: 1},
			Name:   "Test Product 1",
			UnitID: 1,
		},
		{
			Base:   models.Base{ID: 2},
			Name:   "Test Product 2",
			UnitID: 1,
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

	currentId := uint(2000)

	t.Run("should receive purchase order", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant, models.RoleStaff}
		testCases := []struct {
			currentPOStatus       models.PurchaseOrderStatus
			currentPOItem1Status  models.PurchaseOrderItemStatus
			deliveredQuantity1    int
			deliveredQuantity2    int
			expectedPOStatus      models.PurchaseOrderStatus
			expectedPOItem1Status models.PurchaseOrderItemStatus
		}{
			{
				currentPOStatus:       models.PurchaseOrderStatusOrderPlaced,
				currentPOItem1Status:  models.PurchaseOrderItemStatusAwaitingDelivery,
				deliveredQuantity1:    50,
				deliveredQuantity2:    50,
				expectedPOStatus:      models.PurchaseOrderStatusPartiallyDelivered,
				expectedPOItem1Status: models.PurchaseOrderItemStatusPartiallyDelivered,
			},
			{
				currentPOStatus:       models.PurchaseOrderStatusOrderPlaced,
				currentPOItem1Status:  models.PurchaseOrderItemStatusAwaitingDelivery,
				deliveredQuantity1:    100,
				deliveredQuantity2:    50,
				expectedPOStatus:      models.PurchaseOrderStatusPartiallyDelivered,
				expectedPOItem1Status: models.PurchaseOrderItemStatusDelivered,
			},
			{
				currentPOStatus:       models.PurchaseOrderStatusOrderPlaced,
				currentPOItem1Status:  models.PurchaseOrderItemStatusAwaitingDelivery,
				deliveredQuantity1:    100,
				deliveredQuantity2:    100,
				expectedPOStatus:      models.PurchaseOrderStatusFullyDelivered,
				expectedPOItem1Status: models.PurchaseOrderItemStatusDelivered,
			},
			{
				currentPOStatus:       models.PurchaseOrderStatusPartiallyDelivered,
				currentPOItem1Status:  models.PurchaseOrderItemStatusPartiallyDelivered,
				deliveredQuantity1:    100,
				deliveredQuantity2:    100,
				expectedPOStatus:      models.PurchaseOrderStatusFullyDelivered,
				expectedPOItem1Status: models.PurchaseOrderItemStatusDelivered,
			},
		}
		for _, testCase := range testCases {
			for _, role := range roles {
				t.Run(fmt.Sprintf("When current status is %s and user has %s role", testCase.currentPOStatus, role), func(t *testing.T) {
					currentId++
					_, token, err := suite.CreateUniqueEmailAndToken(role)
					require.NoError(t, err)
					testPurchaseOrder := models.PurchaseOrder{
						Base:        models.Base{ID: currentId},
						OrderNumber: uuid.New().String(),
						Status:      testCase.currentPOStatus,
						InventoryID: &testInventories[0].ID,
						Items: []*models.PurchaseOrderItem{
							{
								ProductID:  &testProducts[0].ID,
								SupplierID: &testSuppliers[0].ID,
								Quantity:   100,
							},
							{
								ProductID:  &testProducts[1].ID,
								SupplierID: &testSuppliers[0].ID,
								Quantity:   100,
							},
						},
					}

					err = db.WithContext(ctx).Create(&testPurchaseOrder).Error
					require.NoError(t, err, "Failed to create purchase order")
					purchaseOrderID := testPurchaseOrder.ID

					var purchaseOrderItems []models.PurchaseOrderItem
					err = suite.sharedTestContainer.DB.WithContext(ctx).
						Where("purchase_order_id = ?", purchaseOrderID).
						Order("id ASC").
						Find(&purchaseOrderItems).Error
					require.NoError(t, err)
					require.Len(t, purchaseOrderItems, len(testPurchaseOrder.Items))

					payload := map[string]interface{}{
						"purchase_order_id": purchaseOrderID,
						"items": []map[string]interface{}{
							{
								"id":                purchaseOrderItems[0].ID,
								"received_quantity": testCase.deliveredQuantity1,
							},
							{
								"id":                purchaseOrderItems[1].ID,
								"received_quantity": testCase.deliveredQuantity2,
							},
						},
					}

					// Receive purchase order
					urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d/receive", purchaseOrderID)
					resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, payload)
					require.NoError(t, err)
					defer resp.Body.Close()
					assert.Equal(t, 200, resp.StatusCode)
					var purchaseOrderResp map[string]interface{}
					err = json.NewDecoder(resp.Body).Decode(&purchaseOrderResp)
					require.NoError(t, err)

					var purchaseOrder models.PurchaseOrder
					err = suite.sharedTestContainer.DB.WithContext(ctx).First(&purchaseOrder, "id = ?", purchaseOrderID).Error
					require.NoError(t, err)
					assert.Equal(t, testCase.expectedPOStatus, purchaseOrder.Status)
				})
			}
		}
	})
}
