package apptest

import (
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil"
	"cim-backend/pkg/testutil/fixture"
)

var _ = Describe("Sale Order API", func() {
	Describe("List Sale Orders", func() {
		var testInventory *models.Inventory
		var testMenuItem1, testMenuItem2 *models.MenuItem
		var testSaleOrders []*models.SaleOrder

		BeforeEach(func() {
			// Create test inventory
			testInventory = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name:     fmt.Sprintf("Test Inventory %s", uuid.New().String()),
				Location: "Location 1",
				Status:   models.InventoryStatusActive,
			})

			// Create test menu items
			testMenuItem1 = &models.MenuItem{
				Name: fmt.Sprintf("Test Menu Item 1 %s", uuid.New().String()),
			}
			err := tenv.ContextfulDB().Create(testMenuItem1).Error
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_item_id = ?", testMenuItem1.ID)
				tenv.ContextfulDB().Exec("DELETE FROM menu_item_products WHERE menu_item_id = ?", testMenuItem1.ID)
				tenv.ContextfulDB().Delete(testMenuItem1)
			})

			testMenuItem2 = &models.MenuItem{
				Name: fmt.Sprintf("Test Menu Item 2 %s", uuid.New().String()),
			}
			err = tenv.ContextfulDB().Create(testMenuItem2).Error
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_item_id = ?", testMenuItem2.ID)
				tenv.ContextfulDB().Exec("DELETE FROM menu_item_products WHERE menu_item_id = ?", testMenuItem2.ID)
				tenv.ContextfulDB().Delete(testMenuItem2)
			})

			// Create test sale orders with different tags
			testSaleOrders = []*models.SaleOrder{
				{
					CustomerID:  uuid.New().String()[:26],
					Tag:         1,
					OrderNumber: fmt.Sprintf("SO-TEST-001-%s", uuid.New().String()[:8]),
					InventoryID: pkg.Ptr(testInventory.ID),
					Status:      models.SaleOrderStatusOrdered,
					IsLatest:    true,
				},
				{
					CustomerID:  uuid.New().String()[:26],
					Tag:         2,
					OrderNumber: fmt.Sprintf("SO-TEST-002-%s", uuid.New().String()[:8]),
					InventoryID: pkg.Ptr(testInventory.ID),
					Status:      models.SaleOrderStatusServed,
					IsLatest:    true,
				},
				{
					CustomerID:  uuid.New().String()[:26],
					Tag:         1,
					OrderNumber: fmt.Sprintf("SO-TEST-003-%s", uuid.New().String()[:8]),
					InventoryID: pkg.Ptr(testInventory.ID),
					Status:      models.SaleOrderStatusOrdered,
					IsLatest:    true,
				},
			}

			ctx := pkg.WithUserEmail(tenv.DefaultContext, "test@cim.local")
			// Create sale orders first
			for _, so := range testSaleOrders {
				err := tenv.ContextfulDB().WithContext(ctx).Create(so).Error
				Expect(err).NotTo(HaveOccurred())
			}

			// Then create sale order items with menu items
			item1 := &models.SaleOrderItem{
				SaleOrderID: pkg.Ptr(testSaleOrders[0].ID),
				MenuItems:   []*models.MenuItem{testMenuItem1},
			}
			err = tenv.ContextfulDB().WithContext(ctx).Create(item1).Error
			Expect(err).NotTo(HaveOccurred())

			item2 := &models.SaleOrderItem{
				SaleOrderID: pkg.Ptr(testSaleOrders[1].ID),
				MenuItems:   []*models.MenuItem{testMenuItem2},
			}
			err = tenv.ContextfulDB().WithContext(ctx).Create(item2).Error
			Expect(err).NotTo(HaveOccurred())

			item3 := &models.SaleOrderItem{
				SaleOrderID: pkg.Ptr(testSaleOrders[2].ID),
				MenuItems:   []*models.MenuItem{testMenuItem1},
			}
			err = tenv.ContextfulDB().WithContext(ctx).Create(item3).Error
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				// Clean up sale orders first (before inventory cleanup)
				// Delete all sale orders that reference this inventory to avoid FK constraint
				tenv.ContextfulDB().Exec("DELETE FROM sale_order_item_menu_items WHERE sale_order_item_id IN (SELECT id FROM sale_order_items WHERE sale_order_id IN (SELECT id FROM sale_orders WHERE inventory_id = ?))", testInventory.ID)
				tenv.ContextfulDB().Exec("DELETE FROM sale_order_items WHERE sale_order_id IN (SELECT id FROM sale_orders WHERE inventory_id = ?)", testInventory.ID)
				tenv.ContextfulDB().Exec("DELETE FROM sale_orders WHERE inventory_id = ?", testInventory.ID)
			})
		})

		Context("when user has authorized role", func() {
			It("should list sale orders by pagination request/response", func() {
				client := testutil.NewClient(tenv, models.RoleRestaurantAdmin)

				resp, err := client.MakeRequest("GET", "/api/v1/sale-orders?page=1&limit=20", nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				saleOrdersResp := testutil.ParseResponse(resp)
				Expect(saleOrdersResp["data"]).NotTo(BeNil())
				Expect(saleOrdersResp["total"]).NotTo(BeNil())
				Expect(saleOrdersResp["page"]).To(Equal(float64(1)))
				Expect(saleOrdersResp["limit"]).To(Equal(float64(20)))
				Expect(saleOrdersResp["totalPages"]).NotTo(BeNil())

				data := saleOrdersResp["data"].([]interface{})
				Expect(len(data)).To(BeNumerically(">=", 3))
			})

			It("should list sale orders by tag", func() {
				client := testutil.NewClient(tenv, models.RoleRestaurantAdmin)

				resp, err := client.MakeRequest("GET", "/api/v1/sale-orders?tag=1", nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				saleOrdersResp := testutil.ParseResponse(resp)
				Expect(saleOrdersResp["data"]).NotTo(BeNil())

				data := saleOrdersResp["data"].([]interface{})
				Expect(len(data)).To(Equal(2)) // Should have 2 orders with tag=1

				// Verify all returned orders have tag=1
				for _, order := range data {
					orderMap := order.(map[string]interface{})
					Expect(orderMap["tag"]).To(Equal(float64(1)))
				}
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not list sale orders with unauthorized role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				resp, err := client.MakeRequest("GET", "/api/v1/sale-orders?page=1&limit=20", nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).NotTo(BeNil())
			})
		})
	})

	Describe("Update Sale Order", func() {
		var testInventory *models.Inventory
		var testMenuItem *models.MenuItem
		var testSaleOrder *models.SaleOrder

		BeforeEach(func() {
			// Create test inventory
			testInventory = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name:     fmt.Sprintf("Test Inventory %s", uuid.New().String()),
				Location: "Location 1",
				Status:   models.InventoryStatusActive,
			})

			// Create test menu item
			testMenuItem = &models.MenuItem{
				Name: fmt.Sprintf("Test Menu Item %s", uuid.New().String()),
			}
			err := tenv.ContextfulDB().Create(testMenuItem).Error
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_item_id = ?", testMenuItem.ID)
				tenv.ContextfulDB().Exec("DELETE FROM menu_item_products WHERE menu_item_id = ?", testMenuItem.ID)
				tenv.ContextfulDB().Delete(testMenuItem)
			})

			// Create test sale order
			testSaleOrder = &models.SaleOrder{
				CustomerID:  uuid.New().String()[:26],
				Tag:         1,
				OrderNumber: fmt.Sprintf("SO-TEST-UPDATE-%s", uuid.New().String()[:8]),
				InventoryID: pkg.Ptr(testInventory.ID),
				Status:      models.SaleOrderStatusOrdered,
				IsLatest:    true,
				Notes:       "Original notes",
			}

			ctx := pkg.WithUserEmail(tenv.DefaultContext, "test@cim.local")
			err = tenv.ContextfulDB().WithContext(ctx).Create(testSaleOrder).Error
			Expect(err).NotTo(HaveOccurred())

			// Create sale order item with menu item
			testSaleOrderItem := &models.SaleOrderItem{
				SaleOrderID: pkg.Ptr(testSaleOrder.ID),
				MenuItems:   []*models.MenuItem{testMenuItem},
			}
			err = tenv.ContextfulDB().WithContext(ctx).Create(testSaleOrderItem).Error
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				// Clean up all sale orders (including new versions created during update) by inventory_id to avoid FK constraint
				tenv.ContextfulDB().Exec("DELETE FROM sale_order_item_menu_items WHERE sale_order_item_id IN (SELECT id FROM sale_order_items WHERE sale_order_id IN (SELECT id FROM sale_orders WHERE inventory_id = ?))", testInventory.ID)
				tenv.ContextfulDB().Exec("DELETE FROM sale_order_items WHERE sale_order_id IN (SELECT id FROM sale_orders WHERE inventory_id = ?)", testInventory.ID)
				tenv.ContextfulDB().Exec("DELETE FROM sale_orders WHERE inventory_id = ?", testInventory.ID)
			})
		})

		Context("when user has authorized role", func() {
			It("should update sale order by create new record referencing the previous one and check DB must exist 2 records", func() {
				client := testutil.NewClient(tenv, models.RoleRestaurantAdmin)

				updateData := map[string]interface{}{
					"status": "completed",
					"notes":  "Updated notes",
					"items": []map[string]interface{}{
						{
							"menu_item_ids": []uint{testMenuItem.ID},
						},
					},
				}

				resp, err := client.MakeRequest("PUT", fmt.Sprintf("/api/v1/sale-orders/%d", testSaleOrder.ID), updateData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedResp := testutil.ParseResponse(resp)
				Expect(updatedResp["id"]).NotTo(BeNil())
				Expect(updatedResp["order_number"]).To(Equal(testSaleOrder.OrderNumber))
				Expect(updatedResp["status"]).To(Equal("completed"))
				Expect(updatedResp["notes"]).To(Equal("Updated notes"))
				Expect(updatedResp["is_latest"]).To(Equal(true))
				Expect(updatedResp["previous_order_id"]).NotTo(BeNil())

				// Check DB: should have 2 records with same order_number
				var count int64
				err = tenv.ContextfulDB().Model(&models.SaleOrder{}).
					Where("order_number = ?", testSaleOrder.OrderNumber).
					Count(&count).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(int64(2)))

				// Check that old record has is_latest = false
				var oldOrder models.SaleOrder
				err = tenv.ContextfulDB().Where("id = ? AND is_latest = ?", testSaleOrder.ID, false).First(&oldOrder).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(oldOrder.IsLatest).To(BeFalse())

				// Check that new record has is_latest = true and previous_order_id points to old one
				var newOrder models.SaleOrder
				err = tenv.ContextfulDB().Where("id = ? AND is_latest = ?", uint(updatedResp["id"].(float64)), true).First(&newOrder).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(newOrder.IsLatest).To(BeTrue())
				Expect(newOrder.PreviousOrderID).NotTo(BeNil())
				Expect(*newOrder.PreviousOrderID).To(Equal(testSaleOrder.ID))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not update sale order with unauthorized role", func() {
				client := testutil.NewClient(tenv, models.RoleChef)

				// Send valid request format so it fails on authorization, not validation
				updateData := map[string]interface{}{
					"status": "served",
					"notes":  "Updated notes",
					"items": []map[string]interface{}{
						{
							"menu_item_ids": []uint{testMenuItem.ID},
						},
					},
				}

				resp, err := client.MakeRequest("PUT", fmt.Sprintf("/api/v1/sale-orders/%d", testSaleOrder.ID), updateData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).NotTo(BeNil())
			})
		})
	})

	Describe("Cancel Sale Order", func() {
		var testInventory *models.Inventory
		var testMenuItem *models.MenuItem
		var testSaleOrder *models.SaleOrder

		BeforeEach(func() {
			// Create test inventory
			testInventory = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name:     fmt.Sprintf("Test Inventory %s", uuid.New().String()),
				Location: "Location 1",
				Status:   models.InventoryStatusActive,
			})

			// Create test menu item
			testMenuItem = &models.MenuItem{
				Name: fmt.Sprintf("Test Menu Item %s", uuid.New().String()),
			}
			err := tenv.ContextfulDB().Create(testMenuItem).Error
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_item_id = ?", testMenuItem.ID)
				tenv.ContextfulDB().Exec("DELETE FROM menu_item_products WHERE menu_item_id = ?", testMenuItem.ID)
				tenv.ContextfulDB().Delete(testMenuItem)
			})

			// Create test sale order
			testSaleOrder = &models.SaleOrder{
				CustomerID:  uuid.New().String()[:26],
				Tag:         1,
				OrderNumber: fmt.Sprintf("SO-TEST-CANCEL-%s", uuid.New().String()[:8]),
				InventoryID: pkg.Ptr(testInventory.ID),
				Status:      models.SaleOrderStatusOrdered,
				IsLatest:    true,
				Notes:       "Original notes",
			}

			ctx := pkg.WithUserEmail(tenv.DefaultContext, "test@cim.local")
			err = tenv.ContextfulDB().WithContext(ctx).Create(testSaleOrder).Error
			Expect(err).NotTo(HaveOccurred())

			// Create sale order item with menu item
			testSaleOrderItem := &models.SaleOrderItem{
				SaleOrderID: pkg.Ptr(testSaleOrder.ID),
				MenuItems:   []*models.MenuItem{testMenuItem},
			}
			err = tenv.ContextfulDB().WithContext(ctx).Create(testSaleOrderItem).Error
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				// Clean up sale orders by inventory_id to avoid FK constraint
				tenv.ContextfulDB().Exec("DELETE FROM sale_order_item_menu_items WHERE sale_order_item_id IN (SELECT id FROM sale_order_items WHERE sale_order_id IN (SELECT id FROM sale_orders WHERE inventory_id = ?))", testInventory.ID)
				tenv.ContextfulDB().Exec("DELETE FROM sale_order_items WHERE sale_order_id IN (SELECT id FROM sale_orders WHERE inventory_id = ?)", testInventory.ID)
				tenv.ContextfulDB().Exec("DELETE FROM sale_orders WHERE inventory_id = ?", testInventory.ID)
			})
		})

		Context("when user has authorized role", func() {
			It("should cancel sale order and check DB must exist only 1 record", func() {
				client := testutil.NewClient(tenv, models.RoleRestaurantAdmin)

				// Cancel the order by updating status to cancelled
				// Note: This assumes UpdateSaleOrder handles cancellation differently (doesn't create new record)
				// If there's a separate cancel endpoint, use that instead
				cancelData := map[string]interface{}{
					"status": "cancelled",
					"notes":  "Cancelled order",
					"items": []map[string]interface{}{
						{
							"menu_item_ids": []uint{testMenuItem.ID},
						},
					},
				}

				resp, err := client.MakeRequest("PUT", fmt.Sprintf("/api/v1/sale-orders/%d", testSaleOrder.ID), cancelData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Check DB: should have only 1 record (no new version created for cancellation)
				var count int64
				err = tenv.ContextfulDB().Model(&models.SaleOrder{}).
					Where("order_number = ?", testSaleOrder.OrderNumber).
					Count(&count).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(int64(1)))

				// Verify the order status is cancelled
				var cancelledOrder models.SaleOrder
				err = tenv.ContextfulDB().Where("id = ?", testSaleOrder.ID).First(&cancelledOrder).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(cancelledOrder.Status).To(Equal(models.SaleOrderStatusCancelled))
			})
		})
	})

	Describe("Update Sale Order Status to Served", func() {
		var testInventory *models.Inventory
		var testMenuItem *models.MenuItem
		var testSaleOrder *models.SaleOrder

		BeforeEach(func() {
			// Create test inventory
			testInventory = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name:     fmt.Sprintf("Test Inventory %s", uuid.New().String()),
				Location: "Location 1",
				Status:   models.InventoryStatusActive,
			})

			// Create test menu item
			testMenuItem = &models.MenuItem{
				Name: fmt.Sprintf("Test Menu Item %s", uuid.New().String()),
			}
			err := tenv.ContextfulDB().Create(testMenuItem).Error
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_item_id = ?", testMenuItem.ID)
				tenv.ContextfulDB().Exec("DELETE FROM menu_item_products WHERE menu_item_id = ?", testMenuItem.ID)
				tenv.ContextfulDB().Delete(testMenuItem)
			})

			// Create test sale order
			testSaleOrder = &models.SaleOrder{
				CustomerID:  uuid.New().String()[:26],
				Tag:         1,
				OrderNumber: fmt.Sprintf("SO-TEST-SERVED-%s", uuid.New().String()[:8]),
				InventoryID: pkg.Ptr(testInventory.ID),
				Status:      models.SaleOrderStatusOrdered,
				IsLatest:    true,
				Notes:       "Original notes",
			}

			ctx := pkg.WithUserEmail(tenv.DefaultContext, "test@cim.local")
			err = tenv.ContextfulDB().WithContext(ctx).Create(testSaleOrder).Error
			Expect(err).NotTo(HaveOccurred())

			// Create sale order item with menu item
			testSaleOrderItem := &models.SaleOrderItem{
				SaleOrderID: pkg.Ptr(testSaleOrder.ID),
				MenuItems:   []*models.MenuItem{testMenuItem},
			}
			err = tenv.ContextfulDB().WithContext(ctx).Create(testSaleOrderItem).Error
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				// Clean up sale orders by inventory_id to avoid FK constraint
				tenv.ContextfulDB().Exec("DELETE FROM sale_order_item_menu_items WHERE sale_order_item_id IN (SELECT id FROM sale_order_items WHERE sale_order_id IN (SELECT id FROM sale_orders WHERE inventory_id = ?))", testInventory.ID)
				tenv.ContextfulDB().Exec("DELETE FROM sale_order_items WHERE sale_order_id IN (SELECT id FROM sale_orders WHERE inventory_id = ?)", testInventory.ID)
				tenv.ContextfulDB().Exec("DELETE FROM sale_orders WHERE inventory_id = ?", testInventory.ID)
			})
		})

		Context("when user has authorized role", func() {
			It("should update sale order status to served and check DB must exist only 1 record", func() {
				client := testutil.NewClient(tenv, models.RoleRestaurantAdmin)

				// Update status to served using UpdateSaleOrder endpoint
				updateData := map[string]interface{}{
					"status": "served",
					"notes":  "Order served",
					"items": []map[string]interface{}{
						{
							"menu_item_ids": []uint{testMenuItem.ID},
						},
					},
				}

				resp, err := client.MakeRequest("PUT", fmt.Sprintf("/api/v1/sale-orders/%d", testSaleOrder.ID), updateData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedResp := testutil.ParseResponse(resp)
				Expect(updatedResp["id"]).NotTo(BeNil())
				Expect(updatedResp["order_number"]).To(Equal(testSaleOrder.OrderNumber))
				Expect(updatedResp["status"]).To(Equal("served"))
				Expect(updatedResp["notes"]).To(Equal("Order served"))
				Expect(updatedResp["is_latest"]).To(Equal(true))

				// Check DB: should have only 1 record (no new version created for served status)
				var count int64
				err = tenv.ContextfulDB().Model(&models.SaleOrder{}).
					Where("order_number = ?", testSaleOrder.OrderNumber).
					Count(&count).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(int64(1)))

				// Verify the order status is served
				var servedOrder models.SaleOrder
				err = tenv.ContextfulDB().Where("id = ?", testSaleOrder.ID).First(&servedOrder).Error
				Expect(err).NotTo(HaveOccurred())
				Expect(servedOrder.Status).To(Equal(models.SaleOrderStatusServed))
				Expect(servedOrder.Notes).To(Equal("Order served"))
			})
		})
	})

	Describe("Get Sale Order", func() {
		var testInventory *models.Inventory
		var testMenuItem *models.MenuItem
		var testSaleOrder *models.SaleOrder

		BeforeEach(func() {
			// Create test inventory
			testInventory = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name:     fmt.Sprintf("Test Inventory %s", uuid.New().String()),
				Location: "Location 1",
				Status:   models.InventoryStatusActive,
			})

			// Create test menu item
			testMenuItem = &models.MenuItem{
				Name: fmt.Sprintf("Test Menu Item %s", uuid.New().String()),
			}
			err := tenv.ContextfulDB().Create(testMenuItem).Error
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_item_id = ?", testMenuItem.ID)
				tenv.ContextfulDB().Exec("DELETE FROM menu_item_products WHERE menu_item_id = ?", testMenuItem.ID)
				tenv.ContextfulDB().Delete(testMenuItem)
			})

			// Create test sale order
			testSaleOrder = &models.SaleOrder{
				CustomerID:  uuid.New().String()[:26],
				Tag:         1,
				OrderNumber: fmt.Sprintf("SO-TEST-GET-%s", uuid.New().String()[:8]),
				InventoryID: pkg.Ptr(testInventory.ID),
				Status:      models.SaleOrderStatusOrdered,
				IsLatest:    true,
				Notes:       "Test notes",
			}

			ctx := pkg.WithUserEmail(tenv.DefaultContext, "test@cim.local")
			err = tenv.ContextfulDB().WithContext(ctx).Create(testSaleOrder).Error
			Expect(err).NotTo(HaveOccurred())

			// Create sale order item with menu item
			testSaleOrderItem := &models.SaleOrderItem{
				SaleOrderID: pkg.Ptr(testSaleOrder.ID),
				MenuItems:   []*models.MenuItem{testMenuItem},
			}
			err = tenv.ContextfulDB().WithContext(ctx).Create(testSaleOrderItem).Error
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				// Clean up sale orders by inventory_id to avoid FK constraint
				tenv.ContextfulDB().Exec("DELETE FROM sale_order_item_menu_items WHERE sale_order_item_id IN (SELECT id FROM sale_order_items WHERE sale_order_id IN (SELECT id FROM sale_orders WHERE inventory_id = ?))", testInventory.ID)
				tenv.ContextfulDB().Exec("DELETE FROM sale_order_items WHERE sale_order_id IN (SELECT id FROM sale_orders WHERE inventory_id = ?)", testInventory.ID)
				tenv.ContextfulDB().Exec("DELETE FROM sale_orders WHERE inventory_id = ?", testInventory.ID)
			})
		})

		Context("when user has authorized role", func() {
			It("should get sale order by ID", func() {
				client := testutil.NewClient(tenv, models.RoleRestaurantAdmin)

				resp, err := client.MakeRequest("GET", fmt.Sprintf("/api/v1/sale-orders/%d", testSaleOrder.ID), nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				saleOrderResp := testutil.ParseResponse(resp)
				Expect(saleOrderResp["id"]).NotTo(BeNil())
				Expect(saleOrderResp["order_number"]).To(Equal(testSaleOrder.OrderNumber))
				Expect(saleOrderResp["status"]).To(Equal("ordered"))
				Expect(saleOrderResp["notes"]).To(Equal("Test notes"))
				Expect(saleOrderResp["tag"]).To(Equal(float64(1)))
				Expect(saleOrderResp["customer_id"]).NotTo(BeNil())
				Expect(saleOrderResp["inventory"]).NotTo(BeNil())
				Expect(saleOrderResp["items"]).NotTo(BeNil())
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not get sale order by ID with unauthorized role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				resp, err := client.MakeRequest("GET", fmt.Sprintf("/api/v1/sale-orders/%d", testSaleOrder.ID), nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).NotTo(BeNil())
			})
		})
	})
})
