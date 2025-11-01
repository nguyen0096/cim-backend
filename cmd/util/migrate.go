package main

import (
	"cim-backend/database"
	"cim-backend/internal/config"
	"fmt"
)

// runMigrations runs both auto migrations and SQL scripts (full migration)
func runMigrations() error {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Run migrations up (auto + scripts)
	if err := database.MigrateUp(db, cfg.Migration.Directory); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// runAutoMigrations runs GORM auto migrations only
func runAutoMigrations() error {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Run auto migrations
	if err := database.AutoMigrate(db); err != nil {
		return fmt.Errorf("failed to run auto migrations: %w", err)
	}

	return nil
}

// runScriptMigrations runs SQL migration scripts only
func runScriptMigrations() error {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Run SQL migration scripts
	if err := database.MigrateScripts(db, cfg.Migration.Directory); err != nil {
		return fmt.Errorf("failed to run migration scripts: %w", err)
	}

	return nil
}

// rollbackMigrations rolls back SQL migration scripts (down one step)
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
