package main

import (
	"cim-backend/test/data"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "util",
	Short: "Utility CLI for cim-backend",
	Long:  "A utility CLI application for various cim-backend operations",
}

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed the database with mock data",
	Long:  "Populate the database with predefined mock data for manual testing. Only allowed in non-production environments.",
	Run: func(cmd *cobra.Command, args []string) {
		// Check if environment is production
		env := os.Getenv("ENV")
		if env == "production" {
			fmt.Fprintf(os.Stderr, "Error: Seeding is not allowed in production environment\n")
			os.Exit(1)
		}

		fmt.Println("🌱 Seeding database with mock data...")
		if err := data.SeedDatabase(); err != nil {
			fmt.Fprintf(os.Stderr, "Error seeding database: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Database seeded successfully!")
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migration commands",
	Long:  "Run database migrations to create, update, or rollback the database schema",
	Run: func(cmd *cobra.Command, args []string) {
		// Default to migrate up if no subcommand is provided
		fmt.Println("🔄 Running database migrations...")
		if err := runMigrations(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Database migrations completed successfully!")
	},
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Run full database migrations (auto + scripts)",
	Long:  "Run both GORM auto migrations and SQL migration scripts",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🔄 Running full database migrations (auto + scripts)...")
		if err := runMigrations(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Database migrations completed successfully!")
	},
}

var migrateAutoCmd = &cobra.Command{
	Use:   "auto",
	Short: "Run GORM auto migrations only",
	Long:  "Run GORM auto migrations to create/update table schemas based on models",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🔄 Running GORM auto migrations...")
		if err := runAutoMigrations(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running auto migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Auto migrations completed successfully!")
	},
}

var migrateScriptsCmd = &cobra.Command{
	Use:   "scripts",
	Short: "Run SQL migration scripts only",
	Long:  "Run SQL migration scripts from the migrations directory",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🔄 Running SQL migration scripts...")
		if err := runScriptMigrations(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running migration scripts: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Migration scripts completed successfully!")
	},
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback SQL migration scripts",
	Long:  "Rollback SQL migration scripts (down one step)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("⚠️  Rolling back migration scripts...")
		if err := rollbackMigrations(); err != nil {
			fmt.Fprintf(os.Stderr, "Error rolling back migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Migration rollback completed successfully!")
	},
}

func init() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not load .env file: %v\n", err)
	}

	// Add subcommands to migrate command
	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateAutoCmd)
	migrateCmd.AddCommand(migrateScriptsCmd)
	migrateCmd.AddCommand(migrateDownCmd)

	rootCmd.AddCommand(seedCmd)
	rootCmd.AddCommand(migrateCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
