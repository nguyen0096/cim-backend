package main

import (
	"context"
	"fmt"
	"import-export-backend/internal/config"
	"import-export-backend/internal/database"
	"import-export-backend/internal/models"
	"import-export-backend/pkg"
	"import-export-backend/test/data"
	"time"
)

// SeedData contains all the mock data with references
type SeedData struct {
	Suppliers      []models.Supplier
	Products       []models.Product
	Inventories    []models.Inventory
	InventoryItems []models.InventoryItem
}

// seedDatabase populates the database with mock data
func seedDatabase() error {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Create context with user email for CreatedBy field
	ctx := pkg.WithUserEmail(context.Background(), "seeder@test.com")

	// Generate seed data
	seedData := generateSeedData()

	// Start transaction
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// Seed in correct order (respecting foreign key constraints)

	// 1. Suppliers
	var supplierIDs []uint
	for _, supplier := range seedData.Suppliers {
		if err := tx.Create(&supplier).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create supplier: %w", err)
		}
		supplierIDs = append(supplierIDs, supplier.ID)
	}

	// 2. Products
	var productIDs []uint
	for _, product := range seedData.Products {
		if err := tx.Create(&product).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create product: %w", err)
		}
		productIDs = append(productIDs, product.ID)
	}

	// 2.5. Create product-supplier relationships
	for i, productID := range productIDs {
		// Assign suppliers to products based on product index
		supplierIndex := i % len(supplierIDs) // Cycle through suppliers
		supplierID := supplierIDs[supplierIndex]

		// Create many-to-many relationship (junction table only has foreign keys)
		if err := tx.Exec("INSERT INTO product_suppliers (product_id, supplier_id) VALUES (?, ?)",
			productID, supplierID).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create product-supplier relationship: %w", err)
		}
	}

	// 3. Inventories
	var inventoryIDs []uint
	for _, inventory := range seedData.Inventories {
		if err := tx.Create(&inventory).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create inventory: %w", err)
		}
		inventoryIDs = append(inventoryIDs, inventory.ID)
	}

	// 4. Inventory Items - generate with actual IDs
	inventoryItems := generateInventoryItems(productIDs, inventoryIDs, supplierIDs)
	for _, item := range inventoryItems {
		if err := tx.Create(&item).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create inventory item: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// generateInventoryItems creates inventory items with actual database IDs
func generateInventoryItems(productIDs, inventoryIDs, supplierIDs []uint) []models.InventoryItem {
	now := time.Now()

	// Inventory configurations with pricing data
	configs := []struct {
		ProductIndex  int
		SupplierIndex int
		UnitPrice     float64
		UnitType      string
		Quantity      int
		ReorderLevel  int
		Location      string
	}{
		{0, 0, 2999.99, "piece", 15, 5, "Warehouse A"},
		{1, 0, 599.99, "piece", 30, 10, "Warehouse B"},
		{2, 0, 89.99, "piece", 45, 15, "Warehouse C"},
		{3, 0, 99.99, "piece", 25, 8, "Warehouse A"},
		{4, 1, 1395.00, "piece", 8, 3, "Warehouse B"},
		{5, 1, 799.00, "box", 12, 5, "Warehouse C"},
		{6, 2, 399.95, "piece", 20, 8, "Warehouse A"},
		{7, 2, 199.99, "piece", 35, 12, "Warehouse B"},
		{8, 0, 399.99, "piece", 18, 6, "Warehouse C"},
		{9, 0, 1099.99, "device", 22, 8, "Warehouse A"},
	}

	items := make([]models.InventoryItem, len(configs))
	for i, config := range configs {
		// Map location to inventory ID
		var inventoryID uint
		switch config.Location {
		case "Warehouse A":
			inventoryID = inventoryIDs[0]
		case "Warehouse B":
			inventoryID = inventoryIDs[1]
		case "Warehouse C":
			inventoryID = inventoryIDs[2]
		default:
			inventoryID = inventoryIDs[0]
		}

		items[i] = models.InventoryItem{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			InventoryID:   inventoryID,
			ProductID:     productIDs[config.ProductIndex],
			SupplierID:    supplierIDs[config.SupplierIndex],
			UnitPrice:     config.UnitPrice,
			UnitType:      config.UnitType,
			Quantity:      config.Quantity,
			ReorderLevel:  config.ReorderLevel,
			MaxStockLevel: config.Quantity * 3,
			Status:        models.InventoryItemStatusActive,
		}
	}

	return items
}

// generateSeedData creates the mock data using the centralized test data
func generateSeedData() SeedData {
	suppliers := data.Suppliers()
	products := data.Products(nil) // supplierIDs parameter is no longer needed

	return SeedData{
		Suppliers:      suppliers,
		Products:       products,
		Inventories:    data.Inventory(nil),      // productIDs parameter is no longer needed
		InventoryItems: data.InventoryItems(nil), // productIDs parameter is no longer needed
	}
}
