package apptest

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/shopspring/decimal"
)

var _ = Describe("Purchase Order API", func() {
	Describe("Create Purchase Order", func() {
		var testSuppliers []models.Supplier
		var testBaseUnit *models.Unit
		var testDerivedUnit *models.Unit
		var testProducts []models.Product
		var testInventory *models.Inventory

		BeforeEach(func() {
			// Create suppliers
			supplier1 := tenv.WithSupplier(models.Supplier{
				Name: fmt.Sprintf("Test Supplier 1 %s", uuid.New().String()),
			})
			supplier2 := tenv.WithSupplier(models.Supplier{
				Name: fmt.Sprintf("Test Supplier 2 %s", uuid.New().String()),
			})
			testSuppliers = []models.Supplier{*supplier1, *supplier2}

			// Create units
			testBaseUnit = tenv.WithUnit(models.Unit{
				Name:             fmt.Sprintf("Test Base Unit %s", uuid.New().String()),
				Symbol:           "BU",
				UnitType:         "length",
				ConversionFactor: 1,
			})
			testDerivedUnit = tenv.WithUnit(models.Unit{
				Name:             fmt.Sprintf("Test Derived Unit %s", uuid.New().String()),
				Symbol:           "DU",
				UnitType:         "length",
				ConversionFactor: 10,
				BaseUnitID:       pkg.Ptr(testBaseUnit.ID),
			})

			// Create products
			testProducts = tenv.WithProducts([]models.Product{
				{
					Name:   fmt.Sprintf("Test Product 1 %s", uuid.New().String()),
					UnitID: testBaseUnit.ID,
				},
				{
					Name:   fmt.Sprintf("Test Product 2 %s", uuid.New().String()),
					UnitID: testBaseUnit.ID,
				},
			})

			// Create inventory
			testInventory = tenv.WithInventory(models.Inventory{
				Name: fmt.Sprintf("Test Inventory 1 %s", uuid.New().String()),
			})
		})

		Context("when user has authorized role", func() {
			It("should create purchase order with admin role", func() {
				client := NewClient(tenv, models.RoleAdmin)

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

				resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders", purchaseOrderData, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				purchaseOrderResp := ParseResponse(resp)
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
				client := NewClient(tenv, models.RoleAccountant)

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

				resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders", purchaseOrderData, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				purchaseOrderResp := ParseResponse(resp)
				Expect(purchaseOrderResp["status"]).To(Equal("order_placed"))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not create purchase order with staff role", func() {
				client := NewClient(tenv, models.RoleStaff)

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

				resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders", purchaseOrderData, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot create purchase-orders", models.RoleStaff)))
			})

			It("should not create purchase order with bot form role", func() {
				client := NewClient(tenv, models.RoleBotForm)

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

				resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders", purchaseOrderData, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := ParseResponse(resp)
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
			baseUnit := tenv.WithUnit(models.Unit{
				Name:             fmt.Sprintf("Base Unit %s", uuid.New().String()),
				Symbol:           "bu",
				UnitType:         "general",
				Level:            1,
				ConversionFactor: 1,
			})
			unit2 := tenv.WithUnit(models.Unit{
				Name:             fmt.Sprintf("Derived Unit 1 %s", uuid.New().String()),
				Symbol:           "du1",
				UnitType:         "general",
				Level:            2,
				ConversionFactor: 2,
				BaseUnitID:       pkg.Ptr(baseUnit.ID),
			})
			unit3 := tenv.WithUnit(models.Unit{
				Name:             fmt.Sprintf("Derived Unit 2 %s", uuid.New().String()),
				Symbol:           "du2",
				UnitType:         "general",
				Level:            3,
				ConversionFactor: 4,
				BaseUnitID:       pkg.Ptr(unit2.ID),
			})
			testUnits = []*models.Unit{baseUnit, unit2, unit3}

			testSupplier = tenv.WithSupplier(models.Supplier{
				Name: fmt.Sprintf("Test Supplier %s", uuid.New().String()),
			})

			testProduct = tenv.WithProduct(models.Product{
				Name:   fmt.Sprintf("Test Product %s", uuid.New().String()),
				UnitID: baseUnit.ID,
			})

			testInventory = tenv.WithInventory(models.Inventory{
				Name: fmt.Sprintf("Test Inventory %s", uuid.New().String()),
			})
		})

		It("should create purchase order with different units and same base unit", func() {
			client := NewClient(tenv, models.RoleAdmin)

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

			resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders", payload, WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(201))

			purchaseOrderResp := ParseResponse(resp)
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
			client := NewClient(tenv, models.RoleAdmin)

			differentBaseUnit := tenv.WithUnit(models.Unit{
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

			resp, err := client.MakeRequest("POST", "/api/v1/purchase-orders", payload, WithAuth())
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
			testUnit = tenv.WithUnit(models.Unit{
				Name:             fmt.Sprintf("Test Unit %s", uuid.New().String()),
				UnitType:         uuid.New().String(),
				ConversionFactor: 1,
			})

			testSupplier = tenv.WithSupplier(models.Supplier{
				Name: fmt.Sprintf("Test Supplier %s", uuid.New().String()),
			})

			testProduct = tenv.WithProduct(models.Product{
				Name:   fmt.Sprintf("Test Product %s", uuid.New().String()),
				UnitID: testUnit.ID,
			})

			testInventory = tenv.WithInventory(models.Inventory{
				Name: fmt.Sprintf("Test Inventory %s", uuid.New().String()),
			})
		})

		Context("when user has authorized role", func() {
			It("should cancel order_placed purchase order with admin role", func(ctx SpecContext) {
				client := NewClient(tenv, models.RoleAdmin)

				testPurchaseOrder := tenv.WithPurchaseOrder(models.PurchaseOrder{
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusOrderPlaced,
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
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "cancelled"}, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updateResp := ParseResponse(resp)
				Expect(updateResp["message"]).To(Equal("Purchase order status updated successfully"))

				// Verify in database
				var purchaseOrder models.PurchaseOrder
				err = tenv.DB.WithContext(ctx).First(&purchaseOrder, "id = ?", testPurchaseOrder.ID).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(purchaseOrder.Status).To(Equal(models.PurchaseOrderStatusCancelled))
			})

			It("should cancel order_placed purchase order with accountant role", func() {
				client := NewClient(tenv, models.RoleAccountant)

				testPurchaseOrder := tenv.WithPurchaseOrder(models.PurchaseOrder{
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusOrderPlaced,
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
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "cancelled"}, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))
			})

			It("should not change completed purchase order to cancelled", func(ctx SpecContext) {
				client := NewClient(tenv, models.RoleAdmin)

				testPurchaseOrder := tenv.WithPurchaseOrder(models.PurchaseOrder{
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusCompleted,
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
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "cancelled"}, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Verify status remains completed (immutable)
				var purchaseOrder models.PurchaseOrder
				err = tenv.DB.WithContext(ctx).First(&purchaseOrder, "id = ?", testPurchaseOrder.ID).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(purchaseOrder.Status).To(Equal(models.PurchaseOrderStatusCompleted))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not cancel purchase order with staff role", func() {
				client := NewClient(tenv, models.RoleStaff)

				testPurchaseOrder := tenv.WithPurchaseOrder(models.PurchaseOrder{
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusOrderPlaced,
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
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "cancelled"}, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update purchase-orders", models.RoleStaff)))
			})

			It("should not cancel purchase order with bot form role", func() {
				client := NewClient(tenv, models.RoleBotForm)

				testPurchaseOrder := tenv.WithPurchaseOrder(models.PurchaseOrder{
					OrderNumber: uuid.New().String(),
					Status:      models.PurchaseOrderStatusOrderPlaced,
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
				resp, err := client.MakeRequest("PUT", urlPath, map[string]interface{}{"status": "cancelled"}, WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update purchase-orders", models.RoleBotForm)))
			})
		})
	})
})
