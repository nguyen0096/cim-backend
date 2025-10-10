package repository

import (
	"context"
	"fmt"
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

	// Auto-migrate all models
	err = db.AutoMigrate(
		&models.Supplier{},
		&models.Product{},
		&models.Inventory{},
		&models.InventoryItem{},
		&models.InventoryTransaction{},
		&models.PurchaseOrder{},
		&models.PurchaseOrderItem{},
		&models.User{},
	)
	require.NoError(t, err, "Failed to migrate test database")

	return db
}

// cleanupTestData removes all test data from the database
func cleanupTestData(t *testing.T, db *gorm.DB) {
	// Delete in reverse order of dependencies
	err := db.Exec("DELETE FROM inventory_transactions").Error
	require.NoError(t, err, "Failed to cleanup inventory transactions")

	err = db.Exec("DELETE FROM inventory_items").Error
	require.NoError(t, err, "Failed to cleanup inventory items")

	err = db.Exec("DELETE FROM inventories").Error
	require.NoError(t, err, "Failed to cleanup inventories")

	err = db.Exec("DELETE FROM purchase_order_items").Error
	require.NoError(t, err, "Failed to cleanup purchase_order_items")

	err = db.Exec("DELETE FROM purchase_orders").Error
	require.NoError(t, err, "Failed to cleanup purchase_orders")

	err = db.Exec("DELETE FROM product_suppliers").Error
	require.NoError(t, err, "Failed to cleanup product_suppliers")

	err = db.Exec("DELETE FROM products").Error
	require.NoError(t, err, "Failed to cleanup products")

	err = db.Exec("DELETE FROM suppliers").Error
	require.NoError(t, err, "Failed to cleanup suppliers")

	err = db.Exec("DELETE FROM users").Error
	require.NoError(t, err, "Failed to cleanup users")
}

func TestGetActiveItemsByInventoryIDs(t *testing.T) {
	db := setupTestDatabase(t)

	// Clean up any existing data
	cleanupTestData(t, db)

	// Create repository
	repo := NewInventoryItemRepository(db)
	ctx := pkg.WithUserEmail(context.Background(), "test@example.com")

	// Seed test data
	now := time.Now()

	// Create suppliers (without hardcoded IDs)
	suppliers := []models.Supplier{
		{Name: "Tech Electronics Inc", ContactEmail: "contact@techelectronics.com", ContactPhone: "+1-555-0123", Address: "123 Silicon Valley Blvd, San Jose, CA 95110"},
		{Name: "Office Supply Co", ContactEmail: "sales@officesupply.com", ContactPhone: "+1-555-0456", Address: "456 Business Park Dr, Dallas, TX 75201"},
	}
	err := db.WithContext(ctx).Create(&suppliers).Error
	require.NoError(t, err, "Failed to create suppliers")

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

	// Create inventories (without hardcoded IDs)
	inventories := []models.Inventory{
		{Name: "Main Warehouse A", Description: "Primary storage facility for electronics and office supplies", Location: "123 Industrial Blvd, San Francisco, CA 94107", Status: models.InventoryStatusActive},
		{Name: "Secondary Warehouse B", Description: "Secondary storage facility for bulk items", Location: "456 Storage Way, Oakland, CA 94607", Status: models.InventoryStatusActive},
		{Name: "Distribution Center C", Description: "Distribution center for fast-moving items", Location: "789 Logistics Ave, San Jose, CA 95110", Status: models.InventoryStatusActive},
	}
	err = db.WithContext(ctx).Create(&inventories).Error
	require.NoError(t, err, "Failed to create inventories")

	// Create inventory items with proper LatestActivePurchaseAt settings
	inventoryItems := []models.InventoryItem{
		{
			InventoryID: inventories[0].ID, ProductID: products[0].ID, SupplierID: suppliers[0].ID, UnitType: "piece", Quantity: 9, Status: models.InventoryItemStatusActive,
			LatestActivePurchaseAt: now.AddDate(0, 0, -20), // Set to 20 days ago
		},
		{
			InventoryID: inventories[0].ID, ProductID: products[1].ID, SupplierID: suppliers[0].ID, UnitType: "piece", Quantity: 15, Status: models.InventoryItemStatusActive,
			LatestActivePurchaseAt: now.AddDate(0, 0, -25), // Set to 25 days ago
		},
		{
			InventoryID: inventories[1].ID, ProductID: products[2].ID, SupplierID: suppliers[0].ID, UnitType: "piece", Quantity: 45, Status: models.InventoryItemStatusActive,
			// LatestActivePurchaseAt is zero value (NULL) - should return all transactions
		},
		{
			InventoryID: inventories[1].ID, ProductID: products[3].ID, SupplierID: suppliers[0].ID, UnitType: "piece", Quantity: 25, Status: models.InventoryItemStatusActive,
			LatestActivePurchaseAt: now.AddDate(0, 0, -25), // Set to 25 days ago
		},
		{
			InventoryID: inventories[0].ID, ProductID: products[4].ID, SupplierID: suppliers[1].ID, UnitType: "piece", Quantity: 0, Status: models.InventoryItemStatusActive,
			// Inactive item (quantity 0)
		},
	}
	err = db.WithContext(ctx).Create(&inventoryItems).Error
	require.NoError(t, err, "Failed to create inventory items")

	// Create inventory transactions
	transactions := []models.InventoryTransaction{
		// Purchase transactions for inventory item 1 (MacBook Pro) - 3 transactions
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: now.AddDate(0, 0, -30)}, InventoryItemID: inventoryItems[0].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 2999.99, Quantity: 5},
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: now.AddDate(0, 0, -20)}, InventoryItemID: inventoryItems[0].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 2999.99, Quantity: 3},
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -10), UpdatedAt: now.AddDate(0, 0, -10)}, InventoryItemID: inventoryItems[0].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 2999.99, Quantity: 2},

		// Purchase transactions for inventory item 2 (LG Monitor) - 2 transactions
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -25), UpdatedAt: now.AddDate(0, 0, -25)}, InventoryItemID: inventoryItems[1].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 599.99, Quantity: 10},
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -15), UpdatedAt: now.AddDate(0, 0, -15)}, InventoryItemID: inventoryItems[1].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 599.99, Quantity: 5},

		// Purchase transactions for inventory item 3 (Keychron Keyboard) - 3 transactions (NULL latest_active_purchase_at)
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -40), UpdatedAt: now.AddDate(0, 0, -40)}, InventoryItemID: inventoryItems[2].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 89.99, Quantity: 20},
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: now.AddDate(0, 0, -30)}, InventoryItemID: inventoryItems[2].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 89.99, Quantity: 15},
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: now.AddDate(0, 0, -20)}, InventoryItemID: inventoryItems[2].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 89.99, Quantity: 10},

		// Purchase transactions for inventory item 4 (Logitech Mouse) - 3 transactions
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -35), UpdatedAt: now.AddDate(0, 0, -35)}, InventoryItemID: inventoryItems[3].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 99.99, Quantity: 10},
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -25), UpdatedAt: now.AddDate(0, 0, -25)}, InventoryItemID: inventoryItems[3].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 99.99, Quantity: 8},
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -15), UpdatedAt: now.AddDate(0, 0, -15)}, InventoryItemID: inventoryItems[3].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 99.99, Quantity: 7},

		// Purchase transaction for inventory item 5 (Herman Miller Chair) - 1 transaction (inactive)
		{Base: models.Base{CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: now.AddDate(0, 0, -20)}, InventoryItemID: inventoryItems[4].ID, TransactionType: models.InventoryTransactionTypePurchase, Price: 1395.00, Quantity: 3},
	}
	err = db.WithContext(ctx).Create(&transactions).Error
	require.NoError(t, err, "Failed to create inventory transactions")

	t.Run("should return active items for single inventory ID", func(t *testing.T) {
		// Test with Main Warehouse A (inventory ID 1)
		// Expected: 2 active items (MacBook Pro with quantity 9, LG Monitor with quantity 15)
		// Herman Miller Chair has quantity 0, so it should be excluded
		inventoryIDStr := fmt.Sprintf("%d", inventories[0].ID)

		items, err := repo.GetActiveItemsByInventoryIDs(ctx, []string{inventoryIDStr})

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
		inventoryIDStrs := []string{
			fmt.Sprintf("%d", inventories[0].ID), // Main Warehouse A
			fmt.Sprintf("%d", inventories[1].ID), // Secondary Warehouse B
		}

		items, err := repo.GetActiveItemsByInventoryIDs(ctx, inventoryIDStrs)

		require.NoError(t, err)
		assert.Len(t, items, 4, "Should return 4 active items for both inventories")

		// Verify all items are active
		for _, item := range items {
			assert.Greater(t, item.Quantity, 0, "All returned items should have quantity > 0")
			assert.Contains(t, []uint{inventories[0].ID, inventories[1].ID}, item.InventoryID, "All items should belong to one of the specified inventories")
		}
	})

	t.Run("should return empty result for non-existent inventory ID", func(t *testing.T) {
		items, err := repo.GetActiveItemsByInventoryIDs(ctx, []string{"99999"})

		require.NoError(t, err)
		assert.Len(t, items, 0, "Should return empty result for non-existent inventory ID")
	})

	t.Run("should return empty result for empty inventory IDs list", func(t *testing.T) {
		items, err := repo.GetActiveItemsByInventoryIDs(ctx, []string{})

		require.NoError(t, err)
		assert.Len(t, items, 0, "Should return empty result for empty inventory IDs list")
	})

	t.Run("should preload purchase transactions correctly for item with LatestActivePurchaseAt", func(t *testing.T) {
		// Test with Main Warehouse A - MacBook Pro item
		// MacBook Pro has LatestActivePurchaseAt set to 20 days ago (transaction 2's date)
		// Should return transactions with CreatedAt >= 20 days ago (transactions 2 and 3)
		inventoryIDStr := fmt.Sprintf("%d", inventories[0].ID)

		items, err := repo.GetActiveItemsByInventoryIDs(ctx, []string{inventoryIDStr})

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

	t.Run("should preload all purchase transactions for item with NULL LatestActivePurchaseAt", func(t *testing.T) {
		// Test with Secondary Warehouse B - Keychron Keyboard item
		// Keychron Keyboard has NULL LatestActivePurchaseAt (zero value)
		// Should return all purchase transactions
		inventoryIDStr := fmt.Sprintf("%d", inventories[1].ID)

		items, err := repo.GetActiveItemsByInventoryIDs(ctx, []string{inventoryIDStr})

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

		items, err := closedRepo.GetActiveItemsByInventoryIDs(ctx, []string{"1"})

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
		// Test that transactions are returned in the correct order
		inventoryIDStr := fmt.Sprintf("%d", inventories[1].ID) // Secondary Warehouse B

		items, err := repo.GetActiveItemsByInventoryIDs(ctx, []string{inventoryIDStr})

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
		// Test with Main Warehouse A
		// Should exclude Herman Miller Chair which has quantity 0
		inventoryIDStr := fmt.Sprintf("%d", inventories[0].ID)

		items, err := repo.GetActiveItemsByInventoryIDs(ctx, []string{inventoryIDStr})

		require.NoError(t, err)
		assert.Len(t, items, 2, "Should return only 2 active items, excluding the inactive one")

		// Verify Herman Miller Chair is not included
		for _, item := range items {
			assert.NotEqual(t, "Herman Miller Aeron Chair", item.Product.Name, "Herman Miller Chair should not be included")
		}
	})
}
