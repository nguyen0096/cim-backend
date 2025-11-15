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
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *ComponentTestSuite) TestCreatePurchaseOrder() {
	t := suite.T()
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
	db := suite.sharedTestContainer.DB
	testSuppliers := []models.Supplier{
		{
			Name: "Test Supplier 1",
		},
		{
			Name: "Test Supplier 2",
		},
	}
	err := db.WithContext(ctx).Create(&testSuppliers).Error
	require.NoError(t, err, "Failed to create suppliers")

	testBaseUnit := models.Unit{
		Name:             "Test Base Unit",
		Symbol:           "BU",
		UnitType:         "length",
		ConversionFactor: 1,
	}
	err = db.WithContext(ctx).Create(&testBaseUnit).Error
	require.NoError(t, err, "Failed to create base unit")
	testDerivedUnit := models.Unit{
		Name:             "Test Derived Unit",
		Symbol:           "DU",
		UnitType:         "length",
		ConversionFactor: 10,
		BaseUnitID:       pkg.Ptr(testBaseUnit.ID),
	}
	err = db.WithContext(ctx).Create(&testDerivedUnit).Error
	require.NoError(t, err, "Failed to create derived unit")
	testProducts := []models.Product{
		{
			Name:   "Test Product 1",
			UnitID: testBaseUnit.ID,
		},
		{
			Name:   "Test Product 2",
			UnitID: testBaseUnit.ID,
		},
	}
	err = db.WithContext(ctx).Create(&testProducts).Error
	require.NoError(t, err, "Failed to create products")
	testInventories := []models.Inventory{
		{
			Name: "Test Inventory 1",
		},
	}
	err = db.WithContext(ctx).Create(&testInventories).Error
	require.NoError(t, err, "Failed to create inventories")

	purchaseOrderData := map[string]interface{}{
		"inventory_id": 1,
		"items": []map[string]interface{}{
			{
				"product_id":  testProducts[0].ID,
				"supplier_id": testSuppliers[0].ID,
				"quantity":    1,
				"unit_id":     testBaseUnit.ID,
				"unit_price":  100,
			},
			{
				"product_id":  testProducts[1].ID,
				"supplier_id": testSuppliers[1].ID,
				"quantity":    1,
				"unit_id":     testDerivedUnit.ID,
				"unit_price":  1000,
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
				assert.True(t, strings.HasPrefix(purchaseOrderResp["order_number"].(string), "PO-"+time.Now().Format("060102-1504")), purchaseOrderResp["order_number"])
				assert.Equal(t, 1, int(purchaseOrderResp["inventory_id"].(float64)))
				assert.Equal(t, "order_placed", purchaseOrderResp["status"])
				assert.Equal(t, "1100", purchaseOrderResp["total_amount"].(string))
				assert.Equal(t, "Test purchase order", purchaseOrderResp["notes"])
				assert.Equal(t, 2, len(purchaseOrderResp["items"].([]interface{})))
				items := purchaseOrderResp["items"].([]interface{})
				firstItem := items[0].(map[string]interface{})
				secondItem := items[1].(map[string]interface{})
				assert.Equal(t, testProducts[0].ID, uint(firstItem["product_id"].(float64)))
				assert.Equal(t, testSuppliers[0].ID, uint(firstItem["supplier_id"].(float64)))
				assert.Equal(t, testBaseUnit.ID, uint(firstItem["unit_id"].(float64)))
				assert.Equal(t, "1", firstItem["quantity"].(string))
				assert.Equal(t, float64(100), firstItem["unit_price"].(float64))
				assert.Equal(t, testProducts[1].ID, uint(secondItem["product_id"].(float64)))
				assert.Equal(t, testSuppliers[1].ID, uint(secondItem["supplier_id"].(float64)))
				assert.Equal(t, testBaseUnit.ID, uint(secondItem["unit_id"].(float64))) // converted to base unit
				assert.Equal(t, "10", secondItem["quantity"].(string))
				assert.Equal(t, 100, int(secondItem["unit_price"].(float64)))

				var purchaseOrder models.PurchaseOrder
				err = suite.sharedTestContainer.DB.WithContext(ctx).First(&purchaseOrder, "id = ?", purchaseOrderResp["id"]).Error
				require.NoError(t, err)
				assert.True(t, strings.HasPrefix(purchaseOrder.OrderNumber, "PO-"+time.Now().Format("060102-150405")), purchaseOrder.OrderNumber)
				assert.Equal(t, testInventories[0].ID, *purchaseOrder.InventoryID)
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
		purchaseOrder.CalculateTotalAmount()
		assert.Equal(t, "200", purchaseOrder.TotalAmount.String())
		items := purchaseOrder.Items
		assert.Equal(t, 1, len(items))
		assert.Equal(t, uint(1), *items[0].ProductID)
		assert.Equal(t, uint(1), *items[0].SupplierID)
		assert.Equal(t, uint(1), *items[0].UnitID)
		quantityFloat, _ := items[0].Quantity.Float64()
		assert.Equal(t, 40, int(quantityFloat))
		assert.Equal(t, 5.0, items[0].UnitPrice)
		assert.Equal(t, "200", items[0].TotalAmount.String())
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
							Quantity:   decimal.NewFromInt(1),
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
							Quantity:   decimal.NewFromInt(1),
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

	testBaseUnit := models.Unit{
		Name:             fmt.Sprintf("Test Base Unit %s", uuid.New().String()),
		Symbol:           "BU",
		UnitType:         "length",
		ConversionFactor: 1,
	}
	err := db.WithContext(ctx).Create(&testBaseUnit).Error
	require.NoError(t, err, "Failed to create base unit")
	testDerivedUnit := models.Unit{
		Name:             fmt.Sprintf("Test Derived Unit %s", uuid.New().String()),
		Symbol:           "DU",
		UnitType:         "length",
		ConversionFactor: 10,
		BaseUnitID:       pkg.Ptr(testBaseUnit.ID),
	}
	err = db.WithContext(ctx).Create(&testDerivedUnit).Error
	require.NoError(t, err, "Failed to create derived unit")

	testSuppliers := []models.Supplier{
		{
			Name: fmt.Sprintf("Test Supplier 1 %s", uuid.New().String()),
		},
	}
	err = db.WithContext(ctx).Create(&testSuppliers).Error
	require.NoError(t, err, "Failed to create suppliers")

	testProducts := []models.Product{
		{
			Name:   fmt.Sprintf("Test Product 1 %s", uuid.New().String()),
			UnitID: testBaseUnit.ID,
		},
		{
			Name:   fmt.Sprintf("Test Product 2 %s", uuid.New().String()),
			UnitID: testBaseUnit.ID,
		},
	}
	err = db.WithContext(ctx).Create(&testProducts).Error
	require.NoError(t, err, "Failed to create products")

	testInventories := []models.Inventory{
		{
			Name: fmt.Sprintf("Test Inventory 1 %s", uuid.New().String()),
		},
	}
	err = db.WithContext(ctx).Create(&testInventories).Error
	require.NoError(t, err, "Failed to create inventories")

	defer pkg.CleanUp(t, func() error {
		var purchaseOrderIDs []uint
		db.WithContext(ctx).Model(&models.PurchaseOrder{}).
			Where("inventory_id = ?", testInventories[0].ID).
			Pluck("id", &purchaseOrderIDs)
		if len(purchaseOrderIDs) > 0 {
			db.WithContext(ctx).Where("purchase_order_id IN ?", purchaseOrderIDs).Delete(&models.PurchaseOrderItem{})
			db.WithContext(ctx).Where("id IN ?", purchaseOrderIDs).Delete(&models.PurchaseOrder{})
		}
		db.WithContext(ctx).Where("id IN ?", []uint{testInventories[0].ID}).Delete(&models.Inventory{})
		db.WithContext(ctx).Where("id IN ?", []uint{testProducts[0].ID, testProducts[1].ID}).Delete(&models.Product{})
		db.WithContext(ctx).Where("id IN ?", []uint{testSuppliers[0].ID}).Delete(&models.Supplier{})
		db.WithContext(ctx).Where("id IN ?", []uint{testBaseUnit.ID, testDerivedUnit.ID}).Delete(&models.Unit{})
		return nil
	})

	currentId := uint(1000)

	t.Run("should receive purchase order", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant, models.RoleStaff}
		testCases := []struct {
			currentPOStatus       models.PurchaseOrderStatus
			currentPOItem1Status  models.PurchaseOrderItemStatus
			deliveredQuantity1    float64
			deliveredQuantity2    float64
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
				deliveredQuantity2:    100.00,
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

					// Set up initial received quantity and status
					// For test case 4 (PartiallyDelivered), items need initial received quantity
					// but we're receiving 100 which equals the quantity, so they should start with 0
					// The PO status is PartiallyDelivered but items start fresh for this test
					var item1ReceivedQuantity, item2ReceivedQuantity decimal.Decimal
					var item1Status, item2Status models.PurchaseOrderItemStatus
					item1ReceivedQuantity = decimal.Zero
					item2ReceivedQuantity = decimal.Zero
					item1Status = models.PurchaseOrderItemStatusAwaitingDelivery
					item2Status = models.PurchaseOrderItemStatusAwaitingDelivery

					testPurchaseOrder := models.PurchaseOrder{
						Base:        models.Base{ID: currentId},
						OrderNumber: uuid.New().String(),
						Status:      testCase.currentPOStatus,
						InventoryID: &testInventories[0].ID,
						Items: []*models.PurchaseOrderItem{
							{
								ProductID:        &testProducts[0].ID,
								SupplierID:       &testSuppliers[0].ID,
								UnitID:           &unitID1,
								Quantity:         decimal.NewFromInt(100),
								ReceivedQuantity: item1ReceivedQuantity,
								Status:           item1Status,
							},
							{
								ProductID:        &testProducts[1].ID,
								SupplierID:       &testSuppliers[0].ID,
								UnitID:           &unitID2,
								Quantity:         decimal.NewFromInt(100),
								ReceivedQuantity: item2ReceivedQuantity,
								Status:           item2Status,
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

	t.Run("should not receive purchase order if item quantity is smaller than received quantity", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		testPurchaseOrder := models.PurchaseOrder{
			OrderNumber: uuid.New().String(),
			Status:      models.PurchaseOrderStatusOrderPlaced,
			InventoryID: &testInventories[0].ID,
			Items: []*models.PurchaseOrderItem{
				{
					ProductID:        &testProducts[0].ID,
					SupplierID:       &testSuppliers[0].ID,
					UnitID:           &testBaseUnit.ID,
					Quantity:         decimal.NewFromInt(100),
					ReceivedQuantity: decimal.NewFromInt(50),
				},
			},
		}

		err = db.WithContext(ctx).Create(&testPurchaseOrder).Error
		require.NoError(t, err, "Failed to create purchase order")

		payload := map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"id":                testPurchaseOrder.Items[0].ID,
					"received_quantity": 100,
				},
			},
		}
		urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d/receive", testPurchaseOrder.ID)
		resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, payload)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 400, resp.StatusCode)
		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("received quantity 100 exceeds remaining quantity 50 for item ID %d", testPurchaseOrder.Items[0].ID), errorResp["error"])
	})

	t.Run("should not receive purchase order if decimal places is larger than unit decimal places", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		testPurchaseOrder := models.PurchaseOrder{
			OrderNumber: uuid.New().String(),
			Status:      models.PurchaseOrderStatusOrderPlaced,
			InventoryID: &testInventories[0].ID,
			Items: []*models.PurchaseOrderItem{
				{
					ProductID:  &testProducts[0].ID,
					SupplierID: &testSuppliers[0].ID,
					UnitID:     &testBaseUnit.ID,
					Quantity:   decimal.NewFromInt(100),
				},
			},
		}

		err = db.WithContext(ctx).Create(&testPurchaseOrder).Error
		require.NoError(t, err, "Failed to create purchase order")

		payload := map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"id":                testPurchaseOrder.Items[0].ID,
					"received_quantity": "99.11",
				},
			},
		}
		urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d/receive", testPurchaseOrder.ID)
		resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, payload)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 400, resp.StatusCode)
		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)
		assert.Equal(t, "decimal places must be less than or equal to 0", errorResp["error"])
	})
}

func (suite *ComponentTestSuite) TestUpdatePurchaseOrder() {
	t := suite.T()
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
	db := suite.sharedTestContainer.DB
	testSuppliers := []models.Supplier{
		{
			Name: fmt.Sprintf("Test Supplier %s", uuid.New().String()),
		},
	}
	err := db.WithContext(ctx).Create(&testSuppliers).Error
	require.NoError(t, err, "Failed to create suppliers")

	testBaseUnit := models.Unit{
		Name:             fmt.Sprintf("Test Base Unit %s", uuid.New().String()),
		Symbol:           "BU",
		UnitType:         "length",
		ConversionFactor: 1,
	}
	err = db.WithContext(ctx).Create(&testBaseUnit).Error
	require.NoError(t, err, "Failed to create base unit")
	testDerivedUnit := models.Unit{
		Name:             fmt.Sprintf("Test Derived Unit %s", uuid.New().String()),
		Symbol:           "DU",
		UnitType:         "length",
		ConversionFactor: 2,
		BaseUnitID:       pkg.Ptr(testBaseUnit.ID),
	}
	err = db.WithContext(ctx).Create(&testDerivedUnit).Error
	require.NoError(t, err, "Failed to create derived unit")
	testDerivedUnit2 := models.Unit{
		Name:             fmt.Sprintf("Test Derived Unit 2 %s", uuid.New().String()),
		Symbol:           "DU2",
		UnitType:         "length",
		ConversionFactor: 10,
		BaseUnitID:       pkg.Ptr(testBaseUnit.ID),
	}
	err = db.WithContext(ctx).Create(&testDerivedUnit2).Error
	require.NoError(t, err, "Failed to create derived unit 2")

	testProducts := []models.Product{
		{
			Name:   fmt.Sprintf("Test Product 1 %s", uuid.New().String()),
			UnitID: testBaseUnit.ID,
		},
		{
			Name:   fmt.Sprintf("Test Product 2 %s", uuid.New().String()),
			UnitID: testDerivedUnit.ID,
		},
	}

	err = db.WithContext(ctx).Create(&testProducts).Error
	require.NoError(t, err, "Failed to create products")

	testInventories := []models.Inventory{
		{
			Name: fmt.Sprintf("Test Inventory 1 %s", uuid.New().String()),
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
			orderedQuantity      float64
			deliveredQuantity    float64
			updatedQuantity      float64
			updatedUnit          models.Unit
			expectedPOStatus     models.PurchaseOrderStatus
			expectedPOItemStatus models.PurchaseOrderItemStatus
		}{
			{
				currentPOStatus:      models.PurchaseOrderStatusOrderPlaced,
				currentPOItemStatus:  models.PurchaseOrderItemStatusAwaitingDelivery,
				orderedQuantity:      50,
				deliveredQuantity:    0,
				updatedQuantity:      100,
				updatedUnit:          testBaseUnit,
				expectedPOStatus:     models.PurchaseOrderStatusOrderPlaced,
				expectedPOItemStatus: models.PurchaseOrderItemStatusAwaitingDelivery,
			},
			{
				currentPOStatus:      models.PurchaseOrderStatusPartiallyDelivered,
				currentPOItemStatus:  models.PurchaseOrderItemStatusPartiallyDelivered,
				orderedQuantity:      50,
				deliveredQuantity:    20,
				updatedQuantity:      100,
				updatedUnit:          testBaseUnit,
				expectedPOStatus:     models.PurchaseOrderStatusPartiallyDelivered,
				expectedPOItemStatus: models.PurchaseOrderItemStatusPartiallyDelivered,
			},
			{
				currentPOStatus:      models.PurchaseOrderStatusFullyDelivered,
				currentPOItemStatus:  models.PurchaseOrderItemStatusDelivered,
				orderedQuantity:      50,
				deliveredQuantity:    50,
				updatedQuantity:      100,
				updatedUnit:          testBaseUnit,
				expectedPOStatus:     models.PurchaseOrderStatusPartiallyDelivered,
				expectedPOItemStatus: models.PurchaseOrderItemStatusPartiallyDelivered,
			},
			{
				currentPOStatus:      models.PurchaseOrderStatusPartiallyDelivered,
				currentPOItemStatus:  models.PurchaseOrderItemStatusPartiallyDelivered,
				orderedQuantity:      50,
				deliveredQuantity:    30,
				updatedQuantity:      30,
				updatedUnit:          testBaseUnit,
				expectedPOStatus:     models.PurchaseOrderStatusFullyDelivered,
				expectedPOItemStatus: models.PurchaseOrderItemStatusDelivered,
			},
			// Change unit
			{
				currentPOStatus:      models.PurchaseOrderStatusPartiallyDelivered,
				currentPOItemStatus:  models.PurchaseOrderItemStatusPartiallyDelivered,
				orderedQuantity:      50,
				deliveredQuantity:    40,
				updatedQuantity:      20,
				updatedUnit:          testDerivedUnit,
				expectedPOStatus:     models.PurchaseOrderStatusFullyDelivered,
				expectedPOItemStatus: models.PurchaseOrderItemStatusDelivered,
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
								UnitID:           pkg.Ptr(testCase.updatedUnit.ID),
								Quantity:         decimal.NewFromFloat(testCase.orderedQuantity),
								ReceivedQuantity: decimal.NewFromFloat(testCase.deliveredQuantity),
								Status:           testCase.currentPOItemStatus,
								UnitPrice:        0,
							},
						},
					}

					err = db.WithContext(ctx).Create(&testPurchaseOrder).Error
					require.NoError(t, err, "Failed to create purchase order")
					purchaseOrderID := testPurchaseOrder.ID

					notes := uuid.New().String()
					unitPrice := 1000
					payload := map[string]interface{}{
						"inventory_id": testInventories[0].ID,
						"notes":        notes,
						"items": []map[string]interface{}{
							{
								"product_id":  testProducts[0].ID,
								"supplier_id": testSuppliers[0].ID,
								"unit_id":     testCase.updatedUnit.ID,
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

					// After conversion, unit_id is converted to base unit
					// testDerivedUnit has BaseUnitID: testBaseUnit.ID, so it's converted to base unit
					// Since ConversionFactor is 1, quantity and unit_price remain the same
					expectedUnitID := testBaseUnit.ID

					assert.Equal(t, float64(unitPrice)/testCase.updatedUnit.ConversionFactor, firstItem["unit_price"].(float64))
					// Quantity is serialized as string in JSON
					expectedQuantityStr := fmt.Sprintf("%.0f", testCase.updatedQuantity*testCase.updatedUnit.ConversionFactor)
					assert.Equal(t, expectedQuantityStr, firstItem["quantity"].(string))
					assert.Equal(t, uint(testProducts[0].ID), uint(firstItem["product_id"].(float64)))
					assert.Equal(t, uint(testSuppliers[0].ID), uint(firstItem["supplier_id"].(float64)))
					assert.Equal(t, expectedUnitID, uint(firstItem["unit_id"].(float64)))
					assert.Equal(t, string(testCase.expectedPOStatus), purchaseOrderResp["status"])
					assert.Equal(t, string(testCase.expectedPOItemStatus), firstItem["status"].(string))
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
					ProductID:        &testProducts[0].ID,
					SupplierID:       &testSuppliers[0].ID,
					UnitID:           pkg.Ptr(testBaseUnit.ID),
					Quantity:         decimal.NewFromInt(50),
					ReceivedQuantity: decimal.Zero,
					Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
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
					"unit_id":     testDerivedUnit.ID,
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
		responseQuantity, _ := responseItems[0].Quantity.Float64()
		assert.Equal(t, 100, int(responseQuantity))
		responseReceived, _ := responseItems[0].ReceivedQuantity.Float64()
		assert.Equal(t, 0, int(responseReceived))
		assert.Equal(t, models.PurchaseOrderItemStatusAwaitingDelivery, responseItems[0].Status)

		// Verify database
		var purchaseOrder models.PurchaseOrder
		err = suite.sharedTestContainer.DB.WithContext(ctx).Preload("Items").First(&purchaseOrder, "id = ?", purchaseOrderID).Error
		require.NoError(t, err)
		assert.Equal(t, models.PurchaseOrderStatusOrderPlaced, purchaseOrder.Status)
		items := purchaseOrder.Items
		assert.Equal(t, 1, len(items))
		assert.Equal(t, uint(testProducts[1].ID), *items[0].ProductID)
		itemsQuantity, _ := items[0].Quantity.Float64()
		assert.Equal(t, 100, int(itemsQuantity))
		itemsReceived, _ := items[0].ReceivedQuantity.Float64()
		assert.Equal(t, 0, int(itemsReceived))
		assert.Equal(t, models.PurchaseOrderItemStatusAwaitingDelivery, items[0].Status)
	})

	t.Run("should update quantity & total amount when derived unit is updated", func(t *testing.T) {
		currentId++
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)

		testPurchaseOrder := models.PurchaseOrder{
			Base:        models.Base{ID: currentId},
			OrderNumber: uuid.New().String(),
			Status:      models.PurchaseOrderStatusOrderPlaced,
			InventoryID: &testInventories[0].ID,
			Items: []*models.PurchaseOrderItem{
				{
					ProductID:        &testProducts[0].ID,
					SupplierID:       &testSuppliers[0].ID,
					UnitID:           pkg.Ptr(testBaseUnit.ID),
					Quantity:         decimal.NewFromInt(50),
					ReceivedQuantity: decimal.NewFromInt(0),
					Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
					UnitPrice:        0,
				},
			},
		}

		err = db.WithContext(ctx).Create(&testPurchaseOrder).Error
		require.NoError(t, err, "Failed to create purchase order")
		purchaseOrderID := testPurchaseOrder.ID

		urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", purchaseOrderID)
		// Use derived unit (testDerivedUnit2 has conversion factor 10, base unit ID testBaseUnit.ID)
		derivedUnitID := testDerivedUnit2.ID
		newQuantity := 10
		derivedUnitPrice := 100.0
		// Expected converted values: quantity = 10 * 10 = 100, unit_price = 100 / 10 = 10, base unit ID = testBaseUnit.ID
		expectedBaseQuantity := decimal.NewFromInt(100) // float64(newQuantity) * testDerivedUnit2.ConversionFactor
		expectedBaseUnitPrice := 10.0                   // 100 / 10 = 10
		expectedBaseUnitID := testBaseUnit.ID
		expectedTotalAmount := decimal.NewFromInt(1000) // 100 * 10

		payload := map[string]interface{}{
			"inventory_id": testInventories[0].ID,
			"notes":        uuid.New().String(),
			"items": []map[string]interface{}{
				{
					"product_id":  testProducts[0].ID,
					"supplier_id": testSuppliers[0].ID,
					"unit_id":     derivedUnitID,
					"quantity":    newQuantity,
					"unit_price":  derivedUnitPrice,
				},
			},
		}
		resp, err := helpers.MakeRequest(t, "PUT", suite.sharedTestContainer.BaseURL+urlPath, token, payload)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		// Verify Response Body - should show converted base unit values
		var purchaseOrderResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&purchaseOrderResp)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		// Total amount from JSON response is a string representation of decimal
		totalAmountStr, ok := purchaseOrderResp["total_amount"].(string)
		if ok {
			assert.Equal(t, expectedTotalAmount.String(), totalAmountStr)
		} else {
			// If it's a float64 (for backward compatibility)
			totalAmountFloat, _ := purchaseOrderResp["total_amount"].(float64)
			assert.Equal(t, expectedTotalAmount.InexactFloat64(), totalAmountFloat)
		}
		respItems := purchaseOrderResp["items"].([]interface{})
		firstItem := respItems[0].(map[string]interface{})
		assert.Equal(t, expectedBaseUnitPrice, firstItem["unit_price"].(float64))
		// Quantity is serialized as string in JSON
		assert.Equal(t, expectedBaseQuantity.String(), firstItem["quantity"].(string))
		assert.Equal(t, expectedBaseUnitID, uint(firstItem["unit_id"].(float64)))
		assert.Equal(t, string(models.PurchaseOrderStatusOrderPlaced), purchaseOrderResp["status"].(string))

		// Verify Database - should store base unit values
		var purchaseOrder models.PurchaseOrder
		err = suite.sharedTestContainer.DB.WithContext(ctx).Preload("Items").First(&purchaseOrder, "id = ?", purchaseOrderID).Error
		require.NoError(t, err)
		assert.Equal(t, expectedTotalAmount.String(), purchaseOrder.CalculateTotalAmount().String())
		items := purchaseOrder.Items
		assert.Equal(t, 1, len(items))
		assert.Equal(t, uint(testProducts[0].ID), *items[0].ProductID)
		assert.Equal(t, expectedBaseQuantity.String(), items[0].Quantity.String())
		itemsReceived, _ := items[0].ReceivedQuantity.Float64()
		assert.Equal(t, items[0].Quantity.InexactFloat64(), itemsReceived)
		assert.Equal(t, models.PurchaseOrderItemStatusAwaitingDelivery, items[0].Status)
		assert.Equal(t, expectedBaseUnitID, *items[0].UnitID)
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
					UnitID:           pkg.Ptr(testBaseUnit.ID),
					Quantity:         decimal.NewFromInt(50),
					ReceivedQuantity: decimal.NewFromInt(30),
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
					"unit_id":     testDerivedUnit.ID,
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
							"unit_id":     testDerivedUnit.ID,
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
							UnitID:           pkg.Ptr(testBaseUnit.ID),
							Quantity:         decimal.NewFromInt(50),
							ReceivedQuantity: decimal.Zero,
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
						"unit_id":     testDerivedUnit.ID,
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

func (suite *ComponentTestSuite) TestGetPurchaseOrder() {
	t := suite.T()
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
	db := suite.sharedTestContainer.DB
	testInventories := []models.Inventory{
		{
			Name: fmt.Sprintf("Test Inventory 1 %s", uuid.New().String()),
		},
	}
	err := db.WithContext(ctx).Create(&testInventories).Error
	require.NoError(t, err, "Failed to create inventories")
	testProducts := []models.Product{
		{
			Name:   fmt.Sprintf("Test Product 1 %s", uuid.New().String()),
			UnitID: 1,
		},
	}
	err = db.WithContext(ctx).Create(&testProducts).Error
	require.NoError(t, err, "Failed to create products")
	testSuppliers := []models.Supplier{
		{
			Name: fmt.Sprintf("Test Supplier 1 %s", uuid.New().String()),
		},
	}
	err = db.WithContext(ctx).Create(&testSuppliers).Error
	require.NoError(t, err, "Failed to create suppliers")
	testBaseUnit := models.Unit{
		Name:             fmt.Sprintf("Test Base Unit %s", uuid.New().String()),
		Symbol:           "BU",
		UnitType:         "length",
		ConversionFactor: 1,
	}
	err = db.WithContext(ctx).Create(&testBaseUnit).Error
	require.NoError(t, err, "Failed to create base unit")
	testDerivedUnit := models.Unit{
		Name:             fmt.Sprintf("Test Derived Unit %s", uuid.New().String()),
		Symbol:           "DU",
		UnitType:         "length",
		ConversionFactor: 10,
		BaseUnitID:       pkg.Ptr(testBaseUnit.ID),
	}
	err = db.WithContext(ctx).Create(&testDerivedUnit).Error
	require.NoError(t, err, "Failed to create derived unit")
	testDerivedUnit2 := models.Unit{
		Name:             fmt.Sprintf("Test Derived Unit 2 %s", uuid.New().String()),
		Symbol:           "DU2",
		UnitType:         "length",
		ConversionFactor: 100,
		BaseUnitID:       pkg.Ptr(testDerivedUnit.ID),
	}
	err = db.WithContext(ctx).Create(&testDerivedUnit2).Error
	require.NoError(t, err, "Failed to create derived unit 2")

	t.Run("should get purchase order when item's unit is a base unit", func(t *testing.T) {
		_, token, err := suite.CreateUniqueEmailAndToken(models.RoleAdmin)
		require.NoError(t, err)
		testPurchaseOrder := models.PurchaseOrder{
			OrderNumber: uuid.New().String(),
			Status:      models.PurchaseOrderStatusOrderPlaced,
			InventoryID: &testInventories[0].ID,
			Items: []*models.PurchaseOrderItem{
				{
					ProductID:        &testProducts[0].ID,
					SupplierID:       &testSuppliers[0].ID,
					UnitID:           pkg.Ptr(testBaseUnit.ID),
					Quantity:         decimal.NewFromInt(100),
					ReceivedQuantity: decimal.Zero,
					Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
					UnitPrice:        1000,
				},
			},
		}
		err = db.WithContext(ctx).Create(&testPurchaseOrder).Error
		require.NoError(t, err, "Failed to create purchase order")
		purchaseOrderID := testPurchaseOrder.ID
	
		urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", purchaseOrderID)
		resp, err := helpers.MakeRequest(t, "GET", suite.sharedTestContainer.BaseURL+urlPath, token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
		var purchaseOrderResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&purchaseOrderResp)
		require.NoError(t, err)
		assert.Equal(t, testPurchaseOrder.OrderNumber, purchaseOrderResp["order_number"])
		assert.Equal(t, *testPurchaseOrder.InventoryID, uint(purchaseOrderResp["inventory_id"].(float64)))
		assert.Equal(t, string(testPurchaseOrder.Status), purchaseOrderResp["status"])
		assert.Equal(t, testPurchaseOrder.CalculateTotalAmount().String(), purchaseOrderResp["total_amount"].(string))
		assert.Equal(t, 1, len(purchaseOrderResp["items"].([]interface{})))
		items := purchaseOrderResp["items"].([]interface{})
		POItemResp := items[0].(map[string]interface{})
		assert.Equal(t, testProducts[0].ID, uint(POItemResp["product_id"].(float64)))
		assert.Equal(t, testSuppliers[0].ID, uint(POItemResp["supplier_id"].(float64)))
		assert.Equal(t, testBaseUnit.ID, uint(POItemResp["unit_id"].(float64)))
		assert.Equal(t, testPurchaseOrder.Items[0].Quantity.String(), POItemResp["quantity"].(string))
		assert.Equal(t, testPurchaseOrder.Items[0].UnitPrice, POItemResp["unit_price"].(float64))
		// Assert current unit
		unitResp := POItemResp["unit"].(map[string]interface{})
		assert.Equal(t, testBaseUnit.Name, unitResp["name"])
		assert.Equal(t, testBaseUnit.Symbol, unitResp["symbol"])
		assert.Equal(t, testBaseUnit.UnitType, unitResp["unit_type"])
		assert.Equal(t, testBaseUnit.ConversionFactor, unitResp["conversion_factor"])
		assert.Equal(t, float64(1), unitResp["conversion_factor_to_current"])
		assert.Equal(t, 2, len(unitResp["derived_units"].([]interface{})))

		// Assert derived units
		derivedUnits := unitResp["derived_units"].([]interface{})
		derivedUnitResp := derivedUnits[0].(map[string]interface{})
		assert.Equal(t, testDerivedUnit.Name, derivedUnitResp["name"])
		assert.Equal(t, testDerivedUnit.Symbol, derivedUnitResp["symbol"])
		assert.Equal(t, testDerivedUnit.UnitType, derivedUnitResp["unit_type"])
		assert.Equal(t, testDerivedUnit.ConversionFactor, derivedUnitResp["conversion_factor"])
		assert.Equal(t, float64(0.1), derivedUnitResp["conversion_factor_to_current"])

		derivedUnit2Resp := derivedUnits[1].(map[string]interface{})
		assert.Equal(t, testDerivedUnit2.Name, derivedUnit2Resp["name"])
		assert.Equal(t, testDerivedUnit2.Symbol, derivedUnit2Resp["symbol"])
		assert.Equal(t, testDerivedUnit2.UnitType, derivedUnit2Resp["unit_type"])
		assert.Equal(t, testDerivedUnit2.ConversionFactor, derivedUnit2Resp["conversion_factor"])
		assert.Equal(t, float64(0.001), derivedUnit2Resp["conversion_factor_to_current"])
	})

	t.Run("should get purchase order when item's unit is a derived unit", func(t *testing.T) {
		roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant, models.RoleStaff}
		for _, role := range roles {
			t.Run(fmt.Sprintf("When user has %s role", role), func(t *testing.T) {
				_, token, err := suite.CreateUniqueEmailAndToken(role)
				require.NoError(t, err)

				testPurchaseOrder := models.PurchaseOrder{
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusOrderPlaced,
					InventoryID: &testInventories[0].ID,
					Items: []*models.PurchaseOrderItem{
						{
							ProductID:        &testProducts[0].ID,
							SupplierID:       &testSuppliers[0].ID,
							UnitID:           pkg.Ptr(testDerivedUnit.ID),
							Quantity:         decimal.NewFromInt(100),
							ReceivedQuantity: decimal.Zero,
							Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
							UnitPrice:        1000,
						},
					},
				}
				err = db.WithContext(ctx).Create(&testPurchaseOrder).Error
				require.NoError(t, err, "Failed to create purchase order")
				purchaseOrderID := testPurchaseOrder.ID
			
				urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", purchaseOrderID)
				resp, err := helpers.MakeRequest(t, "GET", suite.sharedTestContainer.BaseURL+urlPath, token, nil)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, 200, resp.StatusCode)
				var purchaseOrderResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&purchaseOrderResp)
				require.NoError(t, err)
				assert.Equal(t, testPurchaseOrder.OrderNumber, purchaseOrderResp["order_number"])
				assert.Equal(t, *testPurchaseOrder.InventoryID, uint(purchaseOrderResp["inventory_id"].(float64)))
				assert.Equal(t, string(testPurchaseOrder.Status), purchaseOrderResp["status"])
				assert.Equal(t, testPurchaseOrder.CalculateTotalAmount().String(), purchaseOrderResp["total_amount"].(string))
				assert.Equal(t, 1, len(purchaseOrderResp["items"].([]interface{})))
				items := purchaseOrderResp["items"].([]interface{})
				POItemResp := items[0].(map[string]interface{})
				assert.Equal(t, testProducts[0].ID, uint(POItemResp["product_id"].(float64)))
				assert.Equal(t, testSuppliers[0].ID, uint(POItemResp["supplier_id"].(float64)))
				assert.Equal(t, testDerivedUnit.ID, uint(POItemResp["unit_id"].(float64)))
				assert.Equal(t, testPurchaseOrder.Items[0].Quantity.String(), POItemResp["quantity"].(string))
				assert.Equal(t, testPurchaseOrder.Items[0].UnitPrice, POItemResp["unit_price"].(float64))

				// Assert current unit
				unitResp := POItemResp["unit"].(map[string]interface{})
				assert.Equal(t, testDerivedUnit.Name, unitResp["name"])
				assert.Equal(t, testDerivedUnit.Symbol, unitResp["symbol"])
				assert.Equal(t, testDerivedUnit.UnitType, unitResp["unit_type"])
				assert.Equal(t, testDerivedUnit.ConversionFactor, unitResp["conversion_factor"])
				assert.Equal(t, float64(1), unitResp["conversion_factor_to_current"])

				// Assert derived units
				assert.Equal(t, 1, len(unitResp["derived_units"].([]interface{})))
				derivedUnits := unitResp["derived_units"].([]interface{})
				derivedUnitResp := derivedUnits[0].(map[string]interface{})
				assert.Equal(t, testDerivedUnit2.Name, derivedUnitResp["name"])
				assert.Equal(t, testDerivedUnit2.Symbol, derivedUnitResp["symbol"])
				assert.Equal(t, testDerivedUnit2.UnitType, derivedUnitResp["unit_type"])
				assert.Equal(t, testDerivedUnit2.ConversionFactor, derivedUnitResp["conversion_factor"])
				assert.Equal(t, float64(0.01), derivedUnitResp["conversion_factor_to_current"])

				// Assert base unit
				baseUnitResp := unitResp["base_unit"].(map[string]interface{})
				assert.Equal(t, testBaseUnit.Name, baseUnitResp["name"])
				assert.Equal(t, testBaseUnit.Symbol, baseUnitResp["symbol"])
				assert.Equal(t, testBaseUnit.UnitType, baseUnitResp["unit_type"])
				assert.Equal(t, testBaseUnit.ConversionFactor, baseUnitResp["conversion_factor"])
				assert.Equal(t, float64(10), baseUnitResp["conversion_factor_to_current"])
			})
		}
	})
}
