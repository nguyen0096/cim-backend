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
	Suppliers []models.Supplier
	Products  []models.Product
	Inventory []models.Inventory
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
	for _, supplier := range seedData.Suppliers {
		if err := tx.Create(&supplier).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create supplier: %w", err)
		}
	}

	// 2. Products
	for _, product := range seedData.Products {
		if err := tx.Create(&product).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create product: %w", err)
		}
	}

	// 3. Inventory
	for _, inventory := range seedData.Inventory {
		if err := tx.Create(&inventory).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create inventory: %w", err)
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
	return SeedData{
		Suppliers: data.Suppliers(),
		Products:  data.Products(),
		Inventory: data.Inventory(),
	}
}
