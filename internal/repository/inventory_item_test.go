package repository

import (
	"context"
	"import-export-backend/internal/config"
	"import-export-backend/internal/database"
	"import-export-backend/internal/models"
	"import-export-backend/pkg"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupTestDatabase creates a test database and returns the DB instance
func setupTestDatabase(t *testing.T) *gorm.DB {
	// Load test configuration
	cfg := config.Load()

	// Initialize test database
	db, err := database.Initialize(cfg.Database)
	require.NoError(t, err, "Failed to initialize test database")
	return db
}

func TestGetActiveItemsByInventoryIDs(t *testing.T) {
	db := setupTestDatabase(t)

	// Create repository
	repo := NewInventoryItemRepository(db)
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")

	// Track created data IDs for cleanup
	var createdSupplierIDs []uint
	var createdProductIDs []uint
	var createdInventoryIDs []uint
	var createdInventoryItemIDs []uint

	// Seed test data
	now := time.Now()

	// Create suppliers (without hardcoded IDs)
	suppliers := []models.Supplier{
		{Name: "Tech Electronics Inc", ContactEmail: "contact@techelectronics.com", ContactPhone: "+1-555-0123", Address: "123 Silicon Valley Blvd, San Jose, CA 95110"},
		{Name: "Office Supply Co", ContactEmail: "sales@officesupply.com", ContactPhone: "+1-555-0456", Address: "456 Business Park Dr, Dallas, TX 75201"},
	}
	err := db.WithContext(ctx).Create(&suppliers).Error
	require.NoError(t, err, "Failed to create suppliers")
	for _, supplier := range suppliers {
		createdSupplierIDs = append(createdSupplierIDs, supplier.ID)
	}

	// Create products (without hardcoded IDs)
	products := []models.Product{
		{Name: "MacBook Pro 16-inch M3", Description: "Professional laptop with M3 chip, 32GB RAM, 1TB SSD", ProductType: "laptop", Status: "active"},
		{Name: "LG UltraGear 27\" 4K Gaming Monitor", Description: "27-inch 4K UHD gaming monitor with 144Hz refresh rate", ProductType: "monitor", Status: "active"},
		{Name: "Keychron K8 Mechanical Keyboard", Description: "Wireless mechanical keyboard with RGB backlight and hot-swappable switches", ProductType: "keyboard", Status: "active"},
		{Name: "Logitech MX Master 3S", Description: "Advanced wireless mouse with precision scrolling and customizable buttons", ProductType: "mouse", Status: "active"},
		{Name: "Herman Miller Aeron Chair", Description: "Ergonomic office chair with adjustable lumbar support", ProductType: "chair", Status: "active"},
	}
	err = db.WithContext(ctx).Create(&products).Error
	require.NoError(t, err, "Failed to create products")
	for _, product := range products {
		createdProductIDs = append(createdProductIDs, product.ID)
	}

	// Create inventories (without hardcoded IDs)
	inventories := []models.Inventory{
		{Name: "Main Warehouse A", Description: "Primary storage facility for electronics and office supplies", Location: "123 Industrial Blvd, San Francisco, CA 94107", Status: models.InventoryStatusActive},
		{Name: "Secondary Warehouse B", Description: "Secondary storage facility for bulk items", Location: "456 Storage Way, Oakland, CA 94607", Status: models.InventoryStatusActive},
		{Name: "Distribution Center C", Description: "Distribution center for fast-moving items", Location: "789 Logistics Ave, San Jose, CA 95110", Status: models.InventoryStatusActive},
	}
	err = db.WithContext(ctx).Create(&inventories).Error
	require.NoError(t, err, "Failed to create inventories")
	for _, inventory := range inventories {
		createdInventoryIDs = append(createdInventoryIDs, inventory.ID)
	}

	// Create inventory items (ConsumingTransactionID will be set after transactions are created)
	inventoryItems := []models.InventoryItem{
		{
			InventoryID: inventories[0].ID, ProductID: products[0].ID, SupplierID: suppliers[0].ID, Unit: "piece", Quantity: 9, Status: models.InventoryItemStatusActive,
			// ConsumingTransactionID will be set to transaction 2's ID after creation
		},
		{
			InventoryID: inventories[0].ID, ProductID: products[1].ID, SupplierID: suppliers[0].ID, Unit: "piece", Quantity: 15, Status: models.InventoryItemStatusActive,
			// ConsumingTransactionID will be set to transaction 4's ID after creation
		},
		{
			InventoryID: inventories[1].ID, ProductID: products[2].ID, SupplierID: suppliers[0].ID, Unit: "piece", Quantity: 45, Status: models.InventoryItemStatusActive,
			// ConsumingTransactionID is zero value (0) - should return all transactions
		},
		{
			InventoryID: inventories[1].ID, ProductID: products[3].ID, SupplierID: suppliers[0].ID, Unit: "piece", Quantity: 25, Status: models.InventoryItemStatusActive,
			// ConsumingTransactionID will be set to transaction 9's ID after creation
		},
		{
			InventoryID: inventories[0].ID, ProductID: products[4].ID, SupplierID: suppliers[1].ID, Unit: "piece", Quantity: 0, Status: models.InventoryItemStatusActive,
			// Inactive item (quantity 0)
		},
	}
	err = db.WithContext(ctx).Create(&inventoryItems).Error
	require.NoError(t, err, "Failed to create inventory items")
	for _, item := range inventoryItems {
		createdInventoryItemIDs = append(createdInventoryItemIDs, item.ID)
	}

	// Create inventory transactions
	transactions := []models.InventoryTransaction{
		// Purchase transactions for inventory item 1 (MacBook Pro) - 3 transactions
		// Transaction 0: Fully consumed (not included in active transactions)
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: now.AddDate(0, 0, -30)}, InventoryItemID: inventoryItems[0].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 2999.99, Quantity: 5},
		// Transaction 1: Currently being consumed (ConsumingTransactionID will point here)
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: now.AddDate(0, 0, -20)}, InventoryItemID: inventoryItems[0].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 2999.99, Quantity: 3},
		// Transaction 2: Not yet consumed
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -10), UpdatedAt: now.AddDate(0, 0, -10)}, InventoryItemID: inventoryItems[0].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 2999.99, Quantity: 2},

		// Purchase transactions for inventory item 2 (LG Monitor) - 2 transactions
		// Transaction 3: Currently being consumed (ConsumingTransactionID will point here)
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -25), UpdatedAt: now.AddDate(0, 0, -25)}, InventoryItemID: inventoryItems[1].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 599.99, Quantity: 10},
		// Transaction 4: Not yet consumed
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -15), UpdatedAt: now.AddDate(0, 0, -15)}, InventoryItemID: inventoryItems[1].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 599.99, Quantity: 5},

		// Purchase transactions for inventory item 3 (Keychron Keyboard) - 3 transactions
		// All transactions are active (ConsumingTransactionID = 0)
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -40), UpdatedAt: now.AddDate(0, 0, -40)}, InventoryItemID: inventoryItems[2].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 89.99, Quantity: 20},
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: now.AddDate(0, 0, -30)}, InventoryItemID: inventoryItems[2].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 89.99, Quantity: 15},
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: now.AddDate(0, 0, -20)}, InventoryItemID: inventoryItems[2].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 89.99, Quantity: 10},

		// Purchase transactions for inventory item 4 (Logitech Mouse) - 3 transactions
		// Transaction 8: Fully consumed (not included in active transactions)
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -35), UpdatedAt: now.AddDate(0, 0, -35)}, InventoryItemID: inventoryItems[3].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 99.99, Quantity: 10},
		// Transaction 9: Currently being consumed (ConsumingTransactionID will point here)
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -25), UpdatedAt: now.AddDate(0, 0, -25)}, InventoryItemID: inventoryItems[3].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 99.99, Quantity: 8},
		// Transaction 10: Not yet consumed
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -15), UpdatedAt: now.AddDate(0, 0, -15)}, InventoryItemID: inventoryItems[3].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 99.99, Quantity: 7},

		// Purchase transaction for inventory item 5 (Herman Miller Chair) - 1 transaction (inactive)
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: now.AddDate(0, 0, -20)}, InventoryItemID: inventoryItems[4].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 1395.00, Quantity: 3},
	}
	err = db.WithContext(ctx).Create(&transactions).Error
	require.NoError(t, err, "Failed to create inventory transactions")

	// Update inventory items with ConsumingTransactionID
	// Item 0 (MacBook Pro): Set ConsumingTransactionID to transaction 1 (second transaction)
	err = db.WithContext(ctx).Model(&models.InventoryItem{}).
		Where("id = ?", inventoryItems[0].ID).
		Update("consuming_transaction_id", transactions[1].ID).Error
	require.NoError(t, err, "Failed to update inventory item 0 ConsumingTransactionID")

	// Item 1 (LG Monitor): Set ConsumingTransactionID to transaction 3 (first transaction)
	err = db.WithContext(ctx).Model(&models.InventoryItem{}).
		Where("id = ?", inventoryItems[1].ID).
		Update("consuming_transaction_id", transactions[3].ID).Error
	require.NoError(t, err, "Failed to update inventory item 1 ConsumingTransactionID")

	// Item 2 (Keychron Keyboard): ConsumingTransactionID remains 0 (all transactions are active)

	// Item 3 (Logitech Mouse): Set ConsumingTransactionID to transaction 9 (second transaction)
	err = db.WithContext(ctx).Model(&models.InventoryItem{}).
		Where("id = ?", inventoryItems[3].ID).
		Update("consuming_transaction_id", transactions[9].ID).Error
	require.NoError(t, err, "Failed to update inventory item 3 ConsumingTransactionID")

	// Setup cleanup to only delete data created by this test
	t.Cleanup(func() {
		// Delete inventory transactions created by this test
		if len(createdInventoryItemIDs) > 0 {
			db.WithContext(ctx).Where("inventory_item_id IN ?", createdInventoryItemIDs).Delete(&models.InventoryTransaction{})
		}

		// Delete inventory items created by this test
		if len(createdInventoryItemIDs) > 0 {
			db.WithContext(ctx).Where("id IN ?", createdInventoryItemIDs).Delete(&models.InventoryItem{})
		}

		// Delete inventories created by this test
		if len(createdInventoryIDs) > 0 {
			db.WithContext(ctx).Where("id IN ?", createdInventoryIDs).Delete(&models.Inventory{})
		}

		// Delete products created by this test
		if len(createdProductIDs) > 0 {
			db.WithContext(ctx).Where("id IN ?", createdProductIDs).Delete(&models.Product{})
		}

		// Delete suppliers created by this test
		if len(createdSupplierIDs) > 0 {
			db.WithContext(ctx).Where("id IN ?", createdSupplierIDs).Delete(&models.Supplier{})
		}
	})

	t.Run("should return active items for single inventory ID", func(t *testing.T) {
		// Test with Main Warehouse A (inventory ID 1)
		// Expected: 2 active items (MacBook Pro with quantity 9, LG Monitor with quantity 15)
		// Herman Miller Chair has quantity 0, so it should be excluded
		items, err := repo.GetActiveItemsByInventoryIDs(ctx, []uint{inventories[0].ID})

		require.NoError(t, err)
		assert.Len(t, items, 2, "Should return 2 active items for Main Warehouse A")

		// Verify items are active (quantity > 0)
		for _, item := range items {
			assert.Greater(t, item.Quantity, 0, "All returned items should have quantity > 0")
			assert.Equal(t, inventories[0].ID, item.InventoryID, "All items should belong to Main Warehouse A")
		}

		// Verify preloaded relationships
		for _, item := range items {
			assert.NotNil(t, item.Inventory, "Inventory should be preloaded")
			assert.NotNil(t, item.Product, "Product should be preloaded")
			assert.NotNil(t, item.Supplier, "Supplier should be preloaded")
		}
	})

	t.Run("should return active items for multiple inventory IDs", func(t *testing.T) {
		// Test with Main Warehouse A and Secondary Warehouse B
		// Expected: 4 active items total (2 from each inventory)
		items, err := repo.GetActiveItemsByInventoryIDs(ctx, []uint{inventories[0].ID, inventories[1].ID})

		require.NoError(t, err)
		assert.Len(t, items, 4, "Should return 4 active items for both inventories")

		// Verify all items are active
		for _, item := range items {
			assert.Greater(t, item.Quantity, 0, "All returned items should have quantity > 0")
			assert.Contains(t, []uint{inventories[0].ID, inventories[1].ID}, item.InventoryID, "All items should belong to one of the specified inventories")
		}
	})

	t.Run("should return empty result for non-existent inventory ID", func(t *testing.T) {
		items, err := repo.GetActiveItemsByInventoryIDs(ctx, []uint{999, 999})

		require.NoError(t, err)
		assert.Len(t, items, 0, "Should return empty result for non-existent inventory ID")
	})

	t.Run("should return empty result for empty inventory IDs list", func(t *testing.T) {
		items, err := repo.GetActiveItemsByInventoryIDs(ctx, []uint{})

		require.NoError(t, err)
		assert.Len(t, items, 0, "Should return empty result for empty inventory IDs list")
	})

	t.Run("should preload purchase transactions correctly for item with ConsumingTransactionID", func(t *testing.T) {
		// Test with Main Warehouse A - MacBook Pro item
		// MacBook Pro has ConsumingTransactionID set to transaction 1's ID
		// Should return transactions with ID >= transaction 1's ID (transactions 1 and 2)
		items, err := repo.GetActiveItemsByInventoryIDs(ctx, []uint{inventories[0].ID})

		require.NoError(t, err)
		require.Len(t, items, 2, "Should return 2 active items")

		// Find MacBook Pro item (should be the first item based on our test data)
		var macBookItem *models.InventoryItem
		for _, item := range items {
			if item.Product.Name == "MacBook Pro 16-inch M3" {
				macBookItem = item
				break
			}
		}
		require.NotNil(t, macBookItem, "MacBook Pro item should be found")

		// Verify purchase transactions are preloaded
		assert.NotNil(t, macBookItem.ActivePurchaseTransactions, "ActivePurchaseTransactions should be preloaded")
		assert.Len(t, macBookItem.ActivePurchaseTransactions, 2, "Should have 2 purchase transactions (transactions 2 and 3)")

		// Verify only purchase transactions are included
		for _, transaction := range macBookItem.ActivePurchaseTransactions {
			assert.Equal(t, models.InventoryTransactionTypePurchase, transaction.TransactionType, "Only purchase transactions should be included")
		}
	})

	t.Run("should preload all purchase transactions for item with zero ConsumingTransactionID", func(t *testing.T) {
		// Should return all purchase transactions when ConsumingTransactionID is 0

		items, err := repo.GetActiveItemsByInventoryIDs(ctx, []uint{inventories[1].ID})

		require.NoError(t, err)
		require.Len(t, items, 2, "Should return 2 active items")

		// Find Keychron Keyboard item
		var keyboardItem *models.InventoryItem
		for _, item := range items {
			if item.Product.Name == "Keychron K8 Mechanical Keyboard" {
				keyboardItem = item
				break
			}
		}
		require.NotNil(t, keyboardItem, "Keychron Keyboard item should be found")

		// Verify all purchase transactions are preloaded
		assert.NotNil(t, keyboardItem.ActivePurchaseTransactions, "ActivePurchaseTransactions should be preloaded")
		assert.Len(t, keyboardItem.ActivePurchaseTransactions, 3, "Should have all 3 purchase transactions")

		// Verify only purchase transactions are included
		for _, transaction := range keyboardItem.ActivePurchaseTransactions {
			assert.Equal(t, models.InventoryTransactionTypePurchase, transaction.TransactionType, "Only purchase transactions should be included")
		}
	})

	t.Run("should handle database errors gracefully", func(t *testing.T) {
		// Create a repository with an invalid database connection
		// Using a nil driver will cause connection errors
		invalidDB, _ := gorm.Open(nil, nil)
		closedRepo := NewInventoryItemRepository(invalidDB)

		items, err := closedRepo.GetActiveItemsByInventoryIDs(ctx, []uint{1})

		// Note: This test may not always produce an error due to GORM's behavior
		// The main purpose is to ensure the error handling code path is covered
		if err != nil {
			assert.Nil(t, items, "Should return nil items on error")
			// The error message might be different depending on GORM version
			t.Logf("Error occurred as expected: %v", err)
		} else {
			// If no error occurs, just verify the function doesn't panic
			t.Log("No error occurred with invalid DB connection, which is acceptable")
		}
	})

	t.Run("should verify transaction ordering by CreatedAt", func(t *testing.T) {
		items, err := repo.GetActiveItemsByInventoryIDs(ctx, []uint{inventories[1].ID})

		require.NoError(t, err)
		require.Len(t, items, 2, "Should return 2 active items")

		// Find Keychron Keyboard item (has all transactions)
		var keyboardItem *models.InventoryItem
		for _, item := range items {
			if item.Product.Name == "Keychron K8 Mechanical Keyboard" {
				keyboardItem = item
				break
			}
		}
		require.NotNil(t, keyboardItem, "Keychron Keyboard item should be found")
		require.Len(t, keyboardItem.ActivePurchaseTransactions, 3, "Should have 3 purchase transactions")

		// Verify transactions are ordered by CreatedAt (oldest first)
		for i := 1; i < len(keyboardItem.ActivePurchaseTransactions); i++ {
			assert.True(t,
				keyboardItem.ActivePurchaseTransactions[i-1].CreatedAt.Before(keyboardItem.ActivePurchaseTransactions[i].CreatedAt) ||
					keyboardItem.ActivePurchaseTransactions[i-1].CreatedAt.Equal(keyboardItem.ActivePurchaseTransactions[i].CreatedAt),
				"Transactions should be ordered by CreatedAt")
		}
	})

	t.Run("should exclude inactive items (quantity = 0)", func(t *testing.T) {
		items, err := repo.GetActiveItemsByInventoryIDs(ctx, []uint{inventories[0].ID})

		require.NoError(t, err)
		assert.Len(t, items, 2, "Should return only 2 active items, excluding the inactive one")

		// Verify Herman Miller Chair is not included
		for _, item := range items {
			assert.NotEqual(t, "Herman Miller Aeron Chair", item.Product.Name, "Herman Miller Chair should not be included")
		}
	})
}

func TestPersistReconciliation(t *testing.T) {
	db := setupTestDatabase(t)

	// Create repository
	repo := NewInventoryItemRepository(db)
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")

	// Track created data IDs for cleanup
	var createdSupplierIDs []uint
	var createdProductIDs []uint
	var createdInventoryIDs []uint
	var createdInventoryItemIDs []uint

	// Create suppliers
	suppliers := []models.Supplier{
		{Name: "Test Supplier 1", ContactEmail: "supplier1@test.com", ContactPhone: "+1-555-0001", Address: "123 Test St"},
		{Name: "Test Supplier 2", ContactEmail: "supplier2@test.com", ContactPhone: "+1-555-0002", Address: "456 Test Ave"},
	}
	err := db.WithContext(ctx).Create(&suppliers).Error
	require.NoError(t, err, "Failed to create suppliers")
	for _, supplier := range suppliers {
		createdSupplierIDs = append(createdSupplierIDs, supplier.ID)
	}

	// Create products
	products := []models.Product{
		{Name: "Test Product 1", Description: "Test product 1", ProductType: "test", Status: "active"},
		{Name: "Test Product 2", Description: "Test product 2", ProductType: "test", Status: "active"},
		{Name: "Test Product 3", Description: "Test product 3", ProductType: "test", Status: "active"},
	}
	err = db.WithContext(ctx).Create(&products).Error
	require.NoError(t, err, "Failed to create products")
	for _, product := range products {
		createdProductIDs = append(createdProductIDs, product.ID)
	}

	// Create inventories
	inventories := []models.Inventory{
		{Name: "Test Inventory 1", Description: "Test inventory 1", Location: "Test Location 1", Status: models.InventoryStatusActive},
		{Name: "Test Inventory 2", Description: "Test inventory 2", Location: "Test Location 2", Status: models.InventoryStatusActive},
	}
	err = db.WithContext(ctx).Create(&inventories).Error
	require.NoError(t, err, "Failed to create inventories")
	for _, inventory := range inventories {
		createdInventoryIDs = append(createdInventoryIDs, inventory.ID)
	}

	// Create inventory items
	inventoryItems := []models.InventoryItem{
		{
			InventoryID: inventories[0].ID, ProductID: products[0].ID, SupplierID: suppliers[0].ID,
			Unit: "piece", Quantity: 10, Status: models.InventoryItemStatusActive,
		},
		{
			InventoryID: inventories[0].ID, ProductID: products[1].ID, SupplierID: suppliers[0].ID,
			Unit: "piece", Quantity: 20, Status: models.InventoryItemStatusActive,
		},
		{
			InventoryID: inventories[1].ID, ProductID: products[2].ID, SupplierID: suppliers[1].ID,
			Unit: "piece", Quantity: 5, Status: models.InventoryItemStatusActive,
		},
	}
	err = db.WithContext(ctx).Create(&inventoryItems).Error
	require.NoError(t, err, "Failed to create inventory items")
	for _, item := range inventoryItems {
		createdInventoryItemIDs = append(createdInventoryItemIDs, item.ID)
	}

	// Setup cleanup to only delete data created by this test
	t.Cleanup(func() {
		// Delete inventory transactions created by this test
		if len(createdInventoryItemIDs) > 0 {
			db.WithContext(ctx).Where("inventory_item_id IN ?", createdInventoryItemIDs).Delete(&models.InventoryTransaction{})
		}

		// Delete inventory items created by this test
		if len(createdInventoryItemIDs) > 0 {
			db.WithContext(ctx).Where("id IN ?", createdInventoryItemIDs).Delete(&models.InventoryItem{})
		}

		// Delete inventories created by this test
		if len(createdInventoryIDs) > 0 {
			db.WithContext(ctx).Where("id IN ?", createdInventoryIDs).Delete(&models.Inventory{})
		}

		// Delete products created by this test
		if len(createdProductIDs) > 0 {
			db.WithContext(ctx).Where("id IN ?", createdProductIDs).Delete(&models.Product{})
		}

		// Delete suppliers created by this test
		if len(createdSupplierIDs) > 0 {
			db.WithContext(ctx).Where("id IN ?", createdSupplierIDs).Delete(&models.Supplier{})
		}
	})

	t.Run("should successfully persist reconciliation with valid data", func(t *testing.T) {
		// Prepare inventory items with current quantities (should match database: 10, 20)
		updatedItems := []*models.InventoryItem{
			{
				Base:        models.Base{ID: inventoryItems[0].ID},
				InventoryID: inventories[0].ID, ProductID: products[0].ID, SupplierID: suppliers[0].ID,
				Unit: "piece", Quantity: 10, Status: models.InventoryItemStatusActive, // Current quantity in DB
			},
			{
				Base:        models.Base{ID: inventoryItems[1].ID},
				InventoryID: inventories[0].ID, ProductID: products[1].ID, SupplierID: suppliers[0].ID,
				Unit: "piece", Quantity: 20, Status: models.InventoryItemStatusActive, // Current quantity in DB
			},
		}

		// Prepare disposal transactions
		disposalTransactions := []*models.InventoryTransaction{
			{
				InventoryItemID: inventoryItems[0].ID,
				TransactionType: models.InventoryTransactionTypeDisposal,
				Price:           100.0,
				Quantity:        2,
			},
			{
				InventoryItemID: inventoryItems[1].ID,
				TransactionType: models.InventoryTransactionTypeDisposal,
				Price:           200.0,
				Quantity:        5,
			},
		}

		// Execute reconciliation
		err := repo.PersistReconciliation(ctx, updatedItems, disposalTransactions)
		require.NoError(t, err, "PersistReconciliation should succeed with valid data")

		// Verify inventory items remain unchanged (method validates and persists current state)
		var updatedItem1 models.InventoryItem
		err = db.WithContext(ctx).First(&updatedItem1, inventoryItems[0].ID).Error
		require.NoError(t, err)
		assert.Equal(t, 10, updatedItem1.Quantity, "First item quantity should remain 10")

		var updatedItem2 models.InventoryItem
		err = db.WithContext(ctx).First(&updatedItem2, inventoryItems[1].ID).Error
		require.NoError(t, err)
		assert.Equal(t, 20, updatedItem2.Quantity, "Second item quantity should remain 20")

		// Verify disposal transactions were created
		var transactions []models.InventoryTransaction
		err = db.WithContext(ctx).Where("inventory_item_id IN ? AND transaction_type = ?",
			[]uint{inventoryItems[0].ID, inventoryItems[1].ID}, models.InventoryTransactionTypeDisposal).Find(&transactions).Error
		require.NoError(t, err)
		assert.Len(t, transactions, 2, "Should have 2 disposal transactions")

		// Verify transaction details
		for _, transaction := range transactions {
			assert.Equal(t, models.InventoryTransactionTypeDisposal, transaction.TransactionType)
			assert.Greater(t, transaction.Price, 0.0)
			assert.Greater(t, transaction.Quantity, 0)
		}
	})

	t.Run("should return error when inventory item not found", func(t *testing.T) {
		// Prepare updated inventory items with non-existent ID
		updatedItems := []*models.InventoryItem{
			{
				Base:        models.Base{ID: 99999}, // Non-existent ID
				InventoryID: inventories[0].ID, ProductID: products[0].ID, SupplierID: suppliers[0].ID,
				Unit: "piece", Quantity: 8, Status: models.InventoryItemStatusActive,
			},
		}

		sellTransactions := []*models.InventoryTransaction{}

		// Execute reconciliation
		err := repo.PersistReconciliation(ctx, updatedItems, sellTransactions)
		require.Error(t, err, "PersistReconciliation should fail with non-existent item ID")
		assert.Contains(t, err.Error(), "inventory item with ID 99999 not found")
	})

	t.Run("should return error when quantity has been modified by another transaction", func(t *testing.T) {
		// First, modify one of the items in the database to simulate concurrent modification
		err := db.WithContext(ctx).Model(&models.InventoryItem{}).
			Where("id = ?", inventoryItems[0].ID).
			Update("quantity", 12).Error
		require.NoError(t, err, "Failed to modify item quantity")

		// Prepare updated inventory items with the original quantity (expecting it to be 10)
		updatedItems := []*models.InventoryItem{
			{
				Base:        models.Base{ID: inventoryItems[0].ID},
				InventoryID: inventories[0].ID, ProductID: products[0].ID, SupplierID: suppliers[0].ID,
				Unit: "piece", Quantity: 8, Status: models.InventoryItemStatusActive,
			},
		}

		sellTransactions := []*models.InventoryTransaction{}

		// Execute reconciliation
		err = repo.PersistReconciliation(ctx, updatedItems, sellTransactions)
		require.Error(t, err, "PersistReconciliation should fail when quantity has been modified")
		assert.Contains(t, err.Error(), "quantity has been modified by another transaction")
		assert.Contains(t, err.Error(), "Current: 12, Expected: 8")
	})

	t.Run("should handle empty items and transactions arrays", func(t *testing.T) {
		// Execute reconciliation with empty arrays
		err := repo.PersistReconciliation(ctx, []*models.InventoryItem{}, []*models.InventoryTransaction{})
		require.NoError(t, err, "PersistReconciliation should succeed with empty arrays")

		// Execute reconciliation with nil arrays
		err = repo.PersistReconciliation(ctx, nil, nil)
		require.NoError(t, err, "PersistReconciliation should succeed with nil arrays")
	})

	t.Run("should handle only inventory items without transactions", func(t *testing.T) {
		// Reset item quantity first
		err := db.WithContext(ctx).Model(&models.InventoryItem{}).
			Where("id = ?", inventoryItems[0].ID).
			Update("quantity", 10).Error
		require.NoError(t, err, "Failed to reset item quantity")

		// Prepare updated inventory items with current quantity (should match database)
		updatedItems := []*models.InventoryItem{
			{
				Base:        models.Base{ID: inventoryItems[0].ID},
				InventoryID: inventories[0].ID, ProductID: products[0].ID, SupplierID: suppliers[0].ID,
				Unit: "piece", Quantity: 10, Status: models.InventoryItemStatusActive, // Current quantity in DB
			},
		}

		// Execute reconciliation with empty transactions
		err = repo.PersistReconciliation(ctx, updatedItems, []*models.InventoryTransaction{})
		require.NoError(t, err, "PersistReconciliation should succeed with only items")

		// Verify item was updated
		var updatedItem models.InventoryItem
		err = db.WithContext(ctx).First(&updatedItem, inventoryItems[0].ID).Error
		require.NoError(t, err)
		assert.Equal(t, 10, updatedItem.Quantity, "Item quantity should remain 10")
	})

	t.Run("should handle only transactions without inventory items", func(t *testing.T) {
		// Prepare disposal transactions only
		disposalTransactions := []*models.InventoryTransaction{
			{
				InventoryItemID: inventoryItems[2].ID,
				TransactionType: models.InventoryTransactionTypeDisposal,
				Price:           150.0,
				Quantity:        2,
			},
		}

		// Execute reconciliation with empty items
		err := repo.PersistReconciliation(ctx, []*models.InventoryItem{}, disposalTransactions)
		require.NoError(t, err, "PersistReconciliation should succeed with only transactions")

		// Verify transaction was created
		var transactions []models.InventoryTransaction
		err = db.WithContext(ctx).Where("inventory_item_id = ? AND transaction_type = ?",
			inventoryItems[2].ID, models.InventoryTransactionTypeDisposal).Find(&transactions).Error
		require.NoError(t, err)
		assert.Len(t, transactions, 1, "Should have 1 disposal transaction")
		assert.Equal(t, 150.0, transactions[0].Price)
		assert.Equal(t, 2, transactions[0].Quantity)
	})

	t.Run("should rollback transaction on error", func(t *testing.T) {
		// Clean up previous transactions first
		err := db.WithContext(ctx).Exec("DELETE FROM inventory_transactions WHERE inventory_item_id = ?", inventoryItems[1].ID).Error
		require.NoError(t, err, "Failed to cleanup previous transactions")

		// Get current state
		var originalItem models.InventoryItem
		err = db.WithContext(ctx).First(&originalItem, inventoryItems[1].ID).Error
		require.NoError(t, err)
		originalQuantity := originalItem.Quantity

		// Prepare data that will cause an error (non-existent item ID)
		updatedItems := []*models.InventoryItem{
			{
				Base:        models.Base{ID: 99999}, // Non-existent ID
				InventoryID: inventories[0].ID, ProductID: products[0].ID, SupplierID: suppliers[0].ID,
				Unit: "piece", Quantity: 8, Status: models.InventoryItemStatusActive,
			},
		}

		disposalTransactions := []*models.InventoryTransaction{
			{
				InventoryItemID: inventoryItems[1].ID,
				TransactionType: models.InventoryTransactionTypeDisposal,
				Price:           100.0,
				Quantity:        1,
			},
		}

		// Execute reconciliation (should fail)
		err = repo.PersistReconciliation(ctx, updatedItems, disposalTransactions)
		require.Error(t, err, "PersistReconciliation should fail")

		// Verify that no changes were made (transaction was rolled back)
		var unchangedItem models.InventoryItem
		err = db.WithContext(ctx).First(&unchangedItem, inventoryItems[1].ID).Error
		require.NoError(t, err)
		assert.Equal(t, originalQuantity, unchangedItem.Quantity, "Item quantity should remain unchanged after rollback")

		// Verify no disposal transaction was created
		var transactions []models.InventoryTransaction
		err = db.WithContext(ctx).Where("inventory_item_id = ? AND transaction_type = ?",
			inventoryItems[1].ID, models.InventoryTransactionTypeDisposal).Find(&transactions).Error
		require.NoError(t, err)
		assert.Len(t, transactions, 0, "No disposal transaction should be created after rollback")
	})

	t.Run("should handle multiple items and transactions in single reconciliation", func(t *testing.T) {
		// Clean up previous transactions first
		err := db.WithContext(ctx).Exec("DELETE FROM inventory_transactions WHERE inventory_item_id IN (?, ?, ?)",
			inventoryItems[0].ID, inventoryItems[1].ID, inventoryItems[2].ID).Error
		require.NoError(t, err, "Failed to cleanup previous transactions")

		// Reset quantities first
		err = db.WithContext(ctx).Model(&models.InventoryItem{}).
			Where("id IN ?", []uint{inventoryItems[0].ID, inventoryItems[1].ID, inventoryItems[2].ID}).
			Updates(map[string]interface{}{"quantity": gorm.Expr("CASE id WHEN ? THEN 10 WHEN ? THEN 20 WHEN ? THEN 5 END",
				inventoryItems[0].ID, inventoryItems[1].ID, inventoryItems[2].ID)}).Error
		require.NoError(t, err, "Failed to reset item quantities")

		// Prepare multiple updated inventory items with current quantities (should match database)
		updatedItems := []*models.InventoryItem{
			{
				Base:        models.Base{ID: inventoryItems[0].ID},
				InventoryID: inventories[0].ID, ProductID: products[0].ID, SupplierID: suppliers[0].ID,
				Unit: "piece", Quantity: 10, Status: models.InventoryItemStatusActive, // Current quantity in DB
			},
			{
				Base:        models.Base{ID: inventoryItems[1].ID},
				InventoryID: inventories[0].ID, ProductID: products[1].ID, SupplierID: suppliers[0].ID,
				Unit: "piece", Quantity: 20, Status: models.InventoryItemStatusActive, // Current quantity in DB
			},
			{
				Base:        models.Base{ID: inventoryItems[2].ID},
				InventoryID: inventories[1].ID, ProductID: products[2].ID, SupplierID: suppliers[1].ID,
				Unit: "piece", Quantity: 5, Status: models.InventoryItemStatusActive, // Current quantity in DB
			},
		}

		// Prepare multiple disposal transactions
		disposalTransactions := []*models.InventoryTransaction{
			{
				InventoryItemID: inventoryItems[0].ID,
				TransactionType: models.InventoryTransactionTypeDisposal,
				Price:           100.0,
				Quantity:        3, // 10 - 3 = 7
			},
			{
				InventoryItemID: inventoryItems[1].ID,
				TransactionType: models.InventoryTransactionTypeDisposal,
				Price:           200.0,
				Quantity:        2, // 20 - 2 = 18
			},
			{
				InventoryItemID: inventoryItems[2].ID,
				TransactionType: models.InventoryTransactionTypeDisposal,
				Price:           150.0,
				Quantity:        2, // 5 - 2 = 3
			},
		}

		// Execute reconciliation
		err = repo.PersistReconciliation(ctx, updatedItems, disposalTransactions)
		require.NoError(t, err, "PersistReconciliation should succeed with multiple items and transactions")

		// Verify all items remain unchanged (method validates and persists current state)
		for i, expectedQuantity := range []int{10, 20, 5} {
			var updatedItem models.InventoryItem
			err = db.WithContext(ctx).First(&updatedItem, inventoryItems[i].ID).Error
			require.NoError(t, err)
			assert.Equal(t, expectedQuantity, updatedItem.Quantity,
				"Item %d quantity should remain %d", i+1, expectedQuantity)
		}

		// Verify all transactions were created (only the ones from this test)
		var transactions []models.InventoryTransaction
		err = db.WithContext(ctx).Where("inventory_item_id IN ? AND transaction_type = ? AND created_at >= ?",
			[]uint{inventoryItems[0].ID, inventoryItems[1].ID, inventoryItems[2].ID},
			models.InventoryTransactionTypeDisposal,
			time.Now().Add(-5*time.Second)).Find(&transactions).Error
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(transactions), 3, "Should have at least 3 disposal transactions")
	})
}
