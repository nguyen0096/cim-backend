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
	"github.com/shopspring/decimal"
)

var _ = Describe("Purchase Order API", func() {
	Describe("Create Purchase Order", func() {
		var testSuppliers []*models.Supplier
		var testBaseUnit *models.Unit
		var testDerivedUnit *models.Unit
		var testProducts []*models.Product
		var testInventory *models.Inventory

		BeforeEach(func() {
			// Create suppliers
			testSuppliers = fixture.WithSuppliers(tenv.ContextfulDB(), []*models.Supplier{
				{Name: fmt.Sprintf("Test Supplier 1 %s", uuid.New().String())},
				{Name: fmt.Sprintf("Test Supplier 2 %s", uuid.New().String())},
			})

			// Create units
			testBaseUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Test Base Unit %s", uuid.New().String()),
				Symbol:           "BU",
				UnitType:         "length",
				ConversionFactor: 1,
			})
			testDerivedUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Test Derived Unit %s", uuid.New().String()),
				Symbol:           "DU",
				UnitType:         "length",
				ConversionFactor: 10,
				BaseUnitID:       pkg.Ptr(testBaseUnit.ID),
			})

			// Create products
			testProducts = fixture.WithProducts(tenv.ContextfulDB(), []*models.Product{
				{Name: fmt.Sprintf("Test Product 1 %s", uuid.New().String()), UnitID: testBaseUnit.ID},
				{Name: fmt.Sprintf("Test Product 2 %s", uuid.New().String()), UnitID: testBaseUnit.ID},
			})

			// Create inventory
			testInventory = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name: fmt.Sprintf("Test Inventory 1 %s", uuid.New().String()),
			})
		})

		Context("when user has authorized role", func() {
			It("should create purchase order with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				purchaseOrderData := map[string]interface{}{
					"inventory_id": testInventory.ID,
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

				resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders", purchaseOrderData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				purchaseOrderResp := testutil.ParseResponse(resp)
				Expect(purchaseOrderResp["id"]).NotTo(BeNil())
				Expect(purchaseOrderResp["order_number"]).To(HavePrefix("PO-" + time.Now().Format("060102-1504")))
				Expect(purchaseOrderResp["inventory_id"]).To(Equal(float64(testInventory.ID)))
				Expect(purchaseOrderResp["status"]).To(Equal("order_placed"))
				Expect(purchaseOrderResp["total_amount"]).To(Equal("1100"))
				Expect(purchaseOrderResp["notes"]).To(Equal("Test purchase order"))

				items := purchaseOrderResp["items"].([]interface{})
				Expect(items).To(HaveLen(2))

				firstItem := items[0].(map[string]interface{})
				Expect(firstItem["product_id"]).To(Equal(float64(testProducts[0].ID)))
				Expect(firstItem["supplier_id"]).To(Equal(float64(testSuppliers[0].ID)))
				Expect(firstItem["unit_id"]).To(Equal(float64(testBaseUnit.ID)))
				Expect(firstItem["quantity"]).To(Equal("1"))
				Expect(firstItem["unit_price"]).To(Equal(float64(100)))

				secondItem := items[1].(map[string]interface{})
				Expect(secondItem["product_id"]).To(Equal(float64(testProducts[1].ID)))
				Expect(secondItem["supplier_id"]).To(Equal(float64(testSuppliers[1].ID)))
				Expect(secondItem["unit_id"]).To(Equal(float64(testBaseUnit.ID))) // converted to base unit
				Expect(secondItem["quantity"]).To(Equal("10"))
				Expect(secondItem["unit_price"]).To(Equal(float64(100)))
			})

			It("should create purchase order with accountant role", func() {
				client := testutil.NewClient(tenv, models.RoleAccountant)

				purchaseOrderData := map[string]interface{}{
					"inventory_id": testInventory.ID,
					"items": []map[string]interface{}{
						{
							"product_id":  testProducts[0].ID,
							"supplier_id": testSuppliers[0].ID,
							"quantity":    1,
							"unit_id":     testBaseUnit.ID,
							"unit_price":  100,
						},
					},
					"notes": "Test purchase order",
				}

				resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders", purchaseOrderData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				purchaseOrderResp := testutil.ParseResponse(resp)
				Expect(purchaseOrderResp["status"]).To(Equal("order_placed"))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not create purchase order with staff role", func() {
				client := testutil.NewClient(tenv, models.RoleStaff)

				purchaseOrderData := map[string]interface{}{
					"inventory_id": testInventory.ID,
					"items": []map[string]interface{}{
						{
							"product_id":  testProducts[0].ID,
							"supplier_id": testSuppliers[0].ID,
							"quantity":    1,
							"unit_id":     testBaseUnit.ID,
							"unit_price":  100,
						},
					},
				}

				resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders", purchaseOrderData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot create purchase-orders", models.RoleStaff)))
			})

			It("should not create purchase order with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				purchaseOrderData := map[string]interface{}{
					"inventory_id": testInventory.ID,
					"items": []map[string]interface{}{
						{
							"product_id":  testProducts[0].ID,
							"supplier_id": testSuppliers[0].ID,
							"quantity":    1,
							"unit_id":     testBaseUnit.ID,
							"unit_price":  100,
						},
					},
				}

				resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders", purchaseOrderData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot create purchase-orders", models.RoleBotForm)))
			})
		})
	})

	Describe("Create Purchase Order with Different Units", func() {
		var testSupplier *models.Supplier
		var testUnits []*models.Unit
		var testProduct *models.Product
		var testInventory *models.Inventory

		BeforeEach(func() {
			// Create unit hierarchy
			baseUnit := fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Base Unit %s", uuid.New().String()),
				Symbol:           "bu",
				UnitType:         "general",
				Level:            1,
				ConversionFactor: 1,
			})
			unit2 := fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Derived Unit 1 %s", uuid.New().String()),
				Symbol:           "du1",
				UnitType:         "general",
				Level:            2,
				ConversionFactor: 2,
				BaseUnitID:       pkg.Ptr(baseUnit.ID),
			})
			unit3 := fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Derived Unit 2 %s", uuid.New().String()),
				Symbol:           "du2",
				UnitType:         "general",
				Level:            3,
				ConversionFactor: 4,
				BaseUnitID:       pkg.Ptr(unit2.ID),
			})
			testUnits = []*models.Unit{baseUnit, unit2, unit3}

			testSupplier = fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name: fmt.Sprintf("Test Supplier %s", uuid.New().String()),
			})

			testProduct = fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:   fmt.Sprintf("Test Product %s", uuid.New().String()),
				UnitID: baseUnit.ID,
			})

			testInventory = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name: fmt.Sprintf("Test Inventory %s", uuid.New().String()),
			})
		})

		It("should create purchase order with different units and same base unit", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			payload := map[string]interface{}{
				"inventory_id": testInventory.ID,
				"items": []map[string]interface{}{
					{
						"product_id":  testProduct.ID,
						"supplier_id": testSupplier.ID,
						"quantity":    2,
						"unit_id":     testUnits[2].ID,
						"unit_price":  100,
					},
				},
			}

			resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders", payload, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(201))

			purchaseOrderResp := testutil.ParseResponse(resp)
			Expect(purchaseOrderResp["total_amount"]).To(Equal("200"))

			items := purchaseOrderResp["items"].([]interface{})
			Expect(items).To(HaveLen(1))

			firstItem := items[0].(map[string]interface{})
			expectedQuantity := decimal.NewFromInt(2).Mul(decimal.NewFromFloat(testUnits[2].ConversionFactor * testUnits[1].ConversionFactor)).String()
			Expect(firstItem["quantity"]).To(Equal(expectedQuantity))
			Expect(firstItem["unit_id"]).To(Equal(float64(testUnits[0].ID)))
			expectedPrice := float64(100) / float64(testUnits[2].ConversionFactor*testUnits[1].ConversionFactor)
			Expect(firstItem["unit_price"]).To(Equal(expectedPrice))
		})

		It("should not create purchase order with different base units", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			differentBaseUnit := fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Different Base Unit %s", uuid.New().String()),
				Symbol:           "DBU",
				UnitType:         "length",
				ConversionFactor: 100,
			})

			payload := map[string]interface{}{
				"inventory_id": testInventory.ID,
				"items": []map[string]interface{}{
					{
						"product_id":  testProduct.ID,
						"supplier_id": testSupplier.ID,
						"quantity":    2,
						"unit_id":     differentBaseUnit.ID,
						"unit_price":  100,
					},
				},
			}

			resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders", payload, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(400))
		})
	})

	Describe("Cancel Purchase Order", func() {
		var testSupplier *models.Supplier
		var testUnit *models.Unit
		var testProduct *models.Product
		var testInventory *models.Inventory

		BeforeEach(func() {
			testUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Test Unit %s", uuid.New().String()),
				UnitType:         uuid.New().String(),
				ConversionFactor: 1,
			})

			testSupplier = fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name: fmt.Sprintf("Test Supplier %s", uuid.New().String()),
			})

			testProduct = fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:   fmt.Sprintf("Test Product %s", uuid.New().String()),
				UnitID: testUnit.ID,
			})

			testInventory = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name: fmt.Sprintf("Test Inventory %s", uuid.New().String()),
			})
		})

		Context("when user has authorized role", func() {
			testCases := []struct {
				name           string
				currentStatus  models.PurchaseOrderStatus
				expectedStatus models.PurchaseOrderStatus
			}{
				{
					name:           "should cancel order_placed purchase order",
					currentStatus:  models.PurchaseOrderStatusOrderPlaced,
					expectedStatus: models.PurchaseOrderStatusCancelled,
				},
				{
					name:           "should cancel partially_delivered purchase order",
					currentStatus:  models.PurchaseOrderStatusPartiallyDelivered,
					expectedStatus: models.PurchaseOrderStatusCancelled,
				},
				{
					name:           "should cancel fully_delivered purchase order",
					currentStatus:  models.PurchaseOrderStatusFullyDelivered,
					expectedStatus: models.PurchaseOrderStatusCancelled,
				},
				{
					name:           "should not change completed purchase order to cancelled",
					currentStatus:  models.PurchaseOrderStatusCompleted,
					expectedStatus: models.PurchaseOrderStatusCompleted,
				},
			}

			for _, tc := range testCases {
				tc := tc // capture loop variable
				roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
				for _, role := range roles {
					role := role // capture loop variable
					It(fmt.Sprintf("%s with %s role", tc.name, role), func(ctx SpecContext) {
						client := testutil.NewClient(tenv, role)

						testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
							OrderNumber: uuid.New().String(),
							Status:      tc.currentStatus,
							InventoryID: &testInventory.ID,
							Items: []*models.PurchaseOrderItem{
								{
									ProductID:  &testProduct.ID,
									SupplierID: &testSupplier.ID,
									UnitID:     &testUnit.ID,
									Quantity:   decimal.NewFromInt(1),
								},
							},
						})

						urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d/status", testPurchaseOrder.ID)
						resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "cancelled"}, testutil.WithAuth())
						Expect(err).NotTo(HaveOccurred())
						Expect(resp.StatusCode).To(Equal(200))

						updateResp := testutil.ParseResponse(resp)
						Expect(updateResp["message"]).To(Equal("Purchase order status updated successfully"))

						// Verify in database
						var purchaseOrder models.PurchaseOrder
						err = tenv.DB.WithContext(ctx).First(&purchaseOrder, "id = ?", testPurchaseOrder.ID).Error
						Expect(err).NotTo(HaveOccurred())
						Expect(purchaseOrder.Status).To(Equal(tc.expectedStatus))
					})
				}
			}
		})

		Context("when user has unauthorized role", func() {
			testCases := []struct {
				name          string
				currentStatus models.PurchaseOrderStatus
			}{
				{
					name:          "should not cancel order_placed purchase order",
					currentStatus: models.PurchaseOrderStatusOrderPlaced,
				},
				{
					name:          "should not cancel partially_delivered purchase order",
					currentStatus: models.PurchaseOrderStatusPartiallyDelivered,
				},
				{
					name:          "should not cancel fully_delivered purchase order",
					currentStatus: models.PurchaseOrderStatusFullyDelivered,
				},
			}

			for _, tc := range testCases {
				tc := tc // capture loop variable
				roles := []models.UserRole{models.RoleStaff, models.RoleBotForm}
				for _, role := range roles {
					role := role // capture loop variable
					It(fmt.Sprintf("%s with %s role", tc.name, role), func() {
						client := testutil.NewClient(tenv, role)

						testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
							OrderNumber: uuid.New().String(),
							Status:      tc.currentStatus,
							InventoryID: &testInventory.ID,
							Items: []*models.PurchaseOrderItem{
								{
									ProductID:  &testProduct.ID,
									SupplierID: &testSupplier.ID,
									UnitID:     &testUnit.ID,
									Quantity:   decimal.NewFromInt(1),
								},
							},
						})

						urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d/status", testPurchaseOrder.ID)
						resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "cancelled"}, testutil.WithAuth())
						Expect(err).NotTo(HaveOccurred())
						Expect(resp.StatusCode).To(Equal(403))

						errorResp := testutil.ParseResponse(resp)
						Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update purchase-orders", role)))
					})
				}
			}
		})
	})

	Describe("Receive Purchase Order", func() {
		var testBaseUnit *models.Unit
		var testSupplier *models.Supplier
		var testProducts []*models.Product
		var testInventory *models.Inventory

		BeforeEach(func() {
			testBaseUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Test Base Unit %s", uuid.New().String()),
				Symbol:           "BU",
				UnitType:         "length",
				ConversionFactor: 1,
			})

			testSupplier = fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name: fmt.Sprintf("Test Supplier 1 %s", uuid.New().String()),
			})

			testProducts = fixture.WithProducts(tenv.ContextfulDB(), []*models.Product{
				{Name: fmt.Sprintf("Test Product 1 %s", uuid.New().String()), UnitID: testBaseUnit.ID},
				{Name: fmt.Sprintf("Test Product 2 %s", uuid.New().String()), UnitID: testBaseUnit.ID},
			})

			testInventory = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name: fmt.Sprintf("Test Inventory 1 %s", uuid.New().String()),
			})
		})

		Context("when user has authorized role", func() {
			testCases := []struct {
				name                  string
				currentPOStatus       models.PurchaseOrderStatus
				currentPOItem1Status  models.PurchaseOrderItemStatus
				receivedQuantity1     float64
				receivedQuantity2     float64
				expectedPOStatus      models.PurchaseOrderStatus
				expectedPOItem1Status models.PurchaseOrderItemStatus
			}{
				{
					name:                  "should receive partial delivery when both items partially delivered",
					currentPOStatus:       models.PurchaseOrderStatusOrderPlaced,
					currentPOItem1Status:  models.PurchaseOrderItemStatusAwaitingDelivery,
					receivedQuantity1:     50,
					receivedQuantity2:     50,
					expectedPOStatus:      models.PurchaseOrderStatusPartiallyDelivered,
					expectedPOItem1Status: models.PurchaseOrderItemStatusPartiallyDelivered,
				},
				{
					name:                  "should receive partial delivery when first item fully delivered",
					currentPOStatus:       models.PurchaseOrderStatusOrderPlaced,
					currentPOItem1Status:  models.PurchaseOrderItemStatusAwaitingDelivery,
					receivedQuantity1:     100,
					receivedQuantity2:     50,
					expectedPOStatus:      models.PurchaseOrderStatusPartiallyDelivered,
					expectedPOItem1Status: models.PurchaseOrderItemStatusDelivered,
				},
				{
					name:                  "should receive full delivery when both items fully delivered",
					currentPOStatus:       models.PurchaseOrderStatusOrderPlaced,
					currentPOItem1Status:  models.PurchaseOrderItemStatusAwaitingDelivery,
					receivedQuantity1:     100,
					receivedQuantity2:     100,
					expectedPOStatus:      models.PurchaseOrderStatusFullyDelivered,
					expectedPOItem1Status: models.PurchaseOrderItemStatusDelivered,
				},
				{
					name:                  "should complete delivery from partially delivered status",
					currentPOStatus:       models.PurchaseOrderStatusPartiallyDelivered,
					currentPOItem1Status:  models.PurchaseOrderItemStatusPartiallyDelivered,
					receivedQuantity1:     100,
					receivedQuantity2:     100,
					expectedPOStatus:      models.PurchaseOrderStatusFullyDelivered,
					expectedPOItem1Status: models.PurchaseOrderItemStatusDelivered,
				},
			}

			for _, tc := range testCases {
				tc := tc // capture loop variable
				roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant, models.RoleStaff}
				for _, role := range roles {
					role := role // capture loop variable
					It(fmt.Sprintf("%s with %s role", tc.name, role), func(ctx SpecContext) {
						client := testutil.NewClient(tenv, role)

						testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
							OrderNumber: uuid.New().String(),
							Status:      tc.currentPOStatus,
							InventoryID: &testInventory.ID,
							Items: []*models.PurchaseOrderItem{
								{
									ProductID:        &testProducts[0].ID,
									SupplierID:       &testSupplier.ID,
									UnitID:           &testBaseUnit.ID,
									Quantity:         decimal.NewFromInt(100),
									ReceivedQuantity: decimal.Zero,
									Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
								},
								{
									ProductID:        &testProducts[1].ID,
									SupplierID:       &testSupplier.ID,
									UnitID:           &testBaseUnit.ID,
									Quantity:         decimal.NewFromInt(100),
									ReceivedQuantity: decimal.Zero,
									Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
								},
							},
						})

						var purchaseOrderItems []models.PurchaseOrderItem
						err := tenv.DB.WithContext(ctx).
							Where("purchase_order_id = ?", testPurchaseOrder.ID).
							Order("id ASC").
							Find(&purchaseOrderItems).Error
						Expect(err).NotTo(HaveOccurred())
						Expect(purchaseOrderItems).To(HaveLen(2))

						payload := map[string]interface{}{
							"items": []map[string]interface{}{
								{
									"id":                purchaseOrderItems[0].ID,
									"received_quantity": tc.receivedQuantity1,
								},
								{
									"id":                purchaseOrderItems[1].ID,
									"received_quantity": tc.receivedQuantity2,
								},
							},
						}

						urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d/receive", testPurchaseOrder.ID)
						resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
						Expect(err).NotTo(HaveOccurred())
						Expect(resp.StatusCode).To(Equal(200))

						// Verify in database
						var purchaseOrder models.PurchaseOrder
						err = tenv.DB.WithContext(ctx).First(&purchaseOrder, "id = ?", testPurchaseOrder.ID).Error
						Expect(err).NotTo(HaveOccurred())
						Expect(purchaseOrder.Status).To(Equal(tc.expectedPOStatus))

						// Regression guard for the PO-id vs PO-item-id bug (commit
						// d2a47c1 / PR #7): receiving must stamp each purchase
						// transaction with the PO ITEM id (poi.ID), not the purchase
						// ORDER id, and link it to an inventory item of that POI's
						// product. The old code wrote poi.PurchaseOrderID here, which
						// silently pointed transactions at an unrelated PO/line.
						for _, poi := range purchaseOrderItems {
							var txn models.InventoryTransaction
							err := tenv.DB.WithContext(ctx).
								Where("purchase_order_item_id = ? AND transaction_type = ?",
									poi.ID, models.InventoryTransactionTypePurchase).
								First(&txn).Error
							Expect(err).NotTo(HaveOccurred(),
								"expected a purchase transaction linked to PO item %d (the bug would link PO id %d instead)",
								poi.ID, testPurchaseOrder.ID)

							var item models.InventoryItem
							err = tenv.DB.WithContext(ctx).First(&item, "id = ?", txn.InventoryItemID).Error
							Expect(err).NotTo(HaveOccurred())
							Expect(item.ProductID).To(Equal(*poi.ProductID),
								"purchase transaction's inventory item product must match its PO item product")
						}
					})
				}
			}
		})

		It("should re-update purchase order status when call receive inventory with quantity 0", func(ctx SpecContext) {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
				OrderNumber: uuid.New().String(),
				Status:      models.PurchaseOrderStatusPartiallyDelivered,
				InventoryID: &testInventory.ID,
				Items: []*models.PurchaseOrderItem{
					{
						ProductID:        &testProducts[0].ID,
						SupplierID:       &testSupplier.ID,
						UnitID:           &testBaseUnit.ID,
						Quantity:         decimal.NewFromInt(100),
						ReceivedQuantity: decimal.NewFromInt(100),
						Status:           models.PurchaseOrderItemStatusDelivered,
					},
				},
			})

			payload := map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"id":                testPurchaseOrder.Items[0].ID,
						"received_quantity": 0,
					},
				},
			}

			urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d/receive", testPurchaseOrder.ID)
			resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			// Verify in database
			var purchaseOrder models.PurchaseOrder
			err = tenv.DB.WithContext(ctx).First(&purchaseOrder, "id = ?", testPurchaseOrder.ID).Error
			Expect(err).NotTo(HaveOccurred())
			Expect(purchaseOrder.Status).To(Equal(models.PurchaseOrderStatusFullyDelivered))
		})

		It("should allow receiving quantity higher than order quantity", func(ctx SpecContext) {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
				OrderNumber: uuid.New().String(),
				Status:      models.PurchaseOrderStatusOrderPlaced,
				InventoryID: &testInventory.ID,
				Items: []*models.PurchaseOrderItem{
					{
						ProductID:        &testProducts[0].ID,
						SupplierID:       &testSupplier.ID,
						UnitID:           &testBaseUnit.ID,
						Quantity:         decimal.NewFromInt(100),
						ReceivedQuantity: decimal.NewFromInt(50),
					},
				},
			})

			payload := map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"id":                testPurchaseOrder.Items[0].ID,
						"received_quantity": 100,
					},
				},
			}
			urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d/receive", testPurchaseOrder.ID)
			resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			// Verify in database that received quantity is now 150 (50 + 100)
			var purchaseOrder models.PurchaseOrder
			err = tenv.DB.WithContext(ctx).Preload("Items").First(&purchaseOrder, "id = ?", testPurchaseOrder.ID).Error
			Expect(err).NotTo(HaveOccurred())
			expectedQuantity := decimal.NewFromInt(150)
			Expect(purchaseOrder.Items[0].ReceivedQuantity.Equal(expectedQuantity)).To(BeTrue(),
				"Expected received quantity to be %s but got %s", expectedQuantity.String(), purchaseOrder.Items[0].ReceivedQuantity.String())

			// Verify purchase order item status is over_delivered (received 150 > ordered 100)
			Expect(purchaseOrder.Items[0].Status).To(Equal(models.PurchaseOrderItemStatusOverDelivered))

			// Verify purchase order status is fully_delivered (over_delivered items are treated as delivered)
			Expect(purchaseOrder.Status).To(Equal(models.PurchaseOrderStatusFullyDelivered))
		})

		It("should not receive purchase order if decimal places is larger than unit decimal places", func(ctx SpecContext) {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			// Create a unit with only 1 decimal place
			unitWith1DecimalPlace := fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Test Unit 1 Decimal %s", uuid.New().String()),
				Symbol:           "BU1",
				UnitType:         "length",
				ConversionFactor: 1,
				DecimalPlaces:    1,
			})

			// Create a product with this unit
			productWith1DecimalUnit := fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:   fmt.Sprintf("Test Product 1 Decimal %s", uuid.New().String()),
				UnitID: unitWith1DecimalPlace.ID,
			})

			testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
				OrderNumber: uuid.New().String(),
				Status:      models.PurchaseOrderStatusOrderPlaced,
				InventoryID: &testInventory.ID,
				Items: []*models.PurchaseOrderItem{
					{
						ProductID:  &productWith1DecimalUnit.ID,
						SupplierID: &testSupplier.ID,
						UnitID:     &unitWith1DecimalPlace.ID,
						Quantity:   decimal.NewFromInt(100),
					},
				},
			})

			payload := map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"id":                testPurchaseOrder.Items[0].ID,
						"received_quantity": "99.11",
					},
				},
			}
			urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d/receive", testPurchaseOrder.ID)
			resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(400))

			errorResp := testutil.ParseResponse(resp)
			Expect(errorResp["message"]).To(ContainSubstring("decimal places"))
		})
	})

	Describe("Update Purchase Order", func() {
		var testSupplier *models.Supplier
		var testBaseUnit *models.Unit
		var testDerivedUnit *models.Unit
		var testDerivedUnit2 *models.Unit
		var testProducts []*models.Product
		var testInventory *models.Inventory

		BeforeEach(func() {
			testSupplier = fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name: fmt.Sprintf("Test Supplier %s", uuid.New().String()),
			})

			testBaseUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Test Base Unit %s", uuid.New().String()),
				Symbol:           "BU",
				UnitType:         "length",
				ConversionFactor: 1,
			})
			testDerivedUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Test Derived Unit %s", uuid.New().String()),
				Symbol:           "DU",
				UnitType:         "length",
				ConversionFactor: 2,
				BaseUnitID:       pkg.Ptr(testBaseUnit.ID),
			})
			testDerivedUnit2 = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Test Derived Unit 2 %s", uuid.New().String()),
				Symbol:           "DU2",
				UnitType:         "length",
				ConversionFactor: 10,
				BaseUnitID:       pkg.Ptr(testBaseUnit.ID),
			})

			testProducts = fixture.WithProducts(tenv.ContextfulDB(), []*models.Product{
				{Name: fmt.Sprintf("Test Product 1 %s", uuid.New().String()), UnitID: testBaseUnit.ID},
				{Name: fmt.Sprintf("Test Product 2 %s", uuid.New().String()), UnitID: testDerivedUnit.ID},
			})

			testInventory = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name: fmt.Sprintf("Test Inventory 1 %s", uuid.New().String()),
			})
		})

		Context("when user has authorized role", func() {
			type testCase struct {
				name                 string
				currentPOStatus      models.PurchaseOrderStatus
				currentPOItemStatus  models.PurchaseOrderItemStatus
				orderedQuantity      float64
				deliveredQuantity    float64
				updatedQuantity      float64
				updatedUnit          func() *models.Unit
				expectedPOStatus     models.PurchaseOrderStatus
				expectedPOItemStatus models.PurchaseOrderItemStatus
			}

			testCases := []testCase{
				{
					name:                 "should update order_placed purchase order",
					currentPOStatus:      models.PurchaseOrderStatusOrderPlaced,
					currentPOItemStatus:  models.PurchaseOrderItemStatusAwaitingDelivery,
					orderedQuantity:      50,
					deliveredQuantity:    0,
					updatedQuantity:      100,
					updatedUnit:          func() *models.Unit { return testBaseUnit },
					expectedPOStatus:     models.PurchaseOrderStatusOrderPlaced,
					expectedPOItemStatus: models.PurchaseOrderItemStatusAwaitingDelivery,
				},
				{
					name:                 "should update partially_delivered purchase order",
					currentPOStatus:      models.PurchaseOrderStatusPartiallyDelivered,
					currentPOItemStatus:  models.PurchaseOrderItemStatusPartiallyDelivered,
					orderedQuantity:      50,
					deliveredQuantity:    20,
					updatedQuantity:      100,
					updatedUnit:          func() *models.Unit { return testBaseUnit },
					expectedPOStatus:     models.PurchaseOrderStatusPartiallyDelivered,
					expectedPOItemStatus: models.PurchaseOrderItemStatusPartiallyDelivered,
				},
				{
					name:                 "should change fully_delivered to partially_delivered when quantity increased",
					currentPOStatus:      models.PurchaseOrderStatusFullyDelivered,
					currentPOItemStatus:  models.PurchaseOrderItemStatusDelivered,
					orderedQuantity:      50,
					deliveredQuantity:    50,
					updatedQuantity:      100,
					updatedUnit:          func() *models.Unit { return testBaseUnit },
					expectedPOStatus:     models.PurchaseOrderStatusPartiallyDelivered,
					expectedPOItemStatus: models.PurchaseOrderItemStatusPartiallyDelivered,
				},
				{
					name:                 "should change partially_delivered to fully_delivered when quantity matches delivered",
					currentPOStatus:      models.PurchaseOrderStatusPartiallyDelivered,
					currentPOItemStatus:  models.PurchaseOrderItemStatusPartiallyDelivered,
					orderedQuantity:      50,
					deliveredQuantity:    30,
					updatedQuantity:      30,
					updatedUnit:          func() *models.Unit { return testBaseUnit },
					expectedPOStatus:     models.PurchaseOrderStatusFullyDelivered,
					expectedPOItemStatus: models.PurchaseOrderItemStatusDelivered,
				},
				{
					name:                 "should update with derived unit conversion",
					currentPOStatus:      models.PurchaseOrderStatusPartiallyDelivered,
					currentPOItemStatus:  models.PurchaseOrderItemStatusPartiallyDelivered,
					orderedQuantity:      50,
					deliveredQuantity:    40,
					updatedQuantity:      20,
					updatedUnit:          func() *models.Unit { return testDerivedUnit },
					expectedPOStatus:     models.PurchaseOrderStatusFullyDelivered,
					expectedPOItemStatus: models.PurchaseOrderItemStatusDelivered,
				},
			}

			for _, tc := range testCases {
				tc := tc // capture loop variable
				roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
				for _, role := range roles {
					role := role // capture loop variable
					It(fmt.Sprintf("%s with %s role", tc.name, role), func(ctx SpecContext) {
						updatedUnit := tc.updatedUnit()
						client := testutil.NewClient(tenv, role)

						testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
							OrderNumber: uuid.New().String(),
							Status:      tc.currentPOStatus,
							InventoryID: &testInventory.ID,
							Items: []*models.PurchaseOrderItem{
								{
									ProductID:        &testProducts[0].ID,
									SupplierID:       &testSupplier.ID,
									UnitID:           pkg.Ptr(updatedUnit.ID),
									Quantity:         decimal.NewFromFloat(tc.orderedQuantity),
									ReceivedQuantity: decimal.NewFromFloat(tc.deliveredQuantity),
									Status:           tc.currentPOItemStatus,
									UnitPrice:        0,
								},
							},
						})

						notes := uuid.New().String()
						unitPrice := 1000
						payload := map[string]interface{}{
							"inventory_id": testInventory.ID,
							"notes":        notes,
							"items": []map[string]interface{}{
								{
									"product_id":  testProducts[0].ID,
									"supplier_id": testSupplier.ID,
									"unit_id":     updatedUnit.ID,
									"quantity":    tc.updatedQuantity,
									"unit_price":  unitPrice,
								},
							},
						}

						urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", testPurchaseOrder.ID)
						resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
						Expect(err).NotTo(HaveOccurred())
						Expect(resp.StatusCode).To(Equal(200))

						purchaseOrderResp := testutil.ParseResponse(resp)
						Expect(purchaseOrderResp["notes"]).To(Equal(notes))
						items := purchaseOrderResp["items"].([]interface{})
						Expect(items).To(HaveLen(1))
						firstItem := items[0].(map[string]interface{})

						expectedUnitID := testBaseUnit.ID
						expectedPrice := float64(unitPrice) / updatedUnit.ConversionFactor
						expectedQuantityStr := fmt.Sprintf("%.0f", tc.updatedQuantity*updatedUnit.ConversionFactor)

						Expect(firstItem["unit_price"]).To(Equal(expectedPrice))
						Expect(firstItem["quantity"]).To(Equal(expectedQuantityStr))
						Expect(firstItem["product_id"]).To(Equal(float64(testProducts[0].ID)))
						Expect(firstItem["supplier_id"]).To(Equal(float64(testSupplier.ID)))
						Expect(firstItem["unit_id"]).To(Equal(float64(expectedUnitID)))
						Expect(purchaseOrderResp["status"]).To(Equal(string(tc.expectedPOStatus)))
						Expect(firstItem["status"]).To(Equal(string(tc.expectedPOItemStatus)))
					})
				}
			}

			It("should remove purchase order items if received quantity is 0, then add new item", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusOrderPlaced,
					InventoryID: &testInventory.ID,
					Items: []*models.PurchaseOrderItem{
						{
							ProductID:        &testProducts[0].ID,
							SupplierID:       &testSupplier.ID,
							UnitID:           pkg.Ptr(testBaseUnit.ID),
							Quantity:         decimal.NewFromInt(50),
							ReceivedQuantity: decimal.Zero,
							Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
							UnitPrice:        0,
						},
					},
				})

				urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", testPurchaseOrder.ID)
				payload := map[string]interface{}{
					"inventory_id": testInventory.ID,
					"notes":        uuid.New().String(),
					"items": []map[string]interface{}{
						{
							"product_id":  testProducts[1].ID,
							"supplier_id": testSupplier.ID,
							"unit_id":     testBaseUnit.ID,
							"quantity":    100,
						},
					},
				}
				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Verify response
				var response models.PurchaseOrder
				err = json.NewDecoder(resp.Body).Decode(&response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal(models.PurchaseOrderStatusOrderPlaced))
				Expect(response.Items).To(HaveLen(1))
				Expect(*response.Items[0].ProductID).To(Equal(testProducts[1].ID))
				responseQuantity, _ := response.Items[0].Quantity.Float64()
				Expect(int(responseQuantity)).To(Equal(100))
				responseReceived, _ := response.Items[0].ReceivedQuantity.Float64()
				Expect(int(responseReceived)).To(Equal(0))
				Expect(response.Items[0].Status).To(Equal(models.PurchaseOrderItemStatusAwaitingDelivery))
			})

			It("should update purchase order status accordingly when items are removed", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				// Create purchase order with multiple items in different states
				testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusPartiallyDelivered,
					InventoryID: &testInventory.ID,
					Items: []*models.PurchaseOrderItem{
						{
							ProductID:        &testProducts[0].ID,
							SupplierID:       &testSupplier.ID,
							UnitID:           pkg.Ptr(testBaseUnit.ID),
							Quantity:         decimal.NewFromInt(100),
							ReceivedQuantity: decimal.NewFromInt(100),
							Status:           models.PurchaseOrderItemStatusDelivered,
							UnitPrice:        1000,
						},
						{
							ProductID:        &testProducts[1].ID,
							SupplierID:       &testSupplier.ID,
							UnitID:           pkg.Ptr(testBaseUnit.ID),
							Quantity:         decimal.NewFromInt(50),
							ReceivedQuantity: decimal.Zero,
							Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
							UnitPrice:        2000,
						},
					},
				})

				// Remove the awaiting delivery item, keeping only the delivered item
				urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", testPurchaseOrder.ID)
				payload := map[string]interface{}{
					"inventory_id": testInventory.ID,
					"notes":        uuid.New().String(),
					"items": []map[string]interface{}{
						{
							"product_id":  testProducts[0].ID,
							"supplier_id": testSupplier.ID,
							"unit_id":     testBaseUnit.ID,
							"quantity":    100,
							"unit_price":  1000,
						},
					},
				}

				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Verify response
				var response models.PurchaseOrder
				err = json.NewDecoder(resp.Body).Decode(&response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal(models.PurchaseOrderStatusFullyDelivered))
				Expect(response.Items).To(HaveLen(1))
				Expect(*response.Items[0].ProductID).To(Equal(testProducts[0].ID))
				Expect(response.Items[0].Status).To(Equal(models.PurchaseOrderItemStatusDelivered))

				// Verify in database
				var purchaseOrder models.PurchaseOrder
				err = tenv.DB.WithContext(ctx).Preload("Items").First(&purchaseOrder, "id = ?", testPurchaseOrder.ID).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(purchaseOrder.Status).To(Equal(models.PurchaseOrderStatusFullyDelivered))
				Expect(purchaseOrder.Items).To(HaveLen(1))
			})

			It("should update purchase order status accordingly when items are removed - partially_delivered remains", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				// Create purchase order with multiple items
				testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusPartiallyDelivered,
					InventoryID: &testInventory.ID,
					Items: []*models.PurchaseOrderItem{
						{
							ProductID:        &testProducts[0].ID,
							SupplierID:       &testSupplier.ID,
							UnitID:           pkg.Ptr(testBaseUnit.ID),
							Quantity:         decimal.NewFromInt(100),
							ReceivedQuantity: decimal.NewFromInt(50),
							Status:           models.PurchaseOrderItemStatusPartiallyDelivered,
							UnitPrice:        1000,
						},
						{
							ProductID:        &testProducts[1].ID,
							SupplierID:       &testSupplier.ID,
							UnitID:           pkg.Ptr(testBaseUnit.ID),
							Quantity:         decimal.NewFromInt(50),
							ReceivedQuantity: decimal.Zero,
							Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
							UnitPrice:        2000,
						},
					},
				})

				// Remove the awaiting delivery item, keeping only the partially delivered item
				urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", testPurchaseOrder.ID)
				payload := map[string]interface{}{
					"inventory_id": testInventory.ID,
					"notes":        uuid.New().String(),
					"items": []map[string]interface{}{
						{
							"product_id":  testProducts[0].ID,
							"supplier_id": testSupplier.ID,
							"unit_id":     testBaseUnit.ID,
							"quantity":    100,
							"unit_price":  1000,
						},
					},
				}

				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Verify response
				var response models.PurchaseOrder
				err = json.NewDecoder(resp.Body).Decode(&response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal(models.PurchaseOrderStatusPartiallyDelivered))
				Expect(response.Items).To(HaveLen(1))
				Expect(*response.Items[0].ProductID).To(Equal(testProducts[0].ID))
				Expect(response.Items[0].Status).To(Equal(models.PurchaseOrderItemStatusPartiallyDelivered))

				// Verify in database
				var purchaseOrder models.PurchaseOrder
				err = tenv.DB.WithContext(ctx).Preload("Items").First(&purchaseOrder, "id = ?", testPurchaseOrder.ID).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(purchaseOrder.Status).To(Equal(models.PurchaseOrderStatusPartiallyDelivered))
				Expect(purchaseOrder.Items).To(HaveLen(1))
			})

			It("should update quantity & total amount when derived unit is updated", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusOrderPlaced,
					InventoryID: &testInventory.ID,
					Items: []*models.PurchaseOrderItem{
						{
							ProductID:        &testProducts[0].ID,
							SupplierID:       &testSupplier.ID,
							UnitID:           pkg.Ptr(testBaseUnit.ID),
							Quantity:         decimal.NewFromInt(50),
							ReceivedQuantity: decimal.NewFromInt(0),
							Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
							UnitPrice:        0,
						},
					},
				})

				urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", testPurchaseOrder.ID)
				newQuantity := 10
				expectedBaseQuantity := decimal.NewFromInt(100)
				expectedBaseUnitPrice := 10.0
				expectedBaseUnitID := testBaseUnit.ID
				expectedTotalAmount := decimal.NewFromInt(1000)

				payload := map[string]interface{}{
					"inventory_id": testInventory.ID,
					"notes":        uuid.New().String(),
					"items": []map[string]interface{}{
						{
							"product_id":  testProducts[0].ID,
							"supplier_id": testSupplier.ID,
							"unit_id":     testDerivedUnit2.ID,
							"quantity":    newQuantity,
							"unit_price":  100.0,
						},
					},
				}
				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Verify Response Body
				purchaseOrderResp := testutil.ParseResponse(resp)
				totalAmountStr, ok := purchaseOrderResp["total_amount"].(string)
				if ok {
					Expect(totalAmountStr).To(Equal(expectedTotalAmount.String()))
				} else {
					totalAmountFloat, _ := purchaseOrderResp["total_amount"].(float64)
					Expect(totalAmountFloat).To(Equal(expectedTotalAmount.InexactFloat64()))
				}
				respItems := purchaseOrderResp["items"].([]interface{})
				firstItem := respItems[0].(map[string]interface{})
				Expect(firstItem["unit_price"]).To(Equal(expectedBaseUnitPrice))
				Expect(firstItem["quantity"]).To(Equal(expectedBaseQuantity.String()))
				Expect(firstItem["unit_id"]).To(Equal(float64(expectedBaseUnitID)))
				Expect(purchaseOrderResp["status"]).To(Equal(string(models.PurchaseOrderStatusOrderPlaced)))

				// Verify Database
				var purchaseOrder models.PurchaseOrder
				err = tenv.DB.WithContext(ctx).Preload("Items").First(&purchaseOrder, "id = ?", testPurchaseOrder.ID).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(purchaseOrder.CalculateTotalAmount().String()).To(Equal(expectedTotalAmount.String()))
				Expect(purchaseOrder.Items).To(HaveLen(1))
				Expect(*purchaseOrder.Items[0].ProductID).To(Equal(testProducts[0].ID))
				Expect(purchaseOrder.Items[0].Quantity.String()).To(Equal(expectedBaseQuantity.String()))
				Expect(*purchaseOrder.Items[0].UnitID).To(Equal(expectedBaseUnitID))
				Expect(purchaseOrder.Items[0].UnitPrice).To(Equal(expectedBaseUnitPrice))
				Expect(purchaseOrder.Items[0].Status).To(Equal(models.PurchaseOrderItemStatusAwaitingDelivery))
			})

			It("should not remove purchase order items if received quantity is not 0", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusOrderPlaced,
					InventoryID: &testInventory.ID,
					Items: []*models.PurchaseOrderItem{
						{
							ProductID:        &testProducts[0].ID,
							SupplierID:       &testSupplier.ID,
							UnitID:           pkg.Ptr(testBaseUnit.ID),
							Quantity:         decimal.NewFromInt(50),
							ReceivedQuantity: decimal.NewFromInt(30),
							Status:           models.PurchaseOrderItemStatusPartiallyDelivered,
							UnitPrice:        0,
						},
					},
				})

				urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", testPurchaseOrder.ID)
				payload := map[string]interface{}{
					"inventory_id": testInventory.ID,
					"notes":        uuid.New().String(),
					"items": []map[string]interface{}{
						{
							"product_id":  testProducts[1].ID,
							"supplier_id": testSupplier.ID,
							"unit_id":     testDerivedUnit.ID,
							"quantity":    100,
						},
					},
				}
				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(400))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["message"]).To(ContainSubstring("Cannot delete item with received quantity"))
			})

			It("should remove item if update quantity is 0", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusOrderPlaced,
					InventoryID: &testInventory.ID,
					Items: []*models.PurchaseOrderItem{
						{
							ProductID:        &testProducts[0].ID,
							SupplierID:       &testSupplier.ID,
							UnitID:           pkg.Ptr(testBaseUnit.ID),
							Quantity:         decimal.NewFromInt(50),
							ReceivedQuantity: decimal.Zero,
							Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
							UnitPrice:        1000,
						},
						{
							ProductID:        &testProducts[1].ID,
							SupplierID:       &testSupplier.ID,
							UnitID:           pkg.Ptr(testBaseUnit.ID),
							Quantity:         decimal.NewFromInt(100),
							ReceivedQuantity: decimal.Zero,
							Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
							UnitPrice:        2000,
						},
					},
				})

				// Update purchase order, setting quantity to 0 for the first item
				urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", testPurchaseOrder.ID)
				payload := map[string]interface{}{
					"inventory_id": testInventory.ID,
					"notes":        uuid.New().String(),
					"items": []map[string]interface{}{
						{
							"product_id":  testProducts[0].ID,
							"supplier_id": testSupplier.ID,
							"unit_id":     testBaseUnit.ID,
							"quantity":    0,
							"unit_price":  1000,
						},
						{
							"product_id":  testProducts[1].ID,
							"supplier_id": testSupplier.ID,
							"unit_id":     testBaseUnit.ID,
							"quantity":    100,
							"unit_price":  2000,
						},
					},
				}
				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Verify response
				var response models.PurchaseOrder
				err = json.NewDecoder(resp.Body).Decode(&response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal(models.PurchaseOrderStatusOrderPlaced))
				Expect(response.Items).To(HaveLen(1))
				Expect(*response.Items[0].ProductID).To(Equal(testProducts[1].ID))
				responseQuantity, _ := response.Items[0].Quantity.Float64()
				Expect(int(responseQuantity)).To(Equal(100))

				// Verify in database that the first item is removed
				var purchaseOrder models.PurchaseOrder
				err = tenv.DB.WithContext(ctx).Preload("Items").First(&purchaseOrder, "id = ?", testPurchaseOrder.ID).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(purchaseOrder.Items).To(HaveLen(1))
				Expect(*purchaseOrder.Items[0].ProductID).To(Equal(testProducts[1].ID))

				// Verify the removed item no longer exists
				var removedItem models.PurchaseOrderItem
				err = tenv.DB.WithContext(ctx).Where("purchase_order_id = ? AND product_id = ?", testPurchaseOrder.ID, testProducts[0].ID).First(&removedItem).Error
				Expect(err).To(HaveOccurred())
			})

			It("should not update purchase order when no items are provided", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusOrderPlaced,
					InventoryID: &testInventory.ID,
				})
				payload := map[string]interface{}{
					"inventory_id": testInventory.ID,
					"notes":        uuid.New().String(),
				}

				urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", testPurchaseOrder.ID)
				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(400))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["message"]).To(ContainSubstring("Validation failed"))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not update purchase order with staff role", func() {
				client := testutil.NewClient(tenv, models.RoleStaff)

				testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusOrderPlaced,
					InventoryID: &testInventory.ID,
					Items: []*models.PurchaseOrderItem{
						{
							ProductID:        &testProducts[0].ID,
							SupplierID:       &testSupplier.ID,
							UnitID:           pkg.Ptr(testBaseUnit.ID),
							Quantity:         decimal.NewFromInt(50),
							ReceivedQuantity: decimal.Zero,
							Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
							UnitPrice:        0,
						},
					},
				})

				payload := map[string]interface{}{
					"inventory_id": testInventory.ID,
					"notes":        uuid.New().String(),
					"items": []map[string]interface{}{
						{
							"product_id":  testProducts[0].ID,
							"supplier_id": testSupplier.ID,
							"unit_id":     testDerivedUnit.ID,
							"quantity":    100,
							"unit_price":  0,
						},
					},
				}
				urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", testPurchaseOrder.ID)
				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update purchase-orders", models.RoleStaff)))
			})

			It("should not update purchase order with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusOrderPlaced,
					InventoryID: &testInventory.ID,
					Items: []*models.PurchaseOrderItem{
						{
							ProductID:        &testProducts[0].ID,
							SupplierID:       &testSupplier.ID,
							UnitID:           pkg.Ptr(testBaseUnit.ID),
							Quantity:         decimal.NewFromInt(50),
							ReceivedQuantity: decimal.Zero,
							Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
							UnitPrice:        0,
						},
					},
				})

				payload := map[string]interface{}{
					"inventory_id": testInventory.ID,
					"notes":        uuid.New().String(),
					"items": []map[string]interface{}{
						{
							"product_id":  testProducts[0].ID,
							"supplier_id": testSupplier.ID,
							"unit_id":     testDerivedUnit.ID,
							"quantity":    100,
							"unit_price":  0,
						},
					},
				}
				urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", testPurchaseOrder.ID)
				resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update purchase-orders", models.RoleBotForm)))
			})
		})

		It("should return 404 when purchase order is not found", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			notFoundId := uint(999999999)
			urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", notFoundId)
			payload := map[string]interface{}{
				"inventory_id": testInventory.ID,
				"notes":        uuid.New().String(),
				"items": []map[string]interface{}{
					{
						"product_id":  testProducts[0].ID,
						"supplier_id": testSupplier.ID,
						"unit_id":     testDerivedUnit.ID,
						"quantity":    100,
					},
				},
			}
			resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(404))

			errorResp := testutil.ParseResponse(resp)
			Expect(errorResp["message"]).To(Equal(fmt.Sprintf("Purchase order with ID %d not found", notFoundId)))
		})

		It("should return 400 when no items are provided", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
				OrderNumber: uuid.New().String(),
				Status:      models.PurchaseOrderStatusOrderPlaced,
				InventoryID: &testInventory.ID,
				Items: []*models.PurchaseOrderItem{
					{
						ProductID:  &testProducts[0].ID,
						SupplierID: &testSupplier.ID,
						UnitID:     pkg.Ptr(testBaseUnit.ID),
						Quantity:   decimal.NewFromInt(100),
						UnitPrice:  1000,
					},
				},
			})

			urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", testPurchaseOrder.ID)
			payload := map[string]interface{}{
				"inventory_id": testInventory.ID,
				"notes":        uuid.New().String(),
			}
			resp, err := client.MakeRequest("PUT", urlPath, payload, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(400))

			errorResp := testutil.ParseResponse(resp)
			Expect(errorResp["message"]).To(ContainSubstring("Validation failed"))
		})
	})

	Describe("Get Purchase Order", func() {
		var testInventory *models.Inventory
		var testSupplier *models.Supplier
		var testBaseUnit *models.Unit
		var testDerivedUnit *models.Unit
		var testDerivedUnit2 *models.Unit
		var testProduct *models.Product

		BeforeEach(func() {
			testInventory = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name: fmt.Sprintf("Test Inventory 1 %s", uuid.New().String()),
			})

			testSupplier = fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
				Name: fmt.Sprintf("Test Supplier 1 %s", uuid.New().String()),
			})

			testBaseUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Test Base Unit %s", uuid.New().String()),
				Symbol:           "BU",
				UnitType:         "length",
				ConversionFactor: 1,
			})
			testDerivedUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Test Derived Unit %s", uuid.New().String()),
				Symbol:           "DU",
				UnitType:         "length",
				ConversionFactor: 10,
				BaseUnitID:       pkg.Ptr(testBaseUnit.ID),
			})
			testDerivedUnit2 = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				Name:             fmt.Sprintf("Test Derived Unit 2 %s", uuid.New().String()),
				Symbol:           "DU2",
				UnitType:         "length",
				ConversionFactor: 100,
				BaseUnitID:       pkg.Ptr(testDerivedUnit.ID),
			})

			testProduct = fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:   fmt.Sprintf("Test Product 1 %s", uuid.New().String()),
				UnitID: testBaseUnit.ID,
			})
		})

		It("should get purchase order when item's unit is a base unit", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
				OrderNumber: uuid.New().String(),
				Status:      models.PurchaseOrderStatusOrderPlaced,
				InventoryID: &testInventory.ID,
				Items: []*models.PurchaseOrderItem{
					{
						ProductID:        &testProduct.ID,
						SupplierID:       &testSupplier.ID,
						UnitID:           pkg.Ptr(testBaseUnit.ID),
						Quantity:         decimal.NewFromInt(100),
						ReceivedQuantity: decimal.Zero,
						Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
						UnitPrice:        1000,
					},
				},
			})

			urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", testPurchaseOrder.ID)
			resp, err := client.MakeRequest("GET", urlPath, nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			purchaseOrderResp := testutil.ParseResponse(resp)
			Expect(purchaseOrderResp["order_number"]).To(Equal(testPurchaseOrder.OrderNumber))
			Expect(purchaseOrderResp["inventory_id"]).To(Equal(float64(*testPurchaseOrder.InventoryID)))
			Expect(purchaseOrderResp["status"]).To(Equal(string(testPurchaseOrder.Status)))
			Expect(purchaseOrderResp["total_amount"]).To(Equal(testPurchaseOrder.CalculateTotalAmount().String()))
			Expect(purchaseOrderResp["items"]).To(HaveLen(1))

			items := purchaseOrderResp["items"].([]interface{})
			POItemResp := items[0].(map[string]interface{})
			Expect(POItemResp["product_id"]).To(Equal(float64(testProduct.ID)))
			Expect(POItemResp["supplier_id"]).To(Equal(float64(testSupplier.ID)))
			Expect(POItemResp["unit_id"]).To(Equal(float64(testBaseUnit.ID)))
			Expect(POItemResp["quantity"]).To(Equal(testPurchaseOrder.Items[0].Quantity.String()))
			Expect(POItemResp["unit_price"]).To(Equal(testPurchaseOrder.Items[0].UnitPrice))

			// Assert current unit
			unitResp := POItemResp["unit"].(map[string]interface{})
			Expect(unitResp["name"]).To(Equal(testBaseUnit.Name))
			Expect(unitResp["symbol"]).To(Equal(testBaseUnit.Symbol))
			Expect(unitResp["unit_type"]).To(Equal(testBaseUnit.UnitType))
			Expect(unitResp["conversion_factor"]).To(Equal(testBaseUnit.ConversionFactor))
			Expect(unitResp["conversion_factor_to_current"]).To(Equal(float64(1)))
			Expect(unitResp["derived_units"]).To(HaveLen(2))

			// Assert derived units
			derivedUnits := unitResp["derived_units"].([]interface{})
			derivedUnitResp := derivedUnits[0].(map[string]interface{})
			Expect(derivedUnitResp["name"]).To(Equal(testDerivedUnit.Name))
			Expect(derivedUnitResp["symbol"]).To(Equal(testDerivedUnit.Symbol))
			Expect(derivedUnitResp["unit_type"]).To(Equal(testDerivedUnit.UnitType))
			Expect(derivedUnitResp["conversion_factor"]).To(Equal(testDerivedUnit.ConversionFactor))
			Expect(derivedUnitResp["conversion_factor_to_current"]).To(Equal(0.1))
		})

		Context("when item's unit is a derived unit", func() {
			roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant, models.RoleStaff}
			for _, role := range roles {
				role := role // capture loop variable
				It(fmt.Sprintf("should get purchase order with %s role", role), func() {
					client := testutil.NewClient(tenv, role)

					testPurchaseOrder := fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
						OrderNumber: uuid.New().String(),
						Status:      models.PurchaseOrderStatusOrderPlaced,
						InventoryID: &testInventory.ID,
						Items: []*models.PurchaseOrderItem{
							{
								ProductID:        &testProduct.ID,
								SupplierID:       &testSupplier.ID,
								UnitID:           pkg.Ptr(testDerivedUnit.ID),
								Quantity:         decimal.NewFromInt(100),
								ReceivedQuantity: decimal.Zero,
								Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
								UnitPrice:        1000,
							},
						},
					})

					urlPath := fmt.Sprintf("/api/v1/purchase-orders/%d", testPurchaseOrder.ID)
					resp, err := client.MakeRequest("GET", urlPath, nil, testutil.WithAuth())
					Expect(err).NotTo(HaveOccurred())
					Expect(resp.StatusCode).To(Equal(200))

					purchaseOrderResp := testutil.ParseResponse(resp)
					Expect(purchaseOrderResp["order_number"]).To(Equal(testPurchaseOrder.OrderNumber))
					Expect(purchaseOrderResp["inventory_id"]).To(Equal(float64(*testPurchaseOrder.InventoryID)))
					Expect(purchaseOrderResp["status"]).To(Equal(string(testPurchaseOrder.Status)))
					Expect(purchaseOrderResp["total_amount"]).To(Equal(testPurchaseOrder.CalculateTotalAmount().String()))
					Expect(purchaseOrderResp["items"]).To(HaveLen(1))

					items := purchaseOrderResp["items"].([]interface{})
					POItemResp := items[0].(map[string]interface{})
					Expect(POItemResp["product_id"]).To(Equal(float64(testProduct.ID)))
					Expect(POItemResp["supplier_id"]).To(Equal(float64(testSupplier.ID)))
					Expect(POItemResp["unit_id"]).To(Equal(float64(testDerivedUnit.ID)))
					Expect(POItemResp["quantity"]).To(Equal(testPurchaseOrder.Items[0].Quantity.String()))
					Expect(POItemResp["unit_price"]).To(Equal(testPurchaseOrder.Items[0].UnitPrice))

					// Assert current unit
					unitResp := POItemResp["unit"].(map[string]interface{})
					Expect(unitResp["name"]).To(Equal(testDerivedUnit.Name))
					Expect(unitResp["symbol"]).To(Equal(testDerivedUnit.Symbol))
					Expect(unitResp["unit_type"]).To(Equal(testDerivedUnit.UnitType))
					Expect(unitResp["conversion_factor"]).To(Equal(testDerivedUnit.ConversionFactor))
					Expect(unitResp["conversion_factor_to_current"]).To(Equal(float64(1)))

					// Assert derived units
					Expect(unitResp["derived_units"]).To(HaveLen(1))
					derivedUnits := unitResp["derived_units"].([]interface{})
					derivedUnitResp := derivedUnits[0].(map[string]interface{})
					Expect(derivedUnitResp["name"]).To(Equal(testDerivedUnit2.Name))
					Expect(derivedUnitResp["symbol"]).To(Equal(testDerivedUnit2.Symbol))
					Expect(derivedUnitResp["unit_type"]).To(Equal(testDerivedUnit2.UnitType))
					Expect(derivedUnitResp["conversion_factor"]).To(Equal(testDerivedUnit2.ConversionFactor))
					Expect(derivedUnitResp["conversion_factor_to_current"]).To(Equal(0.01))

					// Assert base unit
					baseUnitResp := unitResp["base_unit"].(map[string]interface{})
					Expect(baseUnitResp["name"]).To(Equal(testBaseUnit.Name))
					Expect(baseUnitResp["symbol"]).To(Equal(testBaseUnit.Symbol))
					Expect(baseUnitResp["unit_type"]).To(Equal(testBaseUnit.UnitType))
					Expect(baseUnitResp["conversion_factor"]).To(Equal(testBaseUnit.ConversionFactor))
					Expect(baseUnitResp["conversion_factor_to_current"]).To(Equal(float64(10)))
				})
			}
		})

		Context("when searching purchase orders", func() {
			var testPurchaseOrders []*models.PurchaseOrder
			var testSupplier *models.Supplier
			var testUnit *models.Unit
			var testProduct *models.Product
			var uniqueSearchPrefix string

			BeforeEach(func() {
				ctx := pkg.WithUserEmail(context.Background(), "test@example.com")
				uniqueSearchPrefix = "PO-search-unique-" + uuid.New().String()

				// Create test data needed for purchase orders with items
				testSupplier = fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
					Name: fmt.Sprintf("Test Supplier Search %s", uuid.New().String()),
				})
				testUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
					Name:             fmt.Sprintf("Test Unit Search %s", uuid.New().String()),
					Symbol:           "BU",
					UnitType:         "length",
					ConversionFactor: 1,
				})
				testProduct = fixture.WithProduct(tenv.ContextfulDB(), models.Product{
					Name:   fmt.Sprintf("Test Product Search %s", uuid.New().String()),
					UnitID: testUnit.ID,
				})

				testPurchaseOrders = []*models.PurchaseOrder{
					{
						Status:      models.PurchaseOrderStatusOrderPlaced,
						InventoryID: &testInventory.ID,
						OrderNumber: uniqueSearchPrefix + "-" + uuid.New().String(),
						Items: []*models.PurchaseOrderItem{
							{
								ProductID:  &testProduct.ID,
								SupplierID: &testSupplier.ID,
								UnitID:     &testUnit.ID,
								Quantity:   decimal.NewFromInt(1),
								UnitPrice:  100,
							},
						},
					},
					{
						Status:      models.PurchaseOrderStatusPartiallyDelivered,
						InventoryID: &testInventory.ID,
						OrderNumber: uniqueSearchPrefix + "-" + uuid.New().String(),
						Items: []*models.PurchaseOrderItem{
							{
								ProductID:  &testProduct.ID,
								SupplierID: &testSupplier.ID,
								UnitID:     &testUnit.ID,
								Quantity:   decimal.NewFromInt(1),
								UnitPrice:  100,
							},
						},
					},
					{
						Status:      models.PurchaseOrderStatusFullyDelivered,
						InventoryID: &testInventory.ID,
						OrderNumber: uniqueSearchPrefix + "-" + uuid.New().String(),
						Items: []*models.PurchaseOrderItem{
							{
								ProductID:  &testProduct.ID,
								SupplierID: &testSupplier.ID,
								UnitID:     &testUnit.ID,
								Quantity:   decimal.NewFromInt(1),
								UnitPrice:  100,
							},
						},
					},
					{
						Status:      models.PurchaseOrderStatusCompleted,
						InventoryID: &testInventory.ID,
						OrderNumber: uniqueSearchPrefix + "-" + uuid.New().String(),
						Items: []*models.PurchaseOrderItem{
							{
								ProductID:  &testProduct.ID,
								SupplierID: &testSupplier.ID,
								UnitID:     &testUnit.ID,
								Quantity:   decimal.NewFromInt(1),
								UnitPrice:  100,
							},
						},
					},
					{
						Status:      models.PurchaseOrderStatusCancelled,
						InventoryID: &testInventory.ID,
						OrderNumber: uniqueSearchPrefix + "-" + uuid.New().String(),
						Items: []*models.PurchaseOrderItem{
							{
								ProductID:  &testProduct.ID,
								SupplierID: &testSupplier.ID,
								UnitID:     &testUnit.ID,
								Quantity:   decimal.NewFromInt(1),
								UnitPrice:  100,
							},
						},
					},
				}
				Expect(tenv.DB.WithContext(ctx).Create(&testPurchaseOrders).Error).NotTo(HaveOccurred())
			})

			roles := []models.UserRole{models.RoleAdmin, models.RoleAccountant}
			for _, role := range roles {
				role := role // capture loop variable
				It(fmt.Sprintf("should return all purchase orders with %s role", role), func() {
					client := testutil.NewClient(tenv, role)

					resp, err := client.MakeRequest("GET", "/api/v1/purchase-orders", nil,
						testutil.WithAuth(),
						testutil.WithParams(map[string]string{
							"q": uniqueSearchPrefix,
						}),
					)
					Expect(err).NotTo(HaveOccurred())
					Expect(resp.StatusCode).To(Equal(200))

					purchaseOrderResp := testutil.ParseResponse(resp)
					purchaseOrders := purchaseOrderResp["data"].([]interface{})
					Expect(purchaseOrders).To(HaveLen(len(testPurchaseOrders)))
				})
			}

			It("should not return completed/cancelled purchase order when user has staff role", func() {
				client := testutil.NewClient(tenv, models.RoleStaff)

				resp, err := client.MakeRequest("GET", "/api/v1/purchase-orders", nil,
					testutil.WithAuth(),
					testutil.WithParams(map[string]string{
						"q": uniqueSearchPrefix,
					}),
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				purchaseOrderResp := testutil.ParseResponse(resp)
				purchaseOrders := purchaseOrderResp["data"].([]interface{})
				Expect(purchaseOrders).To(HaveLen(3))
				for _, purchaseOrder := range purchaseOrders {
					purchaseOrderMap := purchaseOrder.(map[string]interface{})
					Expect(purchaseOrderMap["status"]).NotTo(Equal(string(models.PurchaseOrderStatusCompleted)))
					Expect(purchaseOrderMap["status"]).NotTo(Equal(string(models.PurchaseOrderStatusCancelled)))
				}
			})

			It("should filter by single status", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				resp, err := client.MakeRequest("GET", "/api/v1/purchase-orders", nil,
					testutil.WithAuth(),
					testutil.WithParams(map[string]string{
						"q":      uniqueSearchPrefix,
						"status": "order_placed",
					}),
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				purchaseOrderResp := testutil.ParseResponse(resp)
				purchaseOrders := purchaseOrderResp["data"].([]interface{})
				Expect(purchaseOrders).To(HaveLen(1))
				for _, purchaseOrder := range purchaseOrders {
					purchaseOrderMap := purchaseOrder.(map[string]interface{})
					Expect(purchaseOrderMap["status"]).To(Equal(string(models.PurchaseOrderStatusOrderPlaced)))
				}
			})

			It("should filter by multiple statuses (comma-separated)", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				resp, err := client.MakeRequest("GET", "/api/v1/purchase-orders", nil,
					testutil.WithAuth(),
					testutil.WithParams(map[string]string{
						"q":      uniqueSearchPrefix,
						"status": "order_placed,partially_delivered",
					}),
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				purchaseOrderResp := testutil.ParseResponse(resp)
				purchaseOrders := purchaseOrderResp["data"].([]interface{})
				Expect(purchaseOrders).To(HaveLen(2))
				for _, purchaseOrder := range purchaseOrders {
					purchaseOrderMap := purchaseOrder.(map[string]interface{})
					status := purchaseOrderMap["status"].(string)
					Expect(status).To(Or(
						Equal(string(models.PurchaseOrderStatusOrderPlaced)),
						Equal(string(models.PurchaseOrderStatusPartiallyDelivered)),
					))
				}
			})

			It("should filter by multiple statuses (comma-separated) - three statuses", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				resp, err := client.MakeRequest("GET", "/api/v1/purchase-orders", nil,
					testutil.WithAuth(),
					testutil.WithParams(map[string]string{
						"q":      uniqueSearchPrefix,
						"status": "fully_delivered,completed,cancelled",
					}),
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				purchaseOrderResp := testutil.ParseResponse(resp)
				purchaseOrders := purchaseOrderResp["data"].([]interface{})
				Expect(purchaseOrders).To(HaveLen(3))
				for _, purchaseOrder := range purchaseOrders {
					purchaseOrderMap := purchaseOrder.(map[string]interface{})
					status := purchaseOrderMap["status"].(string)
					Expect(status).To(Or(
						Equal(string(models.PurchaseOrderStatusFullyDelivered)),
						Equal(string(models.PurchaseOrderStatusCompleted)),
						Equal(string(models.PurchaseOrderStatusCancelled)),
					))
				}
			})

			It("should search purchase orders by supplier name", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				// Create a supplier with a unique name for searching
				uniqueSupplierName := "Supplier-Search-Test-" + uuid.New().String()
				searchSupplier := fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
					Name: uniqueSupplierName,
				})

				// Create another supplier that should not match
				otherSupplier := fixture.WithSupplier(tenv.ContextfulDB(), models.Supplier{
					Name: "Other-Supplier-" + uuid.New().String(),
				})

				// Create purchase orders with the search supplier
				searchPurchaseOrders := fixture.WithPurchaseOrders(tenv.ContextfulDB(), []*models.PurchaseOrder{
					{
						Status:      models.PurchaseOrderStatusOrderPlaced,
						InventoryID: &testInventory.ID,
						OrderNumber: "PO-" + uuid.New().String(),
						Items: []*models.PurchaseOrderItem{
							{
								ProductID:  &testProduct.ID,
								SupplierID: &searchSupplier.ID,
								UnitID:     &testUnit.ID,
								Quantity:   decimal.NewFromInt(1),
								UnitPrice:  100,
							},
						},
					},
					{
						Status:      models.PurchaseOrderStatusPartiallyDelivered,
						InventoryID: &testInventory.ID,
						OrderNumber: "PO-" + uuid.New().String(),
						Items: []*models.PurchaseOrderItem{
							{
								ProductID:  &testProduct.ID,
								SupplierID: &searchSupplier.ID,
								UnitID:     &testUnit.ID,
								Quantity:   decimal.NewFromInt(1),
								UnitPrice:  100,
							},
						},
					},
				})

				// Create a purchase order with a different supplier (should not be returned)
				fixture.WithPurchaseOrder(tenv.ContextfulDB(), models.PurchaseOrder{
					Status:      models.PurchaseOrderStatusOrderPlaced,
					InventoryID: &testInventory.ID,
					OrderNumber: "PO-" + uuid.New().String(),
					Items: []*models.PurchaseOrderItem{
						{
							ProductID:  &testProduct.ID,
							SupplierID: &otherSupplier.ID,
							UnitID:     &testUnit.ID,
							Quantity:   decimal.NewFromInt(1),
							UnitPrice:  100,
						},
					},
				})

				// Search by supplier name
				resp, err := client.MakeRequest("GET", "/api/v1/purchase-orders", nil,
					testutil.WithAuth(),
					testutil.WithParams(map[string]string{
						"q": uniqueSupplierName,
					}),
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				purchaseOrderResp := testutil.ParseResponse(resp)
				purchaseOrders := purchaseOrderResp["data"].([]interface{})
				Expect(purchaseOrders).To(HaveLen(len(searchPurchaseOrders)))

				// Verify all returned purchase orders have the correct supplier
				for _, purchaseOrder := range purchaseOrders {
					purchaseOrderMap := purchaseOrder.(map[string]interface{})
					items := purchaseOrderMap["items"].([]interface{})
					Expect(items).To(HaveLen(1))
					item := items[0].(map[string]interface{})
					supplierID := uint(item["supplier_id"].(float64))
					Expect(supplierID).To(Equal(searchSupplier.ID))
				}
			})
		})
	})
})
