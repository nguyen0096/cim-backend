package scenarios

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/test/components/helpers"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
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
			Base:             models.Base{ID: 2},
			Name:             "Base Unit 2",
			ConversionFactor: 1,
			UnitType:         "general",
		},
		{
			Base:             models.Base{ID: 3},
			Name:             "Base Unit 3",
			ConversionFactor: 1,
			UnitType:         "mass",
		},
		{
			Base:             models.Base{ID: 4},
			BaseUnitID:       pkg.Ptr(uint(1)),
			Name:             "Test Unit 2",
			ConversionFactor: 20,
			UnitType:         "general",
		},
		{
			Base:             models.Base{ID: 5},
			BaseUnitID:       pkg.Ptr(uint(3)),
			Name:             "Test Unit 3",
			ConversionFactor: 20,
			UnitType:         "mass",
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

	t.Run("should create purchase order with different units and same base unit", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		payload := map[string]interface{}{
			"inventory_id": 1,
			"items": []map[string]interface{}{
				{
					"product_id":  1,
					"supplier_id": 1,
					"quantity":    2,
					"unit_id":     4,
					"unit_price":  100,
				},
			},
		}
		resp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/purchase-orders", token, payload)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 201, resp.StatusCode)

		var purchaseOrderResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&purchaseOrderResp)
		require.NoError(t, err)
		assert.Equal(t, 200, int(purchaseOrderResp["total_amount"].(float64)))
		assert.Equal(t, 1, len(purchaseOrderResp["items"].([]interface{})))
		respItems := purchaseOrderResp["items"].([]interface{})
		assert.Equal(t, 40, int(respItems[0].(map[string]interface{})["quantity"].(float64)))
		assert.Equal(t, uint(1), uint(respItems[0].(map[string]interface{})["unit_id"].(float64)))
		// After conversion, unit_id is 1 (base unit), so the unit name should be the base unit's name
		assert.Equal(t, "unit", respItems[0].(map[string]interface{})["unit"].(map[string]interface{})["name"])
		assert.Equal(t, 5.0, respItems[0].(map[string]interface{})["unit_price"].(float64))
		assert.Equal(t, 200, int(respItems[0].(map[string]interface{})["total_amount"].(float64)))

		// Verify database
		var purchaseOrder models.PurchaseOrder
		err = suite.sharedTestContainer.DB.WithContext(ctx).Preload("Items").First(&purchaseOrder, "id = ?", purchaseOrderResp["id"]).Error
		require.NoError(t, err)
		assert.Equal(t, 200, int(purchaseOrder.TotalAmount))
		items := purchaseOrder.Items
		assert.Equal(t, 1, len(items))
		assert.Equal(t, uint(1), *items[0].ProductID)
		assert.Equal(t, uint(1), *items[0].SupplierID)
		assert.Equal(t, uint(1), *items[0].UnitID)
		assert.Equal(t, 40, int(items[0].Quantity))
		assert.Equal(t, 5.0, items[0].UnitPrice)
		assert.Equal(t, 200.0, items[0].TotalAmount)
	})

	t.Run("should not create purchase order with different units and different base unit", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		payload := map[string]interface{}{
			"inventory_id": 1,
			"items": []map[string]interface{}{
				{
					"product_id":  1,
					"supplier_id": 1,
					"quantity":    2,
					"unit_id":     5,
					"unit_price":  100,
				},
			},
		}
		resp, err := helpers.MakeRequest(t, "POST", suite.sharedTestContainer.BaseURL+"/api/v1/purchase-orders", token, payload)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 400, resp.StatusCode)
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

				unitID := testProducts[0].UnitID
				testPurchaseOrder := models.PurchaseOrder{
					Base:        models.Base{ID: currentId},
					OrderNumber: uuid.New().String(),
					Status:      testCase.currentStatus,
					InventoryID: &testInventories[0].ID,
					Items: []*models.PurchaseOrderItem{
						{
							ProductID:  &testProducts[0].ID,
							SupplierID: &testSuppliers[0].ID,
							UnitID:     &unitID,
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
				unitID := testProducts[0].UnitID
				testPurchaseOrder := models.PurchaseOrder{
					Base:        models.Base{ID: currentId},
					OrderNumber: uuid.New().String(),
					Status:      testCase.currentStatus,
					InventoryID: &testInventories[0].ID,
					Items: []*models.PurchaseOrderItem{
						{
							ProductID:  &testProducts[0].ID,
							SupplierID: &testSuppliers[0].ID,
							UnitID:     &unitID,
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
					unitID1 := testProducts[0].UnitID
					unitID2 := testProducts[1].UnitID
					testPurchaseOrder := models.PurchaseOrder{
						Base:        models.Base{ID: currentId},
						OrderNumber: uuid.New().String(),
						Status:      testCase.currentPOStatus,
						InventoryID: &testInventories[0].ID,
						Items: []*models.PurchaseOrderItem{
							{
								ProductID:  &testProducts[0].ID,
								SupplierID: &testSuppliers[0].ID,
								UnitID:     &unitID1,
								Quantity:   100,
							},
							{
								ProductID:  &testProducts[1].ID,
								SupplierID: &testSuppliers[0].ID,
								UnitID:     &unitID2,
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

func (suite *ComponentTestSuite) TestUpdatePurchaseOrder() {
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

	testUnits := []models.Unit{
		{
			Base:             models.Base{ID: 2},
			Name:             "Test Unit 2",
			Symbol:           "unit",
			BaseUnitID:       pkg.Ptr(uint(1)),
			ConversionFactor: 1,
			UnitType:         "general",
		},
	}
	err = db.WithContext(ctx).Create(&testUnits).Error
	require.NoError(t, err, "Failed to create units")

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

	currentId := uint(3000)

	t.Run("should update purchase order", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
		testCases := []struct {
			currentPOStatus      models.PurchaseOrderStatus
			currentPOItemStatus  models.PurchaseOrderItemStatus
			orderedQuantity      int
			deliveredQuantity    int
			updatedQuantity      int
			expectedPOStatus     models.PurchaseOrderStatus
			expectedPOItemStatus models.PurchaseOrderItemStatus
		}{
			{
				currentPOStatus:      models.PurchaseOrderStatusOrderPlaced,
				currentPOItemStatus:  models.PurchaseOrderItemStatusAwaitingDelivery,
				orderedQuantity:      50,
				deliveredQuantity:    0,
				updatedQuantity:      100,
				expectedPOStatus:     models.PurchaseOrderStatusOrderPlaced,
				expectedPOItemStatus: models.PurchaseOrderItemStatusAwaitingDelivery,
			},
			{
				currentPOStatus:      models.PurchaseOrderStatusPartiallyDelivered,
				currentPOItemStatus:  models.PurchaseOrderItemStatusPartiallyDelivered,
				orderedQuantity:      50,
				deliveredQuantity:    20,
				updatedQuantity:      100,
				expectedPOStatus:     models.PurchaseOrderStatusPartiallyDelivered,
				expectedPOItemStatus: models.PurchaseOrderItemStatusPartiallyDelivered,
			},
			{
				currentPOStatus:      models.PurchaseOrderStatusFullyDelivered,
				currentPOItemStatus:  models.PurchaseOrderItemStatusDelivered,
				orderedQuantity:      50,
				deliveredQuantity:    50,
				updatedQuantity:      100,
				expectedPOStatus:     models.PurchaseOrderStatusPartiallyDelivered,
				expectedPOItemStatus: models.PurchaseOrderItemStatusPartiallyDelivered,
			},
			{
				currentPOStatus:      models.PurchaseOrderStatusPartiallyDelivered,
				currentPOItemStatus:  models.PurchaseOrderItemStatusPartiallyDelivered,
				orderedQuantity:      50,
				deliveredQuantity:    30,
				updatedQuantity:      30,
				expectedPOStatus:     models.PurchaseOrderStatusFullyDelivered,
				expectedPOItemStatus: models.PurchaseOrderItemStatusDelivered,
			},
			// No change to quantity, no change to status
			{
				currentPOStatus:      models.PurchaseOrderStatusPartiallyDelivered,
				currentPOItemStatus:  models.PurchaseOrderItemStatusPartiallyDelivered,
				orderedQuantity:      50,
				deliveredQuantity:    30,
				updatedQuantity:      50,
				expectedPOStatus:     models.PurchaseOrderStatusPartiallyDelivered,
				expectedPOItemStatus: models.PurchaseOrderItemStatusPartiallyDelivered,
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
								ProductID:        &testProducts[0].ID,
								SupplierID:       &testSuppliers[0].ID,
								UnitID:           pkg.Ptr(uint(1)),
								Quantity:         testCase.orderedQuantity,
								ReceivedQuantity: testCase.deliveredQuantity,
								Status:           testCase.currentPOItemStatus,
								UnitPrice:        0,
							},
						},
					}

					err = db.WithContext(ctx).Create(&testPurchaseOrder).Error
					require.NoError(t, err, "Failed to create purchase order")
					purchaseOrderID := testPurchaseOrder.ID

					notes := uuid.New().String()
					unitPrice := float64(rand.Intn(900) + 100)
					payload := map[string]interface{}{
						"inventory_id": testInventories[0].ID,
						"notes":        notes,
						"items": []map[string]interface{}{
							{
								"product_id":  testProducts[0].ID,
								"supplier_id": testSuppliers[0].ID,
								"unit_id":     testUnits[0].ID,
								"quantity":    testCase.updatedQuantity,
								"unit_price":  unitPrice,
							},
						},
					}
					urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", purchaseOrderID)
					resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, payload)
					require.NoError(t, err)
					defer resp.Body.Close()
					assert.Equal(t, 200, resp.StatusCode)
					var purchaseOrderResp map[string]interface{}
					err = json.NewDecoder(resp.Body).Decode(&purchaseOrderResp)
					require.NoError(t, err)
					assert.Equal(t, notes, purchaseOrderResp["notes"])
					items := purchaseOrderResp["items"].([]interface{})
					assert.Equal(t, len(items), 1)
					firstItem := items[0].(map[string]interface{})

					// After conversion, unit_id is converted to base unit (1)
					// Test Unit 2 has BaseUnitID: 1, so it's converted to base unit ID 1
					// Since ConversionFactor defaults to 1, quantity and unit_price remain the same
					expectedUnitID := uint(1) // Base unit ID

					assert.Equal(t, unitPrice, firstItem["unit_price"].(float64))
					assert.Equal(t, testCase.updatedQuantity, int(firstItem["quantity"].(float64)))
					assert.Equal(t, uint(testProducts[0].ID), uint(firstItem["product_id"].(float64)))
					assert.Equal(t, uint(testSuppliers[0].ID), uint(firstItem["supplier_id"].(float64)))
					assert.Equal(t, expectedUnitID, uint(firstItem["unit_id"].(float64)))
					assert.Equal(t, string(testCase.expectedPOStatus), purchaseOrderResp["status"])
					assert.Equal(t, string(testCase.expectedPOItemStatus), firstItem["status"])
				})
			}
		}
	})

	t.Run("should remove purchase order items if received quantity is 0, then add new item", func(t *testing.T) {
		currentId++
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		// 1 item in database
		testPurchaseOrder := models.PurchaseOrder{
			Base:        models.Base{ID: currentId},
			OrderNumber: uuid.New().String(),
			Status:      models.PurchaseOrderStatusOrderPlaced,
			InventoryID: &testInventories[0].ID,
			Items: []*models.PurchaseOrderItem{
				{
					ProductID:  &testProducts[0].ID,
					SupplierID: &testSuppliers[0].ID,
					UnitID:     pkg.Ptr(uint(1)),
					Quantity:   50,
					ReceivedQuantity: 0,
					Status:     models.PurchaseOrderItemStatusAwaitingDelivery,
					UnitPrice:  0,
				},
			},
		}

		err = db.WithContext(ctx).Create(&testPurchaseOrder).Error
		require.NoError(t, err, "Failed to create purchase order")
		purchaseOrderID := testPurchaseOrder.ID

		// Remove existing item and add 1 item
		urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", purchaseOrderID)
		payload := map[string]interface{}{
			"inventory_id": testInventories[0].ID,
			"notes":        uuid.New().String(),
			"items": []map[string]interface{}{
				{
					"product_id":  testProducts[1].ID,
					"supplier_id": testSuppliers[0].ID,
					"unit_id":     testUnits[0].ID,
					"quantity":    100,
				},
			},
		}
		resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, payload)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		// Verify response
		var response models.PurchaseOrder
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, models.PurchaseOrderStatusOrderPlaced, response.Status)
		responseItems := response.Items
		assert.Equal(t, len(responseItems), 1)
		assert.Equal(t, uint(testProducts[1].ID), *responseItems[0].ProductID)
		assert.Equal(t, 100, int(responseItems[0].Quantity))
		assert.Equal(t, 0, int(responseItems[0].ReceivedQuantity))
		assert.Equal(t, models.PurchaseOrderItemStatusAwaitingDelivery, responseItems[0].Status)

		// Verify database
		var purchaseOrder models.PurchaseOrder
		err = suite.sharedTestContainer.DB.WithContext(ctx).Preload("Items").First(&purchaseOrder, "id = ?", purchaseOrderID).Error
		require.NoError(t, err)
		assert.Equal(t, models.PurchaseOrderStatusOrderPlaced, purchaseOrder.Status)
		items := purchaseOrder.Items
		assert.Equal(t, 1, len(items))
		assert.Equal(t, uint(testProducts[1].ID), *items[0].ProductID)
		assert.Equal(t, 100, int(items[0].Quantity))
		assert.Equal(t, 0, int(items[0].ReceivedQuantity))
		assert.Equal(t, models.PurchaseOrderItemStatusAwaitingDelivery, items[0].Status)
	})

	t.Run("should not remove purchase order items if received quantity is not 0", func(t *testing.T) {
		currentId++
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		// 1 item in database
		testPurchaseOrder := models.PurchaseOrder{
			Base:        models.Base{ID: currentId},
			OrderNumber: uuid.New().String(),
			Status:      models.PurchaseOrderStatusOrderPlaced,
			InventoryID: &testInventories[0].ID,
			Items: []*models.PurchaseOrderItem{
				{
					ProductID:        &testProducts[0].ID,
					SupplierID:       &testSuppliers[0].ID,
					UnitID:           pkg.Ptr(uint(1)),
					Quantity:         50,
					ReceivedQuantity: 30,
					Status:           models.PurchaseOrderItemStatusPartiallyDelivered,
					UnitPrice:        0,
				},
			},
		}

		err = db.WithContext(ctx).Create(&testPurchaseOrder).Error
		require.NoError(t, err, "Failed to create purchase order")
		purchaseOrderID := testPurchaseOrder.ID

		// Remove existing item and add 1 item
		urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", purchaseOrderID)
		payload := map[string]interface{}{
			"inventory_id": testInventories[0].ID,
			"notes":        uuid.New().String(),
			"items": []map[string]interface{}{
				{
					"product_id":  testProducts[1].ID,
					"supplier_id": testSuppliers[0].ID,
					"unit_id":     testUnits[0].ID,
					"quantity":    100,
				},
			},
		}
		resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, payload)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 400, resp.StatusCode)
		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)
		assert.Equal(t, "cannot delete item with received quantity 30", errorResp["error"])
	})

	t.Run("should not update purchase order", func(t *testing.T) {
		roles := []models.UserRole{models.RoleStaff, models.RoleBotForm}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				currentId++
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)
				payload := map[string]interface{}{
					"inventory_id": testInventories[0].ID,
					"notes":        uuid.New().String(),
					"items": []map[string]interface{}{
						{
							"product_id":  testProducts[0].ID,
							"supplier_id": testSuppliers[0].ID,
							"unit_id":     testUnits[0].ID,
							"quantity":    100,
							"unit_price":  0,
						},
					},
				}
				testPurchaseOrder := models.PurchaseOrder{
					Base:        models.Base{ID: currentId},
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusOrderPlaced,
					InventoryID: &testInventories[0].ID,
					Items: []*models.PurchaseOrderItem{
						{
							ProductID:        &testProducts[0].ID,
							SupplierID:       &testSuppliers[0].ID,
							UnitID:           pkg.Ptr(uint(1)),
							Quantity:         50,
							ReceivedQuantity: 0,
							Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
							UnitPrice:        0,
						},
					},
				}

				err = db.WithContext(ctx).Create(&testPurchaseOrder).Error
				require.NoError(t, err, "Failed to create purchase order")
				purchaseOrderID := testPurchaseOrder.ID

				urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", purchaseOrderID)
				resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, payload)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 403, resp.StatusCode)
				var errorResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&errorResp)
				require.NoError(t, err)
				assert.Equal(t, "Access denied: "+string(role)+" role cannot update purchase-orders", errorResp["error"])
			})
		}

		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)
		t.Run("when purchase order is not found", func(t *testing.T) {
			currentId++
			urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", currentId)
			payload := map[string]interface{}{
				"inventory_id": testInventories[0].ID,
				"notes":        uuid.New().String(),
				"items": []map[string]interface{}{
					{
						"product_id":  testProducts[0].ID,
						"supplier_id": testSuppliers[0].ID,
						"unit_id":     testUnits[0].ID,
						"quantity":    100,
					},
				},
			}
			resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, payload)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, 404, resp.StatusCode)
			var errorResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&errorResp)
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("purchase order with ID %d not found", currentId), errorResp["error"])
		})

		t.Run("when no items are provided", func(t *testing.T) {
			currentId++
			urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", currentId)
			payload := map[string]interface{}{
				"inventory_id": testInventories[0].ID,
				"notes":        uuid.New().String(),
			}
			resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, payload)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, 400, resp.StatusCode)
			var errorResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&errorResp)
			require.NoError(t, err)
			assert.Equal(t, "Validation failed: validation failed", errorResp["error"])
		})
	})
}
