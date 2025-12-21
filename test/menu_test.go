package apptest

import (
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"

	"cim-backend/internal/models"
	"cim-backend/pkg/testutil"
	"cim-backend/pkg/testutil/fixture"
)

var _ = Describe("Menu API", func() {
	Describe("Create Menu", func() {
		var testMenuItem1, testMenuItem2 *models.MenuItem
		var testInventory1, testInventory2 *models.Inventory

		BeforeEach(func() {
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

			// Create test inventories
			testInventory1 = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name:     fmt.Sprintf("Test Inventory 1 %s", uuid.New().String()),
				Location: "Location 1",
				Status:   models.InventoryStatusActive,
			})

			testInventory2 = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name:     fmt.Sprintf("Test Inventory 2 %s", uuid.New().String()),
				Location: "Location 2",
				Status:   models.InventoryStatusActive,
			})
		})

		Context("when user has authorized role", func() {
			It("should create menu with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)
				menuName := fmt.Sprintf("Test Menu %s", uuid.New().String())

				menuData := map[string]interface{}{
					"name":          menuName,
					"menu_item_ids": []uint{testMenuItem1.ID, testMenuItem2.ID},
					"inventory_ids": []uint{testInventory1.ID, testInventory2.ID},
				}

				resp, err := client.MakeRequest("POST", "/api/v1/menus", menuData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				menuResp := testutil.ParseResponse(resp)
				Expect(menuResp["id"]).NotTo(BeNil())
				Expect(menuResp["name"]).To(Equal(menuName))

				// Verify relationships
				menuItems := menuResp["menu_items"].([]interface{})
				Expect(menuItems).To(HaveLen(2))

				inventories := menuResp["inventories"].([]interface{})
				Expect(inventories).To(HaveLen(2))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not create menu with non-admin role", func() {
				client := testutil.NewClient(tenv, models.RoleCashier)

				menuData := map[string]interface{}{
					"name":          "Test Menu",
					"menu_item_ids": []uint{testMenuItem1.ID},
					"inventory_ids": []uint{testInventory1.ID},
				}

				resp, err := client.MakeRequest("POST", "/api/v1/menus", menuData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot create menus", models.RoleCashier)))
			})
		})
	})

	Describe("Get Menu", func() {
		var testMenu *models.Menu
		var testMenuItem *models.MenuItem
		var testInventory *models.Inventory

		BeforeEach(func() {
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

			// Create test inventory
			testInventory = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name:     fmt.Sprintf("Test Inventory %s", uuid.New().String()),
				Location: "Location",
				Status:   models.InventoryStatusActive,
			})

			// Create test menu with relationships
			testMenu = &models.Menu{
				Name:        fmt.Sprintf("Test Menu %s", uuid.New().String()),
				MenuItems:   []*models.MenuItem{testMenuItem},
				Inventories: []*models.Inventory{testInventory},
			}
			err = tenv.ContextfulDB().Create(testMenu).Error
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_id = ?", testMenu.ID)
				tenv.ContextfulDB().Exec("DELETE FROM menu_inventories WHERE menu_id = ?", testMenu.ID)
				tenv.ContextfulDB().Delete(testMenu)
			})
		})

		Context("when user has authorized role", func() {
			It("should get menu with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				urlPath := fmt.Sprintf("/api/v1/menus/%d", testMenu.ID)
				resp, err := client.MakeRequest("GET", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				menuResp := testutil.ParseResponse(resp)
				Expect(menuResp["id"]).NotTo(BeNil())
				Expect(menuResp["name"]).To(Equal(testMenu.Name))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not get menu with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				urlPath := fmt.Sprintf("/api/v1/menus/%d", testMenu.ID)
				resp, err := client.MakeRequest("GET", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot view menus", models.RoleBotForm)))
			})
		})
	})

	Describe("List Menus", func() {
		var testMenu *models.Menu

		BeforeEach(func() {
			testMenu = &models.Menu{
				Name: fmt.Sprintf("Test Menu %s", uuid.New().String()),
			}
			err := tenv.ContextfulDB().Create(testMenu).Error
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_id = ?", testMenu.ID)
				tenv.ContextfulDB().Exec("DELETE FROM menu_inventories WHERE menu_id = ?", testMenu.ID)
				tenv.ContextfulDB().Delete(testMenu)
			})
		})

		Context("when user has authorized role", func() {
			It("should list menus with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				resp, err := client.MakeRequest("GET", "/api/v1/menus?page=1&limit=20", nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				menusResp := testutil.ParseResponse(resp)
				Expect(menusResp["data"]).NotTo(BeNil())
				Expect(menusResp["total"]).NotTo(BeNil())
				Expect(menusResp["page"]).To(Equal(float64(1)))
				Expect(menusResp["limit"]).To(Equal(float64(20)))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not list menus with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				resp, err := client.MakeRequest("GET", "/api/v1/menus?page=1&limit=20", nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot view menus", models.RoleBotForm)))
			})
		})
	})

	Describe("Update Menu", func() {
		var testMenu *models.Menu
		var testMenuItem1, testMenuItem2 *models.MenuItem
		var testInventory1, testInventory2 *models.Inventory

		BeforeEach(func() {
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

			// Create test inventories
			testInventory1 = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name:     fmt.Sprintf("Test Inventory 1 %s", uuid.New().String()),
				Location: "Location 1",
				Status:   models.InventoryStatusActive,
			})

			testInventory2 = fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
				Name:     fmt.Sprintf("Test Inventory 2 %s", uuid.New().String()),
				Location: "Location 2",
				Status:   models.InventoryStatusActive,
			})

			// Create test menu
			testMenu = &models.Menu{
				Name:        fmt.Sprintf("Test Menu %s", uuid.New().String()),
				MenuItems:   []*models.MenuItem{testMenuItem1},
				Inventories: []*models.Inventory{testInventory1},
			}
			err = tenv.ContextfulDB().Create(testMenu).Error
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_id = ?", testMenu.ID)
				tenv.ContextfulDB().Exec("DELETE FROM menu_inventories WHERE menu_id = ?", testMenu.ID)
				tenv.ContextfulDB().Delete(testMenu)
			})
		})

		Context("when user has authorized role", func() {
			It("should update menu with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)
				newMenuName := fmt.Sprintf("Updated Menu %s", uuid.New().String())

				updatedMenuData := map[string]interface{}{
					"name":          newMenuName,
					"menu_item_ids": []uint{testMenuItem1.ID, testMenuItem2.ID},
					"inventory_ids": []uint{testInventory2.ID},
				}

				urlPath := fmt.Sprintf("/api/v1/menus/%d", testMenu.ID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedMenuData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedMenuResp := testutil.ParseResponse(resp)
				Expect(updatedMenuResp["name"]).To(Equal(newMenuName))

				// Verify relationships
				menuItems := updatedMenuResp["menu_items"].([]interface{})
				Expect(menuItems).To(HaveLen(2))

				inventories := updatedMenuResp["inventories"].([]interface{})
				Expect(inventories).To(HaveLen(1))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not update menu with non-admin role", func() {
				client := testutil.NewClient(tenv, models.RoleCashier)

				updatedMenuData := map[string]interface{}{
					"name":          "Updated Name",
					"menu_item_ids": []uint{testMenuItem1.ID},
					"inventory_ids": []uint{testInventory1.ID},
				}

				urlPath := fmt.Sprintf("/api/v1/menus/%d", testMenu.ID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedMenuData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update menus", models.RoleCashier)))
			})
		})
	})

	Describe("Delete Menu", func() {
		var testMenu *models.Menu

		BeforeEach(func() {
			testMenu = &models.Menu{
				Name: fmt.Sprintf("Test Menu %s", uuid.New().String()),
			}
			err := tenv.ContextfulDB().Create(testMenu).Error
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_id = ?", testMenu.ID)
				tenv.ContextfulDB().Exec("DELETE FROM menu_inventories WHERE menu_id = ?", testMenu.ID)
				tenv.ContextfulDB().Delete(testMenu)
			})
		})

		Context("when user has authorized role", func() {
			It("should delete menu with admin role", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				urlPath := fmt.Sprintf("/api/v1/menus/%d", testMenu.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(204))

				// Verify deletion
				var deletedMenu models.Menu
				err = tenv.DB.WithContext(ctx).First(&deletedMenu, "id = ?", testMenu.ID).Error
				Expect(err).To(Equal(gorm.ErrRecordNotFound))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not delete menu with non-admin role", func() {
				client := testutil.NewClient(tenv, models.RoleCashier)

				urlPath := fmt.Sprintf("/api/v1/menus/%d", testMenu.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot delete menus", models.RoleCashier)))
			})
		})
	})
})

var _ = Describe("MenuItem API", func() {
	Describe("Create MenuItem", func() {
		var testProduct1, testProduct2 *models.Product
		var testUnit *models.Unit

		BeforeEach(func() {
			testUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				UnitType:         uuid.New().String(),
				Name:             fmt.Sprintf("Test Unit %s", uuid.New().String()),
				ConversionFactor: 1,
			})

			testProduct1 = fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Test Product 1 %s", uuid.New().String()),
				ProductType: "test",
				UnitID:      testUnit.ID,
				Status:      "active",
			})

			testProduct2 = fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Test Product 2 %s", uuid.New().String()),
				ProductType: "test",
				UnitID:      testUnit.ID,
				Status:      "active",
			})
		})

		Context("when user has authorized role", func() {
			It("should create menu item with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)
				menuItemName := fmt.Sprintf("Test Menu Item %s", uuid.New().String())

				menuItemData := map[string]interface{}{
					"name":        menuItemName,
					"product_ids": []uint{testProduct1.ID, testProduct2.ID},
					"menu_ids":    []uint{},
				}

				resp, err := client.MakeRequest("POST", "/api/v1/menu-items", menuItemData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				menuItemResp := testutil.ParseResponse(resp)
				Expect(menuItemResp["id"]).NotTo(BeNil())
				Expect(menuItemResp["name"]).To(Equal(menuItemName))

				// Verify relationships
				products := menuItemResp["products"].([]interface{})
				Expect(products).To(HaveLen(2))
			})

			It("should create menu item with tags", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)
				menuItemName := fmt.Sprintf("Test Menu Item with Tags %s", uuid.New().String())

				menuItemData := map[string]interface{}{
					"name":        menuItemName,
					"tags":        []string{"appetizer", "salad"},
					"product_ids": []uint{testProduct1.ID},
					"menu_ids":    []uint{},
				}

				resp, err := client.MakeRequest("POST", "/api/v1/menu-items", menuItemData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				menuItemResp := testutil.ParseResponse(resp)
				Expect(menuItemResp["id"]).NotTo(BeNil())
				Expect(menuItemResp["name"]).To(Equal(menuItemName))

				// Verify tags are returned
				tags, ok := menuItemResp["tags"].([]interface{})
				Expect(ok).To(BeTrue())
				Expect(tags).To(HaveLen(2))
				Expect(tags).To(ContainElement("appetizer"))
				Expect(tags).To(ContainElement("salad"))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not create menu item with non-admin role", func() {
				client := testutil.NewClient(tenv, models.RoleCashier)

				menuItemData := map[string]interface{}{
					"name":        "Test Menu Item",
					"product_ids": []uint{testProduct1.ID},
					"menu_ids":    []uint{},
				}

				resp, err := client.MakeRequest("POST", "/api/v1/menu-items", menuItemData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot create menu-items", models.RoleCashier)))
			})
		})
	})

	Describe("Get MenuItem", func() {
		var testMenuItem *models.MenuItem
		var testProduct *models.Product
		var testUnit *models.Unit

		BeforeEach(func() {
			testUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				UnitType:         uuid.New().String(),
				Name:             fmt.Sprintf("Test Unit %s", uuid.New().String()),
				ConversionFactor: 1,
			})

			testProduct = fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Test Product %s", uuid.New().String()),
				ProductType: "test",
				UnitID:      testUnit.ID,
				Status:      "active",
			})

			// Create test menu item with relationships
			testMenuItem = &models.MenuItem{
				Name:     fmt.Sprintf("Test Menu Item %s", uuid.New().String()),
				Products: []*models.Product{testProduct},
			}
			err := tenv.ContextfulDB().Create(testMenuItem).Error
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_item_id = ?", testMenuItem.ID)
				tenv.ContextfulDB().Exec("DELETE FROM menu_item_products WHERE menu_item_id = ?", testMenuItem.ID)
				tenv.ContextfulDB().Delete(testMenuItem)
			})
		})

		Context("when user has authorized role", func() {
			It("should get menu item with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				urlPath := fmt.Sprintf("/api/v1/menu-items/%d", testMenuItem.ID)
				resp, err := client.MakeRequest("GET", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				menuItemResp := testutil.ParseResponse(resp)
				Expect(menuItemResp["id"]).NotTo(BeNil())
				Expect(menuItemResp["name"]).To(Equal(testMenuItem.Name))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not get menu item with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				urlPath := fmt.Sprintf("/api/v1/menu-items/%d", testMenuItem.ID)
				resp, err := client.MakeRequest("GET", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot view menu-items", models.RoleBotForm)))
			})
		})
	})

	Describe("List MenuItems", func() {
		var testMenuItem *models.MenuItem

		BeforeEach(func() {
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
		})

		Context("when user has authorized role", func() {
			It("should list menu items with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				resp, err := client.MakeRequest("GET", "/api/v1/menu-items?page=1&limit=20", nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				menuItemsResp, err := testutil.ParseResponseArray(resp)
				Expect(err).NotTo(HaveOccurred())
				Expect(menuItemsResp).NotTo(BeEmpty())
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not list menu items with bot form role", func() {
				client := testutil.NewClient(tenv, models.RoleBotForm)

				resp, err := client.MakeRequest("GET", "/api/v1/menu-items?page=1&limit=20", nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot view menu-items", models.RoleBotForm)))
			})
		})

		Context("when filtering by tags", func() {
			var appetizerMenuItem, mainsMenuItem, dessertsMenuItem, multiTagMenuItem *models.MenuItem

			BeforeEach(func() {
				// Create menu item with appetizer tag
				appetizerMenuItem = &models.MenuItem{
					Name: fmt.Sprintf("Caesar Salad %s", uuid.New().String()),
					Tags: []string{"appetizer", "salad"},
				}
				err := tenv.ContextfulDB().Create(appetizerMenuItem).Error
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() {
					tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_item_id = ?", appetizerMenuItem.ID)
					tenv.ContextfulDB().Exec("DELETE FROM menu_item_products WHERE menu_item_id = ?", appetizerMenuItem.ID)
					tenv.ContextfulDB().Delete(appetizerMenuItem)
				})

				// Create menu item with mains tag
				mainsMenuItem = &models.MenuItem{
					Name: fmt.Sprintf("Grilled Steak %s", uuid.New().String()),
					Tags: []string{"mains"},
				}
				err = tenv.ContextfulDB().Create(mainsMenuItem).Error
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() {
					tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_item_id = ?", mainsMenuItem.ID)
					tenv.ContextfulDB().Exec("DELETE FROM menu_item_products WHERE menu_item_id = ?", mainsMenuItem.ID)
					tenv.ContextfulDB().Delete(mainsMenuItem)
				})

				// Create menu item with desserts tag
				dessertsMenuItem = &models.MenuItem{
					Name: fmt.Sprintf("Chocolate Cake %s", uuid.New().String()),
					Tags: []string{"desserts"},
				}
				err = tenv.ContextfulDB().Create(dessertsMenuItem).Error
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() {
					tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_item_id = ?", dessertsMenuItem.ID)
					tenv.ContextfulDB().Exec("DELETE FROM menu_item_products WHERE menu_item_id = ?", dessertsMenuItem.ID)
					tenv.ContextfulDB().Delete(dessertsMenuItem)
				})

				// Create menu item with multiple tags
				multiTagMenuItem = &models.MenuItem{
					Name: fmt.Sprintf("Chicken Salad %s", uuid.New().String()),
					Tags: []string{"appetizer", "mains", "salad"},
				}
				err = tenv.ContextfulDB().Create(multiTagMenuItem).Error
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() {
					tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_item_id = ?", multiTagMenuItem.ID)
					tenv.ContextfulDB().Exec("DELETE FROM menu_item_products WHERE menu_item_id = ?", multiTagMenuItem.ID)
					tenv.ContextfulDB().Delete(multiTagMenuItem)
				})
			})

			It("should filter menu items by single tag", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				resp, err := client.MakeRequest("GET", "/api/v1/menu-items?page=1&limit=20&tags=appetizer", nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				menuItemsResp, err := testutil.ParseResponseArray(resp)
				Expect(err).NotTo(HaveOccurred())

				// Should include items with appetizer tag
				foundAppetizer := false
				foundMains := false
				for _, item := range menuItemsResp {
					itemMap := item.(map[string]interface{})
					if itemMap["id"] != nil {
						id := uint(itemMap["id"].(float64))
						if id == appetizerMenuItem.ID {
							foundAppetizer = true
							// Verify tags are returned
							tags, ok := itemMap["tags"].([]interface{})
							Expect(ok).To(BeTrue())
							Expect(tags).To(ContainElement("appetizer"))
						}
						if id == mainsMenuItem.ID {
							foundMains = true
						}
					}
				}
				Expect(foundAppetizer).To(BeTrue(), "Should find appetizer menu item")
				Expect(foundMains).To(BeFalse(), "Should not find mains menu item when filtering by appetizer")
			})

			It("should filter menu items by multiple tags", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				resp, err := client.MakeRequest("GET", "/api/v1/menu-items?page=1&limit=20&tags=appetizer,mains", nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				menuItemsResp, err := testutil.ParseResponseArray(resp)
				Expect(err).NotTo(HaveOccurred())

				// Should include items with either appetizer or mains tag
				foundAppetizer := false
				foundMains := false
				foundDesserts := false
				for _, item := range menuItemsResp {
					itemMap := item.(map[string]interface{})
					if itemMap["id"] != nil {
						id := uint(itemMap["id"].(float64))
						if id == appetizerMenuItem.ID {
							foundAppetizer = true
						}
						if id == mainsMenuItem.ID {
							foundMains = true
						}
						if id == dessertsMenuItem.ID {
							foundDesserts = true
						}
					}
				}
				Expect(foundAppetizer).To(BeTrue(), "Should find appetizer menu item")
				Expect(foundMains).To(BeTrue(), "Should find mains menu item")
				Expect(foundDesserts).To(BeFalse(), "Should not find desserts menu item when filtering by appetizer,mains")
			})

			It("should return all menu items when no tag filter is provided", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				resp, err := client.MakeRequest("GET", "/api/v1/menu-items?page=1&limit=100", nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				menuItemsResp, err := testutil.ParseResponseArray(resp)
				Expect(err).NotTo(HaveOccurred())

				// Should include all items
				foundAppetizer := false
				foundMains := false
				foundDesserts := false
				for _, item := range menuItemsResp {
					itemMap := item.(map[string]interface{})
					if itemMap["id"] != nil {
						id := uint(itemMap["id"].(float64))
						if id == appetizerMenuItem.ID {
							foundAppetizer = true
						}
						if id == mainsMenuItem.ID {
							foundMains = true
						}
						if id == dessertsMenuItem.ID {
							foundDesserts = true
						}
					}
				}
				Expect(foundAppetizer).To(BeTrue(), "Should find appetizer menu item")
				Expect(foundMains).To(BeTrue(), "Should find mains menu item")
				Expect(foundDesserts).To(BeTrue(), "Should find desserts menu item")
			})

			It("should return menu items with tags in response", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				resp, err := client.MakeRequest("GET", fmt.Sprintf("/api/v1/menu-items/%d", appetizerMenuItem.ID), nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				menuItemResp := testutil.ParseResponse(resp)
				Expect(menuItemResp["id"]).NotTo(BeNil())
				Expect(menuItemResp["name"]).To(Equal(appetizerMenuItem.Name))

				// Verify tags are returned
				tags, ok := menuItemResp["tags"].([]interface{})
				Expect(ok).To(BeTrue())
				Expect(tags).To(HaveLen(2))
				Expect(tags).To(ContainElement("appetizer"))
				Expect(tags).To(ContainElement("salad"))
			})

			It("should filter menu items with multiple matching tags", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				// Filter by salad tag - should return both appetizer and multiTagMenuItem
				resp, err := client.MakeRequest("GET", "/api/v1/menu-items?page=1&limit=20&tags=salad", nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				menuItemsResp, err := testutil.ParseResponseArray(resp)
				Expect(err).NotTo(HaveOccurred())

				foundAppetizer := false
				foundMultiTag := false
				for _, item := range menuItemsResp {
					itemMap := item.(map[string]interface{})
					if itemMap["id"] != nil {
						id := uint(itemMap["id"].(float64))
						if id == appetizerMenuItem.ID {
							foundAppetizer = true
						}
						if id == multiTagMenuItem.ID {
							foundMultiTag = true
						}
					}
				}
				Expect(foundAppetizer).To(BeTrue(), "Should find appetizer menu item with salad tag")
				Expect(foundMultiTag).To(BeTrue(), "Should find multi-tag menu item with salad tag")
			})
		})
	})

	Describe("Update MenuItem", func() {
		var testMenuItem *models.MenuItem
		var testProduct1, testProduct2 *models.Product
		var testUnit *models.Unit
		var testMenu *models.Menu

		BeforeEach(func() {
			testUnit = fixture.WithUnit(tenv.ContextfulDB(), models.Unit{
				UnitType:         uuid.New().String(),
				Name:             fmt.Sprintf("Test Unit %s", uuid.New().String()),
				ConversionFactor: 1,
			})

			testProduct1 = fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Test Product 1 %s", uuid.New().String()),
				ProductType: "test",
				UnitID:      testUnit.ID,
				Status:      "active",
			})

			testProduct2 = fixture.WithProduct(tenv.ContextfulDB(), models.Product{
				Name:        fmt.Sprintf("Test Product 2 %s", uuid.New().String()),
				ProductType: "test",
				UnitID:      testUnit.ID,
				Status:      "active",
			})

			testMenu = &models.Menu{
				Name: fmt.Sprintf("Test Menu %s", uuid.New().String()),
			}
			err := tenv.ContextfulDB().Create(testMenu).Error
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_id = ?", testMenu.ID)
				tenv.ContextfulDB().Exec("DELETE FROM menu_inventories WHERE menu_id = ?", testMenu.ID)
				tenv.ContextfulDB().Delete(testMenu)
			})

			// Create test menu item
			testMenuItem = &models.MenuItem{
				Name:     fmt.Sprintf("Test Menu Item %s", uuid.New().String()),
				Products: []*models.Product{testProduct1},
			}
			err = tenv.ContextfulDB().Create(testMenuItem).Error
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_item_id = ?", testMenuItem.ID)
				tenv.ContextfulDB().Exec("DELETE FROM menu_item_products WHERE menu_item_id = ?", testMenuItem.ID)
				tenv.ContextfulDB().Delete(testMenuItem)
			})
		})

		Context("when user has authorized role", func() {
			It("should update menu item with admin role", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)
				newMenuItemName := fmt.Sprintf("Updated Menu Item %s", uuid.New().String())

				updatedMenuItemData := map[string]interface{}{
					"name":        newMenuItemName,
					"product_ids": []uint{testProduct1.ID, testProduct2.ID},
					"menu_ids":    []uint{testMenu.ID},
				}

				urlPath := fmt.Sprintf("/api/v1/menu-items/%d", testMenuItem.ID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedMenuItemData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedMenuItemResp := testutil.ParseResponse(resp)
				Expect(updatedMenuItemResp["name"]).To(Equal(newMenuItemName))

				// Verify relationships
				products := updatedMenuItemResp["products"].([]interface{})
				Expect(products).To(HaveLen(2))

				menus := updatedMenuItemResp["menus"].([]interface{})
				Expect(menus).To(HaveLen(1))
			})

			It("should update menu item with tags", func() {
				client := testutil.NewClient(tenv, models.RoleAdmin)
				newMenuItemName := fmt.Sprintf("Updated Menu Item with Tags %s", uuid.New().String())

				updatedMenuItemData := map[string]interface{}{
					"name":        newMenuItemName,
					"tags":        []string{"mains", "desserts"},
					"product_ids": []uint{testProduct1.ID},
					"menu_ids":    []uint{},
				}

				urlPath := fmt.Sprintf("/api/v1/menu-items/%d", testMenuItem.ID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedMenuItemData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedMenuItemResp := testutil.ParseResponse(resp)
				Expect(updatedMenuItemResp["name"]).To(Equal(newMenuItemName))

				// Verify tags are updated
				tags, ok := updatedMenuItemResp["tags"].([]interface{})
				Expect(ok).To(BeTrue())
				Expect(tags).To(HaveLen(2))
				Expect(tags).To(ContainElement("mains"))
				Expect(tags).To(ContainElement("desserts"))
			})

			It("should clear tags when empty array is provided", func() {
				// First create a menu item with tags
				menuItemWithTags := &models.MenuItem{
					Name: fmt.Sprintf("Menu Item with Tags %s", uuid.New().String()),
					Tags: []string{"appetizer", "salad"},
				}
				err := tenv.ContextfulDB().Create(menuItemWithTags).Error
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() {
					tenv.ContextfulDB().Exec("DELETE FROM menu_menu_items WHERE menu_item_id = ?", menuItemWithTags.ID)
					tenv.ContextfulDB().Exec("DELETE FROM menu_item_products WHERE menu_item_id = ?", menuItemWithTags.ID)
					tenv.ContextfulDB().Delete(menuItemWithTags)
				})

				client := testutil.NewClient(tenv, models.RoleAdmin)
				newMenuItemName := fmt.Sprintf("Updated Menu Item No Tags %s", uuid.New().String())

				updatedMenuItemData := map[string]interface{}{
					"name":        newMenuItemName,
					"tags":        []string{},
					"product_ids": []uint{},
					"menu_ids":    []uint{},
				}

				urlPath := fmt.Sprintf("/api/v1/menu-items/%d", menuItemWithTags.ID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedMenuItemData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				updatedMenuItemResp := testutil.ParseResponse(resp)
				Expect(updatedMenuItemResp["name"]).To(Equal(newMenuItemName))

				// Verify tags are cleared
				tags, ok := updatedMenuItemResp["tags"].([]interface{})
				if ok {
					Expect(tags).To(BeEmpty())
				}
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not update menu item with non-admin role", func() {
				client := testutil.NewClient(tenv, models.RoleCashier)

				updatedMenuItemData := map[string]interface{}{
					"name":        "Updated Name",
					"product_ids": []uint{testProduct1.ID},
					"menu_ids":    []uint{},
				}

				urlPath := fmt.Sprintf("/api/v1/menu-items/%d", testMenuItem.ID)
				resp, err := client.MakeRequest("PUT", urlPath, updatedMenuItemData, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot update menu-items", models.RoleCashier)))
			})

		})
	})

	Describe("Delete MenuItem", func() {
		var testMenuItem *models.MenuItem

		BeforeEach(func() {
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
		})

		Context("when user has authorized role", func() {
			It("should delete menu item with admin role", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				urlPath := fmt.Sprintf("/api/v1/menu-items/%d", testMenuItem.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(204))

				// Verify deletion
				var deletedMenuItem models.MenuItem
				err = tenv.DB.WithContext(ctx).First(&deletedMenuItem, "id = ?", testMenuItem.ID).Error
				Expect(err).To(Equal(gorm.ErrRecordNotFound))
			})
		})

		Context("when user has unauthorized role", func() {
			It("should not delete menu item with non-admin role", func() {
				client := testutil.NewClient(tenv, models.RoleCashier)

				urlPath := fmt.Sprintf("/api/v1/menu-items/%d", testMenuItem.ID)
				resp, err := client.MakeRequest("DELETE", urlPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["error"]).To(Equal(fmt.Sprintf("Access denied: %s role cannot delete menu-items", models.RoleCashier)))
			})
		})
	})
})
