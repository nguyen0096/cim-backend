package main

import (
	"cim-backend/internal/config"
	"cim-backend/internal/database"
	"cim-backend/internal/models"
	"fmt"
)

// runMigrations runs database migrations without starting the web server
func runMigrations() error {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Run migrations
	if err := database.Migrate(db); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// rollbackMigrations drops all database tables (complete rollback)
func rollbackMigrations() error {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Drop tables in reverse order to respect foreign key constraints
	tables := []interface{}{
		&models.InventorySubmission{},
		&models.PurchaseOrderItem{},
		&models.PurchaseOrder{},
		&models.Inventory{},
		&models.InventoryItem{},
		&models.InventoryTransaction{},
		&models.Product{},
		&models.Supplier{},
		&models.Settings{},
		&models.User{},
		&models.PaymentReceiptForm{},
	}

	for _, table := range tables {
		if err := db.Migrator().DropTable(table); err != nil {
			return fmt.Errorf("failed to drop table: %w", err)
		}
	}

	return nil
}
