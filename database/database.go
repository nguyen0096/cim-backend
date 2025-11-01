package database

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migratePostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"cim-backend/internal/config"
	"cim-backend/internal/models"
)

func Initialize(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

// MigrateSQLUp runs SQL migrations from migration directory (up)
func MigrateSQLUp(db *gorm.DB, migrationDir string) error {
	return runSQLMigration(db, migrationDir, "up")
}

// AutoMigrate runs GORM auto migrations
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Supplier{},
		&models.Product{},
		&models.Inventory{},
		&models.InventoryItem{},
		&models.InventoryTransaction{},
		&models.PurchaseOrder{},
		&models.PurchaseOrderItem{},
		&models.Settings{},
		&models.PaymentReceiptForm{},
		&models.InventorySubmission{},
	)
}

// MigrateUp runs SQL migrations from migration directory (up) and then GORM auto migrations
func MigrateUp(db *gorm.DB, migrationDir string) error {
	// Run SQL migrations first
	if err := MigrateSQLUp(db, migrationDir); err != nil {
		return err
	}

	// Run GORM auto migrations
	if err := AutoMigrate(db); err != nil {
		return err
	}

	return nil
}

// MigrateDown rolls back SQL migrations from migration directory (down one step)
func MigrateDown(db *gorm.DB, migrationDir string) error {
	return runSQLMigration(db, migrationDir, "down")
}

// Migrate runs SQL migrations from migration directory and then GORM auto migrations (for backward compatibility)
func Migrate(db *gorm.DB) error {
	return MigrateUp(db, "database/migrations")
}

// runSQLMigration runs SQL migrations in the specified direction
func runSQLMigration(db *gorm.DB, migrationDir, direction string) error {
	// Get underlying SQL DB
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	// Create postgres driver instance
	driver, err := migratePostgres.WithInstance(sqlDB, &migratePostgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	// Create migrate instance with file:// prefix
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationDir),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	// Run migrations based on direction
	switch direction {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("failed to run migrations up: %w", err)
		}
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("failed to run migrations down: %w", err)
		}
	default:
		return fmt.Errorf("invalid migration direction: %s (must be 'up' or 'down')", direction)
	}

	return nil
}
