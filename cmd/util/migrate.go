package main

import (
	"cim-backend/database"
	"cim-backend/internal/config"
	"fmt"
)

// runMigrations runs SQL migration scripts
func runMigrations() error {
	// Initialize database
	db, err := database.Initialize(config.App.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Run migrations up
	if err := database.MigrateUp(db, config.App.Migration.Directory); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// rollbackMigrations rolls back SQL migration scripts (down one step)
func rollbackMigrations() error {
	// Initialize database
	db, err := database.Initialize(config.App.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Run migrations down
	if err := database.MigrateDown(db, config.App.Migration.Directory); err != nil {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}

	return nil
}
