package main

import (
	"cim-backend/database"
	"cim-backend/internal/config"
	"fmt"
)

// runMigrations runs database migrations up (applies migrations)
func runMigrations() error {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Run migrations up
	if err := database.MigrateUp(db, cfg.Migration.Directory); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// rollbackMigrations rolls back database migrations (down one step)
func rollbackMigrations() error {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Run migrations down
	if err := database.MigrateDown(db, cfg.Migration.Directory); err != nil {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}

	return nil
}
