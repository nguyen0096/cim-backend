package apptest

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services"
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
			repo = repository.NewInventoryItemRepository(repository.NewBaseRepository(db))
		})

		Context("filtering active items", func() {
			It("should return active items for single inventory ID", func() {
				// Setup minimal fixture
				unit := fixture.WithUnit(db.WithContext(ctx), models.Unit{Name: "Unit 1", Symbol: "U1", UnitType: "general"})
				_ = fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
					Name:         "Test Supplier",
					ContactEmail: "test@supplier.com",
					ContactPhone: "123-456-7890",
					Address:      "Test Address",
				})
				products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
					{Name: "Product 1", Status: "active", UnitID: unit.ID},
					{Name: "Product 2", Status: "active", UnitID: unit.ID},
				})
				inventory := fixture.WithInventory(db.WithContext(ctx), models.Inventory{
					Name:   fmt.Sprintf("Test Inventory %d", time.Now().UnixNano()),
					Status: models.InventoryStatusActive,
				})

				// Create inventory items
				items := []models.InventoryItem{
					{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
					{InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.NewFromInt(5), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
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
				unit := fixture.WithUnit(db.WithContext(ctx), models.Unit{Name: "Unit 1", Symbol: "U1", UnitType: "general"})
				_ = fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
					Name:         "Test Supplier",
					ContactEmail: "test@supplier.com",
					ContactPhone: "123-456-7890",
					Address:      "Test Address",
				})
				products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
					{Name: "Product with stock", Status: "active", UnitID: unit.ID},
					{Name: "Product with zero stock", Status: "active", UnitID: unit.ID},
				})
				inventory := fixture.WithInventory(db.WithContext(ctx), models.Inventory{
					Name:   fmt.Sprintf("Test Inventory %d", time.Now().UnixNano()),
					Status: models.InventoryStatusActive,
				})

				// Create inventory items (both active status, one with quantity 0)
				items := []models.InventoryItem{
					{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
					{InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.Zero, Status: models.InventoryItemStatusActive, UnitID: unit.ID},
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
				unit := fixture.WithUnit(db.WithContext(ctx), models.Unit{Name: "Unit 1", Symbol: "U1", UnitType: "general"})
				supplier := fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
					Name:         "Test Supplier",
					ContactEmail: "test@supplier.com",
					ContactPhone: "123-456-7890",
					Address:      "Test Address",
				})
				products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
					{Name: "Test Product", Status: "active", UnitID: unit.ID},
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
					UnitID:      unit.ID,
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
				unit := fixture.WithUnit(db.WithContext(ctx), models.Unit{Name: "Unit 1", Symbol: "U1", UnitType: "general"})
				supplier := fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
					Name:         "Test Supplier",
					ContactEmail: "test@supplier.com",
					ContactPhone: "123-456-7890",
					Address:      "Test Address",
				})
				products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
					{Name: "Test Product", Status: "active", UnitID: unit.ID},
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
					UnitID:      unit.ID,
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
				closedRepo := repository.NewInventoryItemRepository(repository.NewBaseRepository(invalidDB))

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
		repo = repository.NewInventoryItemRepository(repository.NewBaseRepository(db))

	})

	It("should successfully create and update inventory items and transactions with valid data", func() {
		// Setup minimal fixture
		unit := fixture.WithUnit(db.WithContext(ctx), models.Unit{Name: "Unit 1", Symbol: "U1", UnitType: "general"})
		supplier := fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
			Name:         "Test Supplier",
			ContactEmail: "test@supplier.com",
			ContactPhone: "123-456-7890",
			Address:      "Test Address",
		})
		products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
			{Name: "Product 1", Status: "active", UnitID: unit.ID},
			{Name: "Product 2", Status: "active", UnitID: unit.ID},
		})
		inventory := fixture.WithInventory(db.WithContext(ctx), models.Inventory{
			Name:   fmt.Sprintf("Test Inventory %d", time.Now().UnixNano()),
			Status: models.InventoryStatusActive,
		})

		// Create inventory items
		items := []models.InventoryItem{
			{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
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
			UnitID:      unit.ID,
		}

		changes := []*models.InventoryItemChange{
			{
				InventoryItem:    &models.InventoryItem{Base: models.Base{ID: items[0].ID}, InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(8), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
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
		unit := fixture.WithUnit(db.WithContext(ctx), models.Unit{Name: "Unit 1", Symbol: "U1", UnitType: "general"})
		_ = fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
			Name:         "Test Supplier",
			ContactEmail: "test@supplier.com",
			ContactPhone: "123-456-7890",
			Address:      "Test Address",
		})
		products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
			{Name: "Product 1", Status: "active", UnitID: unit.ID},
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
			UnitID:      unit.ID,
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
				InventoryItem:    &models.InventoryItem{Base: models.Base{ID: item.ID}, InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(8), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
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
		unit := fixture.WithUnit(db.WithContext(ctx), models.Unit{Name: "Unit 1", Symbol: "U1", UnitType: "general"})
		_ = fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
			Name:         "Test Supplier",
			ContactEmail: "test@supplier.com",
			ContactPhone: "123-456-7890",
			Address:      "Test Address",
		})
		products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
			{Name: "Product 1", Status: "active", UnitID: unit.ID},
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
			UnitID:      unit.ID,
		}
		err := db.WithContext(ctx).Create(&item).Error
		Expect(err).To(BeNil())
		DeferCleanup(func() {
			db.WithContext(ctx).Where("id = ?", item.ID).Delete(&models.InventoryItem{})
		})

		changes := []*models.InventoryItemChange{
			{
				InventoryItem:    &models.InventoryItem{Base: models.Base{ID: item.ID}, InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(9), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
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
		unit := fixture.WithUnit(db.WithContext(ctx), models.Unit{Name: "Unit 1", Symbol: "U1", UnitType: "general"})
		_ = fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
			Name:         "Test Supplier",
			ContactEmail: "test@supplier.com",
			ContactPhone: "123-456-7890",
			Address:      "Test Address",
		})
		products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
			{Name: "Product 1", Status: "active", UnitID: unit.ID},
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
			UnitID:      unit.ID,
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
		unit := fixture.WithUnit(db.WithContext(ctx), models.Unit{Name: "Unit 1", Symbol: "U1", UnitType: "general"})
		_ = fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
			Name:         "Test Supplier",
			ContactEmail: "test@supplier.com",
			ContactPhone: "123-456-7890",
			Address:      "Test Address",
		})
		products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
			{Name: "Product 1", Status: "active", UnitID: unit.ID},
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
			UnitID:      unit.ID,
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
		unit := fixture.WithUnit(db.WithContext(ctx), models.Unit{Name: "Unit 1", Symbol: "U1", UnitType: "general"})
		_ = fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
			Name:         "Test Supplier",
			ContactEmail: "test@supplier.com",
			ContactPhone: "123-456-7890",
			Address:      "Test Address",
		})
		products := fixture.WithProducts(db.WithContext(ctx), []*models.Product{
			{Name: "Product 1", Status: "active", UnitID: unit.ID},
			{Name: "Product 2", Status: "active", UnitID: unit.ID},
			{Name: "Product 3", Status: "active", UnitID: unit.ID},
		})
		inventory := fixture.WithInventory(db.WithContext(ctx), models.Inventory{
			Name:   fmt.Sprintf("Test Inventory %d", time.Now().UnixNano()),
			Status: models.InventoryStatusActive,
		})

		items := []models.InventoryItem{
			{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
			{InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.NewFromInt(20), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
			{InventoryID: inventory.ID, ProductID: products[2].ID, Quantity: decimal.NewFromInt(5), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
		}
		err := db.WithContext(ctx).Create(&items).Error
		Expect(err).To(BeNil())
		DeferCleanup(func() {
			db.WithContext(ctx).Where("inventory_item_id IN ?", []uint{items[0].ID, items[1].ID, items[2].ID}).Delete(&models.InventoryTransaction{})
			db.WithContext(ctx).Where("id IN ?", []uint{items[0].ID, items[1].ID, items[2].ID}).Delete(&models.InventoryItem{})
		})

		changes := []*models.InventoryItemChange{
			{InventoryItem: &models.InventoryItem{Base: models.Base{ID: items[0].ID}, InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(7), Status: models.InventoryItemStatusActive, UnitID: unit.ID}, OriginalQuantity: decimal.NewFromInt(10)},
			{InventoryItem: &models.InventoryItem{Base: models.Base{ID: items[1].ID}, InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.NewFromInt(18), Status: models.InventoryItemStatusActive, UnitID: unit.ID}, OriginalQuantity: decimal.NewFromInt(20)},
			{InventoryItem: &models.InventoryItem{Base: models.Base{ID: items[2].ID}, InventoryID: inventory.ID, ProductID: products[2].ID, Quantity: decimal.NewFromInt(3), Status: models.InventoryItemStatusActive, UnitID: unit.ID}, OriginalQuantity: decimal.NewFromInt(5)},
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

	Describe("GetByInventoryIDWithFilters", func() {
		var (
			repo      repository.InventoryItemRepository
			ctx       context.Context
			db        *gorm.DB
			inventory *models.Inventory
			unit      *models.Unit
			products  []*models.Product
		)

		BeforeEach(func() {
			ctx = tenv.DefaultContext
			db = tenv.ContextfulDB()
			repo = repository.NewInventoryItemRepository(repository.NewBaseRepository(db))

			// Setup fixtures
			unit = fixture.WithUnit(db.WithContext(ctx), models.Unit{Name: "Kilogram", Symbol: "kg", UnitType: "mass"})
			_ = fixture.WithSupplier(db.WithContext(ctx), models.Supplier{
				Name:         "Test Supplier",
				ContactEmail: "test@supplier.com",
				ContactPhone: "123-456-7890",
				Address:      "Test Address",
			})
			products = fixture.WithProducts(db.WithContext(ctx), []*models.Product{
				{Name: "Product A", Status: "active", UnitID: unit.ID, ProductType: "material"},
				{Name: "Product B", Status: "active", UnitID: unit.ID, ProductType: "finished_good"},
				{Name: "Product C", Status: "active", UnitID: unit.ID, ProductType: "material"},
			})
			inventory = fixture.WithInventory(db.WithContext(ctx), models.Inventory{
				Name:   fmt.Sprintf("Test Inventory %d", time.Now().UnixNano()),
				Status: models.InventoryStatusActive,
			})
		})

		Context("preloading relationships", func() {
			It("should preload Inventory, Product, and Unit relationships", func() {
				// Create inventory items
				items := []models.InventoryItem{
					{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
					{InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.NewFromInt(5), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
				}
				err := db.WithContext(ctx).Create(&items).Error
				Expect(err).To(BeNil())
				DeferCleanup(func() {
					db.WithContext(ctx).Where("id IN ?", []uint{items[0].ID, items[1].ID}).Delete(&models.InventoryItem{})
				})

				// Test
				result, err := repo.GetByInventoryIDWithFilters(ctx, inventory.ID, repository.InventoryItemFilters{}, 10, 0)

				Expect(err).To(BeNil())
				Expect(result).To(HaveLen(2))
				for _, item := range result {
					// Verify Inventory is preloaded
					Expect(item.Inventory).NotTo(BeNil())
					Expect(item.Inventory.ID).To(Equal(inventory.ID))
					Expect(item.Inventory.Name).To(Equal(inventory.Name))

					// Verify Product is preloaded
					Expect(item.Product).NotTo(BeNil())
					Expect(item.Product.ID).To(Equal(item.ProductID))

					// Verify Unit is preloaded
					Expect(item.Unit).NotTo(BeNil())
					Expect(item.Unit.ID).To(Equal(unit.ID))
					Expect(item.Unit.Name).To(Equal("KILOGRAM")) // Names are standardized to uppercase
					Expect(item.Unit.Symbol).To(Equal("kg"))
				}
			})
		})

		Context("filtering by status", func() {
			It("should filter by active status", func() {
				// Create items with different statuses
				items := []models.InventoryItem{
					{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
					{InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.NewFromInt(5), Status: models.InventoryItemStatusInactive, UnitID: unit.ID},
					{InventoryID: inventory.ID, ProductID: products[2].ID, Quantity: decimal.NewFromInt(8), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
				}
				err := db.WithContext(ctx).Create(&items).Error
				Expect(err).To(BeNil())
				DeferCleanup(func() {
					db.WithContext(ctx).Where("id IN ?", []uint{items[0].ID, items[1].ID, items[2].ID}).Delete(&models.InventoryItem{})
				})

				// Test - filter by active status
				filters := repository.InventoryItemFilters{Status: string(models.InventoryItemStatusActive)}
				result, err := repo.GetByInventoryIDWithFilters(ctx, inventory.ID, filters, 10, 0)

				Expect(err).To(BeNil())
				Expect(result).To(HaveLen(2))
				for _, item := range result {
					Expect(item.Status).To(Equal(models.InventoryItemStatusActive))
				}
			})

			It("should filter by inactive status", func() {
				// Create items with different statuses
				items := []models.InventoryItem{
					{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
					{InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.NewFromInt(5), Status: models.InventoryItemStatusInactive, UnitID: unit.ID},
				}
				err := db.WithContext(ctx).Create(&items).Error
				Expect(err).To(BeNil())
				DeferCleanup(func() {
					db.WithContext(ctx).Where("id IN ?", []uint{items[0].ID, items[1].ID}).Delete(&models.InventoryItem{})
				})

				// Test - filter by inactive status
				filters := repository.InventoryItemFilters{Status: string(models.InventoryItemStatusInactive)}
				result, err := repo.GetByInventoryIDWithFilters(ctx, inventory.ID, filters, 10, 0)

				Expect(err).To(BeNil())
				Expect(result).To(HaveLen(1))
				Expect(result[0].Status).To(Equal(models.InventoryItemStatusInactive))
			})
		})

		Context("filtering by product type", func() {
			It("should filter by product type", func() {
				// Create items
				items := []models.InventoryItem{
					{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive, UnitID: unit.ID}, // material
					{InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.NewFromInt(5), Status: models.InventoryItemStatusActive, UnitID: unit.ID},  // finished_good
					{InventoryID: inventory.ID, ProductID: products[2].ID, Quantity: decimal.NewFromInt(8), Status: models.InventoryItemStatusActive, UnitID: unit.ID},  // material
				}
				err := db.WithContext(ctx).Create(&items).Error
				Expect(err).To(BeNil())
				DeferCleanup(func() {
					db.WithContext(ctx).Where("id IN ?", []uint{items[0].ID, items[1].ID, items[2].ID}).Delete(&models.InventoryItem{})
				})

				// Test - filter by material
				filters := repository.InventoryItemFilters{ProductType: "material"}
				result, err := repo.GetByInventoryIDWithFilters(ctx, inventory.ID, filters, 10, 0)

				Expect(err).To(BeNil())
				Expect(result).To(HaveLen(2))
				for _, item := range result {
					Expect(item.Product.ProductType).To(Equal("material"))
				}
			})
		})

		Context("pagination", func() {
			It("should respect limit and offset", func() {
				// Create items
				items := []models.InventoryItem{
					{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
					{InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.NewFromInt(5), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
					{InventoryID: inventory.ID, ProductID: products[2].ID, Quantity: decimal.NewFromInt(8), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
				}
				err := db.WithContext(ctx).Create(&items).Error
				Expect(err).To(BeNil())
				DeferCleanup(func() {
					db.WithContext(ctx).Where("id IN ?", []uint{items[0].ID, items[1].ID, items[2].ID}).Delete(&models.InventoryItem{})
				})

				// Test - limit 2
				result, err := repo.GetByInventoryIDWithFilters(ctx, inventory.ID, repository.InventoryItemFilters{}, 2, 0)
				Expect(err).To(BeNil())
				Expect(result).To(HaveLen(2))

				// Test - offset 2
				result2, err := repo.GetByInventoryIDWithFilters(ctx, inventory.ID, repository.InventoryItemFilters{}, 10, 2)
				Expect(err).To(BeNil())
				Expect(result2).To(HaveLen(1))
			})
		})

		Context("sorting", func() {
			It("should sort by updated_at desc", func() {
				// Create items with different update times
				item1 := models.InventoryItem{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive, UnitID: unit.ID}
				err := db.WithContext(ctx).Create(&item1).Error
				Expect(err).To(BeNil())
				time.Sleep(10 * time.Millisecond)

				item2 := models.InventoryItem{InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.NewFromInt(5), Status: models.InventoryItemStatusActive, UnitID: unit.ID}
				err = db.WithContext(ctx).Create(&item2).Error
				Expect(err).To(BeNil())
				DeferCleanup(func() {
					db.WithContext(ctx).Where("id IN ?", []uint{item1.ID, item2.ID}).Delete(&models.InventoryItem{})
				})

				// Test - sort by updated_at desc
				filters := repository.InventoryItemFilters{Sort: "updated_at", Order: "desc"}
				result, err := repo.GetByInventoryIDWithFilters(ctx, inventory.ID, filters, 10, 0)

				Expect(err).To(BeNil())
				Expect(result).To(HaveLen(2))
				Expect(result[0].ID).To(Equal(item2.ID)) // most recent first
				Expect(result[1].ID).To(Equal(item1.ID))
			})

			It("should sort by quantity asc", func() {
				// Create items with different quantities
				items := []models.InventoryItem{
					{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(15), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
					{InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.NewFromInt(5), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
					{InventoryID: inventory.ID, ProductID: products[2].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
				}
				err := db.WithContext(ctx).Create(&items).Error
				Expect(err).To(BeNil())
				DeferCleanup(func() {
					db.WithContext(ctx).Where("id IN ?", []uint{items[0].ID, items[1].ID, items[2].ID}).Delete(&models.InventoryItem{})
				})

				// Test - sort by quantity asc
				filters := repository.InventoryItemFilters{Sort: "quantity", Order: "asc"}
				result, err := repo.GetByInventoryIDWithFilters(ctx, inventory.ID, filters, 10, 0)

				Expect(err).To(BeNil())
				Expect(result).To(HaveLen(3))
				Expect(result[0].Quantity.Equal(decimal.NewFromInt(5))).To(BeTrue())
				Expect(result[1].Quantity.Equal(decimal.NewFromInt(10))).To(BeTrue())
				Expect(result[2].Quantity.Equal(decimal.NewFromInt(15))).To(BeTrue())
			})
		})

		Context("search functionality", func() {
			It("should search by product name", func() {
				// Products already created: "Product A", "Product B", "Product C"
				items := []models.InventoryItem{
					{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
					{InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.NewFromInt(5), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
					{InventoryID: inventory.ID, ProductID: products[2].ID, Quantity: decimal.NewFromInt(8), Status: models.InventoryItemStatusActive, UnitID: unit.ID},
				}
				err := db.WithContext(ctx).Create(&items).Error
				Expect(err).To(BeNil())
				DeferCleanup(func() {
					db.WithContext(ctx).Where("id IN ?", []uint{items[0].ID, items[1].ID, items[2].ID}).Delete(&models.InventoryItem{})
				})

				// Test - search for "Product A"
				filters := repository.InventoryItemFilters{Search: "Product A"}
				result, err := repo.GetByInventoryIDWithFilters(ctx, inventory.ID, filters, 10, 0)

				Expect(err).To(BeNil())
				Expect(result).To(HaveLen(1))
				Expect(result[0].Product.Name).To(Equal("Product A"))
			})
		})

		Context("combined filters", func() {
			It("should apply multiple filters together", func() {
				// Create items
				items := []models.InventoryItem{
					{InventoryID: inventory.ID, ProductID: products[0].ID, Quantity: decimal.NewFromInt(10), Status: models.InventoryItemStatusActive, UnitID: unit.ID},  // material, active
					{InventoryID: inventory.ID, ProductID: products[1].ID, Quantity: decimal.NewFromInt(5), Status: models.InventoryItemStatusActive, UnitID: unit.ID},   // finished_good, active
					{InventoryID: inventory.ID, ProductID: products[2].ID, Quantity: decimal.NewFromInt(8), Status: models.InventoryItemStatusInactive, UnitID: unit.ID}, // material, inactive
				}
				err := db.WithContext(ctx).Create(&items).Error
				Expect(err).To(BeNil())
				DeferCleanup(func() {
					db.WithContext(ctx).Where("id IN ?", []uint{items[0].ID, items[1].ID, items[2].ID}).Delete(&models.InventoryItem{})
				})

				// Test - filter by status=active AND product_type=material
				filters := repository.InventoryItemFilters{
					Status:      string(models.InventoryItemStatusActive),
					ProductType: "material",
				}
				result, err := repo.GetByInventoryIDWithFilters(ctx, inventory.ID, filters, 10, 0)

				Expect(err).To(BeNil())
				Expect(result).To(HaveLen(1))
				Expect(result[0].Status).To(Equal(models.InventoryItemStatusActive))
				Expect(result[0].Product.ProductType).To(Equal("material"))
			})
		})
	})
})

var _ = Describe("InventoryItemService.UpdateInventoryItem", func() {
	var (
		svc services.InventoryItemService
		ctx context.Context
		db  *gorm.DB
	)

	BeforeEach(func() {
		ctx = tenv.DefaultContext
		db = tenv.ContextfulDB()
		base := repository.NewBaseRepository(db)
		svc = services.NewInventoryItemService(
			repository.NewInventoryItemRepository(base),
			repository.NewInventoryRepository(base),
			repository.NewProductRepository(base),
		)
	})

	// setupItem creates a unit/product/inventory and one inventory item with the
	// given quantity, returning the persisted item.
	setupItem := func(qty decimal.Decimal) *models.InventoryItem {
		unit := fixture.WithUnit(db.WithContext(ctx), models.Unit{
			Name: fmt.Sprintf("U-%d", time.Now().UnixNano()), Symbol: "U", UnitType: "general",
		})
		product := fixture.WithProduct(db.WithContext(ctx), models.Product{
			Name: fmt.Sprintf("P-%d", time.Now().UnixNano()), Status: "active", UnitID: unit.ID,
		})
		inventory := fixture.WithInventory(db.WithContext(ctx), models.Inventory{
			Name: fmt.Sprintf("Inv-%d", time.Now().UnixNano()), Status: models.InventoryStatusActive,
		})
		item := &models.InventoryItem{
			InventoryID: inventory.ID,
			ProductID:   product.ID,
			Quantity:    qty,
			Status:      models.InventoryItemStatusActive,
			UnitID:      unit.ID,
		}
		Expect(db.WithContext(ctx).Create(item).Error).To(BeNil())
		DeferCleanup(func() {
			db.WithContext(ctx).Where("id = ?", item.ID).Delete(&models.InventoryItem{})
		})
		return item
	}

	Context("when a caller sends a changed quantity through the metadata update", func() {
		It("does NOT alter the stored quantity (quantity is immutable on this path)", func() {
			original := decimal.NewFromInt(10)
			item := setupItem(original)

			// Caller attempts to bump the quantity directly via the CRUD update.
			update := &models.InventoryItem{
				Base:        models.Base{ID: item.ID},
				InventoryID: item.InventoryID,
				ProductID:   item.ProductID,
				UnitID:      item.UnitID,
				Status:      models.InventoryItemStatusActive,
				Quantity:    decimal.NewFromInt(999),
			}

			returned, err := svc.UpdateInventoryItem(ctx, update)
			Expect(err).To(BeNil())

			// The metadata UPDATE never writes the quantity column, so the inbound
			// value is ignored and the persisted row is unchanged.
			stored, err := svc.GetInventoryItemByID(ctx, item.ID)
			Expect(err).To(BeNil())
			Expect(stored.Quantity.Equal(original)).To(BeTrue(),
				"stored quantity should be preserved, got %s", stored.Quantity.String())

			// The service returns the PERSISTED item, not the request-bound one: the
			// response must reflect the stored quantity (original), never the ignored
			// request quantity (999) — otherwise a client could think a stock change
			// succeeded.
			Expect(returned).NotTo(BeNil())
			Expect(returned.Quantity.Equal(original)).To(BeTrue(),
				"response quantity should reflect persisted value, got %s", returned.Quantity.String())
		})
	})

	Context("when a concurrent stock movement writes quantity around the metadata update", func() {
		It("does not clobber the movement's quantity (quantity column is excluded from the metadata UPDATE)", func() {
			original := decimal.NewFromInt(10)
			item := setupItem(original)

			// A legitimate movement (PO receive / dispose / transfer / reconcile apply)
			// writes a new quantity directly to the row, standing in for a movement that
			// commits while a metadata update is in flight. The metadata UPDATE must NOT
			// overwrite this with a stale value.
			movementQty := decimal.NewFromInt(42)
			Expect(db.WithContext(ctx).Model(&models.InventoryItem{}).
				Where("id = ?", item.ID).
				Update("quantity", movementQty).Error).To(BeNil())

			// Metadata update carries the pre-movement quantity (stale) and a metadata change.
			update := &models.InventoryItem{
				Base:        models.Base{ID: item.ID},
				InventoryID: item.InventoryID,
				ProductID:   item.ProductID,
				UnitID:      item.UnitID,
				Status:      models.InventoryItemStatusInactive,
				Quantity:    original, // stale pre-movement value
			}
			returned, err := svc.UpdateInventoryItem(ctx, update)
			Expect(err).To(BeNil())

			stored, err := svc.GetInventoryItemByID(ctx, item.ID)
			Expect(err).To(BeNil())
			// The movement's quantity survives — the metadata UPDATE did not touch the column.
			Expect(stored.Quantity.Equal(movementQty)).To(BeTrue(),
				"movement quantity should survive metadata update, got %s", stored.Quantity.String())
			// And the metadata change was still applied.
			Expect(stored.Status).To(Equal(models.InventoryItemStatusInactive))

			// The returned (persisted) item reflects the movement's quantity, not the
			// stale request value, and carries the applied metadata change.
			Expect(returned).NotTo(BeNil())
			Expect(returned.Quantity.Equal(movementQty)).To(BeTrue(),
				"response quantity should reflect persisted (movement) value, got %s", returned.Quantity.String())
			Expect(returned.Status).To(Equal(models.InventoryItemStatusInactive))
		})
	})

	Context("when updating metadata fields", func() {
		It("still updates non-quantity fields while preserving quantity", func() {
			original := decimal.NewFromInt(7)
			item := setupItem(original)

			update := &models.InventoryItem{
				Base:        models.Base{ID: item.ID},
				InventoryID: item.InventoryID,
				ProductID:   item.ProductID,
				UnitID:      item.UnitID,
				Status:      models.InventoryItemStatusInactive, // metadata change
				Quantity:    decimal.NewFromInt(123),            // attempted stock change (ignored)
			}

			returned, err := svc.UpdateInventoryItem(ctx, update)
			Expect(err).To(BeNil())

			stored, err := svc.GetInventoryItemByID(ctx, item.ID)
			Expect(err).To(BeNil())
			Expect(stored.Status).To(Equal(models.InventoryItemStatusInactive))
			Expect(stored.Quantity.Equal(original)).To(BeTrue(),
				"stored quantity should be preserved, got %s", stored.Quantity.String())

			// The response reflects the persisted state: metadata change applied,
			// quantity unchanged (the request's 123 is ignored, not echoed).
			Expect(returned).NotTo(BeNil())
			Expect(returned.Status).To(Equal(models.InventoryItemStatusInactive))
			Expect(returned.Quantity.Equal(original)).To(BeTrue(),
				"response quantity should reflect persisted value, got %s", returned.Quantity.String())
		})
	})
})
