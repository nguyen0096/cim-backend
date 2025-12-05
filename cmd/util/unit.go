package main

import (
	"cim-backend/database"
	"cim-backend/internal/config"
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/pkg"
	"context"
	"fmt"
	"os"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

var unitCmd = &cobra.Command{
	Use:   "unit",
	Short: "Unit management commands",
	Long:  "Manage units and their conversions",
}

var unitMigrateConversionsCmd = &cobra.Command{
	Use:   "conversion",
	Short: "Migrate unit conversions",
	Long:  "Migrate unit conversions from the old unit system to the new unit system",
	Run: func(cmd *cobra.Command, args []string) {
		// 1. Initialization
		fmt.Println("🔄 Migrating unit conversions from parent-child relationships...")

		// Initialize database
		db, err := database.Initialize(config.App.Database)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to initialize database: %v\n", err)
			os.Exit(1)
		}

		// Create context with system user
		ctx := pkg.WithUserEmail(context.Background(), "system@cim.local")

		// Initialize repository
		unitConversionRepo := repository.NewUnitRepository(db)

		// 2. Query units with parent relationships
		var units []models.Unit
		if err := db.WithContext(ctx).Where("base_unit_id IS NOT NULL").Find(&units).Error; err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to query units: %v\n", err)
			os.Exit(1)
		}

		// Check if there are any units to migrate
		if len(units) == 0 {
			fmt.Println("ℹ️  No units with parent relationships found. Nothing to migrate.")
			return
		}

		fmt.Printf("Found %d units with parent relationships to migrate\n", len(units))

		// 3. Process each unit
		successCount := 0
		errorCount := 0
		skippedCount := 0

		for _, unit := range units {
			// Convert float64 to decimal.Decimal
			conversionFactor := decimal.NewFromFloat(unit.ConversionFactor)

			// Create UnitConversion record
			unitConversion := &models.UnitConversion{
				FromUnitID:       unit.ID,
				ToUnitID:         *unit.BaseUnitID, // Dereference pointer
				ConversionFactor: conversionFactor,
			}

			// Create the conversion
			err := unitConversionRepo.CreateConversion(ctx, unitConversion)
			if err != nil {
				// Check if it's a duplicate error
				if pkg.IsErrorCode(err, pkg.ErrorCodeUnitConversionAlreadyExists) {
					skippedCount++
					continue
				}
				// Log error but continue processing
				fmt.Fprintf(os.Stderr, "Warning: Failed to create conversion for unit %d: %v\n", unit.ID, err)
				errorCount++
				continue
			}

			successCount++
		}

		// 4. Report results
		fmt.Println("\n✓ Migration completed!")
		fmt.Printf("  - Successfully created: %d conversions\n", successCount)
		if skippedCount > 0 {
			fmt.Printf("  - Skipped (already exist): %d conversions\n", skippedCount)
		}
		if errorCount > 0 {
			fmt.Printf("  - Failed: %d conversions\n", errorCount)
			fmt.Fprintf(os.Stderr, "\n⚠️  Some conversions failed. Review errors above and consider re-running.\n")
			os.Exit(1)
		}
	},
}
