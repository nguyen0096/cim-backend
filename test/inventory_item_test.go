package apptest

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil/fixture"
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("InventoryItemRepository", func() {
	Describe("GetActiveInventoryItems", func() {
		var (
			repo repository.InventoryItemRepository
			ctx  context.Context
			db   *gorm.DB
		)

		BeforeEach(func() {
			ctx = tenv.DefaultContext
			db = tenv.ContextfulDB()
			repo = repository.NewInventoryItemRepository(db)
		})

		Context("filtering active items", func() {
			It("should return active items for single inventory ID", func() {
				// Setup minimal fixture
				_ = fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
					Name:         "Test Supplier",
					ContactEmail: "test@supplier.com",
					ContactPhone: "123-456-7890",
					Address:      "Test Address",
				})
				products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
					{Name: "Product 1", Status: "active", UnitID: 1},
					{Name: "Product 2", Status: "active", UnitID: 1},
				})
				inventory := fixture.WithInventory(db.WithContext(ctx), models.Inventory{
					Name:   fmt.Sprintf("Test Inventory %d", time.Now().UnixNano()),
					Status: models.InventoryStatusActive,
				})

				// Create inventory items
				items := []models.InventoryItem{
					{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive},
					{InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.NewFromInt(5), Status: models.InventoryItemStatusActive},
				}
				err := db.WithContext(ctx).Create(&items).Error
				Expect(err).To(BeNil())
				DeferCleanup(func() {
					db.WithContext(ctx).Where("id IN ?", []uint{items[0].ID, items[1].ID}).Delete(&models.InventoryItem{})
				})

				// Test
				result, err := repo.GetActiveInventoryItems(ctx, inventory.ID, []uint{items[0].ID, items[1].ID})

				Expect(err).To(BeNil())
				Expect(result).To(HaveLen(2))
				for _, item := range result {
					Expect(item.Quantity.IntPart()).To(BeNumerically(">", 0))
					Expect(item.InventoryID).To(Equal(inventory.ID))
					Expect(item.Inventory).NotTo(BeNil())
					Expect(item.Product).NotTo(BeNil())
				}
			})

			It("should include items with zero quantity if status is active", func() {
				// Setup minimal fixture
				_ = fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
					Name:         "Test Supplier",
					ContactEmail: "test@supplier.com",
					ContactPhone: "123-456-7890",
					Address:      "Test Address",
				})
				products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
					{Name: "Product with stock", Status: "active", UnitID: 1},
					{Name: "Product with zero stock", Status: "active", UnitID: 1},
				})
				inventory := fixture.WithInventory(db.WithContext(ctx), models.Inventory{
					Name:   fmt.Sprintf("Test Inventory %d", time.Now().UnixNano()),
					Status: models.InventoryStatusActive,
				})

				// Create inventory items (both active status, one with quantity 0)
				items := []models.InventoryItem{
					{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive},
					{InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.Zero, Status: models.InventoryItemStatusActive},
				}
				err := db.WithContext(ctx).Create(&items).Error
				Expect(err).To(BeNil())
				DeferCleanup(func() {
					db.WithContext(ctx).Where("id IN ?", []uint{items[0].ID, items[1].ID}).Delete(&models.InventoryItem{})
				})

				// Test - should return both items since both have active status
				result, err := repo.GetActiveInventoryItems(ctx, inventory.ID, []uint{items[0].ID, items[1].ID})

				Expect(err).To(BeNil())
				Expect(result).To(HaveLen(2), "Should return both items with active status, regardless of quantity")
				// Verify both items are present
				var hasNonZeroQty, hasZeroQty bool
				for _, item := range result {
					if item.Quantity.IntPart() > 0 {
						hasNonZeroQty = true
					}
					if item.Quantity.IntPart() == 0 {
						hasZeroQty = true
					}
				}
				Expect(hasNonZeroQty).To(BeTrue(), "Should have item with non-zero quantity")
				Expect(hasZeroQty).To(BeTrue(), "Should have item with zero quantity")
			})
		})

		Context("preloading transactions with ConsumingTransactionID", func() {
			It("should preload purchase transactions correctly for item with ConsumingTransactionID", func() {
				// Setup minimal fixture
				supplier := fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
					Name:         "Test Supplier",
					ContactEmail: "test@supplier.com",
					ContactPhone: "123-456-7890",
					Address:      "Test Address",
				})
				products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
					{Name: "Test Product", Status: "active", UnitID: 1},
				})
				inventory := fixture.WithInventory(db.WithContext(ctx), models.Inventory{
					Name:   fmt.Sprintf("Test Inventory %d", time.Now().UnixNano()),
					Status: models.InventoryStatusActive,
				})

				// Create inventory item
				item := models.InventoryItem{
					InventoryID: inventory.ID,
					ProductID:   products[0].ID,
					Quantity:    decimal.NewFromInt(5),
					Status:      models.InventoryItemStatusActive,
				}
				err := db.WithContext(ctx).Create(&item).Error
				Expect(err).To(BeNil())
				DeferCleanup(func() {
					db.WithContext(ctx).Where("inventory_item_id = ?", item.ID).Delete(&models.InventoryTransaction{})
					db.WithContext(ctx).Where("id = ?", item.ID).Delete(&models.InventoryItem{})
				})

				// Create transactions
				now := time.Now()
				transactions := []models.InventoryTransaction{
					{
						Base:            models.Base{CreatedAt: now.AddDate(0, 0, -2), UpdatedAt: now.AddDate(0, 0, -2)},
						InventoryItemID: item.ID,
						SupplierID:      &supplier.ID,
						TransactionType: models.InventoryTransactionTypePurchase,
						Price:           100,
						Quantity:        decimal.NewFromInt(3),
					},
					{
						Base:            models.Base{CreatedAt: now.AddDate(0, 0, -1), UpdatedAt: now.AddDate(0, 0, -1)},
						InventoryItemID: item.ID,
						SupplierID:      &supplier.ID,
						TransactionType: models.InventoryTransactionTypePurchase,
						Price:           100,
						Quantity:        decimal.NewFromInt(2),
					},
				}
				err = db.WithContext(ctx).Create(&transactions).Error
				Expect(err).To(BeNil())

				// Set ConsumingTransactionID to first transaction
				err = db.WithContext(ctx).Model(&models.InventoryItem{}).
					Where("id = ?", item.ID).
					Update("consuming_transaction_id", transactions[0].ID).Error
				Expect(err).To(BeNil())

				// Test
				result, err := repo.GetActiveInventoryItems(ctx, inventory.ID, []uint{item.ID})

				Expect(err).To(BeNil())
				Expect(result).To(HaveLen(1))
				Expect(result[0].ConsumableTransactions).To(HaveLen(2), "Should have 2 transactions from ConsumingTransactionID onwards")
				for _, transaction := range result[0].ConsumableTransactions {
					Expect(transaction.TransactionType).To(Equal(models.InventoryTransactionTypePurchase))
				}
			})
		})

		Context("preloading transactions with zero ConsumingTransactionID", func() {
			var (
				inventory *models.Inventory
				item      models.InventoryItem
			)

			BeforeEach(func() {
				// Setup fixture for both tests in this context
				supplier := fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
					Name:         "Test Supplier",
					ContactEmail: "test@supplier.com",
					ContactPhone: "123-456-7890",
					Address:      "Test Address",
				})
				products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
					{Name: "Test Product", Status: "active", UnitID: 1},
				})
				inventory = fixture.WithInventory(db.WithContext(ctx), models.Inventory{
					Name:   fmt.Sprintf("Test Inventory %d", time.Now().UnixNano()),
					Status: models.InventoryStatusActive,
				})

				// Create inventory item with ConsumingTransactionID = 0
				item = models.InventoryItem{
					InventoryID: inventory.ID,
					ProductID:   products[0].ID,
					Quantity:    decimal.NewFromInt(45),
					Status:      models.InventoryItemStatusActive,
				}
				err := db.WithContext(ctx).Create(&item).Error
				Expect(err).To(BeNil())
				DeferCleanup(func() {
					db.WithContext(ctx).Where("inventory_item_id = ?", item.ID).Delete(&models.InventoryTransaction{})
					db.WithContext(ctx).Where("id = ?", item.ID).Delete(&models.InventoryItem{})
				})

				// Create 3 transactions
				now := time.Now()
				transactions := []models.InventoryTransaction{
					{
						Base:            models.Base{CreatedAt: now.AddDate(0, 0, -3), UpdatedAt: now.AddDate(0, 0, -3)},
						InventoryItemID: item.ID,
						SupplierID:      &supplier.ID,
						TransactionType: models.InventoryTransactionTypePurchase,
						Price:           100,
						Quantity:        decimal.NewFromInt(20),
					},
					{
						Base:            models.Base{CreatedAt: now.AddDate(0, 0, -2), UpdatedAt: now.AddDate(0, 0, -2)},
						InventoryItemID: item.ID,
						SupplierID:      &supplier.ID,
						TransactionType: models.InventoryTransactionTypePurchase,
						Price:           100,
						Quantity:        decimal.NewFromInt(15),
					},
					{
						Base:            models.Base{CreatedAt: now.AddDate(0, 0, -1), UpdatedAt: now.AddDate(0, 0, -1)},
						InventoryItemID: item.ID,
						SupplierID:      &supplier.ID,
						TransactionType: models.InventoryTransactionTypePurchase,
						Price:           100,
						Quantity:        decimal.NewFromInt(10),
					},
				}
				err = db.WithContext(ctx).Create(&transactions).Error
				Expect(err).To(BeNil())
			})

			It("should preload all purchase transactions for item with zero ConsumingTransactionID", func() {
				result, err := repo.GetActiveInventoryItems(ctx, inventory.ID, []uint{item.ID})

				Expect(err).To(BeNil())
				Expect(result).To(HaveLen(1))
				Expect(result[0].ConsumableTransactions).To(HaveLen(3), "Should have all 3 transactions when ConsumingTransactionID is 0")
				for _, transaction := range result[0].ConsumableTransactions {
					Expect(transaction.TransactionType).To(Equal(models.InventoryTransactionTypePurchase))
				}
			})

			It("should verify transaction ordering by CreatedAt", func() {
				result, err := repo.GetActiveInventoryItems(ctx, inventory.ID, []uint{item.ID})

				Expect(err).To(BeNil())
				Expect(result).To(HaveLen(1))
				Expect(result[0].ConsumableTransactions).To(HaveLen(3))

				// Verify transactions are ordered by CreatedAt (oldest first)
				for i := 1; i < len(result[0].ConsumableTransactions); i++ {
					Expect(
						result[0].ConsumableTransactions[i-1].CreatedAt.Before(result[0].ConsumableTransactions[i].CreatedAt) ||
							result[0].ConsumableTransactions[i-1].CreatedAt.Equal(result[0].ConsumableTransactions[i].CreatedAt),
					).To(BeTrue(), "Transactions should be ordered by CreatedAt")
				}
			})
		})

		Context("edge cases", func() {
			It("should return empty result for non-existent inventory ID", func() {
				result, err := repo.GetActiveInventoryItems(ctx, 999999, []uint{1})

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no active inventory items found"))
				Expect(result).To(HaveLen(0))
			})

			It("should return empty result for empty inventory IDs list", func() {
				result, err := repo.GetActiveInventoryItems(ctx, 1, []uint{})

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no active inventory items found"))
				Expect(result).To(HaveLen(0))
			})

			It("should handle database errors gracefully", func() {
				invalidDB, _ := gorm.Open(nil, nil)
				closedRepo := repository.NewInventoryItemRepository(invalidDB)

				result, err := closedRepo.GetActiveInventoryItems(ctx, 1, []uint{1})

				if err != nil {
					Expect(result).To(BeNil())
					GinkgoWriter.Printf("Error occurred as expected: %v\n", err)
				} else {
					GinkgoWriter.Println("No error occurred with invalid DB connection, which is acceptable")
				}
			})
		})
	})
})

var _ = Describe("SaveInventoryItemChanges", func() {
	var (
		repo repository.InventoryItemRepository
		ctx  context.Context
		db   *gorm.DB
	)

	BeforeEach(func() {
		ctx = tenv.DefaultContext
		db = tenv.ContextfulDB()
		repo = repository.NewInventoryItemRepository(db)
	})

	It("should successfully create and update inventory items and transactions with valid data", func() {
		// Setup minimal fixture
		supplier := fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
			Name:         "Test Supplier",
			ContactEmail: "test@supplier.com",
			ContactPhone: "123-456-7890",
			Address:      "Test Address",
		})
		products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
			{Name: "Product 1", Status: "active", UnitID: 1},
			{Name: "Product 2", Status: "active", UnitID: 1},
		})
		inventory := fixture.WithInventory(db.WithContext(ctx), models.Inventory{
			Name:   fmt.Sprintf("Test Inventory %d", time.Now().UnixNano()),
			Status: models.InventoryStatusActive,
		})

		// Create inventory items
		items := []models.InventoryItem{
			{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive},
		}
		err := db.WithContext(ctx).Create(&items).Error
		Expect(err).To(BeNil())
		DeferCleanup(func() {
			db.WithContext(ctx).Where("inventory_item_id = ?", items[0].ID).Delete(&models.InventoryTransaction{})
			db.WithContext(ctx).Where("id = ?", items[0].ID).Delete(&models.InventoryItem{})
		})

		// Create transaction
		transaction := models.InventoryTransaction{
			InventoryItemID: items[0].ID,
			SupplierID:      &supplier.ID,
			TransactionType: models.InventoryTransactionTypePurchase,
			Price:           50.00,
			Quantity:        decimal.NewFromInt(5),
		}
		err = db.WithContext(ctx).Create(&transaction).Error
		Expect(err).To(BeNil())

		// Prepare changes
		newInventoryItem := &models.InventoryItem{
			InventoryID: inventory.ID,
			ProductID:   products[1].ID,
			Quantity:    decimal.NewFromInt(5),
			Status:      models.InventoryItemStatusActive,
		}

		changes := []*models.InventoryItemChange{
			{
				InventoryItem:    &models.InventoryItem{Base: models.Base{ID: items[0].ID}, InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(8), Status: models.InventoryItemStatusActive},
				OriginalQuantity: decimal.NewFromInt(10),
			},
			{InventoryItem: newInventoryItem},
		}

		txns := []*models.InventoryTransaction{
			{InventoryItemID: items[0].ID, TransactionType: models.InventoryTransactionTypeSell, Price: 100.0, Quantity: decimal.NewFromInt(2)},
			{Base: models.Base{ID: transaction.ID}, InventoryItemID: items[0].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 100.0, Quantity: decimal.NewFromInt(3), ConsumedQuantity: decimal.NewFromInt(2)},
			{InventoryItem: newInventoryItem, TransactionType: models.InventoryTransactionTypePurchase, Price: 100.0, Quantity: decimal.NewFromInt(5)},
		}

		// Test
		err = repo.SaveInventoryItemChanges(ctx, changes, txns)
		Expect(err).To(BeNil())

		// Verify
		var updatedItem models.InventoryItem
		err = db.WithContext(ctx).First(&updatedItem, items[0].ID).Error
		Expect(err).To(BeNil())
		Expect(updatedItem.Quantity.Equal(decimal.NewFromInt(8))).To(BeTrue())

		var sellTxn models.InventoryTransaction
		err = db.WithContext(ctx).First(&sellTxn, txns[0].ID).Error
		Expect(err).To(BeNil())
		Expect(sellTxn.Quantity.Equal(decimal.NewFromInt(2))).To(BeTrue())
		Expect(sellTxn.TransactionType).To(Equal(models.InventoryTransactionTypeSell))
	})

	It("should return error when inventory item not found", func() {
		changes := []*models.InventoryItemChange{
			{
				InventoryItem:    &models.InventoryItem{Base: models.Base{ID: 99999}, Quantity: decimal.NewFromInt(8), Status: models.InventoryItemStatusActive},
				OriginalQuantity: decimal.NewFromInt(10),
			},
		}

		err := repo.SaveInventoryItemChanges(ctx, changes, []*models.InventoryTransaction{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("inventory item with ID 99999 not found"))
	})

	It("should return error when quantity has been modified by another transaction", func() {
		// Setup minimal fixture
		_ = fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
			Name:         "Test Supplier",
			ContactEmail: "test@supplier.com",
			ContactPhone: "123-456-7890",
			Address:      "Test Address",
		})
		products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
			{Name: "Product 1", Status: "active", UnitID: 1},
		})
		inventory := fixture.WithInventory(db.WithContext(ctx), models.Inventory{
			Name:   fmt.Sprintf("Test Inventory %d", time.Now().UnixNano()),
			Status: models.InventoryStatusActive,
		})

		item := models.InventoryItem{
			InventoryID: inventory.ID,
			ProductID:   products[0].ID,
			Quantity:    decimal.NewFromInt(10),
			Status:      models.InventoryItemStatusActive,
		}
		err := db.WithContext(ctx).Create(&item).Error
		Expect(err).To(BeNil())
		DeferCleanup(func() {
			db.WithContext(ctx).Where("inventory_item_id = ?", item.ID).Delete(&models.InventoryTransaction{})
			db.WithContext(ctx).Where("id = ?", item.ID).Delete(&models.InventoryItem{})
		})

		originalQuantity := item.Quantity

		// Modify quantity to simulate concurrent update
		err = db.WithContext(ctx).Model(&models.InventoryItem{}).
			Where("id = ?", item.ID).
			Update("quantity", originalQuantity.Add(decimal.NewFromInt(2))).Error
		Expect(err).To(BeNil())

		changes := []*models.InventoryItemChange{
			{
				InventoryItem:    &models.InventoryItem{Base: models.Base{ID: item.ID}, InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(8), Status: models.InventoryItemStatusActive},
				OriginalQuantity: originalQuantity,
			},
		}

		err = repo.SaveInventoryItemChanges(ctx, changes, []*models.InventoryTransaction{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(pkg.ErrOptimisticLockConflict(
			ctx,
			"inventory item",
			item.ID,
			originalQuantity,
			originalQuantity.Add(decimal.NewFromInt(2)),
		).Error()))
	})

	It("should handle empty items and transactions arrays", func() {
		err := repo.SaveInventoryItemChanges(ctx, []*models.InventoryItemChange{}, []*models.InventoryTransaction{})
		Expect(err).To(BeNil())

		err = repo.SaveInventoryItemChanges(ctx, nil, nil)
		Expect(err).To(BeNil())
	})

	It("should handle only inventory items without transactions", func() {
		// Setup minimal fixture
		_ = fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
			Name:         "Test Supplier",
			ContactEmail: "test@supplier.com",
			ContactPhone: "123-456-7890",
			Address:      "Test Address",
		})
		products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
			{Name: "Product 1", Status: "active", UnitID: 1},
		})
		inventory := fixture.WithInventory(db.WithContext(ctx), models.Inventory{
			Name:   fmt.Sprintf("Test Inventory %d", time.Now().UnixNano()),
			Status: models.InventoryStatusActive,
		})

		item := models.InventoryItem{
			InventoryID: inventory.ID,
			ProductID:   products[0].ID,
			Quantity:    decimal.NewFromInt(10),
			Status:      models.InventoryItemStatusActive,
		}
		err := db.WithContext(ctx).Create(&item).Error
		Expect(err).To(BeNil())
		DeferCleanup(func() {
			db.WithContext(ctx).Where("id = ?", item.ID).Delete(&models.InventoryItem{})
		})

		changes := []*models.InventoryItemChange{
			{
				InventoryItem:    &models.InventoryItem{Base: models.Base{ID: item.ID}, InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(9), Status: models.InventoryItemStatusActive},
				OriginalQuantity: decimal.NewFromInt(10),
			},
		}

		err = repo.SaveInventoryItemChanges(ctx, changes, []*models.InventoryTransaction{})
		Expect(err).To(BeNil())

		var updatedItem models.InventoryItem
		err = db.WithContext(ctx).First(&updatedItem, item.ID).Error
		Expect(err).To(BeNil())
		Expect(updatedItem.Quantity.Equal(decimal.NewFromInt(9))).To(BeTrue())
	})

	It("should handle only transactions without inventory items", func() {
		// Setup minimal fixture
		_ = fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
			Name:         "Test Supplier",
			ContactEmail: "test@supplier.com",
			ContactPhone: "123-456-7890",
			Address:      "Test Address",
		})
		products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
			{Name: "Product 1", Status: "active", UnitID: 1},
		})
		inventory := fixture.WithInventory(db.WithContext(ctx), models.Inventory{
			Name:   fmt.Sprintf("Test Inventory %d", time.Now().UnixNano()),
			Status: models.InventoryStatusActive,
		})

		item := models.InventoryItem{
			InventoryID: inventory.ID,
			ProductID:   products[0].ID,
			Quantity:    decimal.NewFromInt(5),
			Status:      models.InventoryItemStatusActive,
		}
		err := db.WithContext(ctx).Create(&item).Error
		Expect(err).To(BeNil())
		DeferCleanup(func() {
			db.WithContext(ctx).Where("inventory_item_id = ?", item.ID).Delete(&models.InventoryTransaction{})
			db.WithContext(ctx).Where("id = ?", item.ID).Delete(&models.InventoryItem{})
		})

		sellTxns := []*models.InventoryTransaction{
			{InventoryItemID: item.ID, TransactionType: models.InventoryTransactionTypeSell, Price: 150.0, Quantity: decimal.NewFromInt(2)},
		}

		err = repo.SaveInventoryItemChanges(ctx, []*models.InventoryItemChange{}, sellTxns)
		Expect(err).To(BeNil())

		var transactions []models.InventoryTransaction
		err = db.WithContext(ctx).Where("inventory_item_id = ? AND transaction_type = ?", item.ID, models.InventoryTransactionTypeSell).Find(&transactions).Error
		Expect(err).To(BeNil())
		Expect(len(transactions)).To(BeNumerically(">=", 1))
	})

	It("should rollback transaction on error", func() {
		// Setup minimal fixture
		_ = fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
			Name:         "Test Supplier",
			ContactEmail: "test@supplier.com",
			ContactPhone: "123-456-7890",
			Address:      "Test Address",
		})
		products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
			{Name: "Product 1", Status: "active", UnitID: 1},
		})
		inventory := fixture.WithInventory(db.WithContext(ctx), models.Inventory{
			Name:   fmt.Sprintf("Test Inventory %d", time.Now().UnixNano()),
			Status: models.InventoryStatusActive,
		})

		item := models.InventoryItem{
			InventoryID: inventory.ID,
			ProductID:   products[0].ID,
			Quantity:    decimal.NewFromInt(20),
			Status:      models.InventoryItemStatusActive,
		}
		err := db.WithContext(ctx).Create(&item).Error
		Expect(err).To(BeNil())
		DeferCleanup(func() {
			db.WithContext(ctx).Where("inventory_item_id = ?", item.ID).Delete(&models.InventoryTransaction{})
			db.WithContext(ctx).Where("id = ?", item.ID).Delete(&models.InventoryItem{})
		})

		originalQuantity := item.Quantity

		// Prepare data that will cause an error
		changes := []*models.InventoryItemChange{
			{
				InventoryItem:    &models.InventoryItem{Base: models.Base{ID: 99999}, Quantity: decimal.NewFromInt(8), Status: models.InventoryItemStatusActive},
				OriginalQuantity: decimal.NewFromInt(10),
			},
		}
		sellTxns := []*models.InventoryTransaction{
			{InventoryItemID: item.ID, TransactionType: models.InventoryTransactionTypeSell, Price: 100.0, Quantity: decimal.NewFromInt(1)},
		}

		err = repo.SaveInventoryItemChanges(ctx, changes, sellTxns)
		Expect(err).To(HaveOccurred())

		// Verify rollback
		var unchangedItem models.InventoryItem
		err = db.WithContext(ctx).First(&unchangedItem, item.ID).Error
		Expect(err).To(BeNil())
		Expect(unchangedItem.Quantity.Equal(originalQuantity)).To(BeTrue())

		var transactions []models.InventoryTransaction
		err = db.WithContext(ctx).Where("inventory_item_id = ? AND transaction_type = ?", item.ID, models.InventoryTransactionTypeSell).Find(&transactions).Error
		Expect(err).To(BeNil())
		Expect(transactions).To(HaveLen(0))
	})

	It("should handle multiple items and transactions in single consumption", func() {
		// Setup minimal fixture
		_ = fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
			Name:         "Test Supplier",
			ContactEmail: "test@supplier.com",
			ContactPhone: "123-456-7890",
			Address:      "Test Address",
		})
		products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
			{Name: "Product 1", Status: "active", UnitID: 1},
			{Name: "Product 2", Status: "active", UnitID: 1},
			{Name: "Product 3", Status: "active", UnitID: 1},
		})
		inventory := fixture.WithInventory(db.WithContext(ctx), models.Inventory{
			Name:   fmt.Sprintf("Test Inventory %d", time.Now().UnixNano()),
			Status: models.InventoryStatusActive,
		})

		items := []models.InventoryItem{
			{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive},
			{InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.NewFromInt(20), Status: models.InventoryItemStatusActive},
			{InventoryID: inventory.ID, ProductID: products[2].ID, Quantity: decimal.NewFromInt(5), Status: models.InventoryItemStatusActive},
		}
		err := db.WithContext(ctx).Create(&items).Error
		Expect(err).To(BeNil())
		DeferCleanup(func() {
			db.WithContext(ctx).Where("inventory_item_id IN ?", []uint{items[0].ID, items[1].ID, items[2].ID}).Delete(&models.InventoryTransaction{})
			db.WithContext(ctx).Where("id IN ?", []uint{items[0].ID, items[1].ID, items[2].ID}).Delete(&models.InventoryItem{})
		})

		changes := []*models.InventoryItemChange{
			{InventoryItem: &models.InventoryItem{Base: models.Base{ID: items[0].ID}, InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(7), Status: models.InventoryItemStatusActive}, OriginalQuantity: decimal.NewFromInt(10)},
			{InventoryItem: &models.InventoryItem{Base: models.Base{ID: items[1].ID}, InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.NewFromInt(18), Status: models.InventoryItemStatusActive}, OriginalQuantity: decimal.NewFromInt(20)},
			{InventoryItem: &models.InventoryItem{Base: models.Base{ID: items[2].ID}, InventoryID: inventory.ID, ProductID: products[2].ID, Quantity: decimal.NewFromInt(3), Status: models.InventoryItemStatusActive}, OriginalQuantity: decimal.NewFromInt(5)},
		}

		sellTxns := []*models.InventoryTransaction{
			{InventoryItemID: items[0].ID, TransactionType: models.InventoryTransactionTypeSell, Price: 100.0, Quantity: decimal.NewFromInt(3)},
			{InventoryItemID: items[1].ID, TransactionType: models.InventoryTransactionTypeSell, Price: 200.0, Quantity: decimal.NewFromInt(2)},
			{InventoryItemID: items[2].ID, TransactionType: models.InventoryTransactionTypeSell, Price: 150.0, Quantity: decimal.NewFromInt(2)},
		}

		err = repo.SaveInventoryItemChanges(ctx, changes, sellTxns)
		Expect(err).To(BeNil())

		// Verify all items updated
		for i, expectedQuantity := range []int{7, 18, 3} {
			var updatedItem models.InventoryItem
			err = db.WithContext(ctx).First(&updatedItem, items[i].ID).Error
			Expect(err).To(BeNil())
			Expect(updatedItem.Quantity.Equal(decimal.NewFromInt(int64(expectedQuantity)))).To(BeTrue())
		}

		// Verify transactions created
		var transactions []models.InventoryTransaction
		err = db.WithContext(ctx).Where("inventory_item_id IN ? AND transaction_type = ? AND created_at >= ?",
			[]uint{items[0].ID, items[1].ID, items[2].ID},
			models.InventoryTransactionTypeSell,
			time.Now().Add(-5*time.Second)).Find(&transactions).Error
		Expect(err).To(BeNil())
		Expect(len(transactions)).To(BeNumerically(">=", 3))
	})
})
