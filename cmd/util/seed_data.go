package main

import (
	"context"
	"fmt"
	"import-export-backend/internal/config"
	"import-export-backend/internal/database"
	"import-export-backend/internal/models"
	"import-export-backend/pkg"
	"import-export-backend/test/data"
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

	// 3. Inventories
	var inventoryIDs []uint
	for _, inventory := range seedData.Inventories {
		if err := tx.Create(&inventory).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create inventory: %w", err)
		}
		inventoryIDs = append(inventoryIDs, inventory.ID)
	}

	// 4. Inventory Items
	for _, item := range seedData.InventoryItems {
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

// generateSeedData creates the mock data using the centralized test data
func generateSeedData() SeedData {
	suppliers := data.Suppliers()
	// Generate supplier IDs (they will be auto-generated, but we need them for products)
	supplierIDs := make([]uint, len(suppliers))
	for i := range suppliers {
		supplierIDs[i] = uint(i + 1) // Assuming suppliers will get IDs 1, 2, 3...
	}

	products := data.Products(supplierIDs)
	// Generate product IDs (they will be auto-generated, but we need them for inventory)
	productIDs := make([]uint, len(products))
	for i := range products {
		productIDs[i] = uint(i + 1) // Assuming products will get IDs 1, 2, 3...
	}

	return SeedData{
		Suppliers:      suppliers,
		Products:       products,
		Inventories:    data.Inventory(productIDs),
		InventoryItems: data.InventoryItems(productIDs),
	}
}
